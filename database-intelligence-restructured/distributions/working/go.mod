module github.com/database-intelligence/distributions/working

go 1.22

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.105.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.105.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.105.0
	go.opentelemetry.io/collector/component v0.105.0
	go.opentelemetry.io/collector/confmap v0.105.0
	go.opentelemetry.io/collector/consumer v0.105.0
	go.opentelemetry.io/collector/exporter v0.105.0
	go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0
	go.opentelemetry.io/collector/extension v0.105.0
	go.opentelemetry.io/collector/otelcol v0.105.0
	go.opentelemetry.io/collector/pdata v1.12.0
	go.opentelemetry.io/collector/processor v0.105.0
	go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.105.0
	go.opentelemetry.io/collector/receiver v0.105.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
)

require (
	github.com/go-viper/mapstructure/v2 v2.0.0-alpha.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v0.105.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v0.105.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v0.105.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.12.0 // indirect
	go.opentelemetry.io/collector/internal/globalgates v0.105.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Add custom processors
require (
	github.com/database-intelligence/processors/adaptivesampler v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/circuitbreaker v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/costcontrol v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/nrerrormonitor v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/planattributeextractor v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/querycorrelator v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/verification v0.0.0-00010101000000-000000000000
)

// Replace directives for local modules
replace (
	github.com/database-intelligence/common/config => ../../common/config
	github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
	github.com/database-intelligence/common/metrics => ../../common/metrics
	github.com/database-intelligence/common/newrelic => ../../common/newrelic
	github.com/database-intelligence/common/telemetry => ../../common/telemetry
	github.com/database-intelligence/common/utils => ../../common/utils
	github.com/database-intelligence/core/dataanonymizer => ../../core/dataanonymizer
	github.com/database-intelligence/core/piidetection => ../../core/piidetection
	github.com/database-intelligence/core/querylens => ../../core/querylens
	github.com/database-intelligence/processors/adaptivesampler => ../../processors/adaptivesampler
	github.com/database-intelligence/processors/circuitbreaker => ../../processors/circuitbreaker
	github.com/database-intelligence/processors/costcontrol => ../../processors/costcontrol
	github.com/database-intelligence/processors/nrerrormonitor => ../../processors/nrerrormonitor
	github.com/database-intelligence/processors/planattributeextractor => ../../processors/planattributeextractor
	github.com/database-intelligence/processors/querycorrelator => ../../processors/querycorrelator
	github.com/database-intelligence/processors/verification => ../../processors/verification
)
