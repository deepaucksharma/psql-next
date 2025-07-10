#!/bin/bash

# Create a working build by using vendor mode
set -e

echo "=== Creating Working Build ==="
echo

cd distributions/enterprise

# Create a simple configuration for testing
cat > test-config.yaml << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317
      http:
        endpoint: localhost:4318

processors:
  batch:
    timeout: 10s
  
  # Custom processors
  adaptivesampler:
    sampling_percentage: 50
  
  circuitbreaker:
    failure_threshold: 5
    timeout: 30s

exporters:
  debug:
    verbosity: detailed
  
  otlphttp/newrelic:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_API_KEY}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [adaptivesampler, circuitbreaker, batch]
      exporters: [debug, otlphttp/newrelic]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug, otlphttp/newrelic]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug, otlphttp/newrelic]
EOF

# Use vendor mode to avoid test dependencies
echo "Setting up vendor mode..."
GOWORK=off go mod vendor

# Build using vendor
echo "Building collector..."
GOWORK=off go build -mod=vendor -o database-intelligence-collector ./main.go

if [ -f database-intelligence-collector ]; then
    echo
    echo "=== Build Successful! ==="
    echo "Binary: $(pwd)/database-intelligence-collector"
    echo "Config: $(pwd)/test-config.yaml"
    echo
    echo "To run the collector:"
    echo "  export NEW_RELIC_API_KEY=your_key_here"
    echo "  ./database-intelligence-collector --config=test-config.yaml"
else
    echo "Build failed"
    exit 1
fi