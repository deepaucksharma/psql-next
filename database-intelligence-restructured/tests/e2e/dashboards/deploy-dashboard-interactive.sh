#!/bin/bash

# Interactive script to deploy the Database Intelligence dashboard
set -euo pipefail

DASHBOARD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DASHBOARD_FILE="$DASHBOARD_DIR/database-intelligence-complete-dashboard.json"
VERIFY_SCRIPT="$DASHBOARD_DIR/verify-and-deploy-dashboard.sh"

echo "=== Database Intelligence Dashboard Deployment ==="
echo ""

# Check if dashboard file exists
if [[ ! -f "$DASHBOARD_FILE" ]]; then
    echo "❌ Error: Dashboard file not found at $DASHBOARD_FILE"
    exit 1
fi

# Check if verify script exists
if [[ ! -f "$VERIFY_SCRIPT" ]]; then
    echo "❌ Error: Verify script not found at $VERIFY_SCRIPT"
    exit 1
fi

# Function to validate account ID
validate_account_id() {
    if [[ "$1" =~ ^[0-9]+$ ]]; then
        return 0
    else
        return 1
    fi
}

# Function to validate API key format
validate_api_key() {
    if [[ ${#1} -ge 32 ]]; then
        return 0
    else
        return 1
    fi
}

# Get New Relic Account ID
echo "Step 1: New Relic Account Information"
echo "====================================="
echo ""

# Try to get from environment first
ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID:-}"
if [[ -n "$ACCOUNT_ID" ]]; then
    echo "Found account ID in environment: $ACCOUNT_ID"
    read -p "Use this account ID? (Y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        ACCOUNT_ID=""
    fi
fi

# If not set, prompt for it
while [[ -z "$ACCOUNT_ID" ]] || ! validate_account_id "$ACCOUNT_ID"; do
    read -p "Enter your New Relic Account ID: " ACCOUNT_ID
    if ! validate_account_id "$ACCOUNT_ID"; then
        echo "❌ Invalid account ID. Please enter a numeric account ID."
        ACCOUNT_ID=""
    fi
done

echo "✅ Account ID: $ACCOUNT_ID"
echo ""

# Get New Relic API Key
echo "Step 2: New Relic API Key"
echo "========================"
echo ""
echo "You need a New Relic User API key with the following permissions:"
echo "- NerdGraph (GraphQL API access)"
echo "- Dashboard management"
echo ""
echo "To create one:"
echo "1. Go to: https://one.newrelic.com/api-keys"
echo "2. Click 'Create a key'"
echo "3. Select 'User' key type"
echo "4. Add permissions for NerdGraph"
echo ""

# Try to get from environment first
API_KEY="${NEW_RELIC_API_KEY:-}"
if [[ -n "$API_KEY" ]]; then
    echo "Found API key in environment"
    read -p "Use this API key? (Y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        API_KEY=""
    fi
fi

# If not set, prompt for it
while [[ -z "$API_KEY" ]] || ! validate_api_key "$API_KEY"; do
    read -s -p "Enter your New Relic API Key: " API_KEY
    echo
    if ! validate_api_key "$API_KEY"; then
        echo "❌ API key seems too short. New Relic API keys are typically 32+ characters."
        API_KEY=""
    fi
done

echo "✅ API Key configured (hidden for security)"
echo ""

# Confirm deployment
echo "Step 3: Deployment Confirmation"
echo "=============================="
echo ""
echo "Ready to deploy the following dashboard:"
echo "- Name: Database Intelligence - Complete Monitoring"
echo "- Pages: 6 (Overview, Query Analysis, System Performance, Error Monitoring, Cost Control, Database Details)"
echo "- Widgets: 40+"
echo "- Account: $ACCOUNT_ID"
echo ""
read -p "Proceed with deployment? (y/N) " -n 1 -r
echo
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Deployment cancelled."
    exit 0
fi

# Run the deployment
echo "Step 4: Running Deployment"
echo "========================="
echo ""

# Export for subprocess
export NEW_RELIC_ACCOUNT_ID="$ACCOUNT_ID"
export NEW_RELIC_API_KEY="$API_KEY"

# Run the verify and deploy script
if "$VERIFY_SCRIPT" "$ACCOUNT_ID" "$API_KEY"; then
    echo ""
    echo "🎉 Success! Dashboard has been deployed."
    echo ""
    echo "=== Next Steps ==="
    echo ""
    echo "1. Configure your collector to send data to New Relic:"
    echo ""
    cat <<EOF
# Add to your collector config:
exporters:
  otlp/newrelic:
    endpoint: "otlp.nr-data.net:4317"
    headers:
      api-key: "$API_KEY"

service:
  pipelines:
    metrics:
      exporters: [otlp/newrelic]
    logs:
      exporters: [otlp/newrelic]
EOF
    echo ""
    echo "2. Start the collector with all components:"
    echo "   ./otelcol-complete --config=your-config.yaml"
    echo ""
    echo "3. Generate some test data or wait for real data"
    echo ""
    echo "4. View your dashboard in New Relic!"
    echo ""
    
    # Save credentials for future use (optional)
    read -p "Save credentials to .env file for future use? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        ENV_FILE="$DASHBOARD_DIR/.env"
        cat > "$ENV_FILE" <<EOF
# New Relic credentials for Database Intelligence
NEW_RELIC_ACCOUNT_ID=$ACCOUNT_ID
NEW_RELIC_API_KEY=$API_KEY
# Add your database connection string here:
# DB_CONNECTION_STRING=postgres://user:pass@localhost/dbname
EOF
        chmod 600 "$ENV_FILE"
        echo "✅ Credentials saved to $ENV_FILE (file permissions set to 600)"
        echo "   You can source this file in the future: source $ENV_FILE"
    fi
else
    echo ""
    echo "❌ Deployment failed. Please check the error messages above."
    exit 1
fi