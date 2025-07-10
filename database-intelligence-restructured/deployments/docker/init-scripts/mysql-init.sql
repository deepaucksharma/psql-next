-- MySQL initialization script for E2E tests

-- Create test database
CREATE DATABASE IF NOT EXISTS testdb;
USE testdb;

-- Create test tables
CREATE TABLE IF NOT EXISTS test_users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS test_orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    amount DECIMAL(10,2),
    status VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES test_users(id)
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
