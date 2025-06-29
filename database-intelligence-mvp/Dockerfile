# Multi-stage Dockerfile for Database Intelligence OTEL Collector

# Stage 1: Builder
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Install OCB (OpenTelemetry Collector Builder)
RUN go install go.opentelemetry.io/collector/cmd/ocb@latest

# Build the collector
RUN ocb --config=ocb-config.yaml

# Stage 2: Runtime
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 otel && \
    adduser -u 1000 -G otel -D otel

# Copy binary from builder
COPY --from=builder /build/dist/otelcol-db-intelligence /otelcol-db-intelligence

# Copy configuration files
COPY --from=builder /build/config /etc/otelcol/config

# Set ownership
RUN chown -R otel:otel /etc/otelcol

# Switch to non-root user
USER otel

# Expose ports
# 13133 - health check extension
# 8888  - Prometheus metrics
# 4317  - OTLP gRPC receiver
# 4318  - OTLP HTTP receiver
EXPOSE 13133 8888 4317 4318

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:13133/ || exit 1

# Default configuration file
ENV OTEL_CONFIG=/etc/otelcol/config/collector.yaml

# Entry point
ENTRYPOINT ["/otelcol-db-intelligence"]
CMD ["--config", "${OTEL_CONFIG}"]