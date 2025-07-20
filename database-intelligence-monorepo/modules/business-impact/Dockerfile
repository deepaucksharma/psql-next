FROM otel/opentelemetry-collector-contrib:0.91.0

# WARNING: DO NOT ADD HEALTHCHECK instructions here!
# Health checks have been intentionally removed from production images.
# Use shared/validation/health-check-all.sh for validation purposes.

# Copy configuration
COPY config/collector.yaml /etc/otelcol-contrib/collector.yaml

# Expose ports for OpenTelemetry functionality only
EXPOSE 8085 4317 4318 8888
# WARNING: DO NOT expose port 13133 (health check port)!

# Set the configuration file
CMD ["--config=/etc/otelcol-contrib/collector.yaml"]