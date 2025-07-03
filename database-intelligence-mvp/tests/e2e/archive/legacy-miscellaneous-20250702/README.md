# Legacy Miscellaneous Files Archive - July 2, 2025

This directory contains various legacy utility scripts, JavaScript files, and other miscellaneous artifacts.

## Archived Files

### Shell Scripts
- `simulate-nrdb-queries.sh` - NRQL query simulation against local JSON exports
- `validate-data-shape.sh` - Data structure validation utility
- `validate_e2e_complete.sh` - Basic E2E completion validation
- `test_pii_queries.sh` - PII data testing script

### JavaScript Files
- `test-api-key.js` - API key testing utility
- `dashboard-metrics-validation.js` - Dashboard metrics validation script

### Documentation
- `comprehensive_test_report.md` - Legacy test execution report

### Binaries
- `database-intelligence-collector` - Old collector binary build artifact

## Replacement

These utilities were replaced by:
- `validators/` - Structured Go-based validation components
- `orchestrator/main.go` - Comprehensive test orchestration
- Unified reporting in the main framework
- Build artifacts are now managed through proper CI/CD

## Reason for Archival

These legacy files had the following issues:
- Inconsistent error handling and output formats
- Hardcoded file paths and assumptions
- Limited reusability across different test scenarios
- Mixed languages (shell, JavaScript, Go) without clear organization
- No integration with the main test framework

The unified framework provides consistent, maintainable alternatives with proper integration.