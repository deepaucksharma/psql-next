module github.com/deepaksharma/db-otel/components/core

go 1.23

require (
	github.com/deepaksharma/db-otel/components/processors/adaptivesampler v0.0.0
	github.com/deepaksharma/db-otel/components/processors/circuitbreaker v0.0.0
	github.com/deepaksharma/db-otel/components/processors/costcontrol v0.0.0
	github.com/deepaksharma/db-otel/components/processors/nrerrormonitor v0.0.0
	github.com/deepaksharma/db-otel/components/processors/planattributeextractor v0.0.0
	github.com/deepaksharma/db-otel/components/processors/querycorrelator v0.0.0
	github.com/deepaksharma/db-otel/components/processors/verification v0.0.0
	github.com/deepaksharma/db-otel/components/extensions/healthcheck v0.0.0
	github.com/deepaksharma/db-otel/components/exporters/nri v0.0.0
	github.com/deepaksharma/db-otel/components/receivers/ash v0.0.0
	github.com/deepaksharma/db-otel/components/receivers/enhancedsql v0.0.0
	github.com/deepaksharma/db-otel/components/receivers/kernelmetrics v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.129.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.129.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.129.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.129.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.129.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.129.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver v0.129.0
	go.opentelemetry.io/collector/component v1.35.0
	go.opentelemetry.io/collector/otelcol v0.129.0
	go.opentelemetry.io/collector/confmap v1.35.0
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.35.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.35.0
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.35.0
	go.opentelemetry.io/collector/exporter v0.129.0
	go.opentelemetry.io/collector/exporter/debugexporter v0.129.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.129.0
	go.opentelemetry.io/collector/processor v1.35.0
	go.opentelemetry.io/collector/processor/batchprocessor v0.129.0
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.129.0
	go.opentelemetry.io/collector/receiver v1.35.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.129.0
	go.opentelemetry.io/collector/extension v1.35.0
	go.opentelemetry.io/collector/connector v0.129.0
)

replace (
	github.com/deepaksharma/db-otel/components/processors/adaptivesampler => ../processors/adaptivesampler
	github.com/deepaksharma/db-otel/components/processors/circuitbreaker => ../processors/circuitbreaker
	github.com/deepaksharma/db-otel/components/processors/costcontrol => ../processors/costcontrol
	github.com/deepaksharma/db-otel/components/processors/nrerrormonitor => ../processors/nrerrormonitor
	github.com/deepaksharma/db-otel/components/processors/planattributeextractor => ../processors/planattributeextractor
	github.com/deepaksharma/db-otel/components/processors/querycorrelator => ../processors/querycorrelator
	github.com/deepaksharma/db-otel/components/processors/verification => ../processors/verification
	github.com/deepaksharma/db-otel/components/extensions/healthcheck => ../extensions/healthcheck
	github.com/deepaksharma/db-otel/components/exporters/nri => ../exporters/nri
	github.com/deepaksharma/db-otel/components/receivers/ash => ../receivers/ash
	github.com/deepaksharma/db-otel/components/receivers/enhancedsql => ../receivers/enhancedsql
	github.com/deepaksharma/db-otel/components/receivers/kernelmetrics => ../receivers/kernelmetrics
	github.com/deepaksharma/db-otel/components/common => ../common
	github.com/deepaksharma/db-otel/components/common/featuredetector => ../common/featuredetector
	github.com/deepaksharma/db-otel/components/common/queryselector => ../common/queryselector
)
