module github.com/deepaksharma/db-otel/components

go 1.21

require (
	go.opentelemetry.io/collector/component v0.92.0
	go.opentelemetry.io/collector/confmap v0.92.0
	go.opentelemetry.io/collector/consumer v0.92.0
	go.opentelemetry.io/collector/exporter v0.92.0
	go.opentelemetry.io/collector/extension v0.92.0
	go.opentelemetry.io/collector/pdata v1.0.1
	go.opentelemetry.io/collector/processor v0.92.0
	go.opentelemetry.io/collector/receiver v0.92.0
	go.opentelemetry.io/collector/semconv v0.92.0
	go.uber.org/zap v1.26.0
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/lib/pq v1.10.9
	github.com/go-sql-driver/mysql v1.7.1
	github.com/prometheus/client_golang v1.17.0
	github.com/hashicorp/golang-lru v1.0.2
)

// All components use same OpenTelemetry version - no conflicts!