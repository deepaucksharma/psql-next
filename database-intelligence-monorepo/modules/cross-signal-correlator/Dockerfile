# Cross-Signal Correlator Dockerfile
FROM otel/opentelemetry-collector-contrib:latest

# WARNING: DO NOT ADD HEALTHCHECK instructions here!
# Health checks have been intentionally removed from production images.
# Use shared/validation/health-check-all.sh for validation purposes.

# Install required packages for log parsing
USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create directories for logs and storage
RUN mkdir -p /var/log/mysql /var/lib/otel/correlation && \
    chown -R 10001:10001 /var/log/mysql /var/lib/otel/correlation

# Copy configuration
COPY config/collector.yaml /etc/otel/collector.yaml
COPY config/collector-enhanced.yaml /etc/otel/collector-enhanced.yaml

USER 10001

# Expose ports for OpenTelemetry functionality only
EXPOSE 4317 4318 8892 1777 55679
# WARNING: DO NOT expose port 13133 (health check port)!
# Port 13137 removed as it was likely a health check port

ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config", "/etc/otel/collector.yaml"]