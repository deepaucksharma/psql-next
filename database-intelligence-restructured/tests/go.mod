module github.com/deepaksharma/db-otel/components/tests

go 1.22

replace (
	github.com/deepaksharma/db-otel/components/common => ../common
	github.com/deepaksharma/db-otel/components/common/featuredetector => ../common/featuredetector
	github.com/deepaksharma/db-otel/components/common/queryselector => ../common/queryselector
	github.com/deepaksharma/db-otel/components/exporters/nri => ../exporters/nri
	github.com/deepaksharma/db-otel/components/extensions/healthcheck => ../extensions/healthcheck
	github.com/deepaksharma/db-otel/components/processors/adaptivesampler => ../processors/adaptivesampler
	github.com/deepaksharma/db-otel/components/processors/circuitbreaker => ../processors/circuitbreaker
	github.com/deepaksharma/db-otel/components/processors/costcontrol => ../processors/costcontrol
	github.com/deepaksharma/db-otel/components/processors/nrerrormonitor => ../processors/nrerrormonitor
	github.com/deepaksharma/db-otel/components/processors/planattributeextractor => ../processors/planattributeextractor
	github.com/deepaksharma/db-otel/components/processors/querycorrelator => ../processors/querycorrelator
	github.com/deepaksharma/db-otel/components/processors/verification => ../processors/verification
)
