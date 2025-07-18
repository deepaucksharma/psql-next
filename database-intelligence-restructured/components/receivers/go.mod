module github.com/database-intelligence/db-intel/components/receivers

go 1.23.0

toolchain go1.24.3

require (
	github.com/database-intelligence/db-intel/components/receivers/mongodb v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/db-intel/components/receivers/redis v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/db-intel/internal/database v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/db-intel/internal/featuredetector v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/db-intel/internal/queryselector v0.0.0-00010101000000-000000000000
	github.com/go-sql-driver/mysql v1.9.3
	github.com/lib/pq v1.10.9
	go.opentelemetry.io/collector/component v0.105.0
	go.opentelemetry.io/collector/config/configretry v1.12.0
	go.opentelemetry.io/collector/consumer v0.105.0
	go.opentelemetry.io/collector/pdata v1.12.0
	go.opentelemetry.io/collector/receiver v0.105.0
	go.uber.org/zap v1.27.0
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/redis/go-redis/v9 v9.5.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.mongodb.org/mongo-driver v1.13.1 // indirect
	go.opentelemetry.io/collector v0.105.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.129.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250519155744-55703ea1f237 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace (
	github.com/database-intelligence/db-intel/components/receivers/mongodb => ./mongodb
	github.com/database-intelligence/db-intel/components/receivers/redis => ./redis
	github.com/database-intelligence/db-intel/internal/database => ../../internal/database
	github.com/database-intelligence/db-intel/internal/featuredetector => ../../internal/featuredetector
	github.com/database-intelligence/db-intel/internal/queryselector => ../../internal/queryselector
)
