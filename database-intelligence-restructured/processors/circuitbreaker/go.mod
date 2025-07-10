module github.com/database-intelligence/processors/circuitbreaker

go 1.22


require (
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v0.105.0
	go.opentelemetry.io/collector/consumer v0.105.0
	go.opentelemetry.io/collector/pdata v1.12.0
	go.opentelemetry.io/collector/processor v0.105.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.105.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
