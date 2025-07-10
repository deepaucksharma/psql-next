module github.com/database-intelligence/tests

go 1.22

replace (
	github.com/database-intelligence/common => ../common
	github.com/database-intelligence/common/featuredetector => ../common/featuredetector
	github.com/database-intelligence/common/queryselector => ../common/queryselector
	github.com/database-intelligence/exporters/nri => ../exporters/nri
	github.com/database-intelligence/extensions/healthcheck => ../extensions/healthcheck
	github.com/database-intelligence/processors/adaptivesampler => ../processors/adaptivesampler
	github.com/database-intelligence/processors/circuitbreaker => ../processors/circuitbreaker
	github.com/database-intelligence/processors/costcontrol => ../processors/costcontrol
	github.com/database-intelligence/processors/nrerrormonitor => ../processors/nrerrormonitor
	github.com/database-intelligence/processors/planattributeextractor => ../processors/planattributeextractor
	github.com/database-intelligence/processors/querycorrelator => ../processors/querycorrelator
	github.com/database-intelligence/processors/verification => ../processors/verification
)
