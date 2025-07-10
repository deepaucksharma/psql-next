-- PostgreSQL initialization script for E2E tests

-- First connect to default postgres database
\c postgres;

-- Drop and recreate test database
DROP DATABASE IF EXISTS testdb;
CREATE DATABASE testdb;

-- Connect to test database
\c testdb;

CREATE TABLE IF NOT EXISTS test_users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS test_orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES test_users(id),
    amount DECIMAL(10,2),
    status VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert test data
INSERT INTO test_users (username, email) VALUES
    ('user1', 'user1@example.com'),
    ('user2', 'user2@example.com'),
    ('user3', 'user3@example.com');

INSERT INTO test_orders (user_id, amount, status) VALUES
    (1, 99.99, 'completed'),
    (1, 149.50, 'pending'),
    (2, 75.00, 'completed'),
    (3, 200.00, 'cancelled');

-- Enable pg_stat_statements if available
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
