FROM otel/opentelemetry-collector-contrib:0.91.0

# Copy configuration
COPY config/collector.yaml /etc/otelcol-contrib/collector.yaml

# Expose ports
EXPOSE 8085 4317 4318 8888 13133

# Set the configuration file
CMD ["--config=/etc/otelcol-contrib/collector.yaml"]