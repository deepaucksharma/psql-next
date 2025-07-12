#!/bin/bash

# Send test metrics to the OpenTelemetry collector
set -euo pipefail

echo "=== Sending Test Data to OpenTelemetry Collector ==="
echo ""

# Check if collector is running
if ! curl -s http://localhost:13133/health >/dev/null 2>&1; then
    echo "❌ Collector is not running on localhost:13133"
    echo "Please start the collector first with: ./start-collector.sh"
    exit 1
fi

echo "✅ Collector is running"
echo ""

# Send test metric using curl to OTLP HTTP endpoint
echo "Sending test metric via OTLP HTTP..."

# Create a test metric in OTLP JSON format
METRIC_DATA='{
  "resourceMetrics": [{
    "resource": {
      "attributes": [{
        "key": "service.name",
        "value": {"stringValue": "database-intelligence-test"}
      }, {
        "key": "instrumentation.provider",
        "value": {"stringValue": "otel"}
      }]
    },
    "scopeMetrics": [{
      "scope": {
        "name": "database-intelligence-test"
      },
      "metrics": [{
        "name": "test.database.connections",
        "description": "Test database connections",
        "unit": "1",
        "gauge": {
          "dataPoints": [{
            "asInt": "42",
            "timeUnixNano": "'$(date +%s)000000000'",
            "attributes": [{
              "key": "database.name",
              "value": {"stringValue": "testdb"}
            }, {
              "key": "database.type",
              "value": {"stringValue": "postgresql"}
            }]
          }]
        }
      }, {
        "name": "db.ash.active_sessions",
        "description": "Active database sessions",
        "unit": "1",
        "gauge": {
          "dataPoints": [{
            "asInt": "15",
            "timeUnixNano": "'$(date +%s)000000000'",
            "attributes": [{
              "key": "state",
              "value": {"stringValue": "active"}
            }]
          }, {
            "asInt": "3",
            "timeUnixNano": "'$(date +%s)000000000'",
            "attributes": [{
              "key": "state",
              "value": {"stringValue": "idle"}
            }]
          }]
        }
      }, {
        "name": "db.ash.blocked_sessions",
        "description": "Blocked database sessions",
        "unit": "1",
        "gauge": {
          "dataPoints": [{
            "asInt": "2",
            "timeUnixNano": "'$(date +%s)000000000'"
          }]
        }
      }, {
        "name": "db.ash.long_running_queries",
        "description": "Number of long running queries",
        "unit": "1",
        "gauge": {
          "dataPoints": [{
            "asInt": "1",
            "timeUnixNano": "'$(date +%s)000000000'"
          }]
        }
      }]
    }]
  }]
}'

# Send the metric
if curl -X POST http://localhost:4318/v1/metrics \
    -H "Content-Type: application/json" \
    -d "$METRIC_DATA" \
    -s -o /dev/null -w "%{http_code}" | grep -q "200"; then
    echo "✅ Test metrics sent successfully"
else
    echo "❌ Failed to send test metrics"
    exit 1
fi

# Send test log
echo ""
echo "Sending test log via OTLP HTTP..."

LOG_DATA='{
  "resourceLogs": [{
    "resource": {
      "attributes": [{
        "key": "service.name",
        "value": {"stringValue": "database-intelligence-test"}
      }, {
        "key": "instrumentation.provider",
        "value": {"stringValue": "otel"}
      }]
    },
    "scopeLogs": [{
      "scope": {
        "name": "database-intelligence-test"
      },
      "logRecords": [{
        "timeUnixNano": "'$(date +%s)000000000'",
        "severityNumber": 9,
        "severityText": "INFO",
        "body": {
          "stringValue": "Database Intelligence collector test log"
        },
        "attributes": [{
          "key": "component",
          "value": {"stringValue": "test-script"}
        }]
      }]
    }]
  }]
}'

if curl -X POST http://localhost:4318/v1/logs \
    -H "Content-Type: application/json" \
    -d "$LOG_DATA" \
    -s -o /dev/null -w "%{http_code}" | grep -q "200"; then
    echo "✅ Test log sent successfully"
else
    echo "❌ Failed to send test log"
fi

echo ""
echo "=== Test data sent! ==="
echo ""
echo "Check your New Relic dashboard in a few minutes:"
echo "https://one.newrelic.com/redirect/entity/MzYzMDA3MnxWSVp8REFTSEJPQVJEfGRhOjEwNDU1MDA5"
echo ""
echo "You can also check the collector metrics at:"
echo "http://localhost:8888/metrics"