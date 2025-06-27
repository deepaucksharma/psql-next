-- Script to generate various types of PostgreSQL load for testing

-- 1. Generate slow queries
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 1..5 LOOP
        PERFORM test_schema.simulate_slow_query(2); -- 2 second delay
        RAISE NOTICE 'Slow query % completed', i;
    END LOOP;
END $$;

-- 2. Generate high-frequency queries
DO $$
DECLARE
    i INTEGER;
    user_count INTEGER;
BEGIN
    FOR i IN 1..100 LOOP
        -- Count queries
        SELECT COUNT(*) INTO user_count FROM test_schema.users;
        
        -- Random user lookup
        SELECT * FROM test_schema.users WHERE id = (RANDOM() * 3 + 1)::INTEGER;
        
        -- Join query
        SELECT u.username, COUNT(o.id) 
        FROM test_schema.users u 
        LEFT JOIN test_schema.orders o ON u.id = o.user_id 
        GROUP BY u.username;
        
        -- Insert some orders
        IF i % 10 = 0 THEN
            INSERT INTO test_schema.orders (user_id, total_amount, status) 
            VALUES 
                ((RANDOM() * 3 + 1)::INTEGER, (RANDOM() * 1000)::DECIMAL(10,2), 'pending'),
                ((RANDOM() * 3 + 1)::INTEGER, (RANDOM() * 1000)::DECIMAL(10,2), 'completed');
        END IF;
    END LOOP;
END $$;

-- 3. Generate blocking sessions (run in separate sessions)
-- Session 1: Hold a lock
BEGIN;
UPDATE test_schema.users SET updated_at = NOW() WHERE id = 1;
-- Don't commit immediately to create blocking

-- 4. Generate wait events
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 1..20 LOOP
        -- Concurrent updates to create lock waits
        UPDATE test_schema.orders 
        SET status = CASE 
            WHEN status = 'pending' THEN 'processing'
            WHEN status = 'processing' THEN 'completed'
            ELSE 'pending'
        END
        WHERE id = (RANDOM() * 10 + 1)::INTEGER;
        
        PERFORM pg_sleep(0.1);
    END LOOP;
END $$;

-- 5. Complex analytical queries
WITH user_orders AS (
    SELECT 
        u.id,
        u.username,
        COUNT(o.id) as order_count,
        SUM(o.total_amount) as total_spent,
        AVG(o.total_amount) as avg_order_value
    FROM test_schema.users u
    LEFT JOIN test_schema.orders o ON u.id = o.user_id
    GROUP BY u.id, u.username
),
order_stats AS (
    SELECT 
        status,
        COUNT(*) as status_count,
        SUM(total_amount) as status_total
    FROM test_schema.orders
    GROUP BY status
)
SELECT * FROM user_orders, order_stats;

-- 6. Force checkpoint and vacuum
CHECKPOINT;
VACUUM ANALYZE test_schema.users;
VACUUM ANALYZE test_schema.orders;

-- 7. Check pg_stat_statements
SELECT 
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    rows
FROM pg_stat_statements
WHERE query NOT LIKE '%pg_stat_statements%'
ORDER BY total_exec_time DESC
LIMIT 10;