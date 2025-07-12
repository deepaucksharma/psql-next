#!/bin/bash

# Continuously generate data for the Database Intelligence dashboard
set -euo pipefail

echo "=== Continuous Data Generator for Database Intelligence ==="
echo ""
echo "Sending metrics every 30 seconds. Press Ctrl+C to stop."
echo ""
echo "Dashboard URL: https://one.newrelic.com/redirect/entity/MzYzMDA3MnxWSVp8REFTSEJPQVJEfGRhOjEwNDU1MDA5"
echo ""

# Counter for iterations
ITERATION=0

while true; do
    ITERATION=$((ITERATION + 1))
    echo -n "[$(date '+%H:%M:%S')] Iteration $ITERATION: "
    
    # Call the dashboard data script silently
    if ./send-dashboard-data.sh > /dev/null 2>&1; then
        echo "✅ Data sent"
    else
        echo "❌ Failed to send data"
    fi
    
    # Wait 30 seconds before next iteration
    sleep 30
done