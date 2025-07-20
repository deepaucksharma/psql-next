#!/bin/bash
# Script to standardize metric names across all modules

set -e

echo "=== MySQL Intelligence Metric Standardization ==="
echo "This script will update metric names to follow the standard naming convention"
echo

# Function to backup a file
backup_file() {
    local file=$1
    cp "$file" "${file}.backup-$(date +%Y%m%d-%H%M%S)"
    echo "Backed up: $file"
}

# Function to standardize metrics in a file
standardize_metrics() {
    local file=$1
    local module=$2
    
    echo "Processing: $file (module: $module)"
    
    case "$module" in
        "sql-intelligence")
            # Change mysql_query_* to mysql.query.*
            sed -i '' 's/metric_name: mysql_query_exec_count/metric_name: mysql.query.exec_total/g' "$file"
            sed -i '' 's/metric_name: mysql_query_latency_total/metric_name: mysql.query.latency_ms/g' "$file"
            sed -i '' 's/metric_name: mysql_query_latency_avg/metric_name: mysql.query.latency_avg_ms/g' "$file"
            sed -i '' 's/metric_name: mysql_query_latency_max/metric_name: mysql.query.latency_max_ms/g' "$file"
            sed -i '' 's/metric_name: mysql_query_rows_examined/metric_name: mysql.query.rows_examined_total/g' "$file"
            sed -i '' 's/metric_name: mysql_query_rows_sent/metric_name: mysql.query.rows_sent_total/g' "$file"
            sed -i '' 's/metric_name: mysql_query_no_index_used/metric_name: mysql.query.no_index_used_total/g' "$file"
            sed -i '' 's/metric_name: mysql_query_no_good_index_used/metric_name: mysql.query.no_good_index_used_total/g' "$file"
            
            # Change mysql_table_* to mysql.table.*
            sed -i '' 's/metric_name: mysql_table_io_read_count/metric_name: mysql.table.io.read.count/g' "$file"
            sed -i '' 's/metric_name: mysql_table_io_write_count/metric_name: mysql.table.io.write.count/g' "$file"
            sed -i '' 's/metric_name: mysql_table_io_read_latency/metric_name: mysql.table.io.read.latency_ms/g' "$file"
            sed -i '' 's/metric_name: mysql_table_io_write_latency/metric_name: mysql.table.io.write.latency_ms/g' "$file"
            sed -i '' 's/metric_name: mysql_table_io_misc_latency/metric_name: mysql.table.io.misc.latency_ms/g' "$file"
            sed -i '' 's/metric_name: mysql_table_rows_fetched/metric_name: mysql.table.rows.fetched_total/g' "$file"
            sed -i '' 's/metric_name: mysql_table_rows_inserted/metric_name: mysql.table.rows.inserted_total/g' "$file"
            sed -i '' 's/metric_name: mysql_table_rows_updated/metric_name: mysql.table.rows.updated_total/g' "$file"
            sed -i '' 's/metric_name: mysql_table_rows_deleted/metric_name: mysql.table.rows.deleted_total/g' "$file"
            
            # Change mysql_index_* to mysql.index.*
            sed -i '' 's/metric_name: mysql_index_cardinality/metric_name: mysql.index.cardinality/g' "$file"
            sed -i '' 's/metric_name: mysql_index_not_used/metric_name: mysql.index.not_used_total/g' "$file"
            ;;
            
        "wait-profiler")
            # Change mysql.wait.* to include proper suffixes
            sed -i '' 's/metric_name: mysql\.wait\.count/metric_name: mysql.wait.count/g' "$file"
            sed -i '' 's/metric_name: mysql\.wait\.time\.total/metric_name: mysql.wait.time_ms/g' "$file"
            sed -i '' 's/metric_name: mysql\.wait\.time\.avg/metric_name: mysql.wait.time_avg_ms/g' "$file"
            sed -i '' 's/metric_name: mysql\.wait\.time\.max/metric_name: mysql.wait.time_max_ms/g' "$file"
            
            # Change mysql.mutex.* to mysql.wait.mutex.*
            sed -i '' 's/metric_name: mysql\.mutex\.wait\.count/metric_name: mysql.wait.mutex.count/g' "$file"
            sed -i '' 's/metric_name: mysql\.mutex\.wait\.time/metric_name: mysql.wait.mutex.time_ms/g' "$file"
            
            # Change mysql.io.* to mysql.wait.io.*
            sed -i '' 's/metric_name: mysql\.io\.wait\.count/metric_name: mysql.wait.io.count/g' "$file"
            sed -i '' 's/metric_name: mysql\.io\.wait\.time/metric_name: mysql.wait.io.time_ms/g' "$file"
            sed -i '' 's/metric_name: mysql\.io\.bytes\.read/metric_name: mysql.io.bytes_read/g' "$file"
            sed -i '' 's/metric_name: mysql\.io\.bytes\.write/metric_name: mysql.io.bytes_written/g' "$file"
            
            # Change mysql.lock.* to include proper suffixes
            sed -i '' 's/metric_name: mysql\.lock\.active\.count/metric_name: mysql.lock.active_count/g' "$file"
            ;;
            
        "core-metrics")
            # Core metrics already follow the standard mostly
            # Just ensure consistency with dots
            sed -i '' 's/mysql_connections/mysql.connections/g' "$file"
            sed -i '' 's/mysql_threads/mysql.threads/g' "$file"
            sed -i '' 's/mysql_operations/mysql.operations/g' "$file"
            ;;
    esac
    
    # Update references in transform processors
    sed -i '' 's/name == "mysql_connections_current"/name == "mysql.connections.current"/g' "$file"
    sed -i '' 's/name == "mysql_query_exec_count"/name == "mysql.query.exec_total"/g' "$file"
    
    echo "Standardized metrics in: $file"
}

# Main execution
MONOREPO_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-monorepo"

# Process each module
for module in sql-intelligence wait-profiler core-metrics; do
    echo
    echo "=== Processing module: $module ==="
    
    # Process collector configs
    for config in "$MONOREPO_ROOT/modules/$module/config/collector"*.yaml; do
        if [ -f "$config" ]; then
            backup_file "$config"
            standardize_metrics "$config" "$module"
        fi
    done
done

echo
echo "=== Metric standardization complete ==="
echo "Backup files created with .backup-* extension"
echo "Review changes and test before committing"