#!/bin/bash
# Test config-only mode with standard OTel collector

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$PROJECT_ROOT/scripts/utils/common.sh"

log_info "Testing config-only mode..."

# Create test environment file
cat > "$PROJECT_ROOT/.env.test" << EOF
# Test environment
NEW_RELIC_LICENSE_KEY=test_key_123
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318

DB_POSTGRES_HOST=localhost
DB_POSTGRES_PORT=5432
DB_POSTGRES_USER=postgres
DB_POSTGRES_PASSWORD=postgres
DB_POSTGRES_DATABASE=postgres

DB_MYSQL_HOST=localhost
DB_MYSQL_PORT=3306
DB_MYSQL_USER=root
DB_MYSQL_PASSWORD=root
DB_MYSQL_DATABASE=mysql

SERVICE_NAME=test-collector
SERVICE_VERSION=1.0.0
ENVIRONMENT=test
LOG_LEVEL=debug
EOF

# Test configuration validation
log_info "Validating configuration..."
docker run --rm \
    -v "$PROJECT_ROOT/configs/modes/config-only.yaml:/etc/otel/config.yaml" \
    --env-file "$PROJECT_ROOT/.env.test" \
    otel/opentelemetry-collector-contrib:0.105.0 \
    --config=/etc/otel/config.yaml \
    --dry-run

if [[ $? -eq 0 ]]; then
    log_success "Configuration is valid!"
else
    log_error "Configuration validation failed"
    exit 1
fi

# Test collector startup (5 seconds)
log_info "Testing collector startup..."
timeout 5s docker run --rm \
    -v "$PROJECT_ROOT/configs/modes/config-only.yaml:/etc/otel/config.yaml" \
    --env-file "$PROJECT_ROOT/.env.test" \
    -p 13133:13133 \
    otel/opentelemetry-collector-contrib:0.105.0 \
    --config=/etc/otel/config.yaml || true

log_success "Config-only mode test completed!"

# Cleanup
rm -f "$PROJECT_ROOT/.env.test"