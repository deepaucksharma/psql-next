#!/bin/bash

source ../../.env

if [ -z "$NEW_RELIC_USER_KEY" ] || [ -z "$NEW_RELIC_ACCOUNT_ID" ]; then
    echo "Error: NEW_RELIC_USER_KEY and NEW_RELIC_ACCOUNT_ID must be set"
    exit 1
fi

echo "🚀 Creating PostgreSQL OpenTelemetry Dashboard"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo "================================================================================"

# Create the dashboard mutation
cat > /tmp/create_dashboard.graphql << 'EOF'
mutation CreateDashboard($accountId: Int!) {
  dashboardCreate(
    accountId: $accountId
    dashboard: {
      name: "PostgreSQL OpenTelemetry Monitoring"
      description: "PostgreSQL monitoring using OpenTelemetry metrics (migrated from OHI)"
      permissions: PUBLIC_READ_WRITE
      pages: [
        {
          name: "Overview"
          widgets: [
            {
              title: "Unique Queries by Database"
              configuration: {
                bar: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT uniqueCount(db.postgresql.query_id) FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' FACET db.name"
                  }]
                }
              }
              layout: { column: 1, row: 1, width: 3, height: 3 }
            }
            {
              title: "Average Query Execution Time (ms)"
              configuration: {
                bar: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT latest(postgres.slow_queries.elapsed_time) FROM Metric WHERE db.statement != '<insufficient privilege>' FACET db.statement LIMIT 20"
                  }]
                }
              }
              layout: { column: 4, row: 1, width: 3, height: 3 }
            }
            {
              title: "Query Execution Count Over Time"
              configuration: {
                line: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT sum(postgres.slow_queries.count) FROM Metric TIMESERIES AUTO"
                  }]
                }
              }
              layout: { column: 7, row: 1, width: 3, height: 3 }
            }
            {
              title: "Top Wait Events"
              configuration: {
                bar: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT sum(postgres.wait_events) FROM Metric FACET db.wait_event.name WHERE db.wait_event.name IS NOT NULL LIMIT 20"
                  }]
                }
              }
              layout: { column: 10, row: 1, width: 3, height: 3 }
            }
            {
              title: "Slowest Queries"
              configuration: {
                table: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT latest(db.name) as 'Database', latest(db.statement) as 'Query', latest(db.schema) as 'Schema', latest(postgres.slow_queries.count) as 'Count', latest(postgres.slow_queries.elapsed_time) as 'Avg Time (ms)', latest(postgres.slow_queries.disk_reads) as 'Disk Reads', latest(postgres.slow_queries.disk_writes) as 'Disk Writes', latest(db.operation) as 'Type' FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' FACET db.postgresql.query_id LIMIT 50"
                  }]
                }
              }
              layout: { column: 1, row: 4, width: 12, height: 5 }
            }
            {
              title: "Active Connections"
              configuration: {
                line: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT latest(postgresql.backends) FROM Metric WHERE metricName = 'postgresql.backends' FACET postgresql.database.name TIMESERIES AUTO"
                  }]
                }
              }
              layout: { column: 1, row: 9, width: 4, height: 3 }
            }
            {
              title: "Database Size (MB)"
              configuration: {
                billboard: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT latest(postgresql.db_size) / 1024 / 1024 as 'Size (MB)' FROM Metric WHERE metricName = 'postgresql.db_size' FACET postgresql.database.name"
                  }]
                }
              }
              layout: { column: 5, row: 9, width: 4, height: 3 }
            }
            {
              title: "Transaction Rate"
              configuration: {
                line: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT sum(postgresql.commits) as 'Commits', sum(postgresql.rollbacks) as 'Rollbacks' FROM Metric WHERE metricName IN ('postgresql.commits', 'postgresql.rollbacks') TIMESERIES AUTO"
                  }]
                }
              }
              layout: { column: 9, row: 9, width: 4, height: 3 }
            }
          ]
        }
        {
          name: "Performance Analysis"
          widgets: [
            {
              title: "Disk I/O - Reads"
              configuration: {
                area: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT average(postgres.slow_queries.disk_reads) as 'Avg Disk Reads' FROM Metric WHERE metricName = 'postgres.slow_queries.disk_reads' FACET db.name TIMESERIES AUTO"
                  }]
                }
              }
              layout: { column: 1, row: 1, width: 6, height: 3 }
            }
            {
              title: "Disk I/O - Writes"
              configuration: {
                area: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT average(postgres.slow_queries.disk_writes) as 'Avg Disk Writes' FROM Metric WHERE metricName = 'postgres.slow_queries.disk_writes' FACET db.name TIMESERIES AUTO"
                  }]
                }
              }
              layout: { column: 7, row: 1, width: 6, height: 3 }
            }
            {
              title: "Blocking Sessions"
              configuration: {
                table: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT latest(db.blocking.blocked_pid) as 'Blocked PID', latest(db.blocking.blocked_query) as 'Blocked Query', latest(db.name) as 'Database', latest(db.blocking.blocking_pid) as 'Blocking PID', latest(db.blocking.blocking_query) as 'Blocking Query' FROM Metric WHERE metricName = 'postgres.blocking_sessions' FACET db.blocking.blocked_pid LIMIT 50"
                  }]
                }
              }
              layout: { column: 1, row: 4, width: 12, height: 4 }
            }
            {
              title: "Wait Events by Category"
              configuration: {
                pie: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT sum(postgres.wait_events) FROM Metric WHERE metricName = 'postgres.wait_events' AND db.wait_event.name IS NOT NULL FACET db.wait_event.category"
                  }]
                }
              }
              layout: { column: 1, row: 8, width: 4, height: 4 }
            }
            {
              title: "Wait Event Timeline"
              configuration: {
                line: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT sum(postgres.wait_events) FROM Metric WHERE metricName = 'postgres.wait_events' AND db.wait_event.name IS NOT NULL FACET db.wait_event.name TIMESERIES AUTO LIMIT 10"
                  }]
                }
              }
              layout: { column: 5, row: 8, width: 8, height: 4 }
            }
          ]
        }
        {
          name: "Database Health"
          widgets: [
            {
              title: "Database Statistics"
              configuration: {
                table: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT latest(postgresql.backends) as 'Connections', latest(postgresql.db_size) / 1024 / 1024 as 'Size (MB)', sum(postgresql.commits) as 'Commits', sum(postgresql.rollbacks) as 'Rollbacks', sum(postgresql.deadlocks) as 'Deadlocks' FROM Metric WHERE db.system = 'postgresql' AND postgresql.database.name IS NOT NULL FACET postgresql.database.name"
                  }]
                }
              }
              layout: { column: 1, row: 1, width: 12, height: 4 }
            }
            {
              title: "Buffer Cache Performance"
              configuration: {
                line: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT sum(postgresql.bgwriter.buffers.writes) FROM Metric WHERE metricName = 'postgresql.bgwriter.buffers.writes' FACET postgresql.bgwriter.buffers.writes TIMESERIES AUTO"
                  }]
                }
              }
              layout: { column: 1, row: 5, width: 6, height: 3 }
            }
            {
              title: "Index Usage"
              configuration: {
                table: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT latest(postgresql.index.scans) as 'Scans', latest(postgresql.index.size) as 'Size' FROM Metric WHERE metricName LIKE 'postgresql.index%' FACET postgresql.index.name, postgresql.table.name LIMIT 50"
                  }]
                }
              }
              layout: { column: 7, row: 5, width: 6, height: 3 }
            }
            {
              title: "All PostgreSQL Metrics"
              configuration: {
                table: {
                  nrqlQueries: [{
                    accountId: $accountId
                    query: "SELECT uniques(metricName) as 'Available Metrics' FROM Metric WHERE metricName LIKE '%postgres%' OR db.system = 'postgresql'"
                  }]
                }
              }
              layout: { column: 1, row: 8, width: 12, height: 3 }
            }
          ]
        }
      ]
    }
  ) {
    entityResult {
      guid
    }
    errors {
      description
      type
    }
  }
}
EOF

# Prepare the GraphQL request
cat > /tmp/dashboard_request.json << EOF
{
  "query": $(jq -Rs . < /tmp/create_dashboard.graphql),
  "variables": {
    "accountId": $NEW_RELIC_ACCOUNT_ID
  }
}
EOF

echo "Creating dashboard..."

# Execute the mutation
response=$(curl -s -X POST https://api.newrelic.com/graphql \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_USER_KEY" \
    -d @/tmp/dashboard_request.json)

# Check for errors
if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
    echo "❌ GraphQL errors:"
    echo "$response" | jq '.errors'
    exit 1
fi

if echo "$response" | jq -e '.data.dashboardCreate.errors | length > 0' > /dev/null 2>&1; then
    echo "❌ Dashboard creation errors:"
    echo "$response" | jq '.data.dashboardCreate.errors'
    exit 1
fi

# Extract GUID
guid=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.guid')

if [ "$guid" != "null" ] && [ -n "$guid" ]; then
    echo "✅ Dashboard created successfully!"
    echo "Dashboard GUID: $guid"
    echo ""
    echo "📊 View your dashboard at:"
    echo "https://one.newrelic.com/dashboards?account=$NEW_RELIC_ACCOUNT_ID&state=$guid"
else
    echo "❌ Dashboard creation failed - no GUID returned"
    echo "Response:"
    echo "$response" | jq .
    exit 1
fi