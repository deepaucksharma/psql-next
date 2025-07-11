#!/bin/bash

# Send metrics that match our dashboard queries
set -euo pipefail

echo "=== Sending Dashboard-Compatible Metrics ==="
echo ""

# Function to generate current timestamp in nanoseconds
get_timestamp() {
    echo "$(date +%s)000000000"
}

# Function to send metrics
send_metrics() {
    local METRIC_DATA='
{
  "resourceMetrics": [{
    "resource": {
      "attributes": [{
        "key": "service.name",
        "value": {"stringValue": "database-intelligence"}
      }, {
        "key": "instrumentation.provider",
        "value": {"stringValue": "otel"}
      }]
    },
    "scopeMetrics": [{
      "scope": {
        "name": "database-intelligence"
      },
      "metrics": [
        {
          "name": "db.ash.active_sessions",
          "description": "Active database sessions by state",
          "unit": "1",
          "gauge": {
            "dataPoints": [
              {
                "asInt": "'$(( RANDOM % 20 + 10 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "state",
                  "value": {"stringValue": "active"}
                }]
              },
              {
                "asInt": "'$(( RANDOM % 10 + 5 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "state",
                  "value": {"stringValue": "idle"}
                }]
              },
              {
                "asInt": "'$(( RANDOM % 5 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "state",
                  "value": {"stringValue": "idle_in_transaction"}
                }]
              }
            ]
          }
        },
        {
          "name": "db.ash.blocked_sessions",
          "description": "Number of blocked sessions",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 5 ))'",
              "timeUnixNano": "'$(get_timestamp)'"
            }]
          }
        },
        {
          "name": "db.ash.long_running_queries",
          "description": "Number of long running queries",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 3 ))'",
              "timeUnixNano": "'$(get_timestamp)'"
            }]
          }
        },
        {
          "name": "db.ash.wait_events",
          "description": "Wait events by type",
          "unit": "1",
          "gauge": {
            "dataPoints": [
              {
                "asInt": "'$(( RANDOM % 100 + 50 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "wait_event",
                  "value": {"stringValue": "ClientRead"}
                }]
              },
              {
                "asInt": "'$(( RANDOM % 50 + 20 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "wait_event",
                  "value": {"stringValue": "DataFileRead"}
                }]
              },
              {
                "asInt": "'$(( RANDOM % 30 + 10 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "wait_event",
                  "value": {"stringValue": "LWLock"}
                }]
              }
            ]
          }
        },
        {
          "name": "kernel.syscall.count",
          "description": "System call counts",
          "unit": "1",
          "sum": {
            "dataPoints": [
              {
                "asInt": "'$(( RANDOM % 1000 + 500 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "syscall",
                  "value": {"stringValue": "read"}
                }]
              },
              {
                "asInt": "'$(( RANDOM % 800 + 400 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "syscall",
                  "value": {"stringValue": "write"}
                }]
              }
            ],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        },
        {
          "name": "kernel.file.read.bytes",
          "description": "File read bytes",
          "unit": "By",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 10000000 + 5000000 ))'",
              "timeUnixNano": "'$(get_timestamp)'"
            }],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        },
        {
          "name": "postgresql.backends",
          "description": "PostgreSQL backend connections",
          "unit": "1",
          "gauge": {
            "dataPoints": [
              {
                "asInt": "'$(( RANDOM % 10 + 5 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "db.postgresql.backend_type",
                  "value": {"stringValue": "client backend"}
                }]
              },
              {
                "asInt": "'$(( RANDOM % 5 + 2 ))'",
                "timeUnixNano": "'$(get_timestamp)'",
                "attributes": [{
                  "key": "db.postgresql.backend_type",
                  "value": {"stringValue": "background worker"}
                }]
              }
            ]
          }
        },
        {
          "name": "postgresql.commits",
          "description": "PostgreSQL commits",
          "unit": "1",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 1000 + 500 ))'",
              "timeUnixNano": "'$(get_timestamp)'"
            }],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        },
        {
          "name": "postgresql.rollbacks",
          "description": "PostgreSQL rollbacks",
          "unit": "1",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 10 + 5 ))'",
              "timeUnixNano": "'$(get_timestamp)'"
            }],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        },
        {
          "name": "postgresql.blocks.hit",
          "description": "PostgreSQL buffer cache hits",
          "unit": "1",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 100000 + 90000 ))'",
              "timeUnixNano": "'$(get_timestamp)'"
            }],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        },
        {
          "name": "postgresql.blocks.read",
          "description": "PostgreSQL blocks read from disk",
          "unit": "1",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 10000 + 5000 ))'",
              "timeUnixNano": "'$(get_timestamp)'"
            }],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        }
      ]
    }]
  }]
}'
    
    curl -X POST http://localhost:4318/v1/metrics \
        -H "Content-Type: application/json" \
        -d "$METRIC_DATA" \
        -s -o /dev/null -w "%{http_code}"
}

# Send metrics multiple times with a delay
echo "Sending metrics batch 1..."
if [[ "$(send_metrics)" == "200" ]]; then
    echo "✅ Batch 1 sent"
else
    echo "❌ Failed to send batch 1"
fi

sleep 2

echo "Sending metrics batch 2..."
if [[ "$(send_metrics)" == "200" ]]; then
    echo "✅ Batch 2 sent"
else
    echo "❌ Failed to send batch 2"
fi

sleep 2

echo "Sending metrics batch 3..."
if [[ "$(send_metrics)" == "200" ]]; then
    echo "✅ Batch 3 sent"
else
    echo "❌ Failed to send batch 3"
fi

echo ""
echo "=== Dashboard metrics sent! ==="
echo ""
echo "Visit your dashboard to see the data:"
echo "https://one.newrelic.com/redirect/entity/MzYzMDA3MnxWSVp8REFTSEJPQVJEfGRhOjEwNDU1MDA5"
echo ""
echo "Note: It may take 1-2 minutes for data to appear in New Relic"