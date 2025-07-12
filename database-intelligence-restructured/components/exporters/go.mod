module github.com/deepaksharma/db-otel/components/exporters

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/exporter v0.105.0
    go.opentelemetry.io/collector/pdata v0.105.0
    go.uber.org/zap v1.27.0
)

replace (
    github.com/deepaksharma/db-otel/components/exporters/nri => ./nri
)