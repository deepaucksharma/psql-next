module github.com/deepaksharma/db-otel/components

go 1.22

toolchain go1.24.3

// All components use OpenTelemetry v0.105.0 for consistency

replace (
	github.com/deepaksharma/db-otel/common/featuredetector => ../common/featuredetector
	github.com/deepaksharma/db-otel/common/queryselector => ../common/queryselector
	github.com/deepaksharma/db-otel/internal/database => ../internal/database
)
