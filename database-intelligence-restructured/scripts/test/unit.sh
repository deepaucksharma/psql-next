#!/bin/bash
# Unit test runner for Database Intelligence

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$SCRIPT_DIR/../utils/common.sh"

# Configuration
DATABASE=${1:-all}
COVERAGE_ENABLED=${COVERAGE_ENABLED:-true}
TEST_TIMEOUT=${TEST_TIMEOUT:-300}

print_header "Unit Tests"
log_info "Database: $DATABASE"
log_info "Coverage: $COVERAGE_ENABLED"

# Change to root directory
cd "$ROOT_DIR"

# Determine which packages to test
if [[ "$DATABASE" == "all" ]]; then
    TEST_PACKAGES="./..."
else
    case "$DATABASE" in
        postgresql)
            TEST_PACKAGES="./components/receivers/ash/... ./internal/database/postgres/..."
            ;;
        mysql)
            TEST_PACKAGES="./components/receivers/enhancedsql/... ./internal/database/mysql/..."
            ;;
        mongodb)
            TEST_PACKAGES="./components/receivers/mongodb/..."
            ;;
        redis)
            TEST_PACKAGES="./components/receivers/redis/..."
            ;;
        *)
            log_error "Unknown database: $DATABASE"
            exit 1
            ;;
    esac
fi

# Run tests
log_info "Running unit tests..."

if [[ "$COVERAGE_ENABLED" == "true" ]]; then
    # Run with coverage
    if go test -v -timeout "${TEST_TIMEOUT}s" -coverprofile=coverage.out -covermode=atomic $TEST_PACKAGES; then
        log_success "Unit tests passed"
        
        # Generate coverage report
        log_info "Generating coverage report..."
        go tool cover -html=coverage.out -o coverage.html
        
        # Display coverage summary
        COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
        log_info "Total coverage: $COVERAGE"
        
        # Check coverage threshold
        THRESHOLD=${COVERAGE_THRESHOLD:-80}
        COVERAGE_NUM=$(echo $COVERAGE | sed 's/%//')
        if (( $(echo "$COVERAGE_NUM < $THRESHOLD" | bc -l) )); then
            log_warning "Coverage $COVERAGE is below threshold of ${THRESHOLD}%"
        else
            log_success "Coverage $COVERAGE meets threshold of ${THRESHOLD}%"
        fi
    else
        log_error "Unit tests failed"
        exit 1
    fi
else
    # Run without coverage
    if go test -v -timeout "${TEST_TIMEOUT}s" $TEST_PACKAGES; then
        log_success "Unit tests passed"
    else
        log_error "Unit tests failed"
        exit 1
    fi
fi

# Run specific component tests if they exist
if [[ -d "$ROOT_DIR/components" ]]; then
    log_info "Running component-specific tests..."
    
    for component in receivers processors exporters extensions; do
        if [[ -d "$ROOT_DIR/components/$component" ]]; then
            log_info "Testing $component..."
            
            for dir in "$ROOT_DIR/components/$component"/*; do
                if [[ -d "$dir" ]] && [[ -f "$dir/go.mod" ]]; then
                    component_name=$(basename "$dir")
                    
                    # Skip if not related to selected database
                    if [[ "$DATABASE" != "all" ]]; then
                        case "$DATABASE" in
                            postgresql)
                                [[ "$component_name" =~ ^(ash|enhancedsql|planattributeextractor)$ ]] || continue
                                ;;
                            mysql)
                                [[ "$component_name" =~ ^(enhancedsql)$ ]] || continue
                                ;;
                            mongodb)
                                [[ "$component_name" =~ ^(mongodb)$ ]] || continue
                                ;;
                            redis)
                                [[ "$component_name" =~ ^(redis)$ ]] || continue
                                ;;
                        esac
                    fi
                    
                    log_info "Testing component: $component_name"
                    (cd "$dir" && go test -v ./... -timeout "${TEST_TIMEOUT}s") || {
                        log_error "Tests failed for $component_name"
                        exit 1
                    }
                fi
            done
        fi
    done
fi

log_success "All unit tests completed successfully"