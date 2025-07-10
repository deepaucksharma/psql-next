module github.com/database-intelligence/core

go 1.23

require (
	github.com/database-intelligence/processors/adaptivesampler v0.0.0
	github.com/database-intelligence/processors/circuitbreaker v0.0.0
	github.com/database-intelligence/processors/costcontrol v0.0.0
	github.com/database-intelligence/processors/nrerrormonitor v0.0.0
	github.com/database-intelligence/processors/planattributeextractor v0.0.0
	github.com/database-intelligence/processors/querycorrelator v0.0.0
	github.com/database-intelligence/processors/verification v0.0.0
	github.com/database-intelligence/extensions/healthcheck v0.0.0
	github.com/database-intelligence/exporters/nri v0.0.0
	github.com/database-intelligence/receivers/ash v0.0.0
	github.com/database-intelligence/receivers/enhancedsql v0.0.0
	github.com/database-intelligence/receivers/kernelmetrics v0.0.0
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
	github.com/database-intelligence/processors/adaptivesampler => ../processors/adaptivesampler
	github.com/database-intelligence/processors/circuitbreaker => ../processors/circuitbreaker
	github.com/database-intelligence/processors/costcontrol => ../processors/costcontrol
	github.com/database-intelligence/processors/nrerrormonitor => ../processors/nrerrormonitor
	github.com/database-intelligence/processors/planattributeextractor => ../processors/planattributeextractor
	github.com/database-intelligence/processors/querycorrelator => ../processors/querycorrelator
	github.com/database-intelligence/processors/verification => ../processors/verification
	github.com/database-intelligence/extensions/healthcheck => ../extensions/healthcheck
	github.com/database-intelligence/exporters/nri => ../exporters/nri
	github.com/database-intelligence/receivers/ash => ../receivers/ash
	github.com/database-intelligence/receivers/enhancedsql => ../receivers/enhancedsql
	github.com/database-intelligence/receivers/kernelmetrics => ../receivers/kernelmetrics
	github.com/database-intelligence/common => ../common
	github.com/database-intelligence/common/featuredetector => ../common/featuredetector
	github.com/database-intelligence/common/queryselector => ../common/queryselector
)
