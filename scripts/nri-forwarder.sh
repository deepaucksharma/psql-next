#!/bin/bash
# NRI Forwarder - Captures stdout from collector and sends to New Relic

# Load environment
source .env

# Function to send metrics to New Relic
send_to_newrelic() {
    local json_data="$1"
    
    # Extract metrics array from NRI format
    local metrics=$(echo "$json_data" | jq -r '.data[0].entity.metrics')
    
    if [ "$metrics" != "null" ] && [ "$metrics" != "[]" ]; then
        echo "Sending metrics to New Relic Infrastructure API..."
        
        # Send to New Relic Infrastructure API
        curl -X POST https://infra-api.newrelic.com/integrations/v1 \
            -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
            -H "Content-Type: application/json" \
            -d "$json_data" \
            -w "\nHTTP Status: %{http_code}\n"
    fi
}

# Follow collector logs and process NRI output
echo "Starting NRI forwarder..."
echo "Capturing metrics from collector and forwarding to New Relic..."

docker logs -f postgres-collector 2>&1 | while read line; do
    # Check if line contains NRI JSON output
    if echo "$line" | grep -q '^{"name":"com.newrelic.postgresql"'; then
        echo "Found NRI metrics, forwarding to New Relic..."
        send_to_newrelic "$line"
    fi
done