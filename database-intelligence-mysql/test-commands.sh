#!/bin/bash
# Test commands for MySQL Wait-Based Monitoring

echo "=== Starting services ==="
docker-compose up -d

echo "=== Waiting for services to be ready ==="
sleep 30

echo "=== Testing MySQL connection ==="
docker exec mysql-primary mysql -u root -p${MYSQL_ROOT_PASSWORD} -e "SHOW DATABASES;"

echo "=== Checking Performance Schema ==="
docker exec mysql-primary mysql -u root -p${MYSQL_ROOT_PASSWORD} -e "
SELECT * FROM performance_schema.setup_instruments 
WHERE NAME LIKE 'wait/%' AND ENABLED = 'NO' LIMIT 10;"

echo "=== Testing collector health ==="
curl -s http://localhost:13133/health | jq .

echo "=== Checking metrics ==="
curl -s http://localhost:8888/metrics | grep mysql_query

echo "=== Running load test ==="
docker exec mysql-primary mysql -u root -p${MYSQL_ROOT_PASSWORD} production -e "
CALL generate_workload(100, 'mixed');"

echo "=== Monitoring waits ==="
./scripts/monitor-waits.sh waits

echo "=== Checking advisories ==="
curl -s http://localhost:9091/metrics | grep advisor_type
