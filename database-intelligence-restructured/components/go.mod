module github.com/database-intelligence/db-intel/components

go 1.23.0

toolchain go1.24.3

// All components use OpenTelemetry v0.105.0 for consistency

replace (
	github.com/database-intelligence/db-intel/internal/featuredetector => ../internal/featuredetector
	github.com/database-intelligence/db-intel/internal/queryselector => ../internal/queryselector
	github.com/database-intelligence/db-intel/internal/database => ../internal/database
)
