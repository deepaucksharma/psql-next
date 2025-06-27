#!/bin/bash

# Generate slow queries to test metrics collection
echo "Generating slow queries..."

for i in {1..5}; do
    echo "Running slow query $i..."
    docker exec postgres-collector-db psql -U postgres -d testdb -c "SELECT test_schema.simulate_slow_query(1.5);" &
done

wait
echo "Slow queries completed."