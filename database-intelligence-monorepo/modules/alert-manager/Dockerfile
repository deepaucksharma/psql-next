FROM otel/opentelemetry-collector-contrib:0.91.0

# Create directories for alerts and configurations
RUN mkdir -p /var/log/alerts /etc/otelcol

# Set up permissions
RUN chown -R 10001:10001 /var/log/alerts

# Copy configuration
COPY config/collector.yaml /etc/otelcol/config.yaml

# Use the default entrypoint and command
USER 10001:10001

EXPOSE 8089 4318 9091 13134

CMD ["--config=/etc/otelcol/config.yaml"]