-- PostgreSQL initialization for Database Intelligence monitoring

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS pg_stat_activity;

-- Configure pg_stat_statements
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET pg_stat_statements.track = 'all';
ALTER SYSTEM SET pg_stat_statements.max = 10000;
ALTER SYSTEM SET pg_stat_statements.track_utility = 'on';
ALTER SYSTEM SET pg_stat_statements.save = 'on';

-- Enable detailed logging for monitoring
ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET log_duration = 'on';
ALTER SYSTEM SET log_min_duration_statement = 0;
ALTER SYSTEM SET log_line_prefix = '%t [%p] %q%u@%d ';
ALTER SYSTEM SET log_checkpoints = 'on';
ALTER SYSTEM SET log_connections = 'on';
ALTER SYSTEM SET log_disconnections = 'on';
ALTER SYSTEM SET log_lock_waits = 'on';
ALTER SYSTEM SET log_temp_files = 0;

-- Create monitoring user
CREATE USER monitoring WITH PASSWORD 'monitoring123';
GRANT pg_read_all_stats TO monitoring;
GRANT pg_monitor TO monitoring;
GRANT CONNECT ON DATABASE testdb TO monitoring;

-- Create sample tables for testing
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    inventory_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    total DECIMAL(10,2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance testing
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_products_name ON products(name);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);

-- Insert sample data
INSERT INTO users (username, email) VALUES 
    ('john_doe', 'john@example.com'),
    ('jane_smith', 'jane@example.com'),
    ('bob_johnson', 'bob@example.com');

INSERT INTO products (name, price, inventory_count) VALUES 
    ('Laptop', 999.99, 50),
    ('Mouse', 29.99, 200),
    ('Keyboard', 79.99, 150);

-- Grant permissions to monitoring user on sample tables
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO monitoring;

-- Reset pg_stat_statements for clean monitoring start
SELECT pg_stat_statements_reset();
