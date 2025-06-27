#!/bin/bash
# Script to send NRI metrics to New Relic Event API (simulating Infrastructure agent)

# Load environment variables
source .env

LICENSE_KEY="${NEW_RELIC_LICENSE_KEY}"
ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID}"

# Extract the latest NRI JSON from collector output
NRI_JSON=$(grep -o '{"name":"com.newrelic.postgresql".*}' collector-output.log | tail -1)

if [ -z "$NRI_JSON" ]; then
    echo "No NRI metrics found in collector output"
    exit 1
fi

# Parse the NRI JSON and convert to New Relic event format
EVENT_JSON=$(echo "$NRI_JSON" | jq -r '.data[0].entity.metrics[] | 
{
    eventType: .event_type,
    entityName: "localhost:5432",
    entityType: "PostgreSQLInstance",
    integrationName: "com.newrelic.postgresql",
    integrationVersion: "2.0.0",
    reportingAgent: "postgres-unified-collector",
    collectionTimestamp: .collection_timestamp,
    databaseName: .database_name,
    queryId: .query_id,
    queryText: .query_text,
    schemaName: .schema_name,
    statementType: .statement_type,
    avgElapsedTimeMs: .avg_elapsed_time_ms,
    avgDiskReads: .avg_disk_reads,
    avgDiskWrites: .avg_disk_writes,
    executionCount: .execution_count,
    timestamp: now | floor
}')

# Send to New Relic Events API
echo "Sending NRI metrics to New Relic Events API..."
echo "$EVENT_JSON" | jq -s '.' > /tmp/nri-event.json

RESPONSE=$(curl -s -X POST "https://insights-collector.newrelic.com/v1/accounts/$ACCOUNT_ID/events" \
  -H "Content-Type: application/json" \
  -H "X-Insert-Key: $LICENSE_KEY" \
  -d @/tmp/nri-event.json)

echo "Response: $RESPONSE"

# Also send as Infrastructure event
INFRA_EVENT=$(echo "$EVENT_JSON" | jq '. + {eventType: "InfrastructureEvent", category: "PostgreSQL", summary: "PostgreSQL slow query detected"}'  | jq -s '.')

echo -e "\nSending as Infrastructure event..."
INFRA_RESPONSE=$(curl -s -X POST "https://insights-collector.newrelic.com/v1/accounts/$ACCOUNT_ID/events" \
  -H "Content-Type: application/json" \
  -H "X-Insert-Key: $LICENSE_KEY" \
  -d "$INFRA_EVENT")

echo "Infrastructure Event Response: $INFRA_RESPONSE"