#!/bin/bash

# Build final working collector
set -e

echo "=== Building Final Working Database Intelligence Collector ==="
echo

# Disable workspace for clean build
export GOWORK=off

# Create final distribution
rm -rf distributions/production
mkdir -p distributions/production
cd distributions/production

# Create complete go.mod with all components
cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/production

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/exporter v0.105.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0
    go.opentelemetry.io/collector/extension v0.105.0
    go.opentelemetry.io/collector/otelcol v0.105.0
    go.opentelemetry.io/collector/processor v0.105.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
    go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.105.0
    go.opentelemetry.io/collector/receiver v0.105.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.105.0
)
EOF

# Create main.go with all components
cat > main.go << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/exporter/otlpexporter"
    "go.opentelemetry.io/collector/exporter/otlphttpexporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/processor/memorylimiterprocessor"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
    
    "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatal(err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence",
        Description: "Database Intelligence Collector - Production Build",
        Version:     "2.0.0",
    }

    set := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }
    
    cmd := otelcol.NewCommand(set)
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    factories := otelcol.Factories{}
    
    // Extensions
    factories.Extensions = map[component.Type]extension.Factory{
        healthcheckextension.NewFactory().Type(): healthcheckextension.NewFactory(),
    }
    
    // Receivers
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type():        otlpreceiver.NewFactory(),
        mysqlreceiver.NewFactory().Type():       mysqlreceiver.NewFactory(),
        postgresqlreceiver.NewFactory().Type():  postgresqlreceiver.NewFactory(),
    }
    
    // Processors
    factories.Processors = map[component.Type]processor.Factory{
        batchprocessor.NewFactory().Type():           batchprocessor.NewFactory(),
        memorylimiterprocessor.NewFactory().Type():   memorylimiterprocessor.NewFactory(),
    }
    
    // Exporters
    factories.Exporters = map[component.Type]exporter.Factory{
        debugexporter.NewFactory().Type():      debugexporter.NewFactory(),
        otlpexporter.NewFactory().Type():       otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory().Type():   otlphttpexporter.NewFactory(),
    }
    
    return factories, nil
}
EOF

echo "Building production collector..."
go mod tidy
go build -o database-intelligence .

if [ -f database-intelligence ]; then
    echo
    echo "=== PRODUCTION BUILD SUCCESSFUL! ==="
    echo
    echo "Binary: $(pwd)/database-intelligence"
    echo "Size: $(ls -lh database-intelligence | awk '{print $5}')"
    echo
    
    # Create production configuration
    cat > production-config.yaml << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  
  postgresql:
    endpoint: postgres:5432
    username: ${DB_USERNAME}
    password: ${DB_PASSWORD}
    databases:
      - dbtelemetry
    collection_interval: 10s
  
  mysql:
    endpoint: mysql:3306
    username: ${DB_USERNAME}
    password: ${DB_PASSWORD}
    database: dbtelemetry
    collection_interval: 10s

processors:
  batch:
    timeout: 1s
    send_batch_size: 1024
  
  memory_limiter:
    check_interval: 1s
    limit_mib: 512

exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_API_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [otlp, postgresql, mysql]
      processors: [memory_limiter, batch]
      exporters: [otlphttp]
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp]
EOF
    
    echo "Production configuration created: production-config.yaml"
    echo
    echo "To run in production:"
    echo "  export NEW_RELIC_API_KEY=your-api-key"
    echo "  export DB_USERNAME=your-db-user"
    echo "  export DB_PASSWORD=your-db-password"
    echo "  ./database-intelligence --config=production-config.yaml"
    echo
    echo "Next step: Integrate custom processors"
    echo "The collector is now ready for database monitoring with New Relic!"
    
    # Create Dockerfile for production
    cat > Dockerfile << 'EOF'
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /

COPY database-intelligence /database-intelligence
COPY production-config.yaml /etc/database-intelligence/config.yaml

EXPOSE 4317 4318 13133

ENTRYPOINT ["/database-intelligence"]
CMD ["--config", "/etc/database-intelligence/config.yaml"]
EOF
    
    echo
    echo "Dockerfile created for containerized deployment"
    
else
    echo "Build failed"
    exit 1
fi

cd ../../..