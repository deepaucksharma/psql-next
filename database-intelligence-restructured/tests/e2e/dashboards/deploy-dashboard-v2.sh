#!/bin/bash

# Enhanced dashboard deployment script with better JSON handling
set -euo pipefail

# Get command line arguments
ACCOUNT_ID="${1:-}"
API_KEY="${2:-}"

# Check arguments
if [[ -z "$ACCOUNT_ID" || -z "$API_KEY" ]]; then
    echo "Usage: $0 <account_id> <api_key>"
    echo ""
    echo "Example:"
    echo "  $0 1234567 NRAK-XXXXXXXXXXXXX"
    exit 1
fi

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DASHBOARD_FILE="$SCRIPT_DIR/database-intelligence-user-session-nerdgraph.json"
NERDGRAPH_URL="https://api.newrelic.com/graphql"

# Verify dashboard file exists
if [[ ! -f "$DASHBOARD_FILE" ]]; then
    echo "❌ Dashboard file not found: $DASHBOARD_FILE"
    exit 1
fi

echo "=== Database Intelligence Dashboard Deployment v2 ==="
echo "Account ID: $ACCOUNT_ID"
echo ""

# Read and prepare dashboard JSON
DASHBOARD_JSON=$(cat "$DASHBOARD_FILE")

# Extract dashboard properties
NAME=$(echo "$DASHBOARD_JSON" | jq -r '.name')
DESCRIPTION=$(echo "$DASHBOARD_JSON" | jq -r '.description')
PAGES=$(echo "$DASHBOARD_JSON" | jq -c '.pages')

# Create the GraphQL mutation using proper JSON structure
echo "Creating GraphQL mutation..."

# Build the mutation as a proper JSON object
MUTATION_JSON=$(jq -n \
  --arg accountId "$ACCOUNT_ID" \
  --arg name "$NAME" \
  --arg description "$DESCRIPTION" \
  --argjson pages "$PAGES" \
  '{
    query: "mutation CreateDashboard($accountId: Int!, $dashboard: DashboardInput!) { dashboardCreate(accountId: $accountId, dashboard: $dashboard) { entityResult { guid name accountId createdAt updatedAt } errors { description type } } }",
    variables: {
      accountId: ($accountId | tonumber),
      dashboard: {
        name: $name,
        description: $description,
        permissions: "PRIVATE",
        pages: $pages
      }
    }
  }')

# Save mutation for debugging
echo "$MUTATION_JSON" > "$SCRIPT_DIR/mutation-debug.json"
echo "✅ Mutation saved to mutation-debug.json for debugging"

# Execute the mutation
echo ""
echo "Deploying dashboard to New Relic..."

RESPONSE=$(curl -s -X POST "$NERDGRAPH_URL" \
    -H "Content-Type: application/json" \
    -H "Api-Key: $API_KEY" \
    -d "$MUTATION_JSON")

# Save response for debugging
echo "$RESPONSE" > "$SCRIPT_DIR/response-debug.json"

# Check for errors
if echo "$RESPONSE" | jq -e '.errors' > /dev/null 2>&1; then
    echo "❌ Failed to create dashboard"
    echo "Error details:"
    echo "$RESPONSE" | jq '.errors'
    
    # Check for specific error types
    if echo "$RESPONSE" | grep -q "UNAUTHORIZED"; then
        echo ""
        echo "Authentication error. Please check:"
        echo "1. API key is valid"
        echo "2. API key has NerdGraph permissions"
        echo "3. Account ID is correct"
    elif echo "$RESPONSE" | grep -q "INVALID_PARAMETER"; then
        echo ""
        echo "Invalid parameter error. Please check:"
        echo "1. Account ID is numeric"
        echo "2. Dashboard JSON structure is valid"
    fi
    exit 1
fi

# Check if dashboard was created
if echo "$RESPONSE" | jq -e '.data.dashboardCreate.errors[]' > /dev/null 2>&1; then
    echo "❌ Dashboard creation failed with errors:"
    echo "$RESPONSE" | jq '.data.dashboardCreate.errors'
    exit 1
fi

# Extract GUID
GUID=$(echo "$RESPONSE" | jq -r '.data.dashboardCreate.entityResult.guid' 2>/dev/null)

if [[ -z "$GUID" || "$GUID" == "null" ]]; then
    echo "❌ Failed to extract dashboard GUID from response"
    echo "Response: $RESPONSE"
    exit 1
fi

# Success!
echo ""
echo "✅ Dashboard created successfully!"
echo ""
echo "Dashboard Details:"
echo "=================="
echo "GUID: $GUID"
echo "Name: $NAME"
echo "Account: $ACCOUNT_ID"
echo ""
echo "Dashboard URL:"
echo "https://one.newrelic.com/redirect/entity/$GUID"

# Save URL for reference
echo "https://one.newrelic.com/redirect/entity/$GUID" > "$SCRIPT_DIR/dashboard-url.txt"
echo ""
echo "URL saved to: $SCRIPT_DIR/dashboard-url.txt"

echo ""
echo "=== Deployment Complete! ==="
echo ""
echo "Next steps:"
echo "1. Visit the dashboard URL above"
echo "2. Start the Database Intelligence collector"
echo "3. Configure it to send data to New Relic"
echo "4. Data should appear within a few minutes"