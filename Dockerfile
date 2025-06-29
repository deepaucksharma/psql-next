# Database Intelligence Collector - Multi-stage Docker Build
# Optimized for production use with minimal attack surface

# ==============================================================================
# Stage 1: Build Environment
# ==============================================================================
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    ca-certificates \
    tzdata

# Set working directory
WORKDIR /src

# Copy go mod files for dependency caching
COPY database-intelligence-mvp/go.mod database-intelligence-mvp/go.sum ./
RUN go mod download

# Copy source code
COPY database-intelligence-mvp/ ./

# Fix module paths for build compatibility
RUN sed -i 's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' otelcol-builder.yaml && \
    sed -i 's|github.com/database-intelligence/|github.com/database-intelligence-mvp/|g' ocb-config.yaml

# Build the collector
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o collector \
    ./cmd/collector

# Verify binary
RUN ./collector --version

# ==============================================================================
# Stage 2: Runtime Environment
# ==============================================================================
FROM alpine:3.18 AS runtime

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Create non-root user
RUN addgroup -g 10001 -S otel && \
    adduser -u 10001 -S otel -G otel

# Create directories with proper permissions
RUN mkdir -p \
    /etc/otel \
    /var/lib/otel \
    /var/log/otel \
    /tmp/otel && \
    chown -R otel:otel /etc/otel /var/lib/otel /var/log/otel /tmp/otel

# Copy binary from builder
COPY --from=builder --chown=otel:otel /src/collector /usr/local/bin/collector

# Copy configuration files
COPY --chown=otel:otel database-intelligence-mvp/config/ /etc/otel/

# Set environment variables
ENV OTEL_CONFIG_FILE=/etc/otel/collector.yaml
ENV OTEL_STORAGE_PATH=/var/lib/otel
ENV OTEL_LOG_LEVEL=info

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8888/health || exit 1

# Switch to non-root user
USER otel

# Expose ports
EXPOSE 8888 8889 4317 4318

# Default command
CMD ["collector", "--config", "/etc/otel/collector.yaml"]

# ==============================================================================
# Stage 3: Development Environment (optional)
# ==============================================================================
FROM runtime AS development

# Switch back to root for dev tools installation
USER root

# Install development tools
RUN apk add --no-cache \
    bash \
    curl \
    wget \
    vim \
    jq \
    htop

# Install Go for development
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# Copy source for development
COPY --chown=otel:otel database-intelligence-mvp/ /src/

# Switch back to otel user
USER otel

# Development working directory
WORKDIR /src

# Development command
CMD ["bash"]