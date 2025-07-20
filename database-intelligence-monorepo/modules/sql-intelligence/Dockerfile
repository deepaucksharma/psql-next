FROM otel/opentelemetry-collector-contrib:latest

# Copy configuration
COPY config/collector.yaml /etc/otel/collector.yaml

# Initialization scripts removed temporarily

# Expose metrics port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:13133/ || exit 1