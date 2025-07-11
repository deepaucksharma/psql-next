#!/bin/bash

# Script to update New Relic credentials in .env file
set -euo pipefail

ENV_FILE="/Users/deepaksharma/syc/db-otel/.env"

echo "=== Update New Relic Credentials ==="
echo ""
echo "This script will help you update your New Relic credentials in:"
echo "  $ENV_FILE"
echo ""

# Check if file exists
if [[ ! -f "$ENV_FILE" ]]; then
    echo "❌ Error: .env file not found at $ENV_FILE"
    exit 1
fi

# Create backup
BACKUP_FILE="${ENV_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
cp "$ENV_FILE" "$BACKUP_FILE"
echo "✅ Created backup: $BACKUP_FILE"
echo ""

# Get current values
CURRENT_ACCOUNT_ID=$(grep "^NEW_RELIC_ACCOUNT_ID=" "$ENV_FILE" | cut -d'=' -f2)
CURRENT_API_KEY=$(grep "^NEW_RELIC_API_KEY=" "$ENV_FILE" | cut -d'=' -f2)

echo "Current values:"
echo "  Account ID: $CURRENT_ACCOUNT_ID"
echo "  API Key: ${CURRENT_API_KEY:0:10}..."
echo ""

# Function to validate account ID
validate_account_id() {
    if [[ "$1" =~ ^[0-9]+$ ]]; then
        return 0
    else
        return 1
    fi
}

# Get new account ID
echo "Step 1: New Relic Account ID"
echo "============================"
echo "To find your account ID:"
echo "1. Log into New Relic"
echo "2. Click on your name (bottom left)"
echo "3. Go to 'Administration'"
echo "4. Your account ID is shown at the top"
echo ""

ACCOUNT_ID=""
while [[ -z "$ACCOUNT_ID" ]] || ! validate_account_id "$ACCOUNT_ID"; do
    read -p "Enter your New Relic Account ID (numeric): " ACCOUNT_ID
    if ! validate_account_id "$ACCOUNT_ID"; then
        echo "❌ Invalid account ID. Please enter only numbers."
        ACCOUNT_ID=""
    fi
done

echo "✅ Account ID: $ACCOUNT_ID"
echo ""

# Get new API key
echo "Step 2: New Relic API Key"
echo "========================="
echo "To create an API key:"
echo "1. Go to: https://one.newrelic.com/api-keys"
echo "2. Click 'Create a key'"
echo "3. Key type: 'User'"
echo "4. Name: 'Database Intelligence Dashboard'"
echo "5. Add NerdGraph permissions"
echo "6. Click 'Create' and copy the key"
echo ""

API_KEY=""
while [[ -z "$API_KEY" ]] || [[ ${#API_KEY} -lt 32 ]]; do
    read -s -p "Enter your New Relic API Key: " API_KEY
    echo
    if [[ ${#API_KEY} -lt 32 ]]; then
        echo "❌ API key seems too short. New Relic API keys are typically 32+ characters."
        API_KEY=""
    fi
done

echo "✅ API Key configured (${#API_KEY} characters)"
echo ""

# Update the file
echo "Updating .env file..."

# Use sed to update the values
if [[ "$(uname)" == "Darwin" ]]; then
    # macOS
    sed -i '' "s/^NEW_RELIC_ACCOUNT_ID=.*/NEW_RELIC_ACCOUNT_ID=$ACCOUNT_ID/" "$ENV_FILE"
    sed -i '' "s/^NEW_RELIC_API_KEY=.*/NEW_RELIC_API_KEY=$API_KEY/" "$ENV_FILE"
else
    # Linux
    sed -i "s/^NEW_RELIC_ACCOUNT_ID=.*/NEW_RELIC_ACCOUNT_ID=$ACCOUNT_ID/" "$ENV_FILE"
    sed -i "s/^NEW_RELIC_API_KEY=.*/NEW_RELIC_API_KEY=$API_KEY/" "$ENV_FILE"
fi

echo "✅ Credentials updated successfully!"
echo ""

# Verify the update
echo "Verifying update..."
NEW_ACCOUNT_ID=$(grep "^NEW_RELIC_ACCOUNT_ID=" "$ENV_FILE" | cut -d'=' -f2)
NEW_API_KEY=$(grep "^NEW_RELIC_API_KEY=" "$ENV_FILE" | cut -d'=' -f2)

if [[ "$NEW_ACCOUNT_ID" == "$ACCOUNT_ID" ]] && [[ "$NEW_API_KEY" == "$API_KEY" ]]; then
    echo "✅ Verification successful!"
    echo ""
    echo "=== Next Steps ==="
    echo "1. Run the dashboard deployment:"
    echo "   cd $(dirname $0)"
    echo "   ./setup-with-env.sh"
    echo ""
    echo "2. The deployment will use your updated credentials"
else
    echo "❌ Verification failed. Please check the file manually."
    exit 1
fi