# Stage 1: Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build with CGO disabled for Alpine compatibility
COPY . .
RUN CGO_ENABLED=0 go build -o /app/api -ldflags="-s -w" ./cmd/api

# Stage 2: Minimal runtime
FROM alpine:3.21

RUN addgroup -S netwatch \
    && adduser -S -G netwatch netwatch \
    && apk add --no-cache libcap

COPY --from=builder /app/api /usr/local/bin/netwatch

# Grant CAP_NET_RAW capability (required for ICMP ping)
# This must be set at runtime via docker run --cap-add=NET_RAW
RUN setcap cap_net_raw=+ep /usr/local/bin/netwatch

USER netwatch
WORKDIR /home/netwatch

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/netwatch"]
