-- MySQL E2E test initialization script

-- Create test database and tables
USE e2e_test;

-- Users table with PII data for testing sanitization
CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    ssn VARCHAR(11),
    phone VARCHAR(20),
    credit_card VARCHAR(20),
    name VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Orders table for testing query correlation
CREATE TABLE orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10,2),
    status VARCHAR(50),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Large table for testing expensive queries
CREATE TABLE events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    event_type VARCHAR(50),
    event_data JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for plan testing
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_orders_user_date ON orders(user_id, order_date);
CREATE INDEX idx_events_type_date ON events(event_type, created_at);

-- Insert test data with PII
INSERT INTO users (email, ssn, phone, credit_card, name) VALUES
    ('john.doe@example.com', '123-45-6789', '555-123-4567', '4111-1111-1111-1111', 'John Doe'),
    ('jane.smith@example.com', '987-65-4321', '555-987-6543', '5500-0000-0000-0004', 'Jane Smith'),
    ('test@example.com', '456-78-9012', '555-456-7890', '3400-0000-0000-009', 'Test User');

-- Insert orders
DELIMITER //
CREATE PROCEDURE insert_test_orders()
BEGIN
    DECLARE i INT DEFAULT 1;
    WHILE i <= 100 DO
        INSERT INTO orders (user_id, total_amount, status) VALUES (
            FLOOR(1 + RAND() * 3),
            ROUND(RAND() * 1000, 2),
            ELT(FLOOR(1 + RAND() * 3), 'pending', 'completed', 'cancelled')
        );
        SET i = i + 1;
    END WHILE;
END //
DELIMITER ;

CALL insert_test_orders();

-- Insert many events for cardinality testing
DELIMITER //
CREATE PROCEDURE insert_test_events()
BEGIN
    DECLARE i INT DEFAULT 1;
    WHILE i <= 10000 DO
        INSERT INTO events (event_type, event_data) VALUES (
            CONCAT('event_type_', FLOOR(RAND() * 10)),
            JSON_OBJECT(
                'value', ROUND(RAND() * 1000, 2),
                'timestamp', DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 30) DAY)
            )
        );
        SET i = i + 1;
    END WHILE;
END //
DELIMITER ;

CALL insert_test_events();

-- Create stored procedures for generating different query patterns
DELIMITER //
CREATE PROCEDURE generate_simple_query()
BEGIN
    SELECT id, email FROM users;
END //

CREATE PROCEDURE generate_join_query()
BEGIN
    SELECT 
        u.name,
        COUNT(o.id) as order_count,
        COALESCE(SUM(o.total_amount), 0) as total_spent
    FROM users u
    LEFT JOIN orders o ON u.id = o.user_id
    GROUP BY u.name;
END //

CREATE PROCEDURE generate_expensive_query()
BEGIN
    SELECT 
        e.event_type,
        COUNT(*) as event_count,
        AVG(JSON_EXTRACT(e.event_data, '$.value')) as avg_value
    FROM events e
    WHERE e.created_at > DATE_SUB(NOW(), INTERVAL 7 DAY)
    GROUP BY e.event_type
    ORDER BY event_count DESC;
END //
DELIMITER ;

-- Performance schema configuration (skip if already enabled)
-- Note: performance_schema is typically enabled by default in MySQL 8.0
-- SET GLOBAL performance_schema_events_statements_history_long_size = 10000;

-- Create a view for testing complex queries
CREATE VIEW user_order_summary AS
SELECT 
    u.id,
    u.name,
    u.email,
    COUNT(DISTINCT o.id) as total_orders,
    SUM(o.total_amount) as total_spent,
    MAX(o.order_date) as last_order_date
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name, u.email;