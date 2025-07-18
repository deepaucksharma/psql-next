-- Create monitoring user for OpenTelemetry collector
CREATE USER IF NOT EXISTS 'otel_monitor'@'%' IDENTIFIED BY 'otelmonitorpass';

-- Grant necessary privileges for monitoring
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON mysql.* TO 'otel_monitor'@'%';

-- If using sys schema (recommended for MySQL 5.7+)
GRANT SELECT ON sys.* TO 'otel_monitor'@'%';

-- Apply privileges
FLUSH PRIVILEGES;

-- Verify grants
SHOW GRANTS FOR 'otel_monitor'@'%';