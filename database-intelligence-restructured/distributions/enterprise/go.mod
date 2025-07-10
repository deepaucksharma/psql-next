module github.com/database-intelligence/distributions/enterprise

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/confmap v1.12.0
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
    github.com/database-intelligence/processors/adaptivesampler v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/circuitbreaker v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/costcontrol v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/nrerrormonitor v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/planattributeextractor v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/querycorrelator v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/verification v0.0.0-00010101000000-000000000000
)

replace (
    github.com/database-intelligence/processors/adaptivesampler => ../../processors/adaptivesampler
    github.com/database-intelligence/processors/circuitbreaker => ../../processors/circuitbreaker
    github.com/database-intelligence/processors/costcontrol => ../../processors/costcontrol
    github.com/database-intelligence/processors/nrerrormonitor => ../../processors/nrerrormonitor
    github.com/database-intelligence/processors/planattributeextractor => ../../processors/planattributeextractor
    github.com/database-intelligence/processors/querycorrelator => ../../processors/querycorrelator
    github.com/database-intelligence/processors/verification => ../../processors/verification
    
    github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
)
