#!/bin/bash

set -euo pipefail

echo "🔍 MySQL OpenTelemetry Setup Diagnostics"
echo "========================================"
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 1. Check Environment
echo "1. Environment Check"
echo "-------------------"

# Check if we're in the right directory
if [ ! -f "docker-compose.yml" ]; then
    log_error "docker-compose.yml not found. Are you in the project root?"
    exit 1
fi

if [ ! -f ".env" ]; then
    log_error ".env file not found. Run: cp .env.example .env"
    exit 1
fi

log_info "✓ Project files found"

# Check .env file
source .env
if [ "$NEW_RELIC_API_KEY" = "your_new_relic_ingest_license_key_here" ] || [ "$NEW_RELIC_API_KEY" = "test_api_key_replace_with_real_key" ]; then
    log_warn "NEW_RELIC_API_KEY is using placeholder value"
    echo "  Get your real API key from: https://one.newrelic.com/api-keys"
fi

if [ -z "${MYSQL_PRIMARY_ENDPOINT:-}" ]; then
    log_error "MYSQL_PRIMARY_ENDPOINT not set in .env"
else
    log_info "✓ Environment variables configured"
fi

echo

# 2. Check Docker
echo "2. Docker Check"
echo "---------------"

if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed"
    echo "  Install Docker from: https://docs.docker.com/get-docker/"
    exit 1
fi

log_info "✓ Docker command found: $(docker --version)"

# Test Docker daemon
if docker info &> /dev/null; then
    log_info "✓ Docker daemon is running"
else
    log_error "Docker daemon is not accessible"
    echo "  Try:"
    echo "    sudo systemctl start docker    # Linux"
    echo "    open -a Docker               # macOS Docker Desktop"
    echo "    # Or start Docker Desktop manually"
    exit 1
fi

# Test Docker Compose
if docker compose version &> /dev/null; then
    log_info "✓ Docker Compose available: $(docker compose version --short)"
else
    log_error "Docker Compose not available"
    echo "  Try: docker-compose instead of 'docker compose'"
    exit 1
fi

echo

# 3. Validate Configuration Files
echo "3. Configuration Validation"
echo "---------------------------"

# Validate YAML syntax
if python3 -c "import yaml; yaml.safe_load(open('docker-compose.yml'))" 2>/dev/null; then
    log_info "✓ docker-compose.yml syntax valid"
else
    log_error "docker-compose.yml has syntax errors"
    exit 1
fi

if python3 -c "import yaml; yaml.safe_load(open('otel/config/otel-collector-config.yaml'))" 2>/dev/null; then
    log_info "✓ OpenTelemetry config syntax valid"
else
    log_error "otel-collector-config.yaml has syntax errors"
    exit 1
fi

# Test docker-compose config
if docker compose config >/dev/null 2>&1; then
    log_info "✓ Docker Compose configuration valid"
else
    log_error "Docker Compose configuration has errors:"
    docker compose config
    exit 1
fi

echo

# 4. Check Required Directories
echo "4. Directory Structure"
echo "---------------------"

required_dirs=("mysql/init" "mysql/conf" "otel/config" "scripts" "app")
for dir in "${required_dirs[@]}"; do
    if [ -d "$dir" ]; then
        log_info "✓ $dir exists"
    else
        log_error "$dir missing"
    fi
done

echo

# 5. Check File Permissions
echo "5. File Permissions"
echo "------------------"

if [ -x "scripts/setup.sh" ]; then
    log_info "✓ setup.sh is executable"
else
    log_warn "setup.sh not executable, fixing..."
    chmod +x scripts/*.sh
fi

if [ -r "mysql/init/01-create-monitoring-user.sql" ]; then
    log_info "✓ MySQL init scripts readable"
else
    log_error "MySQL init scripts not readable"
fi

echo

# 6. Test Network Connectivity (if possible)
echo "6. Network Connectivity"
echo "----------------------"

if command -v nc &> /dev/null; then
    if nc -z -w5 otlp.nr-data.net 4317 2>/dev/null; then
        log_info "✓ Can reach New Relic OTLP endpoint"
    else
        log_warn "Cannot reach New Relic OTLP endpoint (network issue or firewall)"
    fi
else
    log_warn "netcat not available, skipping network test"
fi

echo

# 7. Show Next Steps
echo "7. Next Steps"
echo "-------------"

if docker info &> /dev/null; then
    log_info "Ready to start! Run:"
    echo "  ./scripts/setup.sh"
    echo ""
    echo "Or start services manually:"
    echo "  docker compose up -d"
else
    log_error "Fix Docker daemon first, then run:"
    echo "  ./scripts/setup.sh"
fi

echo
echo "For troubleshooting, see: docs/TROUBLESHOOTING.md"