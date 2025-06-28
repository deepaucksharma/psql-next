# Database Intelligence MVP - Custom OpenTelemetry Collector
# Multi-stage build for security and efficiency

# Build stage - Custom processors
FROM golang:1.21-alpine AS builder

# Security: Create non-root user for build
RUN adduser -D -s /bin/sh -u 10001 oteluser

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

# Set working directory
WORKDIR /app

# Copy go modules first for better caching
COPY processors/planattributeextractor/go.mod processors/planattributeextractor/go.sum ./processors/planattributeextractor/
COPY processors/adaptivesampler/go.mod processors/adaptivesampler/go.sum ./processors/adaptivesampler/

# Download dependencies
WORKDIR /app/processors/planattributeextractor
RUN go mod download

WORKDIR /app/processors/adaptivesampler
RUN go mod download

# Copy source code
WORKDIR /app
COPY processors/ ./processors/

# Build custom processors
WORKDIR /app/processors/planattributeextractor
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o planattributeextractor .

WORKDIR /app/processors/adaptivesampler
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o adaptivesampler .

# Runtime stage - OpenTelemetry Collector with custom processors
FROM otel/opentelemetry-collector-contrib:0.91.0

# Metadata
LABEL maintainer="Database Intelligence Team"
LABEL version="mvp-1.0"
LABEL description="OpenTelemetry Collector with Database Intelligence processors"

# Security: Use non-root user
USER root

# Create required directories with proper permissions
RUN mkdir -p /var/lib/otel/storage && \
    mkdir -p /var/log/otel && \
    mkdir -p /tmp/otel && \
    mkdir -p /var/cache/otel && \
    chown -R 10001:10001 /var/lib/otel && \
    chown -R 10001:10001 /var/log/otel && \
    chown -R 10001:10001 /tmp/otel && \
    chown -R 10001:10001 /var/cache/otel && \
    chmod 755 /var/lib/otel /var/log/otel /tmp/otel /var/cache/otel

# Install additional security updates
RUN apk update && \
    apk upgrade && \
    apk add --no-cache ca-certificates tzdata && \
    rm -rf /var/cache/apk/*

# Copy custom processors from builder stage
COPY --from=builder --chown=10001:10001 /app/processors/planattributeextractor/planattributeextractor /usr/local/bin/
COPY --from=builder --chown=10001:10001 /app/processors/adaptivesampler/adaptivesampler /usr/local/bin/

# Switch to non-root user
USER 10001:10001

# Set working directory
WORKDIR /var/lib/otel

# Environment variables with secure defaults
ENV OTEL_RESOURCE_ATTRIBUTES="service.name=database-intelligence-collector,service.version=mvp-1.0"
ENV OTEL_LOG_LEVEL="info"
ENV OTEL_EXPORTER_OTLP_TIMEOUT="30s"

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:13133/ || exit 1

# Expose ports
EXPOSE 13133 8888 55679

# Default command
ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config=/etc/otel/config.yaml"]