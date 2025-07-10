#!/bin/bash

echo "Starting PostgreSQL with New Relic monitoring..."

# Start the services
docker-compose -f docker-compose.psql-newrelic.yml up -d

# Wait for services to be ready
echo "Waiting for services to start..."
sleep 30

# Check service status
echo -e "\nService Status:"
docker-compose -f docker-compose.psql-newrelic.yml ps

# Show collector logs
echo -e "\nCollector Logs (last 20 lines):"
docker-compose -f docker-compose.psql-newrelic.yml logs --tail=20 otel-collector

echo -e "\nSetup complete! Services are running."
echo "PostgreSQL is available at: localhost:5432"
echo "OTEL Collector metrics at: http://localhost:8888/metrics"
echo "Health check at: http://localhost:13133/"
echo -e "\nCheck New Relic One for incoming data:"
echo "https://one.newrelic.com/"
