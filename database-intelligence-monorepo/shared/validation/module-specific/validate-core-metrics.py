#!/usr/bin/env python3
"""
Core Metrics Module Specific Validation Tool

This tool provides deep validation for the core-metrics module (port 8081).
It validates MySQL connection metrics, thread management, and foundational data.

Usage:
    python3 validate-core-metrics.py --api-key YOUR_API_KEY --account-id YOUR_ACCOUNT_ID
    python3 validate-core-metrics.py --check-connections --check-threads --check-buffer-pool
"""

import argparse
import json
import requests
import sys
import time
import os
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Tuple
import logging
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class CoreMetricsValidator:
    def __init__(self, api_key: str, account_id: str):
        self.api_key = api_key
        self.account_id = account_id
        self.module_name = "core-metrics"
        self.module_port = 8081
        self.session = requests.Session()
        self.session.headers.update({
            'Api-Key': api_key,
            'Content-Type': 'application/json'
        })
        
        # Core metrics expected metrics
        self.expected_metrics = {
            'connections': [
                'mysql_connections_current',
                'mysql_connections_available', 
                'mysql_max_connections',
                'mysql_aborted_connects',
                'mysql_threads_connected'
            ],
            'threads': [
                'mysql_threads_running',
                'mysql_threads_created',
                'mysql_threads_cached'
            ],
            'operations': [
                'mysql_queries',
                'mysql_slow_queries_total',
                'mysql_qcache_hits',
                'mysql_qcache_inserts'
            ],
            'buffer_pool': [
                'mysql_buffer_pool_pages_dirty',
                'mysql_buffer_pool_pages_free',
                'mysql_buffer_pool_pages_total',
                'mysql_buffer_pool_read_requests',
                'mysql_buffer_pool_reads'
            ],
            'innodb': [
                'mysql_innodb_data_reads',
                'mysql_innodb_data_writes',
                'mysql_innodb_log_waits',
                'mysql_innodb_row_lock_waits'
            ]
        }
        
        # Validation thresholds
        self.thresholds = {
            'connection_utilization_warn': 0.8,    # 80% connection usage
            'connection_utilization_critical': 0.95, # 95% connection usage
            'thread_ratio_warn': 0.5,              # 50% threads running vs connected
            'buffer_pool_hit_ratio_warn': 0.99,    # 99% buffer pool hit ratio
            'dirty_pages_ratio_warn': 0.75,        # 75% dirty pages ratio
            'aborted_connects_rate_warn': 5.0      # 5 aborted connects per minute
        }

    def execute_nrql(self, query: str) -> Optional[List[Dict]]:
        """Execute NRQL query against New Relic"""
        graphql_query = {
            "query": f"""
            {{
                actor {{
                    account(id: {self.account_id}) {{
                        nrql(query: "{query}") {{
                            results
                        }}
                    }}
                }}
            }}
            """
        }
        
        try:
            response = self.session.post(
                "https://api.newrelic.com/graphql",
                json=graphql_query,
                timeout=30
            )
            response.raise_for_status()
            
            data = response.json()
            if 'errors' in data:
                logger.error(f"GraphQL errors: {data['errors']}")
                return None
                
            return data['data']['actor']['account']['nrql']['results']
            
        except Exception as e:
            logger.error(f"Failed to execute NRQL: {e}")
            return None

    def validate_connection_metrics(self) -> Dict:
        """Validate MySQL connection-related metrics"""
        logger.info("Validating connection metrics...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check connection utilization
        query = """
        SELECT latest(mysql_connections_current) as 'current_connections',
               latest(mysql_max_connections) as 'max_connections',
               latest(mysql_threads_connected) as 'threads_connected',
               latest(mysql_aborted_connects) as 'aborted_connects'
        FROM Metric 
        WHERE service.name = 'core-metrics'
          AND metricName IN ('mysql_connections_current', 'mysql_max_connections', 'mysql_threads_connected', 'mysql_aborted_connects')
        SINCE 10 minutes ago
        """
        
        data = self.execute_nrql(query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No connection metrics found")
            return results
        
        metrics = data[0]
        current_connections = metrics.get('current_connections', 0)
        max_connections = metrics.get('max_connections', 1)
        threads_connected = metrics.get('threads_connected', 0)
        aborted_connects = metrics.get('aborted_connects', 0)
        
        results['metrics'] = {
            'current_connections': current_connections,
            'max_connections': max_connections,
            'threads_connected': threads_connected,
            'aborted_connects': aborted_connects
        }
        
        # Calculate connection utilization
        connection_utilization = current_connections / max_connections if max_connections > 0 else 0
        results['metrics']['connection_utilization'] = connection_utilization
        
        # Check thresholds
        if connection_utilization >= self.thresholds['connection_utilization_critical']:
            results['status'] = 'FAIL'
            results['issues'].append(f"Critical: Connection utilization at {connection_utilization:.1%}")
            results['recommendations'].append("Immediately increase max_connections or reduce connection load")
        elif connection_utilization >= self.thresholds['connection_utilization_warn']:
            results['status'] = 'WARN'
            results['issues'].append(f"Warning: Connection utilization at {connection_utilization:.1%}")
            results['recommendations'].append("Consider increasing max_connections")
        
        # Check aborted connections rate
        aborted_rate_query = """
        SELECT rate(sum(mysql_aborted_connects), 1 minute) as 'aborted_per_minute'
        FROM Metric 
        WHERE service.name = 'core-metrics'
          AND metricName = 'mysql_aborted_connects'
        SINCE 30 minutes ago
        """
        
        aborted_data = self.execute_nrql(aborted_rate_query)
        if aborted_data and len(aborted_data) > 0:
            aborted_rate = aborted_data[0].get('aborted_per_minute', 0)
            results['metrics']['aborted_connects_per_minute'] = aborted_rate
            
            if aborted_rate >= self.thresholds['aborted_connects_rate_warn']:
                results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                results['issues'].append(f"High aborted connections rate: {aborted_rate:.1f}/min")
                results['recommendations'].append("Check network connectivity and authentication issues")
        
        return results

    def validate_thread_metrics(self) -> Dict:
        """Validate MySQL thread management metrics"""
        logger.info("Validating thread metrics...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        query = """
        SELECT latest(mysql_threads_running) as 'threads_running',
               latest(mysql_threads_connected) as 'threads_connected',
               latest(mysql_threads_created) as 'threads_created',
               latest(mysql_threads_cached) as 'threads_cached'
        FROM Metric 
        WHERE service.name = 'core-metrics'
          AND metricName IN ('mysql_threads_running', 'mysql_threads_connected', 'mysql_threads_created', 'mysql_threads_cached')
        SINCE 10 minutes ago
        """
        
        data = self.execute_nrql(query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No thread metrics found")
            return results
        
        metrics = data[0]
        threads_running = metrics.get('threads_running', 0)
        threads_connected = metrics.get('threads_connected', 0)
        threads_created = metrics.get('threads_created', 0)
        threads_cached = metrics.get('threads_cached', 0)
        
        results['metrics'] = {
            'threads_running': threads_running,
            'threads_connected': threads_connected,
            'threads_created': threads_created,
            'threads_cached': threads_cached
        }
        
        # Calculate thread efficiency ratios
        if threads_connected > 0:
            thread_ratio = threads_running / threads_connected
            results['metrics']['running_to_connected_ratio'] = thread_ratio
            
            if thread_ratio >= self.thresholds['thread_ratio_warn']:
                results['status'] = 'WARN'
                results['issues'].append(f"High thread activity ratio: {thread_ratio:.1%}")
                results['recommendations'].append("Monitor for potential CPU bottlenecks")
        
        # Check thread cache efficiency
        if threads_created > 0:
            cache_miss_rate = (threads_created - threads_cached) / threads_created
            results['metrics']['thread_cache_miss_rate'] = cache_miss_rate
            
            if cache_miss_rate > 0.1:  # 10% cache miss rate
                results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                results['issues'].append(f"Thread cache miss rate: {cache_miss_rate:.1%}")
                results['recommendations'].append("Consider increasing thread_cache_size")
        
        return results

    def validate_buffer_pool_metrics(self) -> Dict:
        """Validate InnoDB buffer pool metrics"""
        logger.info("Validating buffer pool metrics...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        query = """
        SELECT latest(mysql_buffer_pool_pages_dirty) as 'dirty_pages',
               latest(mysql_buffer_pool_pages_free) as 'free_pages',
               latest(mysql_buffer_pool_pages_total) as 'total_pages',
               latest(mysql_buffer_pool_read_requests) as 'read_requests',
               latest(mysql_buffer_pool_reads) as 'physical_reads'
        FROM Metric 
        WHERE service.name = 'core-metrics'
          AND metricName IN ('mysql_buffer_pool_pages_dirty', 'mysql_buffer_pool_pages_free', 'mysql_buffer_pool_pages_total', 'mysql_buffer_pool_read_requests', 'mysql_buffer_pool_reads')
        SINCE 10 minutes ago
        """
        
        data = self.execute_nrql(query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No buffer pool metrics found")
            return results
        
        metrics = data[0]
        dirty_pages = metrics.get('dirty_pages', 0)
        free_pages = metrics.get('free_pages', 0)
        total_pages = metrics.get('total_pages', 1)
        read_requests = metrics.get('read_requests', 0)
        physical_reads = metrics.get('physical_reads', 0)
        
        results['metrics'] = {
            'dirty_pages': dirty_pages,
            'free_pages': free_pages,
            'total_pages': total_pages,
            'read_requests': read_requests,
            'physical_reads': physical_reads
        }
        
        # Calculate buffer pool hit ratio
        if read_requests > 0:
            hit_ratio = (read_requests - physical_reads) / read_requests
            results['metrics']['buffer_pool_hit_ratio'] = hit_ratio
            
            if hit_ratio < self.thresholds['buffer_pool_hit_ratio_warn']:
                results['status'] = 'WARN'
                results['issues'].append(f"Low buffer pool hit ratio: {hit_ratio:.1%}")
                results['recommendations'].append("Consider increasing innodb_buffer_pool_size")
        
        # Calculate dirty pages ratio
        if total_pages > 0:
            dirty_ratio = dirty_pages / total_pages
            results['metrics']['dirty_pages_ratio'] = dirty_ratio
            
            if dirty_ratio >= self.thresholds['dirty_pages_ratio_warn']:
                results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                results['issues'].append(f"High dirty pages ratio: {dirty_ratio:.1%}")
                results['recommendations'].append("Monitor checkpoint frequency and I/O capacity")
        
        # Calculate free pages ratio
        free_ratio = free_pages / total_pages
        results['metrics']['free_pages_ratio'] = free_ratio
        
        if free_ratio < 0.05:  # Less than 5% free
            results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
            results['issues'].append(f"Low free pages: {free_ratio:.1%}")
            results['recommendations'].append("Buffer pool may be undersized")
        
        return results

    def validate_operational_metrics(self) -> Dict:
        """Validate operational metrics like queries and slow queries"""
        logger.info("Validating operational metrics...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check slow query ratio
        query = """
        SELECT rate(sum(mysql_queries), 1 minute) as 'queries_per_minute',
               rate(sum(mysql_slow_queries_total), 1 minute) as 'slow_queries_per_minute'
        FROM Metric 
        WHERE service.name = 'core-metrics'
          AND metricName IN ('mysql_queries', 'mysql_slow_queries_total')
        SINCE 30 minutes ago
        """
        
        data = self.execute_nrql(query)
        if data and len(data) > 0:
            metrics = data[0]
            queries_per_min = metrics.get('queries_per_minute', 0)
            slow_queries_per_min = metrics.get('slow_queries_per_minute', 0)
            
            results['metrics']['queries_per_minute'] = queries_per_min
            results['metrics']['slow_queries_per_minute'] = slow_queries_per_min
            
            if queries_per_min > 0:
                slow_query_ratio = slow_queries_per_min / queries_per_min
                results['metrics']['slow_query_ratio'] = slow_query_ratio
                
                if slow_query_ratio > 0.01:  # 1% slow queries
                    results['status'] = 'WARN'
                    results['issues'].append(f"High slow query ratio: {slow_query_ratio:.2%}")
                    results['recommendations'].append("Review slow query log and optimize queries")
        
        # Check query cache efficiency
        cache_query = """
        SELECT latest(mysql_qcache_hits) as 'cache_hits',
               latest(mysql_qcache_inserts) as 'cache_inserts'
        FROM Metric 
        WHERE service.name = 'core-metrics'
          AND metricName IN ('mysql_qcache_hits', 'mysql_qcache_inserts')
        SINCE 10 minutes ago
        """
        
        cache_data = self.execute_nrql(cache_query)
        if cache_data and len(cache_data) > 0:
            cache_metrics = cache_data[0]
            cache_hits = cache_metrics.get('cache_hits', 0)
            cache_inserts = cache_metrics.get('cache_inserts', 0)
            
            results['metrics']['query_cache_hits'] = cache_hits
            results['metrics']['query_cache_inserts'] = cache_inserts
            
            if cache_hits + cache_inserts > 0:
                cache_hit_ratio = cache_hits / (cache_hits + cache_inserts)
                results['metrics']['query_cache_hit_ratio'] = cache_hit_ratio
                
                if cache_hit_ratio < 0.8:  # 80% cache hit ratio
                    results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                    results['issues'].append(f"Low query cache hit ratio: {cache_hit_ratio:.1%}")
                    results['recommendations'].append("Review query cache configuration")
        
        return results

    def run_comprehensive_validation(self) -> Dict:
        """Run all core metrics validations"""
        logger.info("Starting comprehensive core-metrics validation...")
        
        validation_results = {
            'module': self.module_name,
            'timestamp': datetime.now().isoformat(),
            'overall_status': 'PASS',
            'validations': {},
            'summary': {
                'total_checks': 0,
                'passed': 0,
                'warnings': 0,
                'failures': 0
            },
            'recommendations': []
        }
        
        # Run all validations
        validations = {
            'connections': self.validate_connection_metrics,
            'threads': self.validate_thread_metrics,
            'buffer_pool': self.validate_buffer_pool_metrics,
            'operations': self.validate_operational_metrics
        }
        
        for validation_name, validation_func in validations.items():
            try:
                result = validation_func()
                validation_results['validations'][validation_name] = result
                validation_results['summary']['total_checks'] += 1
                
                if result['status'] == 'PASS':
                    validation_results['summary']['passed'] += 1
                elif result['status'] == 'WARN':
                    validation_results['summary']['warnings'] += 1
                    validation_results['overall_status'] = 'WARN' if validation_results['overall_status'] == 'PASS' else validation_results['overall_status']
                else:
                    validation_results['summary']['failures'] += 1
                    validation_results['overall_status'] = 'FAIL'
                
                validation_results['recommendations'].extend(result['recommendations'])
                
            except Exception as e:
                logger.error(f"Validation {validation_name} failed: {e}")
                validation_results['validations'][validation_name] = {
                    'status': 'FAIL',
                    'error': str(e)
                }
                validation_results['summary']['failures'] += 1
                validation_results['overall_status'] = 'FAIL'
        
        # Remove duplicate recommendations
        validation_results['recommendations'] = list(set(validation_results['recommendations']))
        
        return validation_results

def main():
    parser = argparse.ArgumentParser(description='Core Metrics Module Validation')
    parser.add_argument('--api-key', help='New Relic API Key (overrides .env)')
    parser.add_argument('--account-id', help='New Relic Account ID (overrides .env)')
    parser.add_argument('--check-connections', action='store_true', help='Check connection metrics only')
    parser.add_argument('--check-threads', action='store_true', help='Check thread metrics only')
    parser.add_argument('--check-buffer-pool', action='store_true', help='Check buffer pool metrics only')
    parser.add_argument('--check-operations', action='store_true', help='Check operational metrics only')
    parser.add_argument('--output', help='Output file for results (JSON format)')
    parser.add_argument('--verbose', action='store_true', help='Verbose output')
    
    args = parser.parse_args()
    
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    
    # Get API credentials from arguments or environment
    api_key = args.api_key or os.getenv('NEW_RELIC_API_KEY')
    account_id = args.account_id or os.getenv('NEW_RELIC_ACCOUNT_ID')
    
    if not api_key:
        print("Error: NEW_RELIC_API_KEY not found in environment or arguments")
        sys.exit(1)
    if not account_id:
        print("Error: NEW_RELIC_ACCOUNT_ID not found in environment or arguments")
        sys.exit(1)
    
    validator = CoreMetricsValidator(api_key, account_id)
    
    # Run specific validations if requested
    if any([args.check_connections, args.check_threads, args.check_buffer_pool, args.check_operations]):
        results = {'validations': {}}
        if args.check_connections:
            results['validations']['connections'] = validator.validate_connection_metrics()
        if args.check_threads:
            results['validations']['threads'] = validator.validate_thread_metrics()
        if args.check_buffer_pool:
            results['validations']['buffer_pool'] = validator.validate_buffer_pool_metrics()
        if args.check_operations:
            results['validations']['operations'] = validator.validate_operational_metrics()
    else:
        results = validator.run_comprehensive_validation()
    
    # Output results
    if args.output:
        with open(args.output, 'w') as f:
            json.dump(results, f, indent=2, default=str)
        print(f"Results saved to {args.output}")
    else:
        print(json.dumps(results, indent=2, default=str))
    
    # Exit with appropriate code
    if 'overall_status' in results:
        if results['overall_status'] == 'FAIL':
            sys.exit(1)
        elif results['overall_status'] == 'WARN':
            sys.exit(2)
    
    sys.exit(0)

if __name__ == '__main__':
    main()