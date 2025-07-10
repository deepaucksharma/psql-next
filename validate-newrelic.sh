#!/bin/bash

echo "Validating New Relic integration..."

# Check if collector is running
if curl -s http://localhost:13133/ > /dev/null; then
    echo "✓ OTEL Collector is healthy"
else
    echo "✗ OTEL Collector is not responding"
fi

# Check PostgreSQL connection
if docker exec dbintel-postgres-nr pg_isready -U postgres > /dev/null 2>&1; then
    echo "✓ PostgreSQL is ready"
else
    echo "✗ PostgreSQL is not ready"
fi

# Check metrics endpoint
if curl -s http://localhost:8888/metrics | grep -q "postgresql"; then
    echo "✓ PostgreSQL metrics are being collected"
else
    echo "✗ PostgreSQL metrics not found"
fi

echo -e "\nTo verify data in New Relic:"
echo "1. Go to https://one.newrelic.com/"
echo "2. Navigate to 'All Entities' > 'OpenTelemetry'"
echo "3. Look for service 'postgresql-monitoring'"
echo "4. Check the 'Metrics' tab for PostgreSQL data"
