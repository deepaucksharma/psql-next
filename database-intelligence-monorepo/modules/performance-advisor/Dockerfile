FROM otel/opentelemetry-collector-contrib:latest

# WARNING: DO NOT ADD HEALTHCHECK instructions here!
# Health checks have been intentionally removed from production images.
# Use shared/validation/health-check-all.sh for validation purposes.

# Force cache invalidation
ARG CACHEBUST=1

# Copy configuration
COPY config/collector.yaml /etc/otel/collector.yaml

# Expose ports for OpenTelemetry functionality only
EXPOSE 8087
# WARNING: DO NOT expose port 13133 (health check port)!

