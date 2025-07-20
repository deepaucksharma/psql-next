#!/bin/bash
# Validate required environment variables for Database Intelligence Monorepo

set -e

echo "🔍 Validating required environment variables..."

# Define required variables
REQUIRED_VARS=(
    "NEW_RELIC_LICENSE_KEY"
    "NEW_RELIC_OTLP_ENDPOINT"
    "NEW_RELIC_ACCOUNT_ID"
)

# Define optional but recommended variables
OPTIONAL_VARS=(
    "MYSQL_ENDPOINT"
    "MYSQL_USER"
    "MYSQL_PASSWORD"
    "ENVIRONMENT"
    "CLUSTER_NAME"
)

# Track missing variables
MISSING_REQUIRED=()
MISSING_OPTIONAL=()

# Check required variables
echo ""
echo "📋 Checking required variables:"
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        echo "  ❌ $var is not set"
        MISSING_REQUIRED+=("$var")
    else
        # Mask sensitive values
        if [[ "$var" == *"KEY"* ]] || [[ "$var" == *"PASSWORD"* ]]; then
            echo "  ✅ $var is set (value masked)"
        else
            echo "  ✅ $var = ${!var}"
        fi
    fi
done

# Check optional variables
echo ""
echo "📋 Checking optional variables (will use defaults if not set):"
for var in "${OPTIONAL_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        echo "  ⚠️  $var is not set"
        MISSING_OPTIONAL+=("$var")
    else
        # Mask sensitive values
        if [[ "$var" == *"PASSWORD"* ]]; then
            echo "  ✅ $var is set (value masked)"
        else
            echo "  ✅ $var = ${!var}"
        fi
    fi
done

# Report results
echo ""
echo "📊 Summary:"
echo "  Required variables: $((${#REQUIRED_VARS[@]} - ${#MISSING_REQUIRED[@]}))/${#REQUIRED_VARS[@]} set"
echo "  Optional variables: $((${#OPTIONAL_VARS[@]} - ${#MISSING_OPTIONAL[@]}))/${#OPTIONAL_VARS[@]} set"

# Exit with error if required variables are missing
if [ ${#MISSING_REQUIRED[@]} -gt 0 ]; then
    echo ""
    echo "❌ ERROR: Missing required environment variables:"
    for var in "${MISSING_REQUIRED[@]}"; do
        echo "   - $var"
    done
    echo ""
    echo "Please set these variables before running the modules."
    echo "Example:"
    echo "  export NEW_RELIC_LICENSE_KEY='your-license-key'"
    echo "  export NEW_RELIC_OTLP_ENDPOINT='https://otlp.nr-data.net:4318'"
    echo "  export NEW_RELIC_ACCOUNT_ID='your-account-id'"
    exit 1
fi

# Show default values for optional variables
if [ ${#MISSING_OPTIONAL[@]} -gt 0 ]; then
    echo ""
    echo "ℹ️  Default values will be used for:"
    [ -z "$MYSQL_ENDPOINT" ] && echo "   - MYSQL_ENDPOINT = mysql-test:3306"
    [ -z "$MYSQL_USER" ] && echo "   - MYSQL_USER = root"
    [ -z "$MYSQL_PASSWORD" ] && echo "   - MYSQL_PASSWORD = test"
    [ -z "$ENVIRONMENT" ] && echo "   - ENVIRONMENT = production"
    [ -z "$CLUSTER_NAME" ] && echo "   - CLUSTER_NAME = database-intelligence-cluster"
fi

echo ""
echo "✅ Environment validation complete!"