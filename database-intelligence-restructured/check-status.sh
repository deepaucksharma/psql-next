#!/bin/bash

# Check the status of Database Intelligence setup
set -euo pipefail

echo "=== Database Intelligence Status Check ==="
echo ""

# Check Docker
echo "1. Docker Status:"
if docker info >/dev/null 2>&1; then
    echo "   ✅ Docker is running"
    
    # Check PostgreSQL container
    if docker ps | grep -q "db-intel-postgres"; then
        echo "   ✅ PostgreSQL container is running"
    else
        echo "   ❌ PostgreSQL container is not running"
    fi
else
    echo "   ❌ Docker is not running"
fi

echo ""
echo "2. OpenTelemetry Collector:"
# Check collector health
if curl -s http://localhost:13133/health >/dev/null 2>&1; then
    echo "   ✅ Collector is running (health check passed)"
    
    # Get uptime
    UPTIME=$(curl -s http://localhost:13133/health | jq -r '.uptime' 2>/dev/null || echo "unknown")
    echo "   📊 Uptime: $UPTIME"
else
    echo "   ❌ Collector is not running"
fi

# Check collector metrics endpoint
if curl -s http://localhost:8888/metrics >/dev/null 2>&1; then
    echo "   ✅ Metrics endpoint is available"
    
    # Get some basic metrics
    RECEIVED=$(curl -s http://localhost:8888/metrics | grep "otelcol_receiver_accepted_metric_points" | tail -1 | awk '{print $2}' 2>/dev/null || echo "0")
    EXPORTED=$(curl -s http://localhost:8888/metrics | grep "otelcol_exporter_sent_metric_points" | tail -1 | awk '{print $2}' 2>/dev/null || echo "0")
    
    echo "   📊 Metrics received: ${RECEIVED:-0}"
    echo "   📊 Metrics exported: ${EXPORTED:-0}"
fi

echo ""
echo "3. New Relic Integration:"
echo "   🔗 Dashboard URL:"
echo "      https://one.newrelic.com/redirect/entity/MzYzMDA3MnxWSVp8REFTSEJPQVJEfGRhOjEwNDU1MDA5"
echo ""
echo "   📝 Account ID: ${NEW_RELIC_ACCOUNT_ID:-Not set}"
echo "   🔑 License Key: ${NEW_RELIC_LICENSE_KEY:0:10}..."

echo ""
echo "4. Data Generation:"
echo "   To send test data once:"
echo "      ./send-dashboard-data.sh"
echo ""
echo "   To send data continuously (every 30s):"
echo "      ./continuous-data-generator.sh"

echo ""
echo "=== Status Check Complete ==="