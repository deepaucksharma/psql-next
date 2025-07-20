#!/bin/bash
# Script to validate metric types and data consistency

set -e

echo "=== MySQL Intelligence Metric Validation ==="
echo "Checking metric definitions for type consistency..."
echo

MONOREPO_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-monorepo"
ERRORS=0
WARNINGS=0

# Function to check metric suffix consistency
check_metric_suffix() {
    local metric_name=$1
    local value_column=$2
    local file=$3
    
    # Check if metric suffix matches value column type
    if [[ "$metric_name" == *"_total" ]] || [[ "$metric_name" == *"_count" ]]; then
        if [[ "$value_column" == *"_ms" ]] || [[ "$value_column" == *"_sec" ]] || [[ "$value_column" == *"_percent" ]]; then
            echo "ERROR: Counter metric '$metric_name' uses time/percent value column '$value_column' in $file"
            ((ERRORS++))
        fi
    fi
    
    if [[ "$metric_name" == *"_ms" ]]; then
        if [[ "$value_column" != *"_ms" ]] && [[ "$value_column" != *"latency_ms" ]] && [[ "$value_column" != *"time_ms" ]]; then
            echo "WARNING: Time metric '$metric_name' uses non-time value column '$value_column' in $file"
            ((WARNINGS++))
        fi
    fi
    
    if [[ "$metric_name" == *"_bytes" ]]; then
        if [[ "$value_column" != *"bytes"* ]]; then
            echo "WARNING: Bytes metric '$metric_name' uses non-bytes value column '$value_column' in $file"
            ((WARNINGS++))
        fi
    fi
}

# Function to check for string metrics
check_string_metrics() {
    local file=$1
    
    # Check for known string columns being used as metrics
    if grep -q "value_column: Slave_IO_Running" "$file" 2>/dev/null; then
        echo "ERROR: String value 'Slave_IO_Running' used as metric in $file"
        echo "  -> Should be converted to numeric (1/0) in SQL query"
        ((ERRORS++))
    fi
    
    if grep -q "value_column: Slave_SQL_Running" "$file" 2>/dev/null; then
        echo "ERROR: String value 'Slave_SQL_Running' used as metric in $file"
        echo "  -> Should be converted to numeric (1/0) in SQL query"
        ((ERRORS++))
    fi
}

# Function to check metric naming consistency
check_metric_naming() {
    local file=$1
    local module=$(basename $(dirname $(dirname "$file")))
    
    # Extract metric definitions
    while IFS= read -r line; do
        if [[ "$line" =~ metric_name:[[:space:]]*([a-zA-Z0-9._]+) ]]; then
            metric_name="${BASH_REMATCH[1]}"
            
            # Check module prefix
            case "$module" in
                "core-metrics")
                    if [[ "$metric_name" != mysql.* ]]; then
                        echo "WARNING: Metric '$metric_name' in $module should start with 'mysql.' prefix"
                        ((WARNINGS++))
                    fi
                    ;;
                "sql-intelligence")
                    if [[ "$metric_name" != mysql.query.* ]] && [[ "$metric_name" != mysql.table.* ]] && [[ "$metric_name" != mysql.index.* ]]; then
                        echo "WARNING: Metric '$metric_name' in $module should start with 'mysql.query.', 'mysql.table.', or 'mysql.index.' prefix"
                        ((WARNINGS++))
                    fi
                    ;;
                "wait-profiler")
                    if [[ "$metric_name" != mysql.wait.* ]] && [[ "$metric_name" != mysql.lock.* ]] && [[ "$metric_name" != mysql.io.* ]]; then
                        echo "WARNING: Metric '$metric_name' in $module should start with 'mysql.wait.', 'mysql.lock.', or 'mysql.io.' prefix"
                        ((WARNINGS++))
                    fi
                    ;;
            esac
            
            # Check for old underscore format
            if [[ "$metric_name" == *"_"* ]] && [[ "$metric_name" != *"_total" ]] && [[ "$metric_name" != *"_count" ]] && [[ "$metric_name" != *"_ms" ]] && [[ "$metric_name" != *"_bytes" ]] && [[ "$metric_name" != *"_current" ]] && [[ "$metric_name" != *"_percent" ]]; then
                echo "INFO: Metric '$metric_name' uses underscore in middle (consider dots for namespacing)"
            fi
        fi
    done < "$file"
}

# Function to check SQL query consistency
check_sql_queries() {
    local file=$1
    
    # Check for missing performance_schema prefix
    if grep -q "FROM events_statements_summary_by_digest" "$file" 2>/dev/null; then
        echo "WARNING: SQL query missing 'performance_schema.' prefix in $file"
        ((WARNINGS++))
    fi
    
    # Check for incorrect time conversions
    if grep -q "/1000000000000" "$file" 2>/dev/null; then
        if ! grep -q "as.*_sec" "$file" 2>/dev/null; then
            echo "INFO: Found picosecond to second conversion but no _sec suffix in $file"
        fi
    fi
}

# Main validation loop
echo "Checking collector configurations..."
echo

for config in "$MONOREPO_ROOT/modules/*/config/collector"*.yaml; do
    if [ -f "$config" ]; then
        echo "Validating: $config"
        
        # Check for string metrics
        check_string_metrics "$config"
        
        # Check metric naming consistency
        check_metric_naming "$config"
        
        # Check SQL queries
        check_sql_queries "$config"
        
        # Check metric type consistency
        while IFS= read -r metric_line; do
            if [[ "$metric_line" =~ metric_name:[[:space:]]*([a-zA-Z0-9._]+) ]]; then
                metric_name="${BASH_REMATCH[1]}"
                
                # Look for the next value_column line
                value_line=$(grep -A5 "metric_name:[[:space:]]*$metric_name" "$config" | grep "value_column:" | head -1)
                if [[ "$value_line" =~ value_column:[[:space:]]*([a-zA-Z0-9._]+) ]]; then
                    value_column="${BASH_REMATCH[1]}"
                    check_metric_suffix "$metric_name" "$value_column" "$config"
                fi
            fi
        done < <(grep "metric_name:" "$config")
    fi
done

echo
echo "=== Validation Summary ==="
echo "Errors found: $ERRORS"
echo "Warnings found: $WARNINGS"
echo

if [ $ERRORS -gt 0 ]; then
    echo "❌ Validation failed with $ERRORS errors"
    exit 1
else
    echo "✅ Validation passed"
    if [ $WARNINGS -gt 0 ]; then
        echo "   (with $WARNINGS warnings to review)"
    fi
fi