#!/bin/bash

# Add PostgreSQL to PATH if needed
export PATH="/opt/homebrew/opt/postgresql@16/bin:$PATH"

# Generate PostgreSQL load for testing
echo "Generating PostgreSQL load for testing..."

# Function to run SQL in background
run_sql_background() {
    local sql=$1
    local desc=$2
    echo "Starting: $desc"
    PGPASSWORD=postgres psql -h localhost -U postgres -d testdb -c "$sql" &
}

# 1. Generate slow queries
echo "=== Generating slow queries ==="
for i in {1..5}; do
    run_sql_background "SELECT test_schema.simulate_slow_query(3);" "Slow query $i"
    sleep 0.5
done

# 2. Generate blocking scenario
echo -e "\n=== Creating blocking scenario ==="
# Start a transaction that holds a lock
PGPASSWORD=postgres psql -h localhost -U postgres -d testdb <<EOF &
BEGIN;
UPDATE test_schema.users SET updated_at = NOW() WHERE id = 1;
SELECT pg_sleep(10);
COMMIT;
EOF

sleep 1

# Try to update the same row from another session (will be blocked)
for i in {1..3}; do
    run_sql_background "UPDATE test_schema.users SET email = 'blocked$i@example.com' WHERE id = 1;" "Blocked query $i"
done

# 3. Generate high-frequency queries
echo -e "\n=== Generating high-frequency queries ==="
for i in {1..50}; do
    # Random queries
    run_sql_background "SELECT * FROM test_schema.users WHERE id = $((RANDOM % 3 + 1));" "Select user $i"
    run_sql_background "SELECT COUNT(*) FROM test_schema.orders WHERE status = 'pending';" "Count orders $i"
    
    # Occasionally do writes
    if [ $((i % 10)) -eq 0 ]; then
        run_sql_background "INSERT INTO test_schema.orders (user_id, total_amount, status) VALUES ($((RANDOM % 3 + 1)), $((RANDOM % 1000)).99, 'pending');" "Insert order $i"
    fi
    
    sleep 0.1
done

# 4. Complex analytical queries
echo -e "\n=== Running complex analytical queries ==="
PGPASSWORD=postgres psql -h localhost -U postgres -d testdb <<EOF &
WITH order_summary AS (
    SELECT 
        u.username,
        COUNT(o.id) as order_count,
        SUM(o.total_amount) as total_spent,
        AVG(o.total_amount)::DECIMAL(10,2) as avg_order,
        MAX(o.order_date) as last_order
    FROM test_schema.users u
    JOIN test_schema.orders o ON u.id = o.user_id
    GROUP BY u.username
),
status_summary AS (
    SELECT 
        status,
        COUNT(*) as count,
        SUM(total_amount) as total
    FROM test_schema.orders
    GROUP BY status
)
SELECT * FROM order_summary 
CROSS JOIN status_summary
ORDER BY total_spent DESC;
EOF

# 5. Generate wait events with concurrent updates
echo -e "\n=== Generating concurrent updates ==="
for i in {1..10}; do
    for j in {1..5}; do
        run_sql_background "UPDATE test_schema.orders SET status = 'processing' WHERE id = $j AND status = 'pending';" "Concurrent update $i-$j"
    done
    sleep 0.2
done

# Wait for some operations to complete
sleep 5

# 6. Check results
echo -e "\n=== Checking pg_stat_statements ==="
PGPASSWORD=postgres psql -h localhost -U postgres -d testdb <<EOF
SELECT 
    LEFT(query, 60) as query_snippet,
    calls,
    ROUND(total_exec_time::numeric, 2) as total_ms,
    ROUND(mean_exec_time::numeric, 2) as mean_ms,
    rows
FROM pg_stat_statements
WHERE query NOT LIKE '%pg_stat%'
ORDER BY total_exec_time DESC
LIMIT 10;
EOF

echo -e "\n=== Checking current activity ==="
PGPASSWORD=postgres psql -h localhost -U postgres -d testdb <<EOF
SELECT 
    pid,
    state,
    wait_event_type,
    wait_event,
    LEFT(query, 50) as query_snippet
FROM pg_stat_activity
WHERE state != 'idle'
  AND pid != pg_backend_pid();
EOF

echo -e "\nLoad generation complete!"
echo "Wait for all background jobs to finish..."
wait

echo "All done!"