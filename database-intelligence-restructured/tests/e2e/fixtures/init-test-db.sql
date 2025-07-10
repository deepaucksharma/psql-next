-- Initialize test database with required extensions and sample data

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Try to enable optional extensions (may fail, which is OK)
CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;

-- Grant permissions to test user
GRANT pg_monitor TO test_user;
GRANT SELECT ON pg_stat_statements TO test_user;

-- Create test schemas
CREATE SCHEMA IF NOT EXISTS test_data;
GRANT ALL ON SCHEMA test_data TO test_user;

-- Sample configuration for pg_stat_statements
ALTER SYSTEM SET pg_stat_statements.track = 'all';
ALTER SYSTEM SET pg_stat_statements.track_planning = on;
ALTER SYSTEM SET pg_stat_statements.max = 10000;

-- Create initial test tables
CREATE TABLE IF NOT EXISTS test_data.sample_users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255),
    username VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS test_data.sample_events (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES test_data.sample_users(id),
    event_type VARCHAR(50),
    event_data JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Insert some sample data
INSERT INTO test_data.sample_users (email, username)
SELECT 
    'user' || i || '@example.com',
    'testuser' || i
FROM generate_series(1, 1000) i;

INSERT INTO test_data.sample_events (user_id, event_type, event_data)
SELECT 
    (random() * 999 + 1)::int,
    CASE (random() * 4)::int
        WHEN 0 THEN 'login'
        WHEN 1 THEN 'page_view'
        WHEN 2 THEN 'purchase'
        ELSE 'logout'
    END,
    jsonb_build_object(
        'timestamp', NOW() - (random() * INTERVAL '30 days'),
        'ip_address', '192.168.' || (random() * 255)::int || '.' || (random() * 255)::int,
        'user_agent', 'Mozilla/5.0 Test Browser'
    )
FROM generate_series(1, 10000) i;

-- Create indexes
CREATE INDEX idx_sample_users_email ON test_data.sample_users(email);
CREATE INDEX idx_sample_events_user_id ON test_data.sample_events(user_id);
CREATE INDEX idx_sample_events_created_at ON test_data.sample_events(created_at);
CREATE INDEX idx_sample_events_event_type ON test_data.sample_events(event_type);

-- Analyze tables
ANALYZE test_data.sample_users;
ANALYZE test_data.sample_events;

-- Create a function to generate test load
CREATE OR REPLACE FUNCTION test_data.generate_activity(duration_seconds INT DEFAULT 60)
RETURNS void AS $$
DECLARE
    end_time TIMESTAMP;
BEGIN
    end_time := NOW() + (duration_seconds || ' seconds')::INTERVAL;
    
    WHILE NOW() < end_time LOOP
        -- Random queries
        CASE (random() * 5)::int
            WHEN 0 THEN
                PERFORM COUNT(*) FROM test_data.sample_users;
            WHEN 1 THEN
                PERFORM * FROM test_data.sample_users WHERE id = (random() * 1000 + 1)::int;
            WHEN 2 THEN
                PERFORM u.*, COUNT(e.id) 
                FROM test_data.sample_users u
                LEFT JOIN test_data.sample_events e ON u.id = e.user_id
                GROUP BY u.id
                LIMIT 100;
            WHEN 3 THEN
                UPDATE test_data.sample_events 
                SET event_data = event_data || '{"processed": true}'
                WHERE id = (random() * 10000 + 1)::int;
            ELSE
                PERFORM pg_sleep(0.1);
        END CASE;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Create a blocking scenario function
CREATE OR REPLACE FUNCTION test_data.create_blocking_scenario()
RETURNS void AS $$
BEGIN
    -- This will be called by tests to create blocking chains
    RAISE NOTICE 'Blocking scenario function ready';
END;
$$ LANGUAGE plpgsql;

-- Output confirmation
SELECT 'Test database initialized successfully' as status;