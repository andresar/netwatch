package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andresar/netwatch/internal/api"
	"github.com/andresar/netwatch/internal/config"
	"github.com/andresar/netwatch/internal/scanner"
)

func main() {
	if err := run(signalCh()); err != nil {
		log.Fatal(err)
	}
}

// signalCh creates a channel that receives SIGINT and SIGTERM.
func signalCh() chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	return ch
}

// detectPrivilegedPing tries to open a raw ICMP socket to determine
// if privileged ping (CAP_NET_RAW) is available. Falls back to
// unprivileged UDP-mode ICMP which works on modern Linux kernels.
func detectPrivilegedPing() bool {
	conn, err := net.Dial("ip4:icmp", "127.0.0.1")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// run loads config, starts the HTTP server, and blocks until a signal
// is received on sigCh. Tests can inject a custom sigCh.
func run(sigCh <-chan os.Signal) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Auto-detect ping capabilities
	privileged := detectPrivilegedPing()
	scanner.SetPrivilegedPing(privileged)
	log.Printf("Ping mode: %s", map[bool]string{true: "privileged (CAP_NET_RAW)", false: "unprivileged (UDP ICMP)"}[privileged])

	// Build scanner pipeline with adapter types
	orch := scanner.NewOrchestrator(
		&realPinger{},
		&realARPReader{},
		&realDNSResolver{},
		&realMDNSResolver{},
		&realOUILookup{},
		scanner.ScanConfig{
			Subnet:          cfg.Subnet,
			PingConcurrency: cfg.PingConcurrency,
			ScanTimeout:     cfg.ScanTimeout,
			DNSTimeout:      cfg.DNSTimeout,
		},
	)

	h := api.NewHandler(orch, cfg.Subnet)
	r := api.NewRouter(h)

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Start signal watcher for graceful shutdown
	go func() {
		<-sigCh
		log.Println("Shutting down gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()

	log.Printf("Netwatch listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: %w", err)
	}

	return nil
}

// Adapter types that wrap scanner package functions to satisfy the
// Pinger, ARPReader, DNSResolver, and OUILookup interfaces.

type realPinger struct{}

func (r *realPinger) Ping(ctx context.Context, subnet string, concurrency int) ([]string, error) {
	return scanner.PingICMP(ctx, subnet, concurrency)
}

type realARPReader struct{}

func (r *realARPReader) Read() (map[string]string, error) {
	return scanner.ReadARPFile()
}

type realDNSResolver struct{}

func (r *realDNSResolver) Resolve(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
	return scanner.ReverseLookupBatch(ctx, ips, timeout, 10)
}

type realMDNSResolver struct{}

func (r *realMDNSResolver) ResolveMDNS(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
	return scanner.ResolveMDNSBatch(ctx, ips, timeout)
}

type realOUILookup struct{}

func (r *realOUILookup) Lookup(mac string) string {
	return scanner.LookupOUI(mac)
}
