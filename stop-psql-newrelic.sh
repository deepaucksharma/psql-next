#!/bin/bash

echo "Stopping PostgreSQL and New Relic monitoring..."
docker-compose -f docker-compose.psql-newrelic.yml down -v
echo "Services stopped."
