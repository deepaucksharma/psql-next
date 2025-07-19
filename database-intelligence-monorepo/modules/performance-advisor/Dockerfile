FROM otel/opentelemetry-collector-contrib:0.91.0

# Copy configuration
COPY config/collector.yaml /etc/otelcol-contrib/config.yaml

# Expose ports
EXPOSE 8087  # OTLP HTTP receiver
EXPOSE 13133 # Health check

# Run the collector
CMD ["--config=/etc/otelcol-contrib/config.yaml"]