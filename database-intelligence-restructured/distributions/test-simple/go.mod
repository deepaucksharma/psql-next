module github.com/database-intelligence/distributions/test-simple

go 1.22

require (
    github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.102.1
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.102.1
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.102.1
    go.opentelemetry.io/collector/component v0.102.1
    go.opentelemetry.io/collector/confmap v0.102.1
    go.opentelemetry.io/collector/confmap/converter/expandconverter v0.102.1
    go.opentelemetry.io/collector/confmap/provider/envprovider v0.102.1
    go.opentelemetry.io/collector/confmap/provider/fileprovider v0.102.1
    go.opentelemetry.io/collector/confmap/provider/httpprovider v0.102.1
    go.opentelemetry.io/collector/confmap/provider/httpsprovider v0.102.1
    go.opentelemetry.io/collector/confmap/provider/yamlprovider v0.102.1
    go.opentelemetry.io/collector/exporter v0.102.1
    go.opentelemetry.io/collector/exporter/debugexporter v0.102.1
    go.opentelemetry.io/collector/extension v0.102.1
    go.opentelemetry.io/collector/otelcol v0.102.1
    go.opentelemetry.io/collector/processor v0.102.1
    go.opentelemetry.io/collector/processor/batchprocessor v0.102.1
    go.opentelemetry.io/collector/receiver v0.102.1
)