# Archived Documentation Index

This index provides a comprehensive overview of all archived documentation that contains important historical context, architectural decisions, and implementation details.

## Critical Architecture Documents

### Architecture Review Series (Phase 1)
These documents detail the fundamental architectural issues found and fixed:

1. **[01-executive-summary.md](docs/archive/architecture-review/01-executive-summary.md)**
   - Critical issues: 15+ modules, no abstractions, configuration chaos
   - Required fixes outlined

2. **[02-module-architecture-analysis.md](docs/archive/architecture-review/02-module-architecture-analysis.md)**
   - Detailed analysis of module conflicts
   - Dependency graph issues

3. **[phase1-complete-summary.md](docs/archive/architecture-review/phase1-complete-summary.md)**
   - Summary of all Phase 1 fixes
   - Module consolidation results

4. **[phase1-memory-leak-fixes.md](docs/archive/architecture-review/phase1-memory-leak-fixes.md)**
   - Memory leak sources identified
   - Fixes implemented

## Implementation Analysis

### Core Implementation Documents

1. **[METRIC_SOURCE_ANALYSIS_REPORT.md](tests/e2e/archive/docs/METRIC_SOURCE_ANALYSIS_REPORT.md)**
   - Complete analysis of all metric sources
   - PostgreSQL vs MySQL metrics breakdown

2. **[IMPLEMENTATION_ANALYSIS_SUMMARY.md](tests/e2e/archive/docs/IMPLEMENTATION_ANALYSIS_SUMMARY.md)**
   - Implementation gaps identified
   - Working vs non-working components

3. **[END_TO_END_ANALYSIS_REPORT.md](docs/archive/END_TO_END_ANALYSIS_REPORT.md)**
   - Full system analysis
   - Integration points documented

## Project Status Evolution

### Key Status Reports

1. **[COMPREHENSIVE_FIXES_COMPLETE.md](docs/archive/project-status/COMPREHENSIVE_FIXES_COMPLETE.md)**
   - All fixes applied: error handling, security, configuration
   - Docker support added

2. **[CODE_QUALITY_ISSUES_REPORT.md](docs/archive/project-status/CODE_QUALITY_ISSUES_REPORT.md)**
   - 8 categories of issues across 100+ files
   - Automated analysis results

3. **[COMPLETED_PROJECT_SUMMARY.md](docs/archive/project-status/COMPLETED_PROJECT_SUMMARY.md)**
   - Final project state
   - All achievements documented

## Testing Documentation

### E2E Testing Series

1. **[02-test-strategy.md](docs/archive/02-e2e-testing/02-test-strategy.md)**
   - Comprehensive test approach
   - Validation methodology

2. **[05-final-report.md](docs/archive/02-e2e-testing/05-final-report.md)**
   - Test results summary
   - Coverage analysis

3. **[E2E_TEST_RESULTS.md](docs/archive/E2E_TEST_RESULTS.md)**
   - Detailed test execution results
   - Pass/fail analysis

## OHI Migration Documentation

### Migration Strategy

1. **[01-complete-mapping.md](docs/archive/03-ohi-migration/01-complete-mapping.md)**
   - OHI to OTel metric mapping
   - Transformation rules

2. **[04-unified-platform.md](docs/archive/03-ohi-migration/04-unified-platform.md)**
   - Unified monitoring approach
   - Integration patterns

3. **[06-validation-report.md](docs/archive/03-ohi-migration/06-validation-report.md)**
   - Migration validation results
   - Compatibility verification

## Configuration Documentation

### Configuration Management

1. **[CONFIGURATION.md](docs/archive/CONFIGURATION.md)**
   - Original configuration approach
   - All configuration options

2. **[ENV_VARIABLE_MAPPING.md](docs/archive/ENV_VARIABLE_MAPPING.md)**
   - Complete environment variable reference
   - Default values documented

3. **[environment-variables.md](docs/archive/environment-variables.md)**
   - Production environment setup
   - Security considerations

## Deployment & Operations

### Production Deployment

1. **[DEPLOYMENT.md](docs/archive/DEPLOYMENT.md)**
   - Original deployment strategy
   - Production considerations

2. **[deployment-guide.md](docs/archive/deployment-guide.md)**
   - Step-by-step deployment
   - Troubleshooting guide

3. **[deployment.md](docs/archive/operations/deployment.md)**
   - Operations team guide
   - Monitoring setup

## Architecture Deep Dives

### Component Architecture

1. **[ash-implementation.md](docs/archive/architecture/ash-implementation.md)**
   - Active Session History design
   - Implementation details

2. **[custom-components-design.md](docs/archive/architecture/custom-components-design.md)**
   - Custom component patterns
   - Integration approach

3. **[PROCESSORS.md](docs/archive/architecture/PROCESSORS.md)**
   - All processor designs
   - Configuration options

## Quick References

### Consolidated Summaries

1. **[00-consolidated.md files](docs/archive/*/00-consolidated.md)**
   - Each major section has a consolidated summary
   - Quick overview of each area

### Implementation Guides

1. **[quick-start.md](docs/archive/quick-start.md)**
   - Original quick start guide
   - Basic setup instructions

2. **[QUICK_START.md](docs/QUICK_START.md)**
   - Current simplified version
   - 5-minute setup

## Historical Context

### Project Evolution

1. **[RESTRUCTURING_COMPLETE.md](docs/archive/project-status/RESTRUCTURING_COMPLETE.md)**
   - Major restructuring effort
   - Before/after comparison

2. **[CLEANUP_SUMMARY.md](docs/archive/CLEANUP_SUMMARY.md)**
   - Code cleanup results
   - Technical debt reduction

3. **[SUCCESS_SUMMARY.md](docs/archive/project-status/SUCCESS_SUMMARY.md)**
   - Project achievements
   - Metrics improved

## Security & Maintenance

1. **[secrets-management.md](docs/archive/06-security/secrets-management.md)**
   - Security best practices
   - Credential management

2. **[code-cleanup.md](docs/archive/05-maintenance/code-cleanup.md)**
   - Maintenance procedures
   - Code quality standards

## Dashboard Documentation

1. **[DASHBOARD_GUIDE.md](tests/e2e/dashboards/DASHBOARD_GUIDE.md)**
   - Dashboard creation guide
   - NRQL query reference

2. **[DASHBOARD_MIGRATION_STRATEGY.md](docs/DASHBOARD_MIGRATION_STRATEGY.md)**
   - Migration from Grafana
   - New Relic dashboards

## Accessing Archived Content

All archived documentation remains in the `docs/archive/` directory. Key principles:

1. **Historical Reference**: Use for understanding decisions
2. **Not Current**: May not reflect current implementation
3. **Consolidated**: See [CONSOLIDATED_DOCUMENTATION.md](CONSOLIDATED_DOCUMENTATION.md) for current state

## Important Notes

- **Primary Reference**: Use CONSOLIDATED_DOCUMENTATION.md for current information
- **Archive Purpose**: Historical context and detailed analysis
- **Version Note**: Archive reflects pre-PostgreSQL-only state
- **Current Focus**: PostgreSQL-only implementation with maximum metrics

---

*Index Generated: [Current Date]*
*Total Archived Files: 100+ markdown documents*
*Consolidation Complete: 94% reduction in active documentation*