# Changelog

All notable changes to the Database Intelligence Collector MVP will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Phase 2: Plan Intelligence Implementation
- **AutoExplain Receiver**: Safe execution plan collection from PostgreSQL logs
  - Multi-format support (JSON, CSV, text)
  - Non-blocking log file monitoring with fsnotify
  - Graceful log rotation handling
  
- **Plan Anonymization**: Multi-layer PII protection
  - Pattern-based detection (email, SSN, credit card, phone, IP)
  - Context-aware anonymization preserving plan structure
  - Configurable sensitive node types
  
- **Plan Storage**: Efficient plan history management
  - LRU cache with configurable size limits
  - Plan versioning and deduplication
  - TTL-based automatic cleanup
  
- **Regression Detection**: Statistical plan analysis
  - Welch's t-test for performance comparison
  - Node-specific analyzers (Seq Scan, Nested Loop)
  - Configurable thresholds and confidence levels
  
- **Configuration**: Comprehensive plan intelligence settings
  - `collector-plan-intelligence.yaml` example configuration
  - Integration with existing PostgreSQL receiver
  - Circuit breaker patterns for auto_explain errors

#### Phase 3: Active Session History (ASH) Implementation
- **ASH Receiver**: High-frequency session sampling
  - 1-second collection interval (configurable)
  - Comprehensive session snapshot collection
  - Blocking chain detection with lock analysis
  
- **Adaptive Sampling**: Intelligent load-based sampling
  - Always samples critical sessions (blocked, long-running)
  - Dynamic rate adjustment based on session count
  - Session-type specific sampling rates
  
- **Storage System**: Multi-resolution data management
  - Circular buffer for raw snapshots
  - Time-window aggregations (1m, 5m, 15m, 1h)
  - Optional compression support
  
- **Feature Detection**: Automatic capability discovery
  - Detects pg_stat_statements availability
  - Checks for pg_wait_sampling extension
  - Graceful degradation for missing features
  
- **Wait Analysis Processor**: Wait event categorization
  - Groups events into logical categories (Lock, IO, CPU, Network)
  - Pattern-based severity assignment
  - Alert rule engine with configurable thresholds
  
- **Metrics**: Comprehensive ASH metrics
  - Session distribution by state
  - Wait event analysis with categories
  - Blocking relationship tracking
  - Query activity monitoring

### Documentation
- **Receiver Documentation**:
  - Comprehensive AutoExplain receiver guide
  - Detailed ASH receiver documentation
  - Configuration examples and best practices
  
- **Processor Documentation**:
  - Wait Analysis processor guide
  - Pattern configuration examples
  - Alert rule syntax and examples
  
- **Architecture Documentation**:
  - Plan Intelligence architecture overview
  - ASH implementation details
  - Data flow diagrams
  - Performance considerations
  
- **Main README**: Complete project documentation
  - Quick start guide
  - Configuration examples
  - Deployment options
  - Troubleshooting guide

### Security
- PII protection in execution plans
- Query parameter anonymization
- Configurable sensitive data patterns
- Read-only database access enforcement

### Performance
- Adaptive sampling reduces overhead under load
- LRU caching prevents memory bloat
- Efficient circular buffer implementation
- Non-blocking log file monitoring

## [0.1.0] - Previous Release

### Added
- Phase 1: Critical Security and Robustness
  - PII detection and scrubbing
  - Circuit breaker implementation
  - Feature detection system
  - Graceful degradation

### Changed
- Enhanced error handling across all components
- Improved configuration validation

### Fixed
- Memory leaks in long-running collectors
- Race conditions in concurrent processing