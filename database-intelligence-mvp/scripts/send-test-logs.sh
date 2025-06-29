#!/bin/bash
# Send test logs to New Relic to verify integration

# Load environment
source "$(dirname "$0")/../.env"

# Send a test log entry via curl
curl -X POST "https://log-api.newrelic.com/log/v1" \
  -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  -H "Content-Type: application/json" \
  -d "[{
    \"message\": \"Database Intelligence Test Log\",
    \"attributes\": {
      \"instrumentation.provider\": \"opentelemetry\",
      \"collector.name\": \"database-intelligence\",
      \"database_name\": \"testdb\",
      \"entity.type\": \"DATABASE\",
      \"entity.guid\": \"DATABASE|docker-compose|testdb\",
      \"test\": \"true\",
      \"timestamp\": $(date +%s)000
    }
  }]"

echo "Test log sent to New Relic"