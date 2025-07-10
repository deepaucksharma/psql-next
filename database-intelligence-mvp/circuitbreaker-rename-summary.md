# Circuit Breaker Naming Inconsistency Summary

## Issue
The circuit breaker processor is registered with the component type "circuit_breaker" (with underscore) in the factory.go file, but is referenced as "circuitbreaker" (without underscore) throughout all YAML configuration files.

## Files That Need to Be Updated

### Configuration Files (43 YAML files need "circuitbreaker:" changed to "circuit_breaker:")
1. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/integration/enterprise_pipeline_test.yaml`
2. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/config/unified_test_config.yaml`
3. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/config/collector-config.yaml`
4. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/testdata/full-e2e-collector.yaml`
5. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/testdata/e2e-collector.yaml`
6. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/testdata/custom-processors-e2e.yaml`
7. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/testdata/config-newrelic.yaml`
8. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/testdata/config-monitoring.yaml`
9. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/testdata/collector-e2e-config.yaml`
10. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/configs/working-processor-config.yaml`
11. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/configs/working-final-config.yaml`
12. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/configs/test-processor-config.yaml`
13. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/configs/processor-test-config.yaml`
14. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/configs/final-processor-config.yaml`
15. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/configs/correct-processor-config.yaml`
16. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/configs/comprehensive-test-config.yaml`
17. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/configs/e2e-performance.yaml`
18. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tests/configs/e2e-comprehensive.yaml`
19. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/postgres-collector/values.yaml`
20. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/db-intelligence/values.yaml`
21. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/db-intelligence/values-staging.yaml`
22. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/db-intelligence/values-dev.yaml`
23. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/db-intelligence/templates/configmap.yaml`
24. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/database-intelligence/values.yaml`
25. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/database-intelligence/templates/configmap.yaml`
26. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/test-pipeline.yaml`
27. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/production-secure.yaml`
28. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/environments/staging.yaml`
29. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/environments/production.yaml`
30. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/environments/development.yaml`
31. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/docker-collector.yaml`
32. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/docker-collector-secure.yaml`
33. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-simplified.yaml`
34. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-simple-alternate.yaml`
35. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-secure.yaml`
36. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-querylens.yaml`
37. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-plan-intelligence.yaml`
38. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-ohi-migration.yaml`
39. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-gateway-enterprise.yaml`
40. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-feature-aware.yaml`
41. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/collector-end-to-end-test.yaml`
42. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/base/processors-base.yaml`
43. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/ocb-config.yaml` (Note: This file contains the module path, not the processor name in config)

### YML Files (3 files need "circuitbreaker" changed to "circuit_breaker")
1. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tasks/validate.yml`
2. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/tasks/build.yml`
3. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/monitoring/prometheus/rules/collector-alerts.yml`

### Files That Already Use "circuit_breaker" (10 files - these are correct)
1. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/docker-compose.ohi-migration.yaml`
2. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/kubernetes/configmap.yaml`
3. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/db-intelligence/templates/configmap.yaml`
4. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deployments/helm/database-intelligence/templates/configmap.yaml`
5. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deploy/k8s/otel-collector-config-attribute-mapping.yaml`
6. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deploy/examples/docker-compose-production.yaml`
7. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deploy/examples/configs/collector-production.yaml`
8. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deploy/docker/docker-compose.yaml`
9. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/deploy/docker/docker-compose.experimental.yaml`
10. `/Users/deepaksharma/syc/db-otel/database-intelligence-mvp/config/base/processors-base.yaml` (partial - has mixed usage)

## Required Changes

### 1. Processor Name Changes
All occurrences of `circuitbreaker:` in processor definitions need to be changed to `circuit_breaker:`

### 2. Pipeline References
All occurrences of `- circuitbreaker` in pipeline configurations need to be changed to `- circuit_breaker`

### 3. Environment Variables
The environment variables are already correctly named with underscores (e.g., `CIRCUIT_BREAKER_MAX_FAILURES`), so no changes needed there.

### 4. Module Path
The module path in `ocb-config.yaml` (`github.com/database-intelligence-mvp/processors/circuitbreaker`) should remain as is since it refers to the directory name, not the component type.

## Summary
A total of 46 files need to be updated to replace "circuitbreaker" with "circuit_breaker" to match the component type defined in the factory.go file.