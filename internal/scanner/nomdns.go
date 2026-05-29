//go:build !mdns

package scanner

import (
	"context"
	"time"
)

// MDNSResolve is a no-op stub when built without the mdns build tag.
// It always returns an empty result without performing any network operations.
func MDNSResolve(_ context.Context, _ []string, _ time.Duration) (map[string]string, error) {
	return map[string]string{}, nil
}
