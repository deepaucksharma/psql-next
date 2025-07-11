module github.com/deepaksharma/db-otel/components/distributions/enterprise

go 1.22

require (
    go.opentelemetry.io/collector/component v0.109.0
    go.opentelemetry.io/collector/otelcol v0.109.0
    
    // Custom processors
    github.com/deepaksharma/db-otel/components/processors/adaptivesampler v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/circuitbreaker v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/costcontrol v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/nrerrormonitor v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/planattributeextractor v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/querycorrelator v0.0.0-00010101000000-000000000000
    github.com/deepaksharma/db-otel/components/processors/verification v0.0.0-00010101000000-000000000000
)

// Replace directives for local development
replace (
    github.com/deepaksharma/db-otel/components/processors/adaptivesampler => ../../processors/adaptivesampler
    github.com/deepaksharma/db-otel/components/processors/circuitbreaker => ../../processors/circuitbreaker
    github.com/deepaksharma/db-otel/components/processors/costcontrol => ../../processors/costcontrol
    github.com/deepaksharma/db-otel/components/processors/nrerrormonitor => ../../processors/nrerrormonitor
    github.com/deepaksharma/db-otel/components/processors/planattributeextractor => ../../processors/planattributeextractor
    github.com/deepaksharma/db-otel/components/processors/querycorrelator => ../../processors/querycorrelator
    github.com/deepaksharma/db-otel/components/processors/verification => ../../processors/verification
    
    // Common dependencies
    github.com/deepaksharma/db-otel/components/common/anonutils => ../../common/anonutils
    github.com/deepaksharma/db-otel/components/common/detectutils => ../../common/detectutils
    github.com/deepaksharma/db-otel/components/common/featuredetector => ../../common/featuredetector
    github.com/deepaksharma/db-otel/components/common/newrelicutils => ../../common/newrelicutils
    github.com/deepaksharma/db-otel/components/common/piidetector => ../../common/piidetector
    github.com/deepaksharma/db-otel/components/common/querylens => ../../common/querylens
    github.com/deepaksharma/db-otel/components/common/queryparser => ../../common/queryparser
    github.com/deepaksharma/db-otel/components/common/queryselector => ../../common/queryselector
    github.com/deepaksharma/db-otel/components/common/sqltokenizer => ../../common/sqltokenizer
    github.com/deepaksharma/db-otel/components/common/telemetry => ../../common/telemetry
    github.com/deepaksharma/db-otel/components/common/utils => ../../common/utils
    
    // Core dependencies
    github.com/deepaksharma/db-otel/components/core/clientauth => ../../core/clientauth
    github.com/deepaksharma/db-otel/components/core/costengine => ../../core/costengine
    github.com/deepaksharma/db-otel/components/core/errorhandler => ../../core/errorhandler
    github.com/deepaksharma/db-otel/components/core/errormonitor => ../../core/errormonitor
    github.com/deepaksharma/db-otel/components/core/healthcheck => ../../core/healthcheck
    github.com/deepaksharma/db-otel/components/core/multidb => ../../core/multidb
    github.com/deepaksharma/db-otel/components/core/queryanalyzer => ../../core/queryanalyzer
    github.com/deepaksharma/db-otel/components/core/ratelimiter => ../../core/ratelimiter
    github.com/deepaksharma/db-otel/components/core/verification => ../../core/verification
)