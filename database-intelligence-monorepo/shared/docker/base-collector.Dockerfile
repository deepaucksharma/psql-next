# Base Dockerfile for OpenTelemetry Collector modules
FROM otel/opentelemetry-collector-contrib:latest

# Install debugging tools
RUN apt-get update && apt-get install -y \
    curl \
    netcat \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Create directories
RUN mkdir -p /etc/otel /var/log/otel

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:13133/ || exit 1

# Default user
USER 10001