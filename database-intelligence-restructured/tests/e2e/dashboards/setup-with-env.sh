#!/bin/bash

# Script to set up and deploy dashboard using .env file
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Check for .env file in multiple locations
if [[ -f "$SCRIPT_DIR/.env" ]]; then
    ENV_FILE="$SCRIPT_DIR/.env"
elif [[ -f "/Users/deepaksharma/syc/db-otel/.env" ]]; then
    ENV_FILE="/Users/deepaksharma/syc/db-otel/.env"
else
    ENV_FILE="$SCRIPT_DIR/.env"
fi
ENV_TEMPLATE="$SCRIPT_DIR/.env.template"

echo "=== Database Intelligence Dashboard Setup with .env ==="
echo ""

# Function to create .env from template
create_env_file() {
    echo "Creating .env file from template..."
    cp "$ENV_TEMPLATE" "$ENV_FILE"
    chmod 600 "$ENV_FILE"
    echo "✅ Created .env file with restricted permissions (600)"
    echo ""
    echo "Please edit the .env file and add your credentials:"
    echo "  $ENV_FILE"
    echo ""
    echo "Required fields:"
    echo "  - NEW_RELIC_ACCOUNT_ID: Your numeric account ID"
    echo "  - NEW_RELIC_API_KEY: Your User API key with NerdGraph access"
    echo ""
    echo "You can get these from:"
    echo "  - Account ID: https://one.newrelic.com/admin-portal/organizations/organization-details"
    echo "  - API Key: https://one.newrelic.com/api-keys"
    echo ""
}

# Check if .env exists
if [[ ! -f "$ENV_FILE" ]]; then
    echo "No .env file found."
    read -p "Would you like to create one from the template? (Y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        create_env_file
        echo "After adding your credentials, run this script again."
        exit 0
    else
        echo "Please create a .env file with your credentials."
        exit 1
    fi
fi

# Load .env file
echo "Loading .env file..."
set -a
source "$ENV_FILE"
set +a

# Validate required variables
echo ""
echo "Validating credentials..."

missing_vars=0

if [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
    echo "❌ NEW_RELIC_ACCOUNT_ID is not set in .env"
    ((missing_vars++))
else
    echo "✅ Account ID: $NEW_RELIC_ACCOUNT_ID"
fi

if [[ -z "${NEW_RELIC_API_KEY:-}" ]]; then
    echo "❌ NEW_RELIC_API_KEY is not set in .env"
    ((missing_vars++))
else
    echo "✅ API Key: [HIDDEN]"
fi

if [[ $missing_vars -gt 0 ]]; then
    echo ""
    echo "Please edit $ENV_FILE and add the missing credentials."
    exit 1
fi

# Optional: Show other configured values
echo ""
echo "Additional configuration:"
echo "  Environment: ${ENVIRONMENT:-not set}"
echo "  Database: ${DB_CONNECTION_STRING:+[CONFIGURED]}"
echo "  License Key: ${NEW_RELIC_LICENSE_KEY:+[CONFIGURED]}"

# Confirm deployment
echo ""
echo "Ready to deploy the Database Intelligence dashboard to account $NEW_RELIC_ACCOUNT_ID"
read -p "Continue with deployment? (y/N) " -n 1 -r
echo
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Deployment cancelled."
    exit 0
fi

# Run the deployment
echo "Starting deployment..."
echo "===================="
echo ""

# Execute the verify and deploy script
if "$SCRIPT_DIR/verify-and-deploy-dashboard.sh" "$NEW_RELIC_ACCOUNT_ID" "$NEW_RELIC_API_KEY"; then
    echo ""
    echo "=== ✅ Deployment Successful! ==="
    echo ""
    
    # Generate collector config
    CONFIG_FILE="$SCRIPT_DIR/collector-config-from-env.yaml"
    echo "Generating collector configuration..."
    
    cat > "$CONFIG_FILE" <<EOF
# Generated collector configuration from .env
# Created: $(date)

receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  
  ash:
    driver: postgres
    datasource: "${DB_CONNECTION_STRING:-postgres://localhost/postgres}"
    collection_interval: 30s
    enable_wait_analysis: true
    enable_blocking_analysis: true
  
  kernelmetrics:
    target_process:
      process_name: "postgres"
    programs:
      db_query_trace: true
      cpu_profile: true

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  
  batch:
    timeout: 10s
  
  # Add all custom processors here
  costcontrol: {}
  nrerrormonitor: {}
  querycorrelator:
    retention_period: 5m
    enable_table_correlation: true
  adaptivesampler:
    default_sample_rate: 0.1
  circuit_breaker: {}
  planattributeextractor: {}
  verification: {}

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 100
  
  otlp/newrelic:
    endpoint: "otlp.nr-data.net:4317"
    headers:
      api-key: "${NEW_RELIC_API_KEY}"
    compression: gzip
  
  nri:
    integration_name: "database-intelligence"
    output_mode: "stdout"

service:
  pipelines:
    metrics:
      receivers: [otlp, ash, kernelmetrics]
      processors: [memory_limiter, batch, costcontrol, nrerrormonitor, querycorrelator]
      exporters: [debug, otlp/newrelic]
    
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch, costcontrol, adaptivesampler, circuit_breaker, planattributeextractor, verification]
      exporters: [debug, otlp/newrelic]
  
  telemetry:
    logs:
      level: info
EOF

    echo "✅ Collector configuration saved to: $CONFIG_FILE"
    echo ""
    echo "=== Next Steps ==="
    echo ""
    echo "1. Start the collector:"
    echo "   cd $SCRIPT_DIR/../../../distributions/production"
    echo "   ./otelcol-complete --config=$CONFIG_FILE"
    echo ""
    echo "2. Or use the environment variables directly:"
    echo "   source $ENV_FILE"
    echo "   ./otelcol-complete --config=../../tests/e2e/dashboards/test-collector-config.yaml"
    echo ""
    echo "3. View your dashboard in New Relic!"
    echo ""
    
    # Save dashboard URL if available
    if [[ -f "$SCRIPT_DIR/dashboard-url.txt" ]]; then
        echo "Dashboard URL saved to: $SCRIPT_DIR/dashboard-url.txt"
    fi
else
    echo ""
    echo "❌ Deployment failed. Please check the error messages above."
    echo ""
    echo "Common issues:"
    echo "- Invalid API key (ensure it has NerdGraph permissions)"
    echo "- Incorrect account ID"
    echo "- Network connectivity to api.newrelic.com"
    exit 1
fi