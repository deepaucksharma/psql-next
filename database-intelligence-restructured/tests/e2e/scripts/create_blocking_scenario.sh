#!/bin/bash
# Script to create a blocking scenario in PostgreSQL

CONTAINER="psql-postgres-db-1"

echo "Creating test table..."
docker exec $CONTAINER psql -U postgres -c "
CREATE TABLE IF NOT EXISTS test_blocking (
    id SERIAL PRIMARY KEY,
    data TEXT
);
INSERT INTO test_blocking (data) VALUES ('test1'), ('test2'), ('test3');
"

echo "Starting blocking transaction in background..."
docker exec -d $CONTAINER psql -U postgres -c "
BEGIN;
UPDATE test_blocking SET data = 'blocked' WHERE id = 1;
SELECT pg_sleep(30);
COMMIT;
" &

sleep 2

echo "Attempting conflicting update (will be blocked)..."
docker exec $CONTAINER psql -U postgres -c "
UPDATE test_blocking SET data = 'trying_to_update' WHERE id = 1;
" &

echo "Blocking scenario created. Check metrics..."
sleep 5

echo "Checking for blocking sessions..."
docker exec $CONTAINER psql -U postgres -c "
SELECT 
    blocked.pid as blocked_pid,
    blocked.query as blocked_query,
    blocking.pid as blocking_pid,
    blocking.query as blocking_query
FROM pg_stat_activity blocked
JOIN pg_stat_activity blocking 
    ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
WHERE blocked.wait_event IS NOT NULL;
"