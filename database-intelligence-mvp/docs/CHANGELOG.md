# Changelog

All notable changes to the Database Intelligence Collector project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-06-30

### Added
- Production-ready single-instance deployment architecture
- In-memory state management for all processors
- Comprehensive PII detection patterns (SSN, credit cards, emails, phones)
- Environment-aware configuration system
- Health monitoring endpoints (`:13133/health`)
- Prometheus metrics endpoint (`:8888/metrics`)
- Rate limiting with per-database controls
- Circuit breaker with automatic recovery
- Plan attribute extraction for PostgreSQL and MySQL
- Verification processor with data quality checks
- Complete operations runbook
- Configuration generator script
- Automated deployment procedures

### Changed
- Removed Redis dependency completely
- Converted all processors to in-memory state
- Enhanced PII detection with more patterns
- Improved configuration with environment overrides
- Optimized memory usage (200-300MB typical)
- Reduced startup time to 2-3 seconds
- Updated all documentation to reflect production status

### Fixed
- Module path inconsistencies in build configuration
- Unsafe pg_querylens dependency removed
- Processor pipeline coupling issues
- Memory leaks in cache management
- Circuit breaker state persistence
- Export failures with high cardinality data

### Security
- Enhanced PII detection and sanitization
- Removed all file-based state storage
- Added resource limits and bounds
- Implemented secure defaults

## [0.9.0] - 2025-06-15 (Pre-release)

### Added
- Initial implementation of 4 custom processors
- Basic PostgreSQL and MySQL receivers
- OTLP export to New Relic
- Docker Compose deployment
- Basic documentation

### Known Issues
- Required Redis for state management
- File-based persistence caused I/O bottlenecks
- Limited PII detection patterns
- No production hardening

## Migration Guide

### From 0.9.0 to 1.0.0

1. **Remove Redis Dependencies**
   - Remove Redis from docker-compose.yaml
   - Remove Redis connection configs
   - Update processor configurations

2. **Update Processor Configs**
   ```yaml
   # Old
   adaptive_sampler:
     state_file: /var/lib/sampler.state
   
   # New
   adaptive_sampler:
     in_memory_only: true  # Always true
   ```

3. **Update Environment Variables**
   - Add `ENVIRONMENT` variable
   - Update sampling thresholds
   - Configure rate limits

4. **Deploy New Version**
   ```bash
   # Pull new image
   docker pull database-intelligence/collector:1.0.0
   
   # Update and restart
   docker-compose down
   docker-compose up -d
   ```

---

**Note**: For detailed upgrade procedures, see [Operations Runbook](./operations/RUNBOOK.md)