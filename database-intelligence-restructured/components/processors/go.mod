module github.com/deepaksharma/db-otel/components/processors

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/processor v0.105.0
    go.opentelemetry.io/collector/pdata v1.12.0
    go.opentelemetry.io/collector/consumer v0.105.0
    go.opentelemetry.io/collector/processor/processorhelper v0.105.0
    go.uber.org/zap v1.27.0
    github.com/deepaksharma/db-otel/common/featuredetector v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/internal/boundedmap v0.0.0-00010101000000-000000000000
    github.com/hashicorp/golang-lru/v2 v2.0.7
    github.com/go-redis/redis/v8 v8.11.5
    github.com/tidwall/gjson v1.18.0
    github.com/stretchr/testify v1.9.0
)

replace (
    github.com/deepaksharma/db-otel/common/featuredetector => ../../common/featuredetector
    github.com/deepaksharma/db-otel/components/internal/boundedmap => ../internal/boundedmap
)