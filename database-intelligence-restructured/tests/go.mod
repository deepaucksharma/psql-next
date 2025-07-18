module github.com/database-intelligence/db-intel/components/tests

go 1.23.0

replace (
	github.com/database-intelligence/db-intel/components/exporters/nri => ../exporters/nri
	github.com/database-intelligence/db-intel/components/extensions/healthcheck => ../extensions/healthcheck
	github.com/database-intelligence/db-intel/components/processors/adaptivesampler => ../processors/adaptivesampler
	github.com/database-intelligence/db-intel/components/processors/circuitbreaker => ../processors/circuitbreaker
	github.com/database-intelligence/db-intel/components/processors/costcontrol => ../processors/costcontrol
	github.com/database-intelligence/db-intel/components/processors/nrerrormonitor => ../processors/nrerrormonitor
	github.com/database-intelligence/db-intel/components/processors/planattributeextractor => ../processors/planattributeextractor
	github.com/database-intelligence/db-intel/components/processors/querycorrelator => ../processors/querycorrelator
	github.com/database-intelligence/db-intel/components/processors/verification => ../processors/verification
	github.com/database-intelligence/db-intel/internal/featuredetector => ../internal/featuredetector
	github.com/database-intelligence/db-intel/internal/queryselector => ../internal/queryselector
)

require (
	github.com/lib/pq v1.10.9
	go.uber.org/zap v1.27.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
)
