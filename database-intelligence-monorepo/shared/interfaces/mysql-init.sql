-- Shared MySQL initialization script for all modules
-- Creates monitoring user and enables performance schema

-- Create monitoring user with necessary privileges
CREATE USER IF NOT EXISTS 'otel_monitor'@'%' IDENTIFIED BY 'otelmonitorpass';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON mysql.* TO 'otel_monitor'@'%';
GRANT SELECT ON sys.* TO 'otel_monitor'@'%';
FLUSH PRIVILEGES;

-- Ensure Performance Schema is properly configured
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES'
WHERE NAME LIKE '%statement/%' 
   OR NAME LIKE '%stage/%' 
   OR NAME LIKE '%wait/%'
   OR NAME LIKE '%lock/%';

UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES'
WHERE NAME LIKE '%events%' 
   OR NAME LIKE '%statements%' 
   OR NAME LIKE '%stages%'
   OR NAME LIKE '%waits%';

-- Set performance schema parameters
SET GLOBAL performance_schema_max_digest_length = 4096;
SET GLOBAL performance_schema_max_sql_text_length = 4096;

-- Create test database for demonstrations
CREATE DATABASE IF NOT EXISTS testdb;
USE testdb;

-- Simple test table
CREATE TABLE IF NOT EXISTS test_data (
    id INT AUTO_INCREMENT PRIMARY KEY,
    data VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_created (created_at)
) ENGINE=InnoDB;