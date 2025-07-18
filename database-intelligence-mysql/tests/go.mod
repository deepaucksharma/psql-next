module github.com/database-intelligence/mysql-monitoring/tests

go 1.22

require (
	github.com/stretchr/testify v1.9.0
	github.com/testcontainers/testcontainers-go v0.29.1
	github.com/testcontainers/testcontainers-go/modules/mysql v0.29.1
	github.com/go-sql-driver/mysql v1.7.1
	go.opentelemetry.io/collector/pdata v1.9.0
	go.opentelemetry.io/collector/component v0.102.1
	go.uber.org/zap v1.27.0
	gopkg.in/yaml.v3 v3.0.1
)