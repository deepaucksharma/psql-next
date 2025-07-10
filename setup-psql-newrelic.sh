#!/bin/bash

# PostgreSQL setup script with New Relic monitoring
# License Key: ea7e83e4e29597b0766cf6c4636fba20FFFFNRAL

set -e

# Configuration
POSTGRES_VERSION="15"
NEW_RELIC_LICENSE_KEY="ea7e83e4e29597b0766cf6c4636fba20FFFFNRAL"
NEW_RELIC_OTLP_ENDPOINT="otlp.nr-data.net:4317"
ENVIRONMENT="development"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Setting up PostgreSQL with New Relic monitoring...${NC}"

# Create .env file for docker-compose
cat > .env << EOF
# PostgreSQL Configuration
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=devpassword123
POSTGRES_DB=testdb
POSTGRES_VERSION=${POSTGRES_VERSION}

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY}
NEW_RELIC_OTLP_ENDPOINT=${NEW_RELIC_OTLP_ENDPOINT}
ENVIRONMENT=${ENVIRONMENT}

# Collection Settings
COLLECTION_INTERVAL=30s
BATCH_SIZE=1000
BATCH_TIMEOUT=10s
LOG_LEVEL=info
DEBUG_VERBOSITY=basic

# Service Information
SERVICE_VERSION=1.0.0
DEPLOYMENT_TYPE=docker
AWS_REGION=us-east-1

# TLS Settings
POSTGRES_TLS_INSECURE=true
EOF

echo -e "${GREEN}Created .env file with configuration${NC}"

# Create PostgreSQL initialization script
mkdir -p scripts
cat > scripts/init-postgres.sql << 'EOF'
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
EOF

echo -e "${GREEN}Created PostgreSQL initialization script${NC}"

# Create docker-compose file specifically for PostgreSQL and New Relic
cat > docker-compose.psql-newrelic.yml << 'EOF'
version: '3.8'

services:
  # PostgreSQL Database with monitoring enabled
  postgres:
    image: postgres:${POSTGRES_VERSION:-15}-alpine
    container_name: dbintel-postgres-nr
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
      # Enable statement tracking
      POSTGRES_INITDB_ARGS: "-c shared_preload_libraries=pg_stat_statements"
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init-postgres.sql:/docker-entrypoint-initdb.d/01-init.sql:ro
      - ./logs/postgres:/var/log/postgresql
    command: 
      - "postgres"
      - "-c"
      - "shared_preload_libraries=pg_stat_statements"
      - "-c"
      - "pg_stat_statements.track=all"
      - "-c"
      - "logging_collector=on"
      - "-c"
      - "log_directory=/var/log/postgresql"
      - "-c"
      - "log_filename=postgresql-%Y-%m-%d_%H%M%S.log"
      - "-c"
      - "log_rotation_age=1d"
      - "-c"
      - "log_rotation_size=100MB"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - dbintel-network

  # OpenTelemetry Collector configured for New Relic
  otel-collector:
    build:
      context: ./database-intelligence-mvp
      dockerfile: Dockerfile
    container_name: dbintel-collector-nr
    command: ["--config=/etc/otel/config.yaml"]
    volumes:
      - ./config/collector-newrelic.yaml:/etc/otel/config.yaml:ro
      - ./logs/collector:/var/log/otel
    environment:
      # PostgreSQL connection
      POSTGRES_HOST: postgres
      POSTGRES_PORT: 5432
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DATABASE: ${POSTGRES_DB}
      POSTGRES_TLS_INSECURE: "true"
      
      # New Relic configuration
      NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
      NEW_RELIC_OTLP_ENDPOINT: ${NEW_RELIC_OTLP_ENDPOINT}
      
      # Environment settings
      ENVIRONMENT: ${ENVIRONMENT}
      SERVICE_VERSION: ${SERVICE_VERSION}
      DEPLOYMENT_TYPE: ${DEPLOYMENT_TYPE}
      AWS_REGION: ${AWS_REGION}
      
      # Collection settings
      COLLECTION_INTERVAL: ${COLLECTION_INTERVAL}
      BATCH_SIZE: ${BATCH_SIZE}
      BATCH_TIMEOUT: ${BATCH_TIMEOUT}
      LOG_LEVEL: ${LOG_LEVEL}
      DEBUG_VERBOSITY: ${DEBUG_VERBOSITY}
      
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
      - "8888:8888"   # Prometheus metrics
      - "13133:13133" # Health check
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:13133/"]
      interval: 30s
      timeout: 10s
      retries: 3
    networks:
      - dbintel-network

  # Load generator for testing
  load-generator:
    image: postgres:${POSTGRES_VERSION:-15}-alpine
    container_name: dbintel-load-generator
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      PGPASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - ./scripts/generate-load.sql:/generate-load.sql:ro
    command: >
      sh -c "
        echo 'Waiting for PostgreSQL to be ready...' &&
        sleep 10 &&
        echo 'Starting load generation...' &&
        while true; do
          psql -h postgres -U ${POSTGRES_USER} -d ${POSTGRES_DB} -f /generate-load.sql || true
          sleep 5
        done
      "
    networks:
      - dbintel-network

volumes:
  postgres_data:
  
networks:
  dbintel-network:
    driver: bridge
EOF

echo -e "${GREEN}Created docker-compose.psql-newrelic.yml${NC}"

# Create New Relic specific collector configuration
cat > config/collector-newrelic.yaml << 'EOF'
receivers:
  # PostgreSQL receiver for metrics
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    transport: tcp
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DATABASE}
    collection_interval: ${env:COLLECTION_INTERVAL}
    tls:
      insecure: true
      insecure_skip_verify: true

  # PostgreSQL detailed query stats
  sqlquery/pg_stats:
    driver: postgres
    datasource: "host=${env:POSTGRES_HOST} port=${env:POSTGRES_PORT} user=${env:POSTGRES_USER} password=${env:POSTGRES_PASSWORD} dbname=${env:POSTGRES_DATABASE} sslmode=disable"
    collection_interval: 60s
    queries:
      - sql: |
          SELECT
            queryid::text as query_id,
            LEFT(query, 100) as query_text,
            calls,
            total_exec_time::float8 as total_time_ms,
            mean_exec_time::float8 as mean_time_ms,
            max_exec_time::float8 as max_time_ms,
            rows,
            shared_blks_hit + shared_blks_read as total_blocks
          FROM pg_stat_statements
          WHERE query NOT LIKE '%pg_stat%'
          ORDER BY total_exec_time DESC
          LIMIT 50
        metrics:
          - metric_name: postgresql.query.calls
            value_column: calls
            value_type: int
            attributes:
              - query_id
              - query_text
          - metric_name: postgresql.query.total_time
            value_column: total_time_ms
            value_type: double
            unit: ms
            attributes:
              - query_id
              - query_text

processors:
  # Memory limiter
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128

  # Add New Relic required attributes
  attributes:
    actions:
      - key: db.system
        value: postgresql
        action: insert
      - key: service.name
        value: postgresql-monitoring
        action: insert
      - key: environment
        value: ${env:ENVIRONMENT}
        action: insert

  # Resource processor for New Relic
  resource:
    attributes:
      - key: service.instance.id
        from_attribute: host.name
        action: insert
      - key: telemetry.sdk.name
        value: opentelemetry
        action: insert
      - key: telemetry.sdk.language
        value: go
        action: insert
      - key: telemetry.sdk.version
        value: 1.19.0
        action: insert

  # Batch processor
  batch:
    send_batch_size: 1000
    timeout: 10s

exporters:
  # New Relic OTLP exporter
  otlp/newrelic:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

  # Debug exporter for troubleshooting
  debug:
    verbosity: ${env:DEBUG_VERBOSITY}
    sampling_initial: 5
    sampling_thereafter: 20

  # File exporter for backup
  file:
    path: /var/log/otel/metrics.json
    format: json

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    
  zpages:
    endpoint: 0.0.0.0:55679

service:
  extensions: [health_check, zpages]
  
  pipelines:
    metrics:
      receivers: [postgresql, sqlquery/pg_stats]
      processors: [memory_limiter, attributes, resource, batch]
      exporters: [otlp/newrelic, debug, file]
      
  telemetry:
    logs:
      level: ${env:LOG_LEVEL}
      encoding: json
      output_paths: ["stdout", "/var/log/otel/collector.log"]
    metrics:
      level: detailed
      address: 0.0.0.0:8888
EOF

echo -e "${GREEN}Created New Relic collector configuration${NC}"

# Create load generation script
cat > scripts/generate-load.sql << 'EOF'
-- Load generation script for PostgreSQL monitoring test

-- Random user activity
INSERT INTO users (username, email) 
SELECT 
    'user_' || generate_series,
    'user_' || generate_series || '@example.com'
FROM generate_series(1, 10)
ON CONFLICT (email) DO NOTHING;

-- Random product queries
SELECT * FROM products WHERE price > random() * 1000 LIMIT 10;
SELECT * FROM products WHERE inventory_count < random() * 100 ORDER BY price DESC;

-- Random order creation
INSERT INTO orders (user_id, total, status)
SELECT 
    (random() * 3 + 1)::int,
    random() * 1000,
    CASE WHEN random() > 0.5 THEN 'completed' ELSE 'pending' END
FROM generate_series(1, 5);

-- Complex queries for monitoring
WITH user_orders AS (
    SELECT u.username, COUNT(o.id) as order_count, SUM(o.total) as total_spent
    FROM users u
    LEFT JOIN orders o ON u.id = o.user_id
    GROUP BY u.username
)
SELECT * FROM user_orders WHERE total_spent > 100;

-- Update operations
UPDATE products 
SET inventory_count = inventory_count - 1 
WHERE id = (random() * 3 + 1)::int AND inventory_count > 0;

-- Analytics queries
SELECT 
    DATE_TRUNC('hour', created_at) as hour,
    COUNT(*) as order_count,
    AVG(total) as avg_order_value
FROM orders
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour DESC;

-- Force some slow queries
SELECT pg_sleep(random() * 0.5);
EOF

echo -e "${GREEN}Created load generation script${NC}"

# Create directories for logs
mkdir -p logs/postgres logs/collector

# Create start script
cat > start-psql-newrelic.sh << 'EOF'
#!/bin/bash

echo "Starting PostgreSQL with New Relic monitoring..."

# Start the services
docker-compose -f docker-compose.psql-newrelic.yml up -d

# Wait for services to be ready
echo "Waiting for services to start..."
sleep 30

# Check service status
echo -e "\nService Status:"
docker-compose -f docker-compose.psql-newrelic.yml ps

# Show collector logs
echo -e "\nCollector Logs (last 20 lines):"
docker-compose -f docker-compose.psql-newrelic.yml logs --tail=20 otel-collector

echo -e "\nSetup complete! Services are running."
echo "PostgreSQL is available at: localhost:5432"
echo "OTEL Collector metrics at: http://localhost:8888/metrics"
echo "Health check at: http://localhost:13133/"
echo -e "\nCheck New Relic One for incoming data:"
echo "https://one.newrelic.com/"
EOF

chmod +x start-psql-newrelic.sh

# Create stop script
cat > stop-psql-newrelic.sh << 'EOF'
#!/bin/bash

echo "Stopping PostgreSQL and New Relic monitoring..."
docker-compose -f docker-compose.psql-newrelic.yml down -v
echo "Services stopped."
EOF

chmod +x stop-psql-newrelic.sh

# Create validation script
cat > validate-newrelic.sh << 'EOF'
#!/bin/bash

echo "Validating New Relic integration..."

# Check if collector is running
if curl -s http://localhost:13133/ > /dev/null; then
    echo "✓ OTEL Collector is healthy"
else
    echo "✗ OTEL Collector is not responding"
fi

# Check PostgreSQL connection
if docker exec dbintel-postgres-nr pg_isready -U postgres > /dev/null 2>&1; then
    echo "✓ PostgreSQL is ready"
else
    echo "✗ PostgreSQL is not ready"
fi

# Check metrics endpoint
if curl -s http://localhost:8888/metrics | grep -q "postgresql"; then
    echo "✓ PostgreSQL metrics are being collected"
else
    echo "✗ PostgreSQL metrics not found"
fi

echo -e "\nTo verify data in New Relic:"
echo "1. Go to https://one.newrelic.com/"
echo "2. Navigate to 'All Entities' > 'OpenTelemetry'"
echo "3. Look for service 'postgresql-monitoring'"
echo "4. Check the 'Metrics' tab for PostgreSQL data"
EOF

chmod +x validate-newrelic.sh

echo -e "${GREEN}Setup complete!${NC}"
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Start the services: ./start-psql-newrelic.sh"
echo "2. Validate the setup: ./validate-newrelic.sh"
echo "3. Check New Relic One for incoming data"
echo "4. Stop services when done: ./stop-psql-newrelic.sh"
echo -e "\n${YELLOW}Your New Relic License Key:${NC} ${NEW_RELIC_LICENSE_KEY}"