FROM otel/opentelemetry-collector-contrib:latest

# WARNING: DO NOT ADD HEALTHCHECK instructions here!
# Health checks have been intentionally removed from production images.
# Use shared/validation/health-check-all.sh for validation purposes.

# Copy configuration
COPY config/collector.yaml /etc/otel/collector.yaml

# Initialization scripts removed temporarily

# Expose ports for OpenTelemetry functionality only
EXPOSE 8083
# WARNING: DO NOT expose port 13133 (health check port)!

