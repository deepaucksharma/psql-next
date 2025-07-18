# Configurations Directory Index

## Database-Specific Configurations

Maximum metric extraction configurations for each supported database:

- `postgresql-maximum-extraction.yaml` - PostgreSQL with ASH simulation
- `mysql-maximum-extraction.yaml` - MySQL with Performance Schema
- `mongodb-maximum-extraction.yaml` - MongoDB with Atlas support
- `mssql-maximum-extraction.yaml` - SQL Server with wait stats
- `oracle-maximum-extraction.yaml` - Oracle with V$ views

## Other Configurations

- `collector-test-consolidated.yaml` - Unified test configuration
- `base.yaml` - Base configuration template
- `examples.yaml` - Example configurations

## Environment Templates

Located in `env-templates/`:

- `database-intelligence.env` - Master template with all options
- `*-minimal.env` - Minimal templates for quick setup
- Individual database templates for backward compatibility
