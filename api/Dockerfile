# AegisRun API Server Dockerfile
# Multi-stage build for minimal production image

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=1.0.0" \
    -o /app/bin/server \
    ./cmd/server/main.go

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/bin/server /app/server

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Create non-root user
RUN addgroup -g 1000 aegisrun && \
    adduser -u 1000 -G aegisrun -s /bin/sh -D aegisrun

USER aegisrun

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ready || exit 1

# Run the server
ENTRYPOINT ["/app/server"]
