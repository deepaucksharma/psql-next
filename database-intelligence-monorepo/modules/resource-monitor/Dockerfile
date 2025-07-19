FROM otel/opentelemetry-collector-contrib:0.91.0

# Install additional tools for debugging
USER root
RUN apt-get update && apt-get install -y \
    procps \
    net-tools \
    curl \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Copy configuration
COPY config/collector.yaml /etc/otel-collector/config.yaml

# Run as otel user
USER otel

EXPOSE 4317 4318 8888 13133

ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config", "/etc/otel-collector/config.yaml"]