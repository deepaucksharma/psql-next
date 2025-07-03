# Legacy Test Runners Archive - July 2, 2025

This directory contains legacy test runner scripts and Go programs that were replaced by the unified E2E testing framework.

## Archived Files

- `run_working_e2e_tests.sh` - Old shell-based test runner that only ran "working" tests
- `run_comprehensive_tests.sh` - Old comprehensive test runner with basic error handling
- `run_all_e2e_tests.go` - Go program that attempted to orchestrate multiple test files

## Replacement

These files were replaced by:
- `run-unified-e2e.sh` - Modern unified test runner with comprehensive orchestration
- `orchestrator/main.go` - Go-based test orchestrator with advanced features
- `framework/` - Structured test framework with proper interfaces

## Reason for Archival

These legacy runners had the following issues:
- Inconsistent error handling
- Limited configuration options
- No proper test result aggregation
- Hardcoded test file references
- No parallel execution support

The unified framework addresses all these limitations with a modern, maintainable approach.