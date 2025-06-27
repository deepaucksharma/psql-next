#!/bin/bash
# Test New Relic API connectivity

source .env

echo "Testing New Relic API connectivity..."
echo "Using License Key: ${NEW_RELIC_LICENSE_KEY:0:10}..."

# Test OTLP endpoint
echo -e "\n1. Testing OTLP endpoint (gRPC):"
curl -v -X POST https://otlp.nr-data.net:4317/v1/metrics \
  -H "api-key: $NEW_RELIC_LICENSE_KEY" \
  -H "Content-Type: application/x-protobuf" \
  --data-binary "" \
  --max-time 10 2>&1 | grep -E "(HTTP|< )"

# Test metric API endpoint  
echo -e "\n2. Testing Metric API endpoint:"
curl -X POST https://metric-api.newrelic.com/metric/v1 \
  -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  -H "Content-Type: application/json" \
  -d '[{
    "metrics": [{
      "name": "test.metric",
      "type": "gauge",
      "value": 1.0,
      "timestamp": '$(date +%s)',
      "attributes": {
        "service": "postgres-collector-test"
      }
    }]
  }]' 2>&1

echo -e "\n3. Testing Infrastructure API:"
curl -X POST https://infra-api.newrelic.com/infra/v2/metrics \
  -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "com.newrelic.test",
    "protocol_version": "4",
    "integration_version": "1.0.0",
    "data": [{
      "entity": {
        "name": "test-entity",
        "type": "test"
      }
    }]
  }' 2>&1