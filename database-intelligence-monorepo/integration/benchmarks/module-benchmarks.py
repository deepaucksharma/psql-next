#!/usr/bin/env python3
"""
Module-specific Benchmark Tests for Database Intelligence System

Tests performance of individual monitoring modules including:
- Connection monitoring
- Query performance tracking
- Lock monitoring
- Replication monitoring
- Resource usage tracking
"""

import time
import json
import threading
import random
import mysql.connector
from typing import Dict, List, Any
from pathlib import Path
import sys
import os

# Add parent directory to path
sys.path.append(str(Path(__file__).parent.parent.parent))

from benchmark_framework import BenchmarkFramework, BenchmarkResult


class ModuleBenchmarks:
    """Benchmark tests for individual monitoring modules"""
    
    def __init__(self, db_config: Dict[str, Any]):
        self.db_config = db_config
        self.framework = BenchmarkFramework("Module Performance Benchmarks")
        self.connection = None
        
    def setup(self):
        """Setup database connection"""
        self.connection = mysql.connector.connect(**self.db_config)
        self.cursor = self.connection.cursor()
        
    def teardown(self):
        """Cleanup database connection"""
        if self.cursor:
            self.cursor.close()
        if self.connection:
            self.connection.close()
            
    def run_all_benchmarks(self):
        """Run all module benchmarks"""
        print("Starting module benchmarks...")
        
        try:
            self.setup()
            
            # Benchmark each module
            self.benchmark_connection_module()
            self.benchmark_query_module()
            self.benchmark_lock_module()
            self.benchmark_replication_module()
            self.benchmark_resource_module()
            self.benchmark_metric_collection_pipeline()
            
        finally:
            self.teardown()
            
        # Print and save results
        self.framework.print_summary()
        self.framework.save_results("module_benchmark_results.json")
        
    def benchmark_connection_module(self):
        """Benchmark connection monitoring module"""
        print("\n=== Connection Module Benchmarks ===")
        
        # Test connection status query
        def check_connections():
            self.cursor.execute("""
                SELECT COUNT(*) as total,
                       SUM(CASE WHEN command = 'Sleep' THEN 1 ELSE 0 END) as idle,
                       SUM(CASE WHEN command != 'Sleep' THEN 1 ELSE 0 END) as active
                FROM information_schema.processlist
            """)
            return self.cursor.fetchall()
            
        self.framework.time_function(
            check_connections,
            name="Connection status query",
            category="connection_module",
            iterations=100
        )
        
        # Test connection details query
        def get_connection_details():
            self.cursor.execute("""
                SELECT id, user, host, db, command, time, state
                FROM information_schema.processlist
                WHERE command != 'Sleep'
                LIMIT 100
            """)
            return self.cursor.fetchall()
            
        self.framework.time_function(
            get_connection_details,
            name="Active connection details",
            category="connection_module",
            iterations=100
        )
        
        # Test connection pool monitoring
        with self.framework.benchmark("Connection pool monitoring", "connection_module", iterations=50) as result:
            for _ in range(result.iterations):
                # Simulate connection pool checks
                connections = []
                for i in range(5):
                    conn = mysql.connector.connect(**self.db_config)
                    connections.append(conn)
                    
                # Check pool status
                for conn in connections:
                    conn.ping(reconnect=False)
                    
                # Close connections
                for conn in connections:
                    conn.close()
                    
    def benchmark_query_module(self):
        """Benchmark query performance tracking module"""
        print("\n=== Query Module Benchmarks ===")
        
        # Test slow query detection
        def check_slow_queries():
            self.cursor.execute("""
                SELECT query_time, lock_time, rows_examined, sql_text
                FROM mysql.slow_log
                WHERE query_time > 1
                ORDER BY query_time DESC
                LIMIT 100
            """)
            return self.cursor.fetchall()
            
        # Create slow_log table if it doesn't exist
        try:
            self.cursor.execute("CREATE TABLE IF NOT EXISTS mysql.slow_log LIKE mysql.general_log")
        except:
            pass
            
        self.framework.time_function(
            check_slow_queries,
            name="Slow query detection",
            category="query_module",
            iterations=50
        )
        
        # Test query statistics
        def get_query_stats():
            self.cursor.execute("""
                SELECT 
                    SUBSTRING(sql_text, 1, 50) as query_pattern,
                    COUNT(*) as execution_count,
                    AVG(query_time) as avg_time,
                    MAX(query_time) as max_time
                FROM mysql.slow_log
                GROUP BY query_pattern
                ORDER BY execution_count DESC
                LIMIT 50
            """)
            return self.cursor.fetchall()
            
        self.framework.time_function(
            get_query_stats,
            name="Query statistics aggregation",
            category="query_module",
            iterations=50
        )
        
        # Test real-time query monitoring
        with self.framework.benchmark("Real-time query monitoring", "query_module", iterations=100) as result:
            for _ in range(result.iterations):
                self.cursor.execute("""
                    SELECT id, user, host, db, command, time, state, info
                    FROM information_schema.processlist
                    WHERE command NOT IN ('Sleep', 'Binlog Dump')
                    AND time > 0
                """)
                active_queries = self.cursor.fetchall()
                result.metrics['active_queries'] = len(active_queries)
                
    def benchmark_lock_module(self):
        """Benchmark lock monitoring module"""
        print("\n=== Lock Module Benchmarks ===")
        
        # Test lock detection
        def check_locks():
            self.cursor.execute("""
                SELECT 
                    l.lock_type,
                    l.lock_mode,
                    l.lock_status,
                    l.lock_data
                FROM information_schema.innodb_locks l
            """)
            return self.cursor.fetchall()
            
        self.framework.time_function(
            check_locks,
            name="Lock detection query",
            category="lock_module",
            iterations=100
        )
        
        # Test lock wait detection
        def check_lock_waits():
            self.cursor.execute("""
                SELECT 
                    r.trx_id AS waiting_trx_id,
                    r.trx_mysql_thread_id AS waiting_thread,
                    r.trx_query AS waiting_query,
                    b.trx_id AS blocking_trx_id,
                    b.trx_mysql_thread_id AS blocking_thread,
                    b.trx_query AS blocking_query
                FROM information_schema.innodb_lock_waits w
                JOIN information_schema.innodb_trx r ON r.trx_id = w.requesting_trx_id
                JOIN information_schema.innodb_trx b ON b.trx_id = w.blocking_trx_id
            """)
            return self.cursor.fetchall()
            
        self.framework.time_function(
            check_lock_waits,
            name="Lock wait detection",
            category="lock_module",
            iterations=100
        )
        
        # Test deadlock monitoring
        with self.framework.benchmark("Deadlock monitoring", "lock_module", iterations=50) as result:
            for _ in range(result.iterations):
                self.cursor.execute("SHOW ENGINE INNODB STATUS")
                status = self.cursor.fetchone()
                if status and len(status) > 2:
                    # Parse deadlock information
                    status_text = status[2]
                    deadlock_found = "LATEST DETECTED DEADLOCK" in status_text
                    result.metrics['deadlocks_detected'] = result.metrics.get('deadlocks_detected', 0) + (1 if deadlock_found else 0)
                    
    def benchmark_replication_module(self):
        """Benchmark replication monitoring module"""
        print("\n=== Replication Module Benchmarks ===")
        
        # Test replication status
        def check_replication_status():
            try:
                self.cursor.execute("SHOW SLAVE STATUS")
                return self.cursor.fetchall()
            except:
                # Try replica status for newer versions
                try:
                    self.cursor.execute("SHOW REPLICA STATUS")
                    return self.cursor.fetchall()
                except:
                    return []
                    
        self.framework.time_function(
            check_replication_status,
            name="Replication status check",
            category="replication_module",
            iterations=100
        )
        
        # Test replication lag calculation
        with self.framework.benchmark("Replication lag calculation", "replication_module", iterations=50) as result:
            for _ in range(result.iterations):
                status = check_replication_status()
                if status and len(status) > 0:
                    # Calculate lag from status
                    row = status[0]
                    if len(row) > 32:  # Check if Seconds_Behind_Master field exists
                        lag = row[32]  # Seconds_Behind_Master position
                        result.metrics['max_lag'] = max(result.metrics.get('max_lag', 0), lag or 0)
                        
    def benchmark_resource_module(self):
        """Benchmark resource monitoring module"""
        print("\n=== Resource Module Benchmarks ===")
        
        # Test system variable monitoring
        def check_system_variables():
            self.cursor.execute("""
                SELECT variable_name, variable_value
                FROM information_schema.global_variables
                WHERE variable_name IN (
                    'max_connections', 'thread_cache_size', 
                    'innodb_buffer_pool_size', 'query_cache_size'
                )
            """)
            return self.cursor.fetchall()
            
        self.framework.time_function(
            check_system_variables,
            name="System variable check",
            category="resource_module",
            iterations=100
        )
        
        # Test status variable monitoring
        def check_status_variables():
            self.cursor.execute("""
                SELECT variable_name, variable_value
                FROM information_schema.global_status
                WHERE variable_name IN (
                    'Threads_connected', 'Threads_running',
                    'Questions', 'Slow_queries', 'Connections'
                )
            """)
            return self.cursor.fetchall()
            
        self.framework.time_function(
            check_status_variables,
            name="Status variable check",
            category="resource_module",
            iterations=100
        )
        
        # Test buffer pool monitoring
        with self.framework.benchmark("Buffer pool monitoring", "resource_module", iterations=50) as result:
            for _ in range(result.iterations):
                self.cursor.execute("""
                    SELECT 
                        pool_id,
                        pool_size,
                        free_buffers,
                        database_pages,
                        dirty_pages,
                        pages_read,
                        pages_written
                    FROM information_schema.innodb_buffer_pool_stats
                """)
                stats = self.cursor.fetchall()
                result.metrics['buffer_pools'] = len(stats)
                
    def benchmark_metric_collection_pipeline(self):
        """Benchmark the entire metric collection pipeline"""
        print("\n=== Metric Collection Pipeline Benchmarks ===")
        
        # Test full metric collection cycle
        def collect_all_metrics():
            metrics = {}
            
            # Connection metrics
            self.cursor.execute("SELECT COUNT(*) FROM information_schema.processlist")
            metrics['connections'] = self.cursor.fetchone()[0]
            
            # Query metrics
            self.cursor.execute("""
                SELECT COUNT(*) 
                FROM information_schema.processlist 
                WHERE command NOT IN ('Sleep', 'Binlog Dump')
            """)
            metrics['active_queries'] = self.cursor.fetchone()[0]
            
            # Status metrics
            self.cursor.execute("""
                SELECT variable_name, variable_value
                FROM information_schema.global_status
                WHERE variable_name IN ('Questions', 'Slow_queries', 'Threads_running')
            """)
            for name, value in self.cursor.fetchall():
                metrics[name.lower()] = int(value)
                
            return metrics
            
        self.framework.time_function(
            collect_all_metrics,
            name="Full metric collection cycle",
            category="pipeline",
            iterations=100
        )
        
        # Test concurrent metric collection
        with self.framework.benchmark("Concurrent metric collection", "pipeline", iterations=10) as result:
            for _ in range(result.iterations):
                threads = []
                results = []
                
                def collect_metrics_thread():
                    conn = mysql.connector.connect(**self.db_config)
                    cursor = conn.cursor()
                    
                    # Simulate metric collection
                    cursor.execute("SELECT COUNT(*) FROM information_schema.processlist")
                    count = cursor.fetchone()[0]
                    results.append(count)
                    
                    cursor.close()
                    conn.close()
                    
                # Create multiple collection threads
                for i in range(5):
                    t = threading.Thread(target=collect_metrics_thread)
                    threads.append(t)
                    t.start()
                    
                # Wait for all threads
                for t in threads:
                    t.join()
                    
                result.metrics['concurrent_collections'] = len(results)


def main():
    """Main entry point"""
    # Database configuration
    db_config = {
        'host': os.getenv('MYSQL_HOST', 'localhost'),
        'port': int(os.getenv('MYSQL_PORT', '3306')),
        'user': os.getenv('MYSQL_USER', 'root'),
        'password': os.getenv('MYSQL_PASSWORD', ''),
        'database': os.getenv('MYSQL_DATABASE', 'mysql')
    }
    
    # Run benchmarks
    benchmarks = ModuleBenchmarks(db_config)
    benchmarks.run_all_benchmarks()


if __name__ == "__main__":
    main()