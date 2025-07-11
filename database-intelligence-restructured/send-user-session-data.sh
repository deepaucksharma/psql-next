#!/bin/bash

# Send user and session-focused metrics to match the new dashboard
set -euo pipefail

echo "=== Sending User & Session Metrics ==="
echo ""

# Function to generate current timestamp in nanoseconds
get_timestamp() {
    echo "$(date +%s)000000000"
}

# Array of sample users
USERS=("john_doe" "jane_smith" "bob_johnson" "alice_williams" "charlie_brown" "david_miller" "emma_davis" "frank_wilson")
DATABASES=("production" "analytics" "reporting" "development")
USER_GROUPS=("developers" "analysts" "administrators" "applications")
QUERY_TYPES=("SELECT" "INSERT" "UPDATE" "DELETE" "DDL")
WAIT_EVENTS=("ClientRead" "DataFileRead" "LWLock" "BufferMapping" "IO" "CPU")
SESSION_STATES=("active" "idle" "idle_in_transaction" "idle_in_transaction_aborted")
TERMINATION_REASONS=("normal" "timeout" "admin_command" "error" "crash")

# Function to get random element from array
get_random() {
    local array=("$@")
    echo "${array[$RANDOM % ${#array[@]}]}"
}

# Function to send metrics
send_metrics() {
    local USER=$(get_random "${USERS[@]}")
    local DATABASE=$(get_random "${DATABASES[@]}")
    local USER_GROUP=$(get_random "${USER_GROUPS[@]}")
    local QUERY_TYPE=$(get_random "${QUERY_TYPES[@]}")
    local WAIT_EVENT=$(get_random "${WAIT_EVENTS[@]}")
    local SESSION_STATE=$(get_random "${SESSION_STATES[@]}")
    local SESSION_ID="sess_$(date +%s)_${RANDOM}"
    
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
        "name": "user-session-metrics"
      },
      "metrics": [
        {
          "name": "db.ash.active_sessions",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 10 + 1 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "state", "value": {"stringValue": "'$SESSION_STATE'"}},
                {"key": "database_name", "value": {"stringValue": "'$DATABASE'"}}
              ]
            }]
          }
        },
        {
          "name": "user_id",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 1000 + 1 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "database_name", "value": {"stringValue": "'$DATABASE'"}}
              ]
            }]
          }
        },
        {
          "name": "session.duration.seconds",
          "unit": "s",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 600 + 30 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "query.execution_time_ms",
          "unit": "ms",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 5000 + 10 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "query_type", "value": {"stringValue": "'$QUERY_TYPE'"}}
              ]
            }]
          }
        },
        {
          "name": "user.query.count",
          "unit": "1",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 100 + 10 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "query_type", "value": {"stringValue": "'$QUERY_TYPE'"}}
              ]
            }],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        },
        {
          "name": "wait_time_ms",
          "unit": "ms",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 500 + 10 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "wait_event", "value": {"stringValue": "'$WAIT_EVENT'"}}
              ]
            }]
          }
        },
        {
          "name": "session.cpu_usage_percent",
          "unit": "%",
          "gauge": {
            "dataPoints": [{
              "asDouble": "'$(( RANDOM % 90 + 10 ))'.0",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}},
                {"key": "user_group", "value": {"stringValue": "'$USER_GROUP'"}}
              ]
            }]
          }
        },
        {
          "name": "session.memory_mb",
          "unit": "MB",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 500 + 50 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}},
                {"key": "user_group", "value": {"stringValue": "'$USER_GROUP'"}}
              ]
            }]
          }
        },
        {
          "name": "session.io_read_mb",
          "unit": "MB",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 100 + 10 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "user.transaction.commits",
          "unit": "1",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 50 + 10 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}}
              ]
            }],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        },
        {
          "name": "user.transaction.rollbacks",
          "unit": "1",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 5 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}}
              ]
            }],
            "aggregationTemporality": 2,
            "isMonotonic": true
          }
        },
        {
          "name": "user.lock.wait_time_ms",
          "unit": "ms",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 2000 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}}
              ]
            }]
          }
        },
        {
          "name": "user.connection.pool.active",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 20 + 1 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}}
              ]
            }]
          }
        },
        {
          "name": "user.connection.pool.idle",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 10 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}}
              ]
            }]
          }
        },
        {
          "name": "user.total_wait_time_ms",
          "unit": "ms",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 10000 + 1000 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}}
              ]
            }]
          }
        },
        {
          "name": "session.is_blocked",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 100 < 10 ? 1 : 0 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "session.health",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "1",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}},
                {"key": "health", "value": {"stringValue": "'$(get_random "healthy" "degraded" "critical")'"}}
              ]
            }]
          }
        },
        {
          "name": "user.query.queue_depth",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 20 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}}
              ]
            }]
          }
        },
        {
          "name": "session.recovery_time_ms",
          "unit": "ms",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 5000 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "user.is_privileged",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(if [[ "$USER_GROUP" == "administrators" ]]; then echo 1; else echo 0; fi)'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "user_group", "value": {"stringValue": "'$USER_GROUP'"}}
              ]
            }]
          }
        },
        {
          "name": "session.queries_per_second",
          "unit": "1/s",
          "gauge": {
            "dataPoints": [{
              "asDouble": "'$(( RANDOM % 20 + 1 ))'.0",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "session.failed_queries",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 10 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "session.termination_reason",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "1",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "termination_reason", "value": {"stringValue": "'$(get_random "${TERMINATION_REASONS[@]}")'"}}
              ]
            }]
          }
        },
        {
          "name": "compliance.status",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "1",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "status", "value": {"stringValue": "'$(get_random "compliant" "non_compliant" "pending_review")'"}}
              ]
            }]
          }
        },
        {
          "name": "session.encryption_enabled",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 100 < 80 ? 1 : 0 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "session.estimated_cost_usd",
          "unit": "USD",
          "gauge": {
            "dataPoints": [{
              "asDouble": "'$(awk -v min=0.01 -v max=5.00 "BEGIN{print min+rand()*(max-min)}")'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "query.estimated_cost_usd",
          "unit": "USD",
          "gauge": {
            "dataPoints": [{
              "asDouble": "'$(awk -v min=0.001 -v max=0.5 "BEGIN{print min+rand()*(max-min)}")'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "query_type", "value": {"stringValue": "'$QUERY_TYPE'"}}
              ]
            }]
          }
        },
        {
          "name": "session.cpu_seconds",
          "unit": "s",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 1000 + 100 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "session.memory_gb_hours",
          "unit": "GB*h",
          "gauge": {
            "dataPoints": [{
              "asDouble": "'$(awk -v min=0.1 -v max=2.0 "BEGIN{print min+rand()*(max-min)}")'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "session.io_gb",
          "unit": "GB",
          "gauge": {
            "dataPoints": [{
              "asDouble": "'$(awk -v min=0.1 -v max=10.0 "BEGIN{print min+rand()*(max-min)}")'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "session_id", "value": {"stringValue": "'$SESSION_ID'"}}
              ]
            }]
          }
        },
        {
          "name": "user.work_units",
          "unit": "1",
          "gauge": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 10000 + 1000 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}}
              ]
            }]
          }
        },
        {
          "name": "user.data.rows_read",
          "unit": "1",
          "sum": {
            "dataPoints": [{
              "asInt": "'$(( RANDOM % 100000 + 10000 ))'",
              "timeUnixNano": "'$(get_timestamp)'",
              "attributes": [
                {"key": "user_name", "value": {"stringValue": "'$USER'"}},
                {"key": "table_name", "value": {"stringValue": "table_'$(( RANDOM % 5 + 1 ))'"}}
              ]
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

# Send metrics for multiple users
echo "Sending user session metrics..."
for i in {1..5}; do
    if [[ "$(send_metrics)" == "200" ]]; then
        echo "✅ Batch $i sent"
    else
        echo "❌ Failed to send batch $i"
    fi
    sleep 1
done

# Send some log data for security/audit trail
send_logs() {
    local USER=$(get_random "${USERS[@]}")
    local LOG_DATA='
{
  "resourceLogs": [{
    "resource": {
      "attributes": [{
        "key": "service.name",
        "value": {"stringValue": "database-intelligence"}
      }, {
        "key": "instrumentation.provider",
        "value": {"stringValue": "otel"}
      }]
    },
    "scopeLogs": [{
      "scope": {
        "name": "user-activity-logs"
      },
      "logRecords": [
        {
          "timeUnixNano": "'$(get_timestamp)'",
          "severityNumber": 9,
          "severityText": "INFO",
          "body": {
            "stringValue": "User authentication successful"
          },
          "attributes": [
            {"key": "user_name", "value": {"stringValue": "'$USER'"}},
            {"key": "audit", "value": {"boolValue": true}},
            {"key": "activity_type", "value": {"stringValue": "login"}},
            {"key": "result", "value": {"stringValue": "success"}}
          ]
        },
        {
          "timeUnixNano": "'$(get_timestamp)'",
          "severityNumber": 13,
          "severityText": "WARN",
          "body": {
            "stringValue": "Permission denied: user attempted to access restricted table"
          },
          "attributes": [
            {"key": "user_name", "value": {"stringValue": "'$USER'"}},
            {"key": "object_name", "value": {"stringValue": "restricted_table"}},
            {"key": "message", "value": {"stringValue": "access denied"}},
            {"key": "error", "value": {"boolValue": true}}
          ]
        }
      ]
    }]
  }]
}'
    
    curl -X POST http://localhost:4318/v1/logs \
        -H "Content-Type: application/json" \
        -d "$LOG_DATA" \
        -s -o /dev/null -w "%{http_code}"
}

# Send some logs
echo ""
echo "Sending audit logs..."
if [[ "$(send_logs)" == "200" ]]; then
    echo "✅ Logs sent"
else
    echo "❌ Failed to send logs"
fi

echo ""
echo "=== User & Session metrics sent! ==="
echo ""
echo "Visit your new dashboard to see the data:"
echo "https://one.newrelic.com/redirect/entity/MzYzMDA3MnxWSVp8REFTSEJPQVJEfGRhOjEwNDU1MTQx"
echo ""
echo "Note: The dashboard focuses on:"
echo "- User activity and session patterns"
echo "- Performance metrics by user"
echo "- User behavior analysis"
echo "- Security and compliance tracking"
echo "- Cost analysis by user/session"