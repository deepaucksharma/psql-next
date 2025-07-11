module github.com/deepaksharma/db-otel/components/distributions/enterprise

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/confmap v0.105.0
    go.opentelemetry.io/collector/exporter v0.105.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0
    go.opentelemetry.io/collector/extension v0.105.0
    go.opentelemetry.io/collector/otelcol v0.105.0
    go.opentelemetry.io/collector/processor v0.105.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
    go.opentelemetry.io/collector/receiver v0.105.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
    
    // Custom processors only - no contrib to avoid conflicts
    github.com/deepaksharma/db-otel/components/processors/adaptivesampler v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/circuitbreaker v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/costcontrol v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/nrerrormonitor v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/planattributeextractor v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/querycorrelator v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/verification v0.0.0-00010101000000-000000000000
)

replace (
    github.com/deepaksharma/db-otel/components/processors/adaptivesampler => ../../processors/adaptivesampler
    github.com/deepaksharma/db-otel/components/processors/circuitbreaker => ../../processors/circuitbreaker
    github.com/deepaksharma/db-otel/components/processors/costcontrol => ../../processors/costcontrol
    github.com/deepaksharma/db-otel/components/processors/nrerrormonitor => ../../processors/nrerrormonitor
    github.com/deepaksharma/db-otel/components/processors/planattributeextractor => ../../processors/planattributeextractor
    github.com/deepaksharma/db-otel/components/processors/querycorrelator => ../../processors/querycorrelator
    github.com/deepaksharma/db-otel/components/processors/verification => ../../processors/verification
    
    github.com/deepaksharma/db-otel/components/common/featuredetector => ../../common/featuredetector
)

replace (
    github.com/deepaksharma/db-otel/components/common => ../../common
    github.com/deepaksharma/db-otel/components/common/featuredetector => ../../common/featuredetector
    github.com/deepaksharma/db-otel/components/common/queryselector => ../../common/queryselector
    github.com/deepaksharma/db-otel/components/processors/adaptivesampler => ../../processors/adaptivesampler
    github.com/deepaksharma/db-otel/components/processors/circuitbreaker => ../../processors/circuitbreaker
    github.com/deepaksharma/db-otel/components/processors/costcontrol => ../../processors/costcontrol
    github.com/deepaksharma/db-otel/components/processors/nrerrormonitor => ../../processors/nrerrormonitor
    github.com/deepaksharma/db-otel/components/processors/planattributeextractor => ../../processors/planattributeextractor
    github.com/deepaksharma/db-otel/components/processors/querycorrelator => ../../processors/querycorrelator
    github.com/deepaksharma/db-otel/components/processors/verification => ../../processors/verification
    github.com/deepaksharma/db-otel/components/exporters/nri => ../../exporters/nri
    github.com/deepaksharma/db-otel/components/extensions/healthcheck => ../../extensions/healthcheck
)
