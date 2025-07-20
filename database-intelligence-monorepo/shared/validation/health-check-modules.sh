#!/bin/bash

# Health check script for all modules
MODULES=(core-metrics sql-intelligence wait-profiler anomaly-detector business-impact replication-monitor performance-advisor resource-monitor)
PORTS=(8081 8082 8083 8084 8085 8086 8087 8088)

echo "Checking health of all modules..."
echo "================================"

for i in "${!MODULES[@]}"; do
    MODULE="${MODULES[$i]}"
    PORT="${PORTS[$i]}"
    
    echo -n "Checking $MODULE (port $PORT)... "
    
    if curl -f -s http://localhost:$PORT/metrics > /dev/null 2>&1; then
        echo "✓ Running"
    else
        echo "✗ Not running or unhealthy"
    fi
done

echo "================================"