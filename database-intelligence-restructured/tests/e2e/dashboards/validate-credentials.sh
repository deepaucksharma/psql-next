#!/bin/bash

# Script to validate New Relic credentials
set -euo pipefail

# Load .env file
if [[ -f ".env" ]]; then
    source .env
elif [[ -f "/Users/deepaksharma/syc/db-otel/.env" ]]; then
    source /Users/deepaksharma/syc/db-otel/.env
else
    echo "❌ No .env file found"
    exit 1
fi

echo "=== New Relic Credentials Validation ==="
echo ""

# Check if credentials are still placeholders
if [[ "$NEW_RELIC_ACCOUNT_ID" == "YOUR_ACCOUNT_ID_HERE" ]]; then
    echo "❌ Account ID is still a placeholder: $NEW_RELIC_ACCOUNT_ID"
    echo ""
    echo "To update your credentials, run:"
    echo "  ./update-credentials.sh"
    echo ""
    echo "You can find your account ID at:"
    echo "  https://one.newrelic.com/admin-portal/organizations/organization-details"
    exit 1
fi

if [[ "$NEW_RELIC_API_KEY" == "YOUR_API_KEY_HERE" ]]; then
    echo "❌ API Key is still a placeholder"
    echo ""
    echo "To update your credentials, run:"
    echo "  ./update-credentials.sh"
    echo ""
    echo "You can create an API key at:"
    echo "  https://one.newrelic.com/api-keys"
    exit 1
fi

echo "✅ Credentials are not placeholders"
echo "  Account ID: $NEW_RELIC_ACCOUNT_ID"
echo "  API Key: ${NEW_RELIC_API_KEY:0:10}... (${#NEW_RELIC_API_KEY} chars)"
echo ""

# Test API key by making a simple NerdGraph query
echo "Testing API key with NerdGraph..."

NERDGRAPH_URL="https://api.newrelic.com/graphql"
TEST_QUERY='{"query":"{ actor { user { email } } }"}'

response=$(curl -s -X POST "$NERDGRAPH_URL" \
    -H "Content-Type: application/json" \
    -H "Api-Key: $NEW_RELIC_API_KEY" \
    -d "$TEST_QUERY" 2>/dev/null || echo '{"errors":[{"message":"curl failed"}]}')

if echo "$response" | grep -q '"errors"'; then
    error_msg=$(echo "$response" | jq -r '.errors[0].message' 2>/dev/null || echo "Unknown error")
    echo "❌ API key validation failed: $error_msg"
    echo ""
    echo "Please ensure:"
    echo "1. The API key is a valid User key (not License key)"
    echo "2. The key has NerdGraph permissions"
    echo "3. The key is not expired"
    exit 1
else
    email=$(echo "$response" | jq -r '.data.actor.user.email' 2>/dev/null || echo "unknown")
    echo "✅ API key is valid! Authenticated as: $email"
fi

echo ""
echo "=== Credentials are valid and ready for use! ==="
echo ""
echo "To deploy the dashboard, run:"
echo "  ./setup-with-env.sh"