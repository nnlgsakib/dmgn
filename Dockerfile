# Build stage
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make protoc gcc musl-dev linux-headers

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO_ENABLED=1 for native cryptography
RUN CGO_ENABLED=1 go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" -o dmgn ./cmd/dmgn

# Final stage
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/dmgn .

# Create non-root user
RUN adduser -D -u 1000 dmgn && \
    chown -R dmgn:dmgn /app

USER dmgn

# Expose ports
# API port, libp2p port, MCP IPC port
EXPOSE 8080 4001

# Environment variables
ENV DMGN_VERSION=dev
ENV GIN_MODE=release

# Default command
ENTRYPOINT ["./dmgn"]
