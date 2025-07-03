# Legacy Configuration Files Archive - July 2, 2025

This directory contains legacy configuration files that were used for E2E testing setup.

## Archived Files

- `docker-compose-test.yaml` - Old Docker Compose configuration for test databases
- `docker-compose.e2e.yml` - Alternative E2E Docker Compose setup
- `e2e-test-config.yaml` - Legacy collector configuration for E2E tests
- `collector-e2e-test.yaml` - Old collector configuration file

## Replacement

These files were replaced by:
- `config/unified_test_config.yaml` - Centralized configuration for all test scenarios
- `config/e2e-test-collector-*.yaml` - Specialized collector configurations
- `testdata/docker-compose.test.yml` - Modern Docker Compose setup
- `containers/` - Structured container configuration approach

## Reason for Archival

These legacy configurations had the following issues:
- Inconsistent naming conventions
- Duplicated configuration blocks
- Hardcoded port assignments that could conflict
- Limited parameterization
- No environment-specific variations

The unified configuration approach provides better organization, consistency, and maintainability.