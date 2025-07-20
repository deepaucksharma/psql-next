#!/usr/bin/env python3
"""
Integration Test Suite for NRDB Data Consistency

This test suite validates data consistency across the entire database intelligence
pipeline, ensuring that data flows correctly from MySQL through OpenTelemetry
collectors to New Relic and maintains consistency across all modules.

Test Categories:
1. Data Flow Integration Tests
2. Cross-Module Consistency Tests  
3. Federation Data Integrity Tests
4. Timestamp and Ordering Tests
5. Metric Correlation Tests
6. Performance and Latency Tests

Usage:
    python3 integration-test-suite.py
    python3 integration-test-suite.py --category data-flow --category consistency
    python3 integration-test-suite.py --modules core-metrics sql-intelligence
"""

import os
import sys
import json
import time
import requests
import argparse
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any, Tuple
import logging
from dataclasses import dataclass
from enum import Enum
from concurrent.futures import ThreadPoolExecutor, as_completed
import statistics
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class TestCategory(Enum):
    DATA_FLOW = "data-flow"
    CONSISTENCY = "consistency"
    FEDERATION = "federation"
    TIMESTAMPS = "timestamps"
    CORRELATION = "correlation"
    PERFORMANCE = "performance"

class TestResult(Enum):
    PASS = "PASS"
    FAIL = "FAIL"
    SKIP = "SKIP"
    ERROR = "ERROR"

@dataclass
class IntegrationTestCase:
    test_id: str
    category: TestCategory
    name: str
    description: str
    result: TestResult
    duration_ms: float
    timestamp: datetime
    details: Dict[str, Any]
    error_message: Optional[str] = None
    
class NRDBIntegrationTestSuite:
    def __init__(self):
        self.nr_api_key = os.getenv('NEW_RELIC_API_KEY')
        self.nr_account_id = os.getenv('NEW_RELIC_ACCOUNT_ID')
        
        if not all([self.nr_api_key, self.nr_account_id]):
            raise ValueError("NEW_RELIC_API_KEY and NEW_RELIC_ACCOUNT_ID must be set")
        
        # Setup HTTP session
        self.session = requests.Session()
        self.session.headers.update({
            'Api-Key': self.nr_api_key,
            'Content-Type': 'application/json'
        })
        
        self.modules = [
            'core-metrics', 'sql-intelligence', 'wait-profiler', 'anomaly-detector',
            'business-impact', 'replication-monitor', 'performance-advisor', 
            'resource-monitor', 'alert-manager', 'canary-tester', 'cross-signal-correlator'
        ]
        
        self.test_results = []

    def execute_nrql(self, query: str, timeout: int = 30) -> Optional[List[Dict]]:
        """Execute NRQL query against New Relic"""
        graphql_query = {
            "query": f"""
            {{
                actor {{
                    account(id: {self.nr_account_id}) {{
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
                timeout=timeout
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

    def run_test(self, test_func, test_id: str, category: TestCategory, 
                 name: str, description: str, *args, **kwargs) -> IntegrationTestCase:
        """Run a single test case with timing and error handling"""
        start_time = time.time()
        
        test_case = IntegrationTestCase(
            test_id=test_id,
            category=category,
            name=name,
            description=description,
            result=TestResult.ERROR,
            duration_ms=0,
            timestamp=datetime.now(),
            details={}
        )
        
        try:
            logger.info(f"Running test: {name}")
            result = test_func(*args, **kwargs)
            
            if isinstance(result, dict):
                test_case.result = TestResult.PASS if result.get('success', False) else TestResult.FAIL
                test_case.details = result
                if 'error' in result:
                    test_case.error_message = result['error']
            else:
                test_case.result = TestResult.PASS if result else TestResult.FAIL
                
        except Exception as e:
            test_case.result = TestResult.ERROR
            test_case.error_message = str(e)
            test_case.details = {'exception': str(e)}
            logger.error(f"Test {name} failed with exception: {e}")
        finally:
            test_case.duration_ms = (time.time() - start_time) * 1000
            
        return test_case

    # Data Flow Integration Tests
    def test_data_flow_end_to_end(self, modules: List[str] = None) -> Dict:
        """Test complete data flow from source to New Relic"""
        if modules is None:
            modules = self.modules[:3]  # Test subset for speed
        
        results = {
            'success': True,
            'modules_tested': len(modules),
            'modules_passing': 0,
            'flow_stages': {}
        }
        
        for module in modules:
            module_flow = {
                'data_in_last_hour': False,
                'recent_data': False,
                'consistent_flow': False
            }
            
            # Check data presence in last hour
            hour_query = f"""
            SELECT count(*) as 'records'
            FROM Metric 
            WHERE service.name = '{module}'
            SINCE 1 hour ago
            """
            
            hour_data = self.execute_nrql(hour_query)
            if hour_data and hour_data[0].get('records', 0) > 0:
                module_flow['data_in_last_hour'] = True
            
            # Check very recent data (last 5 minutes)
            recent_query = f"""
            SELECT count(*) as 'records'
            FROM Metric 
            WHERE service.name = '{module}'
            SINCE 5 minutes ago
            """
            
            recent_data = self.execute_nrql(recent_query)
            if recent_data and recent_data[0].get('records', 0) > 0:
                module_flow['recent_data'] = True
            
            # Check for consistent data flow (multiple time buckets)
            consistency_query = f"""
            SELECT count(*) as 'records'
            FROM Metric 
            WHERE service.name = '{module}'
            SINCE 1 hour ago
            FACET floor(timestamp / (5*60*1000)) * 5*60*1000
            LIMIT 12
            """
            
            consistency_data = self.execute_nrql(consistency_query)
            if consistency_data and len(consistency_data) >= 8:  # At least 8 of 12 buckets
                module_flow['consistent_flow'] = True
            
            # Module passes if at least 2/3 checks pass
            module_score = sum(module_flow.values()) / len(module_flow)
            if module_score >= 0.67:
                results['modules_passing'] += 1
            
            results['flow_stages'][module] = module_flow
        
        # Overall success if most modules pass
        if results['modules_passing'] < results['modules_tested'] * 0.7:
            results['success'] = False
            results['error'] = f"Only {results['modules_passing']}/{results['modules_tested']} modules have healthy data flow"
        
        return results

    def test_metric_cardinality_limits(self, modules: List[str] = None) -> Dict:
        """Test that metric cardinality is within acceptable limits"""
        if modules is None:
            modules = self.modules
        
        results = {
            'success': True,
            'cardinality_checks': {},
            'high_cardinality_modules': []
        }
        
        cardinality_limit = 10000  # Reasonable limit for most modules
        
        for module in modules:
            cardinality_query = f"""
            SELECT uniqueCount(metricName) as 'unique_metrics',
                   uniqueCount(concat(metricName, entity.guid)) as 'unique_series'
            FROM Metric 
            WHERE service.name = '{module}'
            SINCE 1 hour ago
            """
            
            cardinality_data = self.execute_nrql(cardinality_query)
            if cardinality_data:
                unique_metrics = cardinality_data[0].get('unique_metrics', 0)
                unique_series = cardinality_data[0].get('unique_series', 0)
                
                results['cardinality_checks'][module] = {
                    'unique_metrics': unique_metrics,
                    'unique_series': unique_series,
                    'within_limits': unique_series < cardinality_limit
                }
                
                if unique_series >= cardinality_limit:
                    results['high_cardinality_modules'].append(module)
        
        if results['high_cardinality_modules']:
            results['success'] = False
            results['error'] = f"High cardinality detected in modules: {', '.join(results['high_cardinality_modules'])}"
        
        return results

    # Cross-Module Consistency Tests
    def test_cross_module_timestamp_consistency(self, modules: List[str] = None) -> Dict:
        """Test that timestamps are consistent across modules"""
        if modules is None:
            modules = self.modules[:5]  # Test subset
        
        results = {
            'success': True,
            'timestamp_analysis': {},
            'inconsistent_modules': []
        }
        
        # Get latest timestamps from each module
        module_timestamps = {}
        
        for module in modules:
            timestamp_query = f"""
            SELECT latest(timestamp) as 'latest_timestamp'
            FROM Metric 
            WHERE service.name = '{module}'
            SINCE 1 hour ago
            """
            
            timestamp_data = self.execute_nrql(timestamp_query)
            if timestamp_data and timestamp_data[0].get('latest_timestamp'):
                try:
                    ts_str = timestamp_data[0]['latest_timestamp']
                    ts = datetime.fromisoformat(ts_str.replace('Z', '+00:00'))
                    module_timestamps[module] = ts
                except Exception:
                    results['inconsistent_modules'].append(module)
        
        if len(module_timestamps) >= 2:
            timestamps = list(module_timestamps.values())
            time_diffs = []
            
            # Calculate differences between all pairs
            for i, ts1 in enumerate(timestamps):
                for ts2 in timestamps[i+1:]:
                    diff_seconds = abs((ts1 - ts2).total_seconds())
                    time_diffs.append(diff_seconds)
            
            max_diff = max(time_diffs) if time_diffs else 0
            avg_diff = sum(time_diffs) / len(time_diffs) if time_diffs else 0
            
            results['timestamp_analysis'] = {
                'modules_with_timestamps': len(module_timestamps),
                'max_difference_seconds': max_diff,
                'avg_difference_seconds': avg_diff,
                'timestamps': {k: v.isoformat() for k, v in module_timestamps.items()}
            }
            
            # Flag if timestamps are too far apart (more than 10 minutes)
            if max_diff > 600:
                results['success'] = False
                results['error'] = f"Timestamp inconsistency detected: max difference {max_diff:.0f} seconds"
        else:
            results['success'] = False
            results['error'] = "Insufficient timestamp data for comparison"
        
        return results

    def test_metric_value_consistency(self) -> Dict:
        """Test that related metrics have consistent values"""
        results = {
            'success': True,
            'consistency_checks': {},
            'inconsistencies': []
        }
        
        # Test consistency between core-metrics and sql-intelligence
        consistency_query = """
        SELECT latest(mysql_connections_current) as 'core_connections',
               latest(mysql.query.active_connections) as 'sql_connections'
        FROM Metric 
        WHERE service.name IN ('core-metrics', 'sql-intelligence')
          AND metricName IN ('mysql_connections_current', 'mysql.query.active_connections')
        SINCE 30 minutes ago
        """
        
        consistency_data = self.execute_nrql(consistency_query)
        if consistency_data:
            core_conn = consistency_data[0].get('core_connections')
            sql_conn = consistency_data[0].get('sql_connections')
            
            if core_conn is not None and sql_conn is not None:
                # Allow 20% variance between related metrics
                variance = abs(core_conn - sql_conn) / max(core_conn, sql_conn, 1)
                
                results['consistency_checks']['connection_metrics'] = {
                    'core_metrics_value': core_conn,
                    'sql_intelligence_value': sql_conn,
                    'variance_percentage': variance * 100,
                    'consistent': variance < 0.2
                }
                
                if variance >= 0.2:
                    results['inconsistencies'].append('connection_metrics')
        
        # Additional consistency checks can be added here
        
        if results['inconsistencies']:
            results['success'] = False
            results['error'] = f"Value inconsistencies detected: {', '.join(results['inconsistencies'])}"
        
        return results

    # Federation Data Integrity Tests
    def test_federation_data_integrity(self) -> Dict:
        """Test that federated data maintains integrity"""
        results = {
            'success': True,
            'federation_checks': {},
            'failed_federations': []
        }
        
        # Test anomaly-detector federation from core-metrics
        federation_query = """
        SELECT count(*) as 'federated_records',
               count(DISTINCT federated_from) as 'unique_sources'
        FROM Metric 
        WHERE service.name = 'anomaly-detector'
          AND federated_from IS NOT NULL
        SINCE 30 minutes ago
        """
        
        federation_data = self.execute_nrql(federation_query)
        if federation_data:
            federated_records = federation_data[0].get('federated_records', 0)
            unique_sources = federation_data[0].get('unique_sources', 0)
            
            results['federation_checks']['anomaly_detector'] = {
                'federated_records': federated_records,
                'unique_sources': unique_sources,
                'has_federation': federated_records > 0
            }
            
            if federated_records == 0:
                results['failed_federations'].append('anomaly-detector')
        
        # Test federation data consistency
        source_vs_federated_query = """
        SELECT service.name,
               count(*) as 'records'
        FROM Metric 
        WHERE service.name IN ('core-metrics', 'anomaly-detector')
          AND metricName = 'mysql_connections_current'
        FACET service.name
        SINCE 30 minutes ago
        """
        
        source_federated_data = self.execute_nrql(source_vs_federated_query)
        if source_federated_data:
            source_count = 0
            federated_count = 0
            
            for item in source_federated_data:
                service = item.get('service.name')
                count = item.get('records', 0)
                
                if service == 'core-metrics':
                    source_count = count
                elif service == 'anomaly-detector':
                    federated_count = count
            
            if source_count > 0 and federated_count == 0:
                results['failed_federations'].append('federation_data_flow')
                
            results['federation_checks']['data_flow'] = {
                'source_records': source_count,
                'federated_records': federated_count,
                'ratio': federated_count / source_count if source_count > 0 else 0
            }
        
        if results['failed_federations']:
            results['success'] = False
            results['error'] = f"Federation failures: {', '.join(results['failed_federations'])}"
        
        return results

    # Timestamp and Ordering Tests
    def test_timestamp_ordering(self, modules: List[str] = None) -> Dict:
        """Test that timestamps are properly ordered"""
        if modules is None:
            modules = ['core-metrics', 'sql-intelligence']
        
        results = {
            'success': True,
            'ordering_checks': {},
            'disordered_modules': []
        }
        
        for module in modules:
            ordering_query = f"""
            SELECT timestamp, count(*) as 'records'
            FROM Metric 
            WHERE service.name = '{module}'
            SINCE 1 hour ago
            FACET timestamp
            ORDER BY timestamp
            LIMIT 50
            """
            
            ordering_data = self.execute_nrql(ordering_query)
            if ordering_data:
                timestamps = []
                for item in ordering_data:
                    try:
                        ts_str = item.get('timestamp')
                        if ts_str:
                            ts = datetime.fromisoformat(ts_str.replace('Z', '+00:00'))
                            timestamps.append(ts)
                    except Exception:
                        continue
                
                # Check if timestamps are properly ordered
                is_ordered = all(timestamps[i] <= timestamps[i+1] for i in range(len(timestamps)-1))
                
                # Check for duplicate timestamps
                unique_timestamps = len(set(timestamps))
                total_timestamps = len(timestamps)
                
                results['ordering_checks'][module] = {
                    'total_timestamps': total_timestamps,
                    'unique_timestamps': unique_timestamps,
                    'properly_ordered': is_ordered,
                    'duplicate_ratio': 1 - (unique_timestamps / total_timestamps) if total_timestamps > 0 else 0
                }
                
                if not is_ordered or unique_timestamps < total_timestamps * 0.9:
                    results['disordered_modules'].append(module)
        
        if results['disordered_modules']:
            results['success'] = False
            results['error'] = f"Timestamp ordering issues in: {', '.join(results['disordered_modules'])}"
        
        return results

    # Performance and Latency Tests
    def test_query_performance(self) -> Dict:
        """Test NRQL query performance"""
        results = {
            'success': True,
            'performance_metrics': {},
            'slow_queries': []
        }
        
        test_queries = [
            ("simple_count", "SELECT count(*) FROM Metric SINCE 10 minutes ago"),
            ("faceted_count", "SELECT count(*) FROM Metric FACET service.name SINCE 10 minutes ago"),
            ("time_series", "SELECT count(*) FROM Metric TIMESERIES 1 minute SINCE 10 minutes ago"),
            ("complex_aggregation", "SELECT average(value), percentile(value, 95) FROM Metric WHERE metricName LIKE 'mysql_%' SINCE 10 minutes ago")
        ]
        
        for query_name, query in test_queries:
            start_time = time.time()
            
            try:
                data = self.execute_nrql(query)
                query_time = (time.time() - start_time) * 1000
                
                results['performance_metrics'][query_name] = {
                    'duration_ms': query_time,
                    'success': data is not None,
                    'result_count': len(data) if data else 0
                }
                
                # Flag queries taking longer than 5 seconds
                if query_time > 5000:
                    results['slow_queries'].append(query_name)
                    
            except Exception as e:
                results['performance_metrics'][query_name] = {
                    'duration_ms': 0,
                    'success': False,
                    'error': str(e)
                }
        
        if results['slow_queries']:
            results['success'] = False
            results['error'] = f"Slow queries detected: {', '.join(results['slow_queries'])}"
        
        return results

    def run_integration_tests(self, 
                            categories: List[TestCategory] = None,
                            modules: List[str] = None) -> Dict:
        """Run complete integration test suite"""
        logger.info("Starting NRDB integration test suite...")
        
        if categories is None:
            categories = list(TestCategory)
        
        test_start = time.time()
        test_cases = []
        
        # Define test cases
        test_definitions = [
            # Data Flow Tests
            (TestCategory.DATA_FLOW, "test_001", "End-to-End Data Flow", 
             "Validates complete data flow from modules to New Relic",
             self.test_data_flow_end_to_end, modules),
            
            (TestCategory.DATA_FLOW, "test_002", "Metric Cardinality Limits",
             "Ensures metric cardinality stays within acceptable limits",
             self.test_metric_cardinality_limits, modules),
            
            # Consistency Tests
            (TestCategory.CONSISTENCY, "test_003", "Cross-Module Timestamp Consistency",
             "Validates timestamp consistency across modules",
             self.test_cross_module_timestamp_consistency, modules),
             
            (TestCategory.CONSISTENCY, "test_004", "Metric Value Consistency",
             "Checks consistency of related metric values",
             self.test_metric_value_consistency),
            
            # Federation Tests
            (TestCategory.FEDERATION, "test_005", "Federation Data Integrity",
             "Validates federated data maintains integrity",
             self.test_federation_data_integrity),
            
            # Timestamp Tests
            (TestCategory.TIMESTAMPS, "test_006", "Timestamp Ordering",
             "Ensures timestamps are properly ordered",
             self.test_timestamp_ordering, modules),
            
            # Performance Tests
            (TestCategory.PERFORMANCE, "test_007", "Query Performance",
             "Tests NRQL query performance",
             self.test_query_performance),
        ]
        
        # Filter tests by requested categories
        filtered_tests = [
            test for test in test_definitions 
            if test[0] in categories
        ]
        
        # Run tests
        for category, test_id, name, description, test_func, *args in filtered_tests:
            test_case = self.run_test(
                test_func, test_id, category, name, description, *args
            )
            test_cases.append(test_case)
        
        # Calculate summary statistics
        total_tests = len(test_cases)
        passed_tests = len([t for t in test_cases if t.result == TestResult.PASS])
        failed_tests = len([t for t in test_cases if t.result == TestResult.FAIL])
        error_tests = len([t for t in test_cases if t.result == TestResult.ERROR])
        skipped_tests = len([t for t in test_cases if t.result == TestResult.SKIP])
        
        # Determine overall result
        if error_tests > 0 or failed_tests > total_tests * 0.2:  # More than 20% failures
            overall_result = "FAIL"
        elif failed_tests > 0:
            overall_result = "PARTIAL"
        else:
            overall_result = "PASS"
        
        total_duration = time.time() - test_start
        
        return {
            'integration_test_results': {
                'timestamp': datetime.now().isoformat(),
                'overall_result': overall_result,
                'total_duration_seconds': total_duration,
                'success_rate': (passed_tests / total_tests * 100) if total_tests > 0 else 0
            },
            'test_summary': {
                'total_tests': total_tests,
                'passed': passed_tests,
                'failed': failed_tests,
                'errors': error_tests,
                'skipped': skipped_tests
            },
            'test_cases': [
                {
                    'test_id': tc.test_id,
                    'category': tc.category.value,
                    'name': tc.name,
                    'description': tc.description,
                    'result': tc.result.value,
                    'duration_ms': tc.duration_ms,
                    'timestamp': tc.timestamp.isoformat(),
                    'details': tc.details,
                    'error_message': tc.error_message
                }
                for tc in test_cases
            ],
            'categories_tested': [cat.value for cat in categories],
            'modules_tested': modules or self.modules
        }

def main():
    parser = argparse.ArgumentParser(description='NRDB Integration Test Suite')
    parser.add_argument('--category', action='append', 
                       choices=[cat.value for cat in TestCategory],
                       help='Test categories to run (can specify multiple)')
    parser.add_argument('--modules', nargs='+', help='Specific modules to test')
    parser.add_argument('--output', help='Output file for results (JSON format)')
    parser.add_argument('--verbose', action='store_true', help='Verbose output')
    
    args = parser.parse_args()
    
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    
    try:
        test_suite = NRDBIntegrationTestSuite()
    except ValueError as e:
        print(f"Configuration error: {e}")
        sys.exit(1)
    
    # Parse categories
    categories = None
    if args.category:
        categories = [TestCategory(cat) for cat in args.category]
    
    # Run tests
    results = test_suite.run_integration_tests(
        categories=categories,
        modules=args.modules
    )
    
    # Output results
    if args.output:
        with open(args.output, 'w') as f:
            json.dump(results, f, indent=2, default=str)
        print(f"Results saved to {args.output}")
    else:
        print(json.dumps(results, indent=2, default=str))
    
    # Print summary
    test_results = results['integration_test_results']
    summary = results['test_summary']
    
    print(f"\n{'='*60}")
    print(f"NRDB INTEGRATION TEST RESULTS")
    print(f"{'='*60}")
    print(f"Overall Result: {test_results['overall_result']}")
    print(f"Success Rate: {test_results['success_rate']:.1f}%")
    print(f"Total Duration: {test_results['total_duration_seconds']:.1f}s")
    
    print(f"\nTest Summary:")
    print(f"  ✅ Passed: {summary['passed']}")
    print(f"  ❌ Failed: {summary['failed']}")
    print(f"  ⚠️  Errors: {summary['errors']}")
    print(f"  ⏭  Skipped: {summary['skipped']}")
    print(f"  📊 Total: {summary['total_tests']}")
    
    # Show failed tests
    failed_tests = [tc for tc in results['test_cases'] if tc['result'] in ['FAIL', 'ERROR']]
    if failed_tests:
        print(f"\nFailed Tests:")
        for test in failed_tests:
            print(f"  - {test['test_id']}: {test['name']} ({test['result']})")
            if test['error_message']:
                print(f"    Error: {test['error_message']}")
    
    # Exit with appropriate code
    if test_results['overall_result'] == 'FAIL':
        sys.exit(1)
    elif test_results['overall_result'] == 'PARTIAL':
        sys.exit(2)
    else:
        sys.exit(0)

if __name__ == '__main__':
    main()