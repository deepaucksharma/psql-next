#!/bin/bash

# Script to verify NRQL queries and deploy the Database Intelligence dashboard
# Usage: ./verify-and-deploy-dashboard.sh <account_id> <api_key>

set -euo pipefail

ACCOUNT_ID="${1:-}"
API_KEY="${2:-}"
DASHBOARD_FILE="database-intelligence-complete-dashboard.json"
NERDGRAPH_URL="https://api.newrelic.com/graphql"

if [[ -z "$ACCOUNT_ID" || -z "$API_KEY" ]]; then
    echo "Usage: $0 <account_id> <api_key>"
    exit 1
fi

# Function to validate NRQL query using NerdGraph
validate_query() {
    local query="$1"
    local title="$2"
    
    echo "Validating query for widget: $title"
    
    # Escape the query for JSON
    escaped_query=$(echo "$query" | sed 's/"/\\"/g')
    
    # Create GraphQL query
    local graphql_query=$(cat <<EOF
{
  "query": "query { actor { account(id: $ACCOUNT_ID) { nrql(query: \\"$escaped_query\\") { results } } } }"
}
EOF
)
    
    # Execute query
    response=$(curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "Api-Key: $API_KEY" \
        -d "$graphql_query")
    
    # Check for errors
    if echo "$response" | grep -q '"errors"'; then
        echo "  ❌ FAILED: $title"
        echo "  Error: $(echo "$response" | jq -r '.errors[0].message' 2>/dev/null || echo "$response")"
        return 1
    else
        echo "  ✅ PASSED: $title"
        return 0
    fi
}

# Function to create dashboard using NerdGraph
create_dashboard() {
    echo "Creating dashboard via NerdGraph..."
    
    # Read dashboard JSON
    dashboard_json=$(cat "$DASHBOARD_FILE")
    
    # Extract dashboard properties
    name=$(echo "$dashboard_json" | jq -r '.name')
    description=$(echo "$dashboard_json" | jq -r '.description')
    
    # Create pages array for GraphQL
    pages_json=$(echo "$dashboard_json" | jq -c '.pages')
    
    # Create the GraphQL mutation
    local graphql_mutation=$(cat <<EOF
{
  "query": "mutation {
    dashboardCreate(
      accountId: $ACCOUNT_ID,
      dashboard: {
        name: \\"$name\\",
        description: \\"$description\\",
        permissions: PRIVATE,
        pages: $(echo "$pages_json" | sed 's/"/\\"/g')
      }
    ) {
      entityResult {
        guid
        name
        accountId
        createdAt
        updatedAt
      }
      errors {
        description
        type
      }
    }
  }"
}
EOF
)
    
    # Execute mutation
    response=$(curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "Api-Key: $API_KEY" \
        -d "$graphql_mutation")
    
    # Check result
    if echo "$response" | grep -q '"errors"'; then
        echo "❌ Failed to create dashboard"
        echo "Error: $(echo "$response" | jq -r '.errors' 2>/dev/null || echo "$response")"
        return 1
    else
        guid=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.guid' 2>/dev/null)
        echo "✅ Dashboard created successfully!"
        echo "GUID: $guid"
        echo "URL: https://one.newrelic.com/redirect/entity/$guid"
        return 0
    fi
}

# Main execution
echo "=== Database Intelligence Dashboard Verification and Deployment ==="
echo "Account ID: $ACCOUNT_ID"
echo ""

# First, verify all queries
echo "Step 1: Verifying NRQL queries..."
echo "================================="

failed_queries=0
total_queries=0

# Extract and validate each query
while IFS= read -r widget; do
    title=$(echo "$widget" | jq -r '.title')
    query=$(echo "$widget" | jq -r '.query')
    
    if [[ -n "$query" && "$query" != "null" ]]; then
        ((total_queries++))
        if ! validate_query "$query" "$title"; then
            ((failed_queries++))
        fi
    fi
done < <(jq -c '.pages[].widgets[]' "$DASHBOARD_FILE")

echo ""
echo "Query validation complete: $((total_queries - failed_queries))/$total_queries passed"

if [[ $failed_queries -gt 0 ]]; then
    echo ""
    echo "⚠️  Warning: $failed_queries queries failed validation"
    echo "These queries may not return data until the metrics are being collected."
    echo ""
    read -p "Do you want to continue with dashboard creation anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Dashboard creation cancelled."
        exit 1
    fi
fi

# Create the dashboard
echo ""
echo "Step 2: Creating dashboard..."
echo "============================="

if create_dashboard; then
    echo ""
    echo "🎉 Dashboard deployment successful!"
else
    echo ""
    echo "❌ Dashboard deployment failed!"
    exit 1
fi

# Additional information
echo ""
echo "=== Next Steps ==="
echo "1. Ensure the Database Intelligence collector is running with all components"
echo "2. Configure the collector to send data to New Relic"
echo "3. Wait a few minutes for data to appear"
echo "4. Visit the dashboard URL to view your metrics"
echo ""
echo "Sample collector configuration snippet:"
echo "----------------------------------------"
cat <<EOF
exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: YOUR_NEW_RELIC_LICENSE_KEY
    compression: gzip

service:
  pipelines:
    metrics:
      receivers: [otlp, ash, kernelmetrics]
      processors: [memory_limiter, batch, costcontrol, nrerrormonitor, querycorrelator]
      exporters: [otlp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch, costcontrol, adaptivesampler, circuit_breaker, planattributeextractor, verification]
      exporters: [otlp]
EOF