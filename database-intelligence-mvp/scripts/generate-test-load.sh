#!/bin/bash
# Generate test database load for OHI validation

echo "Generating database load to trigger slow queries and other events..."

# Run various queries to generate activity
for i in {1..10}; do
    echo "Iteration $i..."
    
    # Slow query (cross join to make it slow)
    docker exec db-intel-postgres-primary psql -U postgres -d testdb -c "
        SELECT c1.*, c2.*, o.* 
        FROM customers c1 
        CROSS JOIN customers c2 
        JOIN orders o ON o.customer_id = c1.id
        WHERE c1.id > 0;" >/dev/null 2>&1
    
    # Create some blocking scenarios
    docker exec db-intel-postgres-primary psql -U postgres -d testdb -c "
        BEGIN;
        UPDATE customers SET email = 'test$i@example.com' WHERE id = 1;
        SELECT pg_sleep(2);
        COMMIT;" >/dev/null 2>&1 &
    
    # Another connection trying to update same row
    docker exec db-intel-postgres-primary psql -U postgres -d testdb -c "
        BEGIN;
        UPDATE customers SET name = 'Test User $i' WHERE id = 1;
        COMMIT;" >/dev/null 2>&1 &
    
    # Some aggregation queries
    docker exec db-intel-postgres-primary psql -U postgres -d testdb -c "
        SELECT 
            COUNT(*) as order_count,
            SUM(total) as total_revenue,
            AVG(total) as avg_order_value
        FROM orders
        WHERE customer_id IN (SELECT id FROM customers);" >/dev/null 2>&1
    
    # Index scan query
    docker exec db-intel-postgres-primary psql -U postgres -d testdb -c "
        SELECT * FROM orders WHERE id = $i;" >/dev/null 2>&1
    
    sleep 1
done

# Wait for background jobs
wait

echo "Load generation complete!"
echo "Waiting for metrics to be collected..."
sleep 30