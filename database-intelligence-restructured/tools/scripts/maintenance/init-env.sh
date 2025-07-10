#!/bin/bash
# Database Intelligence MVP - Environment Initialization Script
# This script helps set up the environment configuration

set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common functions
source "${SCRIPT_DIR}/lib/common.sh"

# Get project root using common function
PROJECT_ROOT="$(get_project_root)"

# Check if .env already exists
check_existing_env() {
    if [ -f "$PROJECT_ROOT/.env" ]; then
        warning ".env file already exists!"
        read -p "Do you want to overwrite it? (y/N): " -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log "Keeping existing .env file"
            exit 0
        fi
        # Backup existing file
        cp "$PROJECT_ROOT/.env" "$PROJECT_ROOT/.env.backup.$(date +%Y%m%d_%H%M%S)"
        success "Backed up existing .env file"
    fi
}

# Wrapper for license key validation with user confirmation
validate_license_key_interactive() {
    local key="$1"
    if ! validate_license_key "$key"; then
        warning "License key format appears incorrect. New Relic license keys are typically 40 characters."
        read -p "Continue anyway? (y/N): " -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            return 1
        fi
    fi
    return 0
}

# Wrapper for PostgreSQL DSN validation with error message
validate_pg_dsn_interactive() {
    local dsn="$1"
    if ! validate_postgresql_dsn "$dsn"; then
        warning "PostgreSQL DSN format appears incorrect."
        echo "Expected format: postgres://username:password@host:port/database?sslmode=require"
        return 1
    fi
    return 0
}

# Interactive configuration
configure_environment() {
    log "Starting interactive environment configuration..."
    
    # Copy from example
    cp "$PROJECT_ROOT/.env.example" "$PROJECT_ROOT/.env"
    
    echo ""
    echo "==================================="
    echo "Database Intelligence MVP Setup"
    echo "==================================="
    echo ""
    
    # New Relic Configuration
    echo "📊 New Relic Configuration"
    echo "--------------------------"
    
    while true; do
        read -p "New Relic License Key: " -r nr_license_key
        if [ -z "$nr_license_key" ]; then
            error "License key is required"
            continue
        fi
        if validate_license_key_interactive "$nr_license_key"; then
            break
        fi
    done
    
    read -p "New Relic Region (US/EU) [US]: " -r nr_region
    nr_region="${nr_region:-US}"
    nr_region_upper=$(echo "$nr_region" | tr '[:lower:]' '[:upper:]')
    if [[ "$nr_region_upper" == "EU" ]]; then
        nr_endpoint="https://otlp.eu01.nr-data.net:4317"
    else
        nr_endpoint="https://otlp.nr-data.net:4317"
    fi
    
    # Update .env file
    sed -i.bak "s|NEW_RELIC_LICENSE_KEY=.*|NEW_RELIC_LICENSE_KEY=${nr_license_key}|" "$PROJECT_ROOT/.env"
    sed -i.bak "s|OTLP_ENDPOINT=.*|OTLP_ENDPOINT=${nr_endpoint}|" "$PROJECT_ROOT/.env"
    
    echo ""
    echo "🗄️ Database Configuration"
    echo "------------------------"
    
    # PostgreSQL
    read -p "Configure PostgreSQL? (Y/n): " -r
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        read -p "PostgreSQL Host [localhost]: " -r pg_host
        pg_host="${pg_host:-localhost}"
        
        read -p "PostgreSQL Port [5432]: " -r pg_port
        pg_port="${pg_port:-5432}"
        
        read -p "PostgreSQL Database: " -r pg_db
        read -p "PostgreSQL User [newrelic_monitor]: " -r pg_user
        pg_user="${pg_user:-newrelic_monitor}"
        
        read -p "PostgreSQL Password: " -s -r pg_pass
        echo ""
        
        read -p "Use SSL? (Y/n): " -r
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            pg_ssl="require"
        else
            pg_ssl="disable"
        fi
        
        pg_dsn="postgres://${pg_user}:${pg_pass}@${pg_host}:${pg_port}/${pg_db}?sslmode=${pg_ssl}"
        sed -i.bak "s|PG_REPLICA_DSN=.*|PG_REPLICA_DSN=${pg_dsn}|" "$PROJECT_ROOT/.env"
        success "PostgreSQL configured"
    fi
    
    # MySQL
    echo ""
    read -p "Configure MySQL? (y/N): " -r
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        read -p "MySQL Host [localhost]: " -r mysql_host
        mysql_host="${mysql_host:-localhost}"
        
        read -p "MySQL Port [3306]: " -r mysql_port
        mysql_port="${mysql_port:-3306}"
        
        read -p "MySQL Database: " -r mysql_db
        read -p "MySQL User [newrelic_monitor]: " -r mysql_user
        mysql_user="${mysql_user:-newrelic_monitor}"
        
        read -p "MySQL Password: " -s -r mysql_pass
        echo ""
        
        read -p "Use TLS? (Y/n): " -r
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            mysql_tls="true"
        else
            mysql_tls="false"
        fi
        
        mysql_dsn="${mysql_user}:${mysql_pass}@tcp(${mysql_host}:${mysql_port})/${mysql_db}?tls=${mysql_tls}"
        sed -i.bak "s|MYSQL_READONLY_DSN=.*|MYSQL_READONLY_DSN=${mysql_dsn}|" "$PROJECT_ROOT/.env"
        success "MySQL configured"
    fi
    
    # Environment settings
    echo ""
    echo "🚀 Deployment Settings"
    echo "---------------------"
    
    read -p "Environment name (development/staging/production) [development]: " -r env_name
    env_name="${env_name:-development}"
    sed -i.bak "s|DEPLOYMENT_ENV=.*|DEPLOYMENT_ENV=${env_name}|" "$PROJECT_ROOT/.env"
    
    # Clean up backup files
    rm -f "$PROJECT_ROOT/.env.bak"
    
    echo ""
    success "Environment configuration complete!"
    echo ""
    echo "📁 Configuration saved to: $PROJECT_ROOT/.env"
    echo ""
    echo "Next steps:"
    echo "1. Review the configuration in .env"
    echo "2. Test database connections: ./scripts/test-connections.sh"
    echo "3. Start the collector: ./quickstart.sh start"
    echo ""
}

# Test database connections
test_connections() {
    log "Testing database connections..."
    
    # Load environment
    if ! load_env_file "$PROJECT_ROOT/.env"; then
        return 1
    fi
    
    # Test PostgreSQL
    if [ -n "${PG_REPLICA_DSN:-}" ]; then
        test_postgresql_connection
    fi
    
    # Test MySQL
    if [ -n "${MYSQL_READONLY_DSN:-}" ]; then
        test_mysql_connection
    fi
}

# Show current configuration
show_config() {
    if [ ! -f "$PROJECT_ROOT/.env" ]; then
        error "No .env file found. Run 'init-env.sh setup' first."
        exit 1
    fi
    
    log "Current environment configuration:"
    echo ""
    grep -E "^[^#].*=" "$PROJECT_ROOT/.env" | grep -v "PASSWORD\|KEY" | while IFS= read -r line; do
        echo "  $line"
    done
    echo ""
    warning "Sensitive values (passwords, keys) are hidden"
}

# Main script logic
main() {
    case "${1:-setup}" in
        setup)
            check_existing_env
            configure_environment
            ;;
        test)
            test_connections
            ;;
        show)
            show_config
            ;;
        *)
            echo "Usage: $0 {setup|test|show}"
            echo ""
            echo "Commands:"
            echo "  setup  - Interactive environment configuration"
            echo "  test   - Test database connections"
            echo "  show   - Show current configuration (hides sensitive data)"
            exit 1
            ;;
    esac
}

# Run main function
main "$@"