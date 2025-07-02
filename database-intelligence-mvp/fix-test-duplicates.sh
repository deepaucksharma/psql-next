#!/bin/bash

# Script to fix duplicate function definitions in test files

echo "Fixing duplicate function definitions in test files..."

# Fix getNewConnection duplicates
echo "Fixing getNewConnection duplicates..."
sed -i '' 's/func getNewConnection(/func getNewConnectionASH(/' tests/e2e/ash_test.go

# Fix testASHConfig duplicates
echo "Fixing testASHConfig duplicates..."
sed -i '' 's/type testASHConfig/type ashTestConfig/' tests/e2e/ash_test.go

# Fix getEnvOrDefault duplicates
echo "Fixing getEnvOrDefault duplicates..."
sed -i '' 's/func getEnvOrDefault(/func getEnvOrDefaultNRDB(/' tests/e2e/database_to_nrdb_test.go
sed -i '' 's/getEnvOrDefault(/getEnvOrDefaultNRDB(/' tests/e2e/database_to_nrdb_test.go

# Fix testFullIntegrationConfig duplicates
echo "Fixing testFullIntegrationConfig duplicates..."
sed -i '' 's/type testFullIntegrationConfig/type fullIntegrationTestConfig/' tests/e2e/integration_test.go

# Fix generateASHActivity duplicates
echo "Fixing generateASHActivity duplicates..."
sed -i '' 's/func generateASHActivity(/func generateASHActivityMonitoring(/' tests/e2e/monitoring_test.go

# Fix generateHighCardinalityQueries duplicates
echo "Fixing generateHighCardinalityQueries duplicates..."
sed -i '' 's/func generateHighCardinalityQueries(/func generateHighCardinalityQueriesMonitoring(/' tests/e2e/monitoring_test.go

# Fix causePlanRegression duplicates
echo "Fixing causePlanRegression duplicates..."
sed -i '' 's/func causePlanRegression(/func causePlanRegressionMonitoring(/' tests/e2e/monitoring_test.go

# Fix createLockContention duplicates
echo "Fixing createLockContention duplicates..."
sed -i '' 's/func createLockContention(/func createLockContentionMonitoring(/' tests/e2e/monitoring_test.go

# Fix generateRandomString duplicates
echo "Fixing generateRandomString duplicates..."
sed -i '' 's/func generateRandomString(/func generateRandomStringMonitoring(/' tests/e2e/monitoring_test.go

# Fix getEnvAsInt duplicate
echo "Fixing getEnvAsInt duplicates..."
sed -i '' 's/func getEnvAsInt(/func getEnvAsIntNRDB(/' tests/e2e/database_to_nrdb_test.go

echo "Duplicate function fix complete!"