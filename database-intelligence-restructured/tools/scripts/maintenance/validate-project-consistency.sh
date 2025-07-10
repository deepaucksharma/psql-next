#!/bin/bash
# Database Intelligence MVP - Project Consistency Validation
# Validates all components are aligned and references are correct

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
PASSED=0
FAILED=0
WARNINGS=0

# Project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED++))
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    ((WARNINGS++))
}

# Check if file exists
check_file_exists() {
    local file="$1"
    local description="$2"
    
    if [[ -f "$PROJECT_ROOT/$file" ]]; then
        log_success "$description exists: $file"
        return 0
    else
        log_error "$description missing: $file"
        return 1
    fi
}

# Check if directory exists
check_directory_exists() {
    local dir="$1"
    local description="$2"
    
    if [[ -d "$PROJECT_ROOT/$dir" ]]; then
        log_success "$description exists: $dir"
        return 0
    else
        log_error "$description missing: $dir"
        return 1
    fi
}

# Validate file references within a file
validate_file_references() {
    local file="$1"
    local description="$2"
    
    if [[ ! -f "$PROJECT_ROOT/$file" ]]; then
        log_error "$description file not found: $file"
        return 1
    fi
    
    log_info "Validating references in $description..."
    
    # Check for references to non-existent configuration files
    local bad_configs=(
        "collector-improved.yaml"
        "collector-newrelic-optimized.yaml" 
        "collector-newrelic.yaml"
    )
    
    for config in "${bad_configs[@]}"; do
        if grep -q "$config" "$PROJECT_ROOT/$file" 2>/dev/null; then
            log_error "$description references non-existent config: $config"
            return 1
        fi
    done
    
    # Check for references to old deployment paths
    if grep -q "deployments/" "$PROJECT_ROOT/$file" 2>/dev/null; then
        log_error "$description references old deployments/ path"
        return 1
    fi
    
    log_success "$description has valid references"
    return 0
}

# Validate Go module consistency
validate_go_modules() {
    log_info "Validating Go module consistency..."
    
    local processors=(
        "processors/adaptivesampler"
        "processors/circuitbreaker" 
        "processors/planattributeextractor"
        "processors/verification"
    )
    
    local expected_version="v0.96.0"
    local expected_pdata_version="v1.3.0"
    
    for processor in "${processors[@]}"; do
        local go_mod="$PROJECT_ROOT/$processor/go.mod"
        
        if [[ ! -f "$go_mod" ]]; then
            log_error "Go module missing: $processor/go.mod"
            continue
        fi
        
        # Check OpenTelemetry version consistency
        if ! grep -q "go.opentelemetry.io/collector/component $expected_version" "$go_mod"; then
            log_error "$processor has wrong collector component version (expected $expected_version)"
            continue
        fi
        
        # Check pdata version
        if ! grep -q "go.opentelemetry.io/collector/pdata $expected_pdata_version" "$go_mod"; then
            log_error "$processor has wrong pdata version (expected $expected_pdata_version)"
            continue
        fi
        
        # Check module path
        if ! grep -q "module github.com/database-intelligence-mvp/$processor" "$go_mod"; then
            log_error "$processor has incorrect module path"
            continue
        fi
        
        log_success "$processor Go module is consistent"
    done
}

# Validate processor factory components
validate_processor_factories() {
    log_info "Validating processor factory component types..."
    
    local processors=(
        "adaptivesampler:adaptivesampler"
        "circuitbreaker:circuitbreaker"
        "planattributeextractor:planattributeextractor"
    )
    
    for processor_info in "${processors[@]}"; do
        IFS=':' read -r processor expected_type <<< "$processor_info"
        local factory_file="$PROJECT_ROOT/processors/$processor/factory.go"
        
        if [[ ! -f "$factory_file" ]]; then
            log_error "Factory file missing: processors/$processor/factory.go"
            continue
        fi
        
        if grep -q "component.MustNewType(\"$expected_type\")" "$factory_file"; then
            log_success "$processor factory has correct component type: $expected_type"
        else
            log_error "$processor factory has incorrect component type (expected $expected_type)"
        fi
    done
}

# Validate configuration files
validate_configurations() {
    log_info "Validating configuration files..."
    
    # Main configurations that should exist
    local configs=(
        "config/collector.yaml:Main collector configuration"
        "config/collector-unified.yaml:Unified collector configuration"
        "config/attribute-mapping.yaml:Attribute mapping configuration"
    )
    
    for config_info in "${configs[@]}"; do
        IFS=':' read -r config description <<< "$config_info"
        check_file_exists "$config" "$description"
    done
    
    # Check deployment configuration references
    local deployments=(
        "deploy/docker/docker-compose.yaml"
        "deploy/unified/docker-compose.yaml"
        "deploy/k8s/statefulset.yaml"
    )
    
    for deployment in "${deployments[@]}"; do
        if [[ -f "$PROJECT_ROOT/$deployment" ]]; then
            validate_file_references "$deployment" "Deployment $deployment"
        fi
    done
}

# Validate test and script paths
validate_test_scripts() {
    log_info "Validating test scripts and paths..."
    
    # Test scripts referenced in Makefile
    local test_scripts=(
        "tests/integration/test-postgresql.sh:PostgreSQL integration test"
        "tests/load/load-test.sh:Load test script"
        "tests/unit/processor_test.go:Unit test file"
    )
    
    for test_info in "${test_scripts[@]}"; do
        IFS=':' read -r test description <<< "$test_info"
        check_file_exists "$test" "$description"
    done
    
    # Processor directories for testing
    local processors=(
        "processors/adaptivesampler:Adaptive sampler processor"
        "processors/circuitbreaker:Circuit breaker processor"
        "processors/planattributeextractor:Plan attribute extractor processor"
        "processors/verification:Verification processor"
    )
    
    for processor_info in "${processors[@]}"; do
        IFS=':' read -r processor description <<< "$processor_info"
        check_directory_exists "$processor" "$description"
    done
}

# Validate documentation links
validate_documentation() {
    log_info "Validating documentation consistency..."
    
    # Files referenced in README.md
    local readme_links=(
        "ARCHITECTURE.md:Architecture documentation"
        "CONFIGURATION.md:Configuration documentation"  
        "DEPLOYMENT.md:Deployment documentation"
        "DEPLOYMENT-OPTIONS.md:Deployment options documentation"
        "EXPERIMENTAL-FEATURES.md:Experimental features documentation"
        "IMPLEMENTATION-GUIDE.md:Implementation guide"
        "OPERATIONS.md:Operations documentation"
        "TROUBLESHOOTING-GUIDE.md:Troubleshooting guide"
    )
    
    for link_info in "${readme_links[@]}"; do
        IFS=':' read -r link description <<< "$link_info"
        check_file_exists "$link" "$description"
    done
    
    # Validate key documentation files for outdated references
    local docs_to_validate=(
        "README.md"
        "DEPLOYMENT.md"
        "ARCHITECTURE.md" 
        "CONFIGURATION.md"
    )
    
    for doc in "${docs_to_validate[@]}"; do
        if [[ -f "$PROJECT_ROOT/$doc" ]]; then
            validate_file_references "$doc" "Documentation $doc"
        fi
    done
}

# Validate Makefile targets
validate_makefile() {
    log_info "Validating Makefile targets and paths..."
    
    local makefile="$PROJECT_ROOT/Makefile"
    
    if [[ ! -f "$makefile" ]]; then
        log_error "Makefile not found"
        return 1
    fi
    
    # Check for references to old deployment paths
    if grep -q "deployments/" "$makefile"; then
        log_error "Makefile references old deployments/ path"
        return 1
    fi
    
    # Check for references to non-existent targets
    if grep -q "deps" "$makefile" && ! grep -q "^deps:" "$makefile"; then
        log_error "Makefile references non-existent deps target"
        return 1
    fi
    
    # Check Docker target paths
    if grep -q "deploy/docker" "$makefile"; then
        log_success "Makefile Docker targets reference correct paths"
    else
        log_error "Makefile Docker targets missing or incorrect"
        return 1
    fi
    
    log_success "Makefile validation passed"
}

# Validate environment configuration
validate_environment() {
    log_info "Validating environment configuration..."
    
    # Check for .env.example
    check_file_exists ".env.example" "Environment example file"
    
    # Check for required environment variables in .env.example
    if [[ -f "$PROJECT_ROOT/.env.example" ]]; then
        local required_vars=(
            "NEW_RELIC_LICENSE_KEY"
            "PG_REPLICA_DSN" 
            "OTLP_ENDPOINT"
            "DEPLOYMENT_ENV"
        )
        
        for var in "${required_vars[@]}"; do
            if grep -q "^$var=" "$PROJECT_ROOT/.env.example"; then
                log_success "Environment variable $var documented in .env.example"
            else
                log_warning "Environment variable $var missing from .env.example"
            fi
        done
    fi
}

# Main validation function
main() {
    echo "Database Intelligence MVP - Project Consistency Validation"
    echo "========================================================"
    echo ""
    
    # Run all validations
    validate_configurations
    echo ""
    
    validate_go_modules
    echo ""
    
    validate_processor_factories
    echo ""
    
    validate_test_scripts
    echo ""
    
    validate_documentation
    echo ""
    
    validate_makefile
    echo ""
    
    validate_environment
    echo ""
    
    # Generate summary
    echo "========================================================"
    echo "Validation Summary:"
    echo "  Passed:   $PASSED"
    echo "  Failed:   $FAILED"
    echo "  Warnings: $WARNINGS"
    echo ""
    
    if [[ $FAILED -eq 0 ]]; then
        echo -e "${GREEN}✅ All validations passed!${NC}"
        if [[ $WARNINGS -gt 0 ]]; then
            echo -e "${YELLOW}⚠️  There are $WARNINGS warnings to review${NC}"
        fi
        exit 0
    else
        echo -e "${RED}❌ $FAILED validations failed${NC}"
        exit 1
    fi
}

# Run main function
main "$@"