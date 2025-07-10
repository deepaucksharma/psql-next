# Database Intelligence Collector Dockerfile
# Multi-stage build for optimal size

# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# Copy go.work and all go.mod files first for better caching
COPY go.work go.sum* ./
COPY */go.mod */go.sum ./
COPY */*/go.mod */*/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the collector
RUN go build -o /database-intelligence-collector ./distributions/standard

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 otel && \
    adduser -u 1000 -G otel -D otel

WORKDIR /app

# Copy the collector binary
COPY --from=builder /database-intelligence-collector /app/collector

# Copy default configuration
COPY configs/examples/collector-simplified.yaml /app/config.yaml

# Set ownership
RUN chown -R otel:otel /app

USER otel

# Expose ports
EXPOSE 4317 4318 8888 8889 13133 55679

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/collector", "--config=/app/config.yaml", "--dry-run"] || exit 1

# Default command
ENTRYPOINT ["/app/collector"]
CMD ["--config=/app/config.yaml"]