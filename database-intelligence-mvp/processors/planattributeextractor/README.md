# Plan Attribute Extractor Processor

A custom OpenTelemetry Collector processor that extracts structured attributes from database query execution plans.

## Features

- **PostgreSQL JSON Plan Parsing**: Extracts cost, rows, operations, and performance metrics
- **MySQL Metadata Extraction**: Processes performance schema digest data
- **Safety Controls**: Timeout protection and error handling modes
- **Plan Hash Generation**: Creates consistent hashes for deduplication
- **Derived Attributes**: Computes plan characteristics like depth and efficiency

## Configuration

```yaml
processors:
  plan_attribute_extractor:
    timeout_ms: 100
    error_mode: ignore
    
    postgresql_rules:
      detection_jsonpath: "$[0].Plan"
      extractions:
        db.query.plan.cost: "$[0].Plan['Total Cost']"
        db.query.plan.rows: "$[0].Plan['Plan Rows']"
        db.query.plan.operation: "$[0].Plan['Node Type']"
      
    hash_config:
      include:
        - query_text
        - db.query.plan.operation
        - database_name
      output: db.query.plan.hash
      algorithm: sha256
```

## Extracted Attributes

### PostgreSQL Plans
- `db.query.plan.cost` - Total estimated cost
- `db.query.plan.rows` - Estimated row count
- `db.query.plan.operation` - Primary operation type
- `db.query.plan.has_seq_scan` - Contains sequential scan
- `db.query.plan.depth` - Plan tree depth
- `db.query.plan.efficiency` - Rows per cost unit

### MySQL Metadata
- `db.query.plan.avg_rows` - Average rows examined
- `db.query.digest` - Query digest hash
- `db.query.plan.execution_count` - Number of executions

## Safety Features

- **Timeout Protection**: Configurable per-record processing timeout
- **Error Handling**: Ignore or propagate modes
- **Memory Safety**: Bounded JSON processing
- **Debug Logging**: Optional detailed logging

## Usage in Collector

```yaml
service:
  pipelines:
    logs:
      processors:
        - memory_limiter
        - plan_attribute_extractor
        - batch
```

## Building

To use this processor in a custom collector build:

1. Add to your `go.mod`:
```go
require github.com/newrelic/database-intelligence-mvp/processors/planattributeextractor v1.0.0
```

2. Import in your main.go:
```go
import "github.com/newrelic/database-intelligence-mvp/processors/planattributeextractor"
```

3. Register the factory:
```go
factories.Processors[planattributeextractor.GetType()] = planattributeextractor.NewFactory()
```