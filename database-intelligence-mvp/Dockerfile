# Multi-stage Dockerfile for Database Intelligence OTEL Collector

# Stage 1: Builder
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    curl \
    ca-certificates \
    && update-ca-certificates

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Copy processor modules
COPY processors/ ./processors/
COPY common/ ./common/

# Download dependencies
RUN go mod download

# Copy remaining source files
COPY Makefile ./
COPY main.go ./

# Build the collector
RUN make build

# Stage 2: Runtime
FROM alpine:3.19

# Install runtime dependencies and security updates
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && update-ca-certificates \
    && apk upgrade --no-cache \
    && rm -rf /var/cache/apk/* \
    && rm -rf /tmp/*

# Create non-root user and directories with strict permissions
RUN addgroup -g 1000 -S otel && \
    adduser -u 1000 -S otel -G otel && \
    mkdir -p /etc/otel /var/lib/otel /var/log/otel && \
    chmod 750 /etc/otel /var/lib/otel /var/log/otel && \
    chown -R otel:otel /etc/otel /var/lib/otel /var/log/otel

# Copy binary from builder
COPY --from=builder /build/bin/otelcol-custom /usr/local/bin/otelcol

# Make binary executable
RUN chmod +x /usr/local/bin/otelcol

# Copy configuration files with secure permissions
COPY --chown=otel:otel --chmod=640 config/*.yaml /etc/otel/

# Set environment variables for performance tuning and security
ENV GOGC=80 \
    GOMEMLIMIT=512MiB \
    OTEL_RESOURCE_ATTRIBUTES="service.name=database-intelligence,service.version=1.0.0" \
    OTEL_LOG_LEVEL=info \
    TMPDIR=/tmp \
    HOME=/home/otel

# Remove any potentially sensitive files and set secure defaults
RUN rm -rf /root/.* 2>/dev/null || true && \
    chmod 700 /home/otel

# Switch to non-root user (security hardening)
USER otel:otel

# Expose ports
# 4317  - OTLP gRPC receiver
# 4318  - OTLP HTTP receiver  
# 8888  - Prometheus metrics
# 13133 - Health check extension
# 55679 - ZPages (debug)
EXPOSE 4317 4318 8888 13133 55679

# Volume for persistent state
VOLUME ["/var/lib/otel"]

# Health check with timeout
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:13133/ || exit 1

# Entry point with signal handling
ENTRYPOINT ["/usr/local/bin/otelcol"]
CMD ["--config", "/etc/otel/config.yaml"]

# Metadata labels
LABEL org.opencontainers.image.title="Database Intelligence Collector" \
      org.opencontainers.image.description="OpenTelemetry-based database monitoring solution" \
      org.opencontainers.image.vendor="Database Intelligence MVP" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/database-intelligence-mvp/database-intelligence-mvp" \
      maintainer="Database Intelligence Team"