#!/usr/bin/env python3
"""
PostgreSQL Unified Collector Demo
Simulates the behavior of the Rust collector for demonstration purposes
"""

import os
import json
import time
import psycopg2
import requests
from datetime import datetime
from typing import Dict, List, Any
import logging

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger('postgres-collector')

class PostgresUnifiedCollector:
    def __init__(self):
        self.config = {
            'host': os.getenv('POSTGRES_HOST', 'localhost'),
            'port': int(os.getenv('POSTGRES_PORT', 5432)),
            'user': os.getenv('POSTGRES_USER', 'postgres'),
            'password': os.getenv('POSTGRES_PASSWORD', 'postgres'),
            'database': os.getenv('POSTGRES_DB', 'postgres'),
            'collection_interval': int(os.getenv('COLLECTION_INTERVAL', 60)),
            'nri_enabled': os.getenv('NRI_ENABLED', 'true').lower() == 'true',
            'otlp_enabled': os.getenv('OTLP_ENABLED', 'true').lower() == 'true',
            'otlp_endpoint': os.getenv('OTLP_ENDPOINT', 'http://localhost:4317'),
            'new_relic_license_key': os.getenv('NEW_RELIC_LICENSE_KEY', '')
        }
        self.connection = None
        self.version = None
        self.capabilities = {}
        
    def connect(self):
        """Establish database connection"""
        try:
            self.connection = psycopg2.connect(
                host=self.config['host'],
                port=self.config['port'],
                user=self.config['user'],
                password=self.config['password'],
                database=self.config['database']
            )
            logger.info(f"Connected to PostgreSQL at {self.config['host']}:{self.config['port']}")
            return True
        except Exception as e:
            logger.error(f"Failed to connect to database: {e}")
            return False
    
    def detect_capabilities(self):
        """Detect PostgreSQL version and extensions"""
        with self.connection.cursor() as cursor:
            # Get version
            cursor.execute("SELECT current_setting('server_version_num')::integer / 10000 AS version")
            self.version = cursor.fetchone()[0]
            logger.info(f"PostgreSQL version: {self.version}")
            
            # Check extensions
            cursor.execute("SELECT extname FROM pg_extension")
            extensions = [row[0] for row in cursor.fetchall()]
            
            self.capabilities = {
                'version': self.version,
                'has_pg_stat_statements': 'pg_stat_statements' in extensions,
                'has_pg_stat_monitor': 'pg_stat_monitor' in extensions,
                'has_pg_wait_sampling': 'pg_wait_sampling' in extensions,
                'is_rds': self._check_is_rds()
            }
            logger.info(f"Capabilities: {self.capabilities}")
    
    def _check_is_rds(self):
        """Check if running on AWS RDS"""
        try:
            with self.connection.cursor() as cursor:
                cursor.execute("SELECT 1 FROM pg_settings WHERE name = 'rds.superuser_reserved_connections'")
                return cursor.fetchone() is not None
        except:
            return False
    
    def collect_slow_queries(self) -> List[Dict[str, Any]]:
        """Collect slow query metrics (OHI compatible)"""
        if not self.capabilities.get('has_pg_stat_statements'):
            logger.warning("pg_stat_statements not available, skipping slow query collection")
            return []
        
        query = """
            SELECT 
                'newrelic' as newrelic,
                queryid::text AS query_id,
                LEFT(query, 4095) AS query_text,
                current_database() AS database_name,
                current_schema() AS schema_name,
                calls AS execution_count,
                ROUND((total_exec_time / NULLIF(calls, 0))::numeric, 3) AS avg_elapsed_time_ms,
                shared_blks_read::float / NULLIF(calls, 0) AS avg_disk_reads,
                shared_blks_written::float / NULLIF(calls, 0) AS avg_disk_writes,
                CASE
                    WHEN query ILIKE 'SELECT%%' THEN 'SELECT'
                    WHEN query ILIKE 'INSERT%%' THEN 'INSERT'
                    WHEN query ILIKE 'UPDATE%%' THEN 'UPDATE'
                    WHEN query ILIKE 'DELETE%%' THEN 'DELETE'
                    ELSE 'OTHER'
                END AS statement_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM pg_stat_statements
            WHERE query NOT ILIKE 'EXPLAIN%%'
            ORDER BY total_exec_time DESC
            LIMIT 20
        """
        
        try:
            with self.connection.cursor() as cursor:
                cursor.execute(query)
                columns = [desc[0] for desc in cursor.description]
                return [dict(zip(columns, row)) for row in cursor.fetchall()]
        except Exception as e:
            logger.error(f"Failed to collect slow queries: {e}")
            return []
    
    def collect_blocking_sessions(self) -> List[Dict[str, Any]]:
        """Collect blocking session metrics"""
        if self.version < 12:
            logger.warning("PostgreSQL version too old for blocking session collection")
            return []
        
        query = """
            SELECT
                blocking.pid AS blocking_pid,
                blocked.pid AS blocked_pid,
                LEFT(blocking.query, 4095) AS blocking_query,
                LEFT(blocked.query, 4095) AS blocked_query,
                blocking.datname AS blocking_database,
                blocked.datname AS blocked_database,
                blocking.usename AS blocking_user,
                blocked.usename AS blocked_user,
                EXTRACT(EPOCH FROM (NOW() - blocking.query_start)) * 1000 AS blocking_duration_ms,
                EXTRACT(EPOCH FROM (NOW() - blocked.query_start)) * 1000 AS blocked_duration_ms,
                'AccessShareLock' AS lock_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM pg_stat_activity blocked
            JOIN pg_stat_activity blocking ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
            LIMIT 100
        """
        
        try:
            with self.connection.cursor() as cursor:
                cursor.execute(query)
                columns = [desc[0] for desc in cursor.description]
                return [dict(zip(columns, row)) for row in cursor.fetchall()]
        except Exception as e:
            logger.error(f"Failed to collect blocking sessions: {e}")
            return []
    
    def collect_wait_events(self) -> List[Dict[str, Any]]:
        """Collect wait event metrics"""
        # Simplified - just collect from pg_stat_activity
        query = """
            SELECT
                pid,
                wait_event_type,
                wait_event,
                0 AS wait_time_ms,
                state,
                usename,
                datname AS database_name,
                LEFT(query, 4095) AS query_text,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM pg_stat_activity
            WHERE state != 'idle'
                AND wait_event IS NOT NULL
            LIMIT 100
        """
        
        try:
            with self.connection.cursor() as cursor:
                cursor.execute(query)
                columns = [desc[0] for desc in cursor.description]
                return [dict(zip(columns, row)) for row in cursor.fetchall()]
        except Exception as e:
            logger.error(f"Failed to collect wait events: {e}")
            return []
    
    def format_nri_output(self, metrics: Dict[str, List[Dict]]) -> Dict:
        """Format metrics in NRI v4 protocol format"""
        entity_key = f"{self.config['host']}:{self.config['port']}"
        
        output = {
            "name": "com.newrelic.postgresql",
            "protocol_version": "4",
            "integration_version": "2.0.0",
            "data": [{
                "entity": {
                    "name": entity_key,
                    "type": "pg-instance",
                    "metrics": []
                },
                "common": {}
            }]
        }
        
        entity_metrics = output["data"][0]["entity"]["metrics"]
        
        # Add slow query metrics
        for metric in metrics.get('slow_queries', []):
            metric_set = {
                "event_type": "PostgresSlowQueries",
                **{k: v for k, v in metric.items() if v is not None}
            }
            entity_metrics.append(metric_set)
        
        # Add blocking sessions
        for metric in metrics.get('blocking_sessions', []):
            metric_set = {
                "event_type": "PostgresBlockingSessions",
                **{k: v for k, v in metric.items() if v is not None}
            }
            entity_metrics.append(metric_set)
        
        # Add wait events
        for metric in metrics.get('wait_events', []):
            metric_set = {
                "event_type": "PostgresWaitEvents",
                **{k: v for k, v in metric.items() if v is not None}
            }
            entity_metrics.append(metric_set)
        
        return output
    
    def format_otlp_metrics(self, metrics: Dict[str, List[Dict]]) -> List[Dict]:
        """Format metrics for OTLP/Prometheus export"""
        prometheus_metrics = []
        timestamp = int(time.time() * 1000)
        
        # Slow query metrics
        for metric in metrics.get('slow_queries', []):
            if metric.get('avg_elapsed_time_ms'):
                prometheus_metrics.append({
                    'name': 'postgresql_query_duration_milliseconds',
                    'value': metric['avg_elapsed_time_ms'],
                    'timestamp': timestamp,
                    'labels': {
                        'query_id': str(metric.get('query_id', '')),
                        'database': metric.get('database_name', ''),
                        'statement_type': metric.get('statement_type', '')
                    }
                })
            
            if metric.get('execution_count'):
                prometheus_metrics.append({
                    'name': 'postgresql_query_executions_total',
                    'value': metric['execution_count'],
                    'timestamp': timestamp,
                    'labels': {
                        'query_id': str(metric.get('query_id', '')),
                        'database': metric.get('database_name', '')
                    }
                })
        
        # Blocking sessions
        blocking_count = len(metrics.get('blocking_sessions', []))
        if blocking_count > 0:
            prometheus_metrics.append({
                'name': 'postgresql_blocking_sessions_total',
                'value': blocking_count,
                'timestamp': timestamp,
                'labels': {
                    'instance': f"{self.config['host']}:{self.config['port']}"
                }
            })
        
        return prometheus_metrics
    
    def send_metrics(self, metrics: Dict[str, List[Dict]]):
        """Send metrics to configured outputs"""
        # Log summary
        logger.info(f"Collected metrics: {len(metrics.get('slow_queries', []))} slow queries, "
                   f"{len(metrics.get('blocking_sessions', []))} blocking sessions, "
                   f"{len(metrics.get('wait_events', []))} wait events")
        
        # Send NRI format
        if self.config['nri_enabled']:
            nri_output = self.format_nri_output(metrics)
            logger.info("NRI Output (first 500 chars):")
            logger.info(json.dumps(nri_output, indent=2)[:500])
        
        # Send OTLP format
        if self.config['otlp_enabled']:
            otlp_metrics = self.format_otlp_metrics(metrics)
            logger.info(f"OTLP Metrics: {len(otlp_metrics)} metric points")
            # In a real implementation, these would be sent to the OTLP endpoint
    
    def run(self):
        """Main collection loop"""
        logger.info("Starting PostgreSQL Unified Collector Demo")
        
        if not self.connect():
            return
        
        self.detect_capabilities()
        
        while True:
            try:
                start_time = time.time()
                
                # Collect all metrics
                metrics = {
                    'slow_queries': self.collect_slow_queries(),
                    'blocking_sessions': self.collect_blocking_sessions(),
                    'wait_events': self.collect_wait_events()
                }
                
                # Send metrics
                self.send_metrics(metrics)
                
                # Calculate collection duration
                duration = time.time() - start_time
                logger.info(f"Collection completed in {duration:.2f} seconds")
                
                # Wait for next interval
                sleep_time = max(0, self.config['collection_interval'] - duration)
                time.sleep(sleep_time)
                
            except KeyboardInterrupt:
                logger.info("Shutting down...")
                break
            except Exception as e:
                logger.error(f"Collection error: {e}")
                time.sleep(10)  # Wait before retry

if __name__ == "__main__":
    collector = PostgresUnifiedCollector()
    collector.run()