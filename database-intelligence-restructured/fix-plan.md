# Detailed Fix Plan

## Phase 1: Module Path Alignment
1. Update all go.mod files to use `github.com/database-intelligence/` (remove -restructured)
2. Update all import statements in .go files
3. Update replace directives

## Phase 2: Version Alignment (Option C - Latest)
### Base Packages (v1.35.0)
- component, confmap, consumer, pdata, processor, receiver, extension, exporter

### Implementation Packages (v0.129.0)
- Specific processors (batchprocessor, memorylimiterprocessor)
- Specific receivers (otlpreceiver, postgresqlreceiver)
- Contrib packages

### Update Order:
1. Common modules first
2. Processors/Receivers
3. Core module
4. Distributions

## Phase 3: Import Cleanup
1. Remove direct confmap imports from processors/receivers
2. Use component.Config interface
3. Let the framework handle configuration

## Phase 4: Testing
1. Build each module individually
2. Run unit tests
3. Build complete collector
4. Run E2E tests

## Version Mapping Table
| Package Type | Current | Target |
|-------------|---------|---------|
| component   | mixed   | v1.35.0 |
| confmap     | mixed   | v1.35.0 |
| pdata       | v1.16.0 | v1.35.0 |
| processor   | mixed   | v1.35.0 |
| batchprocessor | mixed | v0.129.0 |
| contrib     | mixed   | v0.129.0 |
