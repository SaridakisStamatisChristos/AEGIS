# =============================================================================
# Build Stage
# =============================================================================
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o aegis-verify ./cmd/verify/main.go

# =============================================================================
# Runtime Stage
# =============================================================================
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 verifier && \
    adduser -D -u 1000 -G verifier verifier

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/aegis-verify .

# Set ownership
RUN chown -R verifier:verifier /app

# Switch to non-root user
USER verifier

ENTRYPOINT ["./aegis-verify"]
