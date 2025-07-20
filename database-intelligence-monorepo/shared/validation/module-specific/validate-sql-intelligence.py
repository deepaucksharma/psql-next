#!/usr/bin/env python3
"""
SQL Intelligence Module Specific Validation Tool

This tool provides deep validation for the sql-intelligence module (port 8082).
It validates query performance analysis, statement digest tracking, and execution metrics.

Usage:
    python3 validate-sql-intelligence.py --api-key YOUR_API_KEY --account-id YOUR_ACCOUNT_ID
    python3 validate-sql-intelligence.py --check-query-performance --check-digests --check-execution-stats
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
import re
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class SQLIntelligenceValidator:
    def __init__(self, api_key: str, account_id: str):
        self.api_key = api_key
        self.account_id = account_id
        self.module_name = "sql-intelligence"
        self.module_port = 8082
        self.session = requests.Session()
        self.session.headers.update({
            'Api-Key': api_key,
            'Content-Type': 'application/json'
        })
        
        # SQL Intelligence expected metrics
        self.expected_metrics = {
            'query_execution': [
                'mysql.query.exec_total',
                'mysql.query.exec_time_total',
                'mysql.query.latency_ms',
                'mysql.query.latency_avg_ms',
                'mysql.query.latency_max_ms'
            ],
            'query_analysis': [
                'mysql.query.rows_examined_total',
                'mysql.query.rows_sent_total',
                'mysql.query.select_full_join_total',
                'mysql.query.select_range_total',
                'mysql.query.select_scan_total'
            ],
            'statement_tracking': [
                'mysql.statement.executions',
                'mysql.statement.errors',
                'mysql.statement.warnings',
                'mysql.statement.timer_wait',
                'mysql.statement.lock_time'
            ],
            'index_usage': [
                'mysql.query.no_index_used_total',
                'mysql.query.no_good_index_used_total',
                'mysql.table.index_io_waits_total',
                'mysql.table.io_read_requests_total'
            ],
            'performance_schema': [
                'mysql.perf_schema.events_statements_current',
                'mysql.perf_schema.events_statements_history'
            ]
        }
        
        # Validation thresholds
        self.thresholds = {
            'slow_query_latency_ms': 1000,        # 1 second
            'critical_query_latency_ms': 5000,    # 5 seconds  
            'high_row_examine_ratio': 10.0,       # 10:1 examined to sent ratio
            'no_index_usage_threshold': 0.05,     # 5% queries without index
            'full_table_scan_threshold': 0.10,    # 10% queries doing full scans
            'digest_tracking_min_queries': 10,    # Minimum unique queries to track
            'query_error_rate_threshold': 0.01    # 1% query error rate
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

    def validate_query_performance(self) -> Dict:
        """Validate query performance metrics and latency"""
        logger.info("Validating query performance metrics...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check average query latency
        latency_query = """
        SELECT average(mysql.query.latency_avg_ms) as 'avg_latency_ms',
               max(mysql.query.latency_max_ms) as 'max_latency_ms',
               percentile(mysql.query.latency_ms, 95) as 'p95_latency_ms',
               count(*) as 'total_queries'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND metricName IN ('mysql.query.latency_avg_ms', 'mysql.query.latency_max_ms', 'mysql.query.latency_ms')
        SINCE 30 minutes ago
        """
        
        data = self.execute_nrql(latency_query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No query performance metrics found")
            return results
        
        metrics = data[0]
        avg_latency = metrics.get('avg_latency_ms', 0)
        max_latency = metrics.get('max_latency_ms', 0)
        p95_latency = metrics.get('p95_latency_ms', 0)
        total_queries = metrics.get('total_queries', 0)
        
        results['metrics'] = {
            'avg_latency_ms': avg_latency,
            'max_latency_ms': max_latency,
            'p95_latency_ms': p95_latency,
            'total_queries': total_queries
        }
        
        # Check latency thresholds
        if max_latency >= self.thresholds['critical_query_latency_ms']:
            results['status'] = 'FAIL'
            results['issues'].append(f"Critical: Maximum query latency {max_latency:.0f}ms exceeds {self.thresholds['critical_query_latency_ms']}ms")
            results['recommendations'].append("Investigate and optimize slowest queries immediately")
        elif avg_latency >= self.thresholds['slow_query_latency_ms']:
            results['status'] = 'WARN'
            results['issues'].append(f"Warning: Average query latency {avg_latency:.0f}ms exceeds {self.thresholds['slow_query_latency_ms']}ms")
            results['recommendations'].append("Review query performance and consider optimization")
        
        if p95_latency >= self.thresholds['slow_query_latency_ms']:
            results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
            results['issues'].append(f"Warning: 95th percentile latency {p95_latency:.0f}ms is high")
            results['recommendations'].append("Optimize top 5% slowest queries")
        
        # Check query throughput
        throughput_query = """
        SELECT rate(sum(mysql.query.exec_total), 1 minute) as 'queries_per_minute'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND metricName = 'mysql.query.exec_total'
        SINCE 30 minutes ago
        """
        
        throughput_data = self.execute_nrql(throughput_query)
        if throughput_data and len(throughput_data) > 0:
            qpm = throughput_data[0].get('queries_per_minute', 0)
            results['metrics']['queries_per_minute'] = qpm
            
            if qpm == 0:
                results['status'] = 'FAIL'
                results['issues'].append("No query execution detected")
                results['recommendations'].append("Check SQL Intelligence module connectivity to MySQL")
        
        return results

    def validate_query_digest_tracking(self) -> Dict:
        """Validate statement digest tracking and analysis"""
        logger.info("Validating query digest tracking...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check digest diversity
        digest_query = """
        SELECT count(DISTINCT statement_digest) as 'unique_digests',
               count(*) as 'total_executions'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND statement_digest IS NOT NULL
          AND metricName = 'mysql.query.exec_total'
        SINCE 1 hour ago
        """
        
        data = self.execute_nrql(digest_query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No statement digest tracking found")
            results['recommendations'].append("Verify performance_schema.events_statements_summary_by_digest is enabled")
            return results
        
        metrics = data[0]
        unique_digests = metrics.get('unique_digests', 0)
        total_executions = metrics.get('total_executions', 0)
        
        results['metrics'] = {
            'unique_digests': unique_digests,
            'total_executions': total_executions
        }
        
        # Check digest tracking adequacy
        if unique_digests < self.thresholds['digest_tracking_min_queries']:
            results['status'] = 'WARN'
            results['issues'].append(f"Low digest diversity: only {unique_digests} unique queries tracked")
            results['recommendations'].append("Check if performance_schema is properly collecting statement data")
        
        # Calculate execution frequency
        if unique_digests > 0:
            avg_executions_per_digest = total_executions / unique_digests
            results['metrics']['avg_executions_per_digest'] = avg_executions_per_digest
        
        # Check top queries by execution count
        top_queries_query = """
        SELECT statement_digest,
               sum(mysql.query.exec_total) as 'executions',
               average(mysql.query.latency_avg_ms) as 'avg_latency_ms'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND statement_digest IS NOT NULL
          AND metricName = 'mysql.query.exec_total'
        FACET statement_digest
        SINCE 1 hour ago
        LIMIT 10
        """
        
        top_queries_data = self.execute_nrql(top_queries_query)
        if top_queries_data:
            results['metrics']['top_queries'] = top_queries_data[:5]  # Store top 5
            
            # Check for problematic queries
            for query in top_queries_data:
                executions = query.get('executions', 0)
                avg_latency = query.get('avg_latency_ms', 0)
                
                if avg_latency >= self.thresholds['slow_query_latency_ms'] and executions > 100:
                    results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                    results['issues'].append(f"High-frequency slow query detected: {executions} executions with {avg_latency:.0f}ms avg latency")
                    results['recommendations'].append("Prioritize optimization of frequently executed slow queries")
        
        return results

    def validate_execution_statistics(self) -> Dict:
        """Validate query execution statistics and efficiency"""
        logger.info("Validating execution statistics...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check row examination efficiency
        efficiency_query = """
        SELECT sum(mysql.query.rows_examined_total) as 'total_rows_examined',
               sum(mysql.query.rows_sent_total) as 'total_rows_sent',
               count(*) as 'query_count'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND metricName IN ('mysql.query.rows_examined_total', 'mysql.query.rows_sent_total')
        SINCE 30 minutes ago
        """
        
        data = self.execute_nrql(efficiency_query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No execution statistics found")
            return results
        
        metrics = data[0]
        rows_examined = metrics.get('total_rows_examined', 0)
        rows_sent = metrics.get('total_rows_sent', 0)
        query_count = metrics.get('query_count', 0)
        
        results['metrics'] = {
            'total_rows_examined': rows_examined,
            'total_rows_sent': rows_sent,
            'query_count': query_count
        }
        
        # Calculate examination efficiency
        if rows_sent > 0:
            examine_ratio = rows_examined / rows_sent
            results['metrics']['rows_examined_to_sent_ratio'] = examine_ratio
            
            if examine_ratio >= self.thresholds['high_row_examine_ratio']:
                results['status'] = 'WARN'
                results['issues'].append(f"Inefficient queries: {examine_ratio:.1f}:1 examined to sent ratio")
                results['recommendations'].append("Review queries with poor selectivity and add appropriate indexes")
        
        # Check query error rates
        error_query = """
        SELECT sum(mysql.statement.errors) as 'total_errors',
               sum(mysql.statement.executions) as 'total_executions'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND metricName IN ('mysql.statement.errors', 'mysql.statement.executions')
        SINCE 30 minutes ago
        """
        
        error_data = self.execute_nrql(error_query)
        if error_data and len(error_data) > 0:
            error_metrics = error_data[0]
            total_errors = error_metrics.get('total_errors', 0)
            total_executions = error_metrics.get('total_executions', 0)
            
            results['metrics']['total_errors'] = total_errors
            results['metrics']['total_executions'] = total_executions
            
            if total_executions > 0:
                error_rate = total_errors / total_executions
                results['metrics']['query_error_rate'] = error_rate
                
                if error_rate >= self.thresholds['query_error_rate_threshold']:
                    results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                    results['issues'].append(f"High query error rate: {error_rate:.2%}")
                    results['recommendations'].append("Review query errors and fix syntax/permission issues")
        
        return results

    def validate_index_usage(self) -> Dict:
        """Validate index usage efficiency"""
        logger.info("Validating index usage...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check queries without index usage
        index_query = """
        SELECT sum(mysql.query.no_index_used_total) as 'no_index_queries',
               sum(mysql.query.no_good_index_used_total) as 'no_good_index_queries',
               sum(mysql.query.select_full_join_total) as 'full_join_queries',
               sum(mysql.query.select_scan_total) as 'full_scan_queries',
               sum(mysql.query.exec_total) as 'total_queries'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND metricName IN ('mysql.query.no_index_used_total', 'mysql.query.no_good_index_used_total', 
                           'mysql.query.select_full_join_total', 'mysql.query.select_scan_total', 'mysql.query.exec_total')
        SINCE 30 minutes ago
        """
        
        data = self.execute_nrql(index_query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No index usage metrics found")
            return results
        
        metrics = data[0]
        no_index_queries = metrics.get('no_index_queries', 0)
        no_good_index_queries = metrics.get('no_good_index_queries', 0)
        full_join_queries = metrics.get('full_join_queries', 0)
        full_scan_queries = metrics.get('full_scan_queries', 0)
        total_queries = metrics.get('total_queries', 0)
        
        results['metrics'] = {
            'no_index_queries': no_index_queries,
            'no_good_index_queries': no_good_index_queries,
            'full_join_queries': full_join_queries,
            'full_scan_queries': full_scan_queries,
            'total_queries': total_queries
        }
        
        # Calculate index usage ratios
        if total_queries > 0:
            no_index_ratio = no_index_queries / total_queries
            full_scan_ratio = full_scan_queries / total_queries
            
            results['metrics']['no_index_usage_ratio'] = no_index_ratio
            results['metrics']['full_scan_ratio'] = full_scan_ratio
            
            if no_index_ratio >= self.thresholds['no_index_usage_threshold']:
                results['status'] = 'WARN'
                results['issues'].append(f"High percentage of queries without indexes: {no_index_ratio:.1%}")
                results['recommendations'].append("Identify and add missing indexes for frequently executed queries")
            
            if full_scan_ratio >= self.thresholds['full_table_scan_threshold']:
                results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                results['issues'].append(f"High percentage of full table scans: {full_scan_ratio:.1%}")
                results['recommendations'].append("Optimize queries to avoid full table scans")
        
        return results

    def validate_schema_coverage(self) -> Dict:
        """Validate schema and table coverage"""
        logger.info("Validating schema coverage...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check schema diversity
        schema_query = """
        SELECT count(DISTINCT schema_name) as 'unique_schemas',
               count(DISTINCT table_name) as 'unique_tables'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND schema_name IS NOT NULL
          AND metricName = 'mysql.query.exec_total'
        SINCE 1 hour ago
        """
        
        data = self.execute_nrql(schema_query)
        if data and len(data) > 0:
            metrics = data[0]
            unique_schemas = metrics.get('unique_schemas', 0)
            unique_tables = metrics.get('unique_tables', 0)
            
            results['metrics'] = {
                'unique_schemas': unique_schemas,
                'unique_tables': unique_tables
            }
            
            if unique_schemas == 0:
                results['status'] = 'WARN'
                results['issues'].append("No schema information tracked")
                results['recommendations'].append("Verify schema tracking is enabled in performance_schema")
        
        # Check table access patterns
        table_query = """
        SELECT table_name,
               sum(mysql.table.io_read_requests_total) as 'read_requests',
               sum(mysql.table.io_write_requests_total) as 'write_requests'
        FROM Metric 
        WHERE service.name = 'sql-intelligence'
          AND table_name IS NOT NULL
          AND metricName IN ('mysql.table.io_read_requests_total', 'mysql.table.io_write_requests_total')
        FACET table_name
        SINCE 1 hour ago
        LIMIT 10
        """
        
        table_data = self.execute_nrql(table_query)
        if table_data:
            results['metrics']['top_accessed_tables'] = table_data[:5]
            
            # Check for hot tables
            for table in table_data:
                read_requests = table.get('read_requests', 0)
                write_requests = table.get('write_requests', 0)
                
                if read_requests > 10000:  # High read activity
                    results['status'] = 'INFO' if results['status'] == 'PASS' else results['status']
                    results['recommendations'].append(f"Monitor table {table.get('table_name')} for potential optimization")
        
        return results

    def run_comprehensive_validation(self) -> Dict:
        """Run all SQL Intelligence validations"""
        logger.info("Starting comprehensive sql-intelligence validation...")
        
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
            'query_performance': self.validate_query_performance,
            'digest_tracking': self.validate_query_digest_tracking,
            'execution_stats': self.validate_execution_statistics,
            'index_usage': self.validate_index_usage,
            'schema_coverage': self.validate_schema_coverage
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
                elif result['status'] == 'INFO':
                    validation_results['summary']['passed'] += 1  # Count INFO as passed
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
    parser = argparse.ArgumentParser(description='SQL Intelligence Module Validation')
    parser.add_argument('--api-key', help='New Relic API Key (overrides .env)')
    parser.add_argument('--account-id', help='New Relic Account ID (overrides .env)')
    parser.add_argument('--check-query-performance', action='store_true', help='Check query performance only')
    parser.add_argument('--check-digests', action='store_true', help='Check digest tracking only')
    parser.add_argument('--check-execution-stats', action='store_true', help='Check execution statistics only')
    parser.add_argument('--check-index-usage', action='store_true', help='Check index usage only')
    parser.add_argument('--check-schema-coverage', action='store_true', help='Check schema coverage only')
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
    
    validator = SQLIntelligenceValidator(api_key, account_id)
    
    # Run specific validations if requested
    if any([args.check_query_performance, args.check_digests, args.check_execution_stats, args.check_index_usage, args.check_schema_coverage]):
        results = {'validations': {}}
        if args.check_query_performance:
            results['validations']['query_performance'] = validator.validate_query_performance()
        if args.check_digests:
            results['validations']['digest_tracking'] = validator.validate_query_digest_tracking()
        if args.check_execution_stats:
            results['validations']['execution_stats'] = validator.validate_execution_statistics()
        if args.check_index_usage:
            results['validations']['index_usage'] = validator.validate_index_usage()
        if args.check_schema_coverage:
            results['validations']['schema_coverage'] = validator.validate_schema_coverage()
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