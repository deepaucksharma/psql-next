module github.com/database-intelligence-mvp/receivers/enhancedsql

go 1.21

require (
	github.com/database-intelligence-mvp/common/featuredetector v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/collector/component v0.128.0
	go.opentelemetry.io/collector/receiver v0.128.0
	go.opentelemetry.io/collector/pdata v1.34.0
	go.uber.org/zap v1.27.0
)

replace github.com/database-intelligence-mvp/common/featuredetector => ../../common/featuredetector