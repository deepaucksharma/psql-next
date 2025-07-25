module github.com/database-intelligence/db-intel/tests/e2e

go 1.23.0

require (
	github.com/go-sql-driver/mysql v1.9.3
	github.com/lib/pq v1.10.9
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.10.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace (
	github.com/database-intelligence/db-intel/components/common => ../../common
	github.com/database-intelligence/db-intel/components/processors/adaptivesampler => ../../processors/adaptivesampler
	github.com/database-intelligence/db-intel/components/processors/circuitbreaker => ../../processors/circuitbreaker
	github.com/database-intelligence/db-intel/components/processors/verification => ../../processors/verification
)
