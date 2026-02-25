# CLAWNET - Distributed AI Mesh Network
# Multi-stage build for production

# Build stage
FROM golang:1.21-alpine AS builder

# Cache buster
ARG CACHEBUST=1

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Explicitly copy cmd directory if it was ignored
RUN mkdir -p cmd/clawnet
COPY cmd/clawnet/main.go cmd/clawnet/main.go

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o clawnet \
    ./cmd/clawnet

# Runtime stage
FROM scratch

# Copy certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /build/clawnet /clawnet

# Expose ports
# 4001 - QUIC
# 4002 - TCP
EXPOSE 4001/udp 4002/tcp

# Set entrypoint
ENTRYPOINT ["/clawnet"]

# Default command
CMD ["--config", "/data/config.yaml"]
