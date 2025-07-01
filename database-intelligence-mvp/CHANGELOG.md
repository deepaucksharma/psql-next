# Changelog

All notable changes to the Database Intelligence Collector will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2025-01-01

### Added

#### Enterprise Processors
- **Cost Control Processor** - Intelligent telemetry cost management with monthly budget enforcement
  - Automatic cardinality reduction when approaching budget limits
  - Support for standard ($0.35/GB) and Data Plus ($0.55/GB) pricing
  - Aggressive mode activation for over-budget scenarios
  
- **NR Error Monitor Processor** - Proactive NrIntegrationError detection
  - Validates semantic conventions before data reaches New Relic
  - Monitors attribute lengths and cardinality thresholds
  - Generates alerts before data rejection occurs
  
- **Query Correlator Processor** - Advanced query-to-database correlation
  - Links individual queries to table and database metrics
  - Categorizes query performance (slow/moderate/fast)
  - Tracks maintenance indicators and load contribution

#### OHI Migration Support
- Full metric name compatibility with New Relic On-Host Integration
- Automatic transformation from OpenTelemetry to OHI metric names
- Query performance monitoring with pg_stat_statements
- InnoDB metrics collection for MySQL
- Side-by-side validation tool for migration confidence

#### Enhanced Query Monitoring
- Query text anonymization with PII redaction
- Query fingerprinting for deduplication
- Individual query correlation with database objects
- Performance categorization and tracking

#### E2E Testing Framework
- Comprehensive tests from database to New Relic (NRDB)
- PostgreSQL and MySQL metric flow validation
- Query performance metric verification
- OHI compatibility checks
- Performance benchmarking suite

#### Documentation
- OHI Migration Guide with step-by-step instructions
- Enterprise Architecture documentation
- Comprehensive processor documentation
- Dashboard migration examples

### Changed

- Updated main.go to register all 7 processors (4 core + 3 enterprise)
- Enhanced ARCHITECTURE.md with enterprise processor details
- Updated CONFIGURATION.md with OHI migration examples
- Improved README.md with v2.0.0 features
- Enhanced Makefile with new test targets and OHI migration commands

### Fixed

- Module path consistency across all processors
- Processor type name consistency in configurations
- Import paths in main.go for all processors

### Security

- Enhanced PII detection in verification processor
- mTLS support in enterprise configurations
- Comprehensive query text anonymization

## [1.0.0] - 2025-06-30

### Added

#### Core Database Processors
- **Adaptive Sampler** - Rule-based intelligent sampling
  - LRU cache with TTL for deduplication
  - Expression evaluation for complex rules
  - In-memory state management
  
- **Circuit Breaker** - Database overload protection
  - Per-database state tracking
  - Adaptive timeout calculation
  - Automatic recovery with backoff
  
- **Plan Attribute Extractor** - Query plan analysis
  - PostgreSQL and MySQL plan parsing
  - Safe mode operation (no direct EXPLAIN)
  - Plan hash generation
  
- **Verification Processor** - Data quality and compliance
  - PII detection (SSN, credit cards, emails)
  - Data validation rules
  - Auto-tuning capabilities

#### Standard Components
- PostgreSQL receiver integration
- MySQL receiver integration
- SQL query receiver for custom metrics
- OTLP exporter to New Relic
- Prometheus exporter
- Memory limiter and batch processors

### Infrastructure
- Docker Compose deployment
- Kubernetes manifests
- Health check endpoints
- Metrics endpoints
- zPages debugging

### Documentation
- Architecture overview
- Configuration guide
- Deployment guide
- Troubleshooting guide

## [0.1.0] - 2025-06-15

### Added
- Initial project structure
- OpenTelemetry Collector Builder configuration
- Basic PostgreSQL and MySQL receivers
- Standard OTEL processors
- Initial documentation

[2.0.0]: https://github.com/database-intelligence-mvp/database-intelligence-mvp/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/database-intelligence-mvp/database-intelligence-mvp/compare/v0.1.0...v1.0.0
[0.1.0]: https://github.com/database-intelligence-mvp/database-intelligence-mvp/releases/tag/v0.1.0