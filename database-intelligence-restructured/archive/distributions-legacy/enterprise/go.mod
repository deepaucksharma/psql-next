module github.com/database-intelligence/db-intel/components/distributions/enterprise

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/confmap v0.105.0
    go.opentelemetry.io/collector/exporter v0.105.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0
    go.opentelemetry.io/collector/extension v0.105.0
    go.opentelemetry.io/collector/extension/ballastextension v0.105.0
    go.opentelemetry.io/collector/extension/zpagesextension v0.105.0
    go.opentelemetry.io/collector/otelcol v0.105.0
    go.opentelemetry.io/collector/processor v0.105.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
    go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.105.0
    go.opentelemetry.io/collector/receiver v0.105.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
    
    // Contrib components
    github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver v0.105.0
    
    // Custom components
    github.com/database-intelligence/db-intel/components v0.0.0-00010101000000-000000000000
)

replace (
    github.com/database-intelligence/db-intel/components => ../../components
    github.com/database-intelligence/db-intel/internal/featuredetector => ../../internal/featuredetector
    github.com/database-intelligence/db-intel/internal/queryselector => ../../internal/queryselector
    github.com/database-intelligence/db-intel/internal/database => ../../internal/database
)
