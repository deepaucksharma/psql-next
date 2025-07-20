#!/usr/bin/env python3
"""
Automated NRDB Validation Framework for Database Intelligence Monorepo

This framework validates data flow from all 11 modules into New Relic Database (NRDB)
and provides comprehensive reporting and troubleshooting capabilities.

Usage:
    python3 automated-nrdb-validator.py --api-key YOUR_API_KEY --account-id YOUR_ACCOUNT_ID
    python3 automated-nrdb-validator.py --config config.json --module core-metrics
    python3 automated-nrdb-validator.py --validate-all --generate-report
"""

import json
import time
import requests
import argparse
import sys
import os
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Tuple
import logging
from dataclasses import dataclass
from enum import Enum
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler(f'nrdb_validation_{datetime.now().strftime("%Y%m%d_%H%M%S")}.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)

class ValidationStatus(Enum):
    PASS = "PASS"
    FAIL = "FAIL"
    WARN = "WARN"
    INFO = "INFO"

@dataclass
class ValidationResult:
    module: str
    test_name: str
    status: ValidationStatus
    message: str
    data_points: int = 0
    expected_metrics: List[str] = None
    found_metrics: List[str] = None
    recommendations: List[str] = None
    timestamp: datetime = None
    
    def __post_init__(self):
        if self.timestamp is None:
            self.timestamp = datetime.now()
        if self.expected_metrics is None:
            self.expected_metrics = []
        if self.found_metrics is None:
            self.found_metrics = []
        if self.recommendations is None:
            self.recommendations = []

class NRDBValidator:
    """Main validation framework for Database Intelligence modules"""
    
    def __init__(self, api_key: str, account_id: str, base_url: str = "https://api.newrelic.com"):
        self.api_key = api_key
        self.account_id = account_id
        self.base_url = base_url
        self.session = requests.Session()
        self.session.headers.update({
            'Api-Key': api_key,
            'Content-Type': 'application/json'
        })
        
        # Module configuration
        self.modules = {
            'core-metrics': {
                'port': 8081,
                'expected_metrics': [
                    'mysql_connections_current',
                    'mysql_threads_running', 
                    'mysql_slow_queries_total',
                    'mysql_buffer_pool_pages_dirty'
                ],
                'critical_attributes': ['mysql.endpoint', 'entity.type'],
                'data_freshness_minutes': 5
            },
            'sql-intelligence': {
                'port': 8082,
                'expected_metrics': [
                    'mysql.query.exec_total',
                    'mysql.query.latency_ms',
                    'mysql.query.latency_avg_ms',
                    'mysql.query.rows_examined_total'
                ],
                'critical_attributes': ['statement_digest', 'schema_name'],
                'data_freshness_minutes': 3
            },
            'wait-profiler': {
                'port': 8083,
                'expected_metrics': [
                    'mysql.wait.count',
                    'mysql.wait.time_ms',
                    'mysql.wait.time_avg_ms',
                    'mysql.wait.mutex.count'
                ],
                'critical_attributes': ['EVENT_NAME'],
                'data_freshness_minutes': 5
            },
            'anomaly-detector': {
                'port': 8084,
                'expected_metrics': [
                    'anomaly_score_cpu',
                    'anomaly_score_memory',
                    'anomaly_score_connections',
                    'anomaly_detected'
                ],
                'critical_attributes': ['anomaly_type', 'baseline_mean'],
                'data_freshness_minutes': 10
            },
            'business-impact': {
                'port': 8085,
                'expected_metrics': [
                    'business_impact_score',
                    'revenue_impact_hourly',
                    'sla_impact'
                ],
                'critical_attributes': ['business_criticality'],
                'data_freshness_minutes': 5
            },
            'replication-monitor': {
                'port': 8086,
                'expected_metrics': [
                    'mysql_replica_lag',
                    'mysql_replication_running',
                    'mysql_gtid_executed'
                ],
                'critical_attributes': ['replication_status'],
                'data_freshness_minutes': 3
            },
            'performance-advisor': {
                'port': 8087,
                'expected_metrics': [
                    'db.performance.recommendation.missing_index',
                    'db.performance.recommendation.slow_query',
                    'db.performance.recommendation.connection_pool'
                ],
                'critical_attributes': ['recommendation_type', 'severity'],
                'data_freshness_minutes': 15
            },
            'resource-monitor': {
                'port': 8088,
                'expected_metrics': [
                    'system.cpu.utilization',
                    'system.memory.usage',
                    'system.disk.io.time'
                ],
                'critical_attributes': ['host.name'],
                'data_freshness_minutes': 3
            },
            'alert-manager': {
                'port': 8089,
                'expected_metrics': [
                    'alert.processed',
                    'alert.severity',
                    'alert.type'
                ],
                'critical_attributes': ['alert.state'],
                'data_freshness_minutes': 5
            },
            'canary-tester': {
                'port': 8090,
                'expected_metrics': [
                    'canary.test.success_rate',
                    'canary.test.response_time',
                    'canary.test.error_count'
                ],
                'critical_attributes': ['test.name'],
                'data_freshness_minutes': 10
            },
            'cross-signal-correlator': {
                'port': 8099,
                'expected_metrics': [
                    'correlation.strength',
                    'cross_signal.match_count'
                ],
                'critical_attributes': ['exemplar.trace_id'],
                'data_freshness_minutes': 10
            }
        }
        
        self.results: List[ValidationResult] = []
    
    def execute_nrql(self, query: str) -> Optional[Dict]:
        """Execute NRQL query against New Relic GraphQL API"""
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
                f"{self.base_url}/graphql",
                json=graphql_query,
                timeout=30
            )
            response.raise_for_status()
            
            data = response.json()
            if 'errors' in data:
                logger.error(f"GraphQL errors: {data['errors']}")
                return None
                
            return data['data']['actor']['account']['nrql']['results']
            
        except requests.exceptions.RequestException as e:
            logger.error(f"API request failed: {e}")
            return None
        except Exception as e:
            logger.error(f"Unexpected error executing NRQL: {e}")
            return None
    
    def validate_module_data_presence(self, module: str) -> ValidationResult:
        """Validate that expected metrics are present for a module"""
        config = self.modules[module]
        expected_metrics = config['expected_metrics']
        
        # Check for data presence in last 10 minutes
        metric_list = "', '".join(expected_metrics)
        query = f"""
        SELECT count(*) as 'data_points'
        FROM Metric 
        WHERE service.name = '{module}' 
          AND metricName IN ('{metric_list}')
          AND timestamp > (now() - 10 * 60 * 1000)
        FACET metricName
        """
        
        results = self.execute_nrql(query)
        if results is None:
            return ValidationResult(
                module=module,
                test_name="data_presence",
                status=ValidationStatus.FAIL,
                message="Failed to execute NRQL query",
                recommendations=["Check API key and account ID", "Verify network connectivity"]
            )
        
        found_metrics = [result['metricName'] for result in results if result['data_points'] > 0]
        missing_metrics = [metric for metric in expected_metrics if metric not in found_metrics]
        
        total_data_points = sum(result['data_points'] for result in results)
        
        if len(missing_metrics) == 0:
            status = ValidationStatus.PASS
            message = f"All {len(expected_metrics)} expected metrics found"
        elif len(missing_metrics) < len(expected_metrics) / 2:
            status = ValidationStatus.WARN
            message = f"{len(missing_metrics)} metrics missing: {', '.join(missing_metrics)}"
        else:
            status = ValidationStatus.FAIL
            message = f"Most metrics missing: {', '.join(missing_metrics)}"
        
        recommendations = []
        if missing_metrics:
            recommendations.extend([
                f"Check {module} module health and configuration",
                "Verify MySQL connectivity if applicable",
                "Check container logs for errors",
                f"Run: ./troubleshoot-missing-data.sh {module}"
            ])
        
        return ValidationResult(
            module=module,
            test_name="data_presence",
            status=status,
            message=message,
            data_points=total_data_points,
            expected_metrics=expected_metrics,
            found_metrics=found_metrics,
            recommendations=recommendations
        )
    
    def validate_module_data_freshness(self, module: str) -> ValidationResult:
        """Validate that data is fresh (within expected timeframe)"""
        config = self.modules[module]
        freshness_threshold = config['data_freshness_minutes']
        primary_metric = config['expected_metrics'][0]  # Use first metric as representative
        
        query = f"""
        SELECT (now() - latest(timestamp))/1000/60 as 'minutes_since_last_data'
        FROM Metric 
        WHERE service.name = '{module}'
          AND metricName = '{primary_metric}'
        SINCE 1 hour ago
        """
        
        results = self.execute_nrql(query)
        if not results or len(results) == 0:
            return ValidationResult(
                module=module,
                test_name="data_freshness",
                status=ValidationStatus.FAIL,
                message="No data found for freshness check",
                recommendations=[f"Check {module} module is running and sending data"]
            )
        
        minutes_since_last = results[0].get('minutes_since_last_data', float('inf'))
        
        if minutes_since_last <= freshness_threshold:
            status = ValidationStatus.PASS
            message = f"Data is fresh ({minutes_since_last:.1f} minutes old)"
        elif minutes_since_last <= freshness_threshold * 2:
            status = ValidationStatus.WARN
            message = f"Data is stale ({minutes_since_last:.1f} minutes old)"
        else:
            status = ValidationStatus.FAIL
            message = f"Data is very stale ({minutes_since_last:.1f} minutes old)"
        
        recommendations = []
        if minutes_since_last > freshness_threshold:
            recommendations.extend([
                f"Check {module} collection interval configuration",
                "Verify module is actively running",
                "Check for export failures to New Relic"
            ])
        
        return ValidationResult(
            module=module,
            test_name="data_freshness",
            status=status,
            message=message,
            recommendations=recommendations
        )
    
    def validate_module_attributes(self, module: str) -> ValidationResult:
        """Validate that critical attributes are present"""
        config = self.modules[module]
        critical_attributes = config['critical_attributes']
        
        # Check attribute presence
        results_summary = []
        total_records = 0
        
        for attribute in critical_attributes:
            query = f"""
            SELECT count(*) as 'records_with_attribute'
            FROM Metric 
            WHERE service.name = '{module}'
              AND {attribute} IS NOT NULL
            SINCE 30 minutes ago
            """
            
            results = self.execute_nrql(query)
            if results and len(results) > 0:
                count = results[0].get('records_with_attribute', 0)
                results_summary.append(f"{attribute}: {count} records")
                total_records += count
        
        if total_records > 0:
            status = ValidationStatus.PASS
            message = f"Critical attributes found: {', '.join(results_summary)}"
        else:
            status = ValidationStatus.FAIL
            message = "No records with critical attributes found"
        
        return ValidationResult(
            module=module,
            test_name="attribute_validation",
            status=status,
            message=message,
            data_points=total_records
        )
    
    def validate_entity_synthesis(self, module: str) -> ValidationResult:
        """Validate New Relic entity synthesis"""
        query = f"""
        SELECT count(DISTINCT entity.guid) as 'unique_entities'
        FROM Metric 
        WHERE service.name = '{module}'
          AND entity.type = 'MYSQL_INSTANCE'
          AND entity.guid IS NOT NULL
          AND newrelic.entity.synthesis = 'true'
        SINCE 30 minutes ago
        """
        
        results = self.execute_nrql(query)
        if not results or len(results) == 0:
            return ValidationResult(
                module=module,
                test_name="entity_synthesis",
                status=ValidationStatus.FAIL,
                message="No entity synthesis data found",
                recommendations=["Check entity synthesis processor configuration"]
            )
        
        entity_count = results[0].get('unique_entities', 0)
        
        if entity_count > 0:
            status = ValidationStatus.PASS
            message = f"Entity synthesis working: {entity_count} entities"
        else:
            status = ValidationStatus.WARN
            message = "Entity synthesis not configured or not working"
        
        return ValidationResult(
            module=module,
            test_name="entity_synthesis",
            status=status,
            message=message,
            data_points=entity_count
        )
    
    def validate_new_relic_attribution(self, module: str) -> ValidationResult:
        """Validate proper New Relic attribution"""
        query = f"""
        SELECT count(*) as 'attributed_records'
        FROM Metric 
        WHERE service.name = '{module}'
          AND newrelic.source = 'opentelemetry'
          AND instrumentation.name = 'mysql-otel-collector'
          AND instrumentation.provider = 'opentelemetry'
        SINCE 30 minutes ago
        """
        
        results = self.execute_nrql(query)
        if not results or len(results) == 0:
            return ValidationResult(
                module=module,
                test_name="nr_attribution",
                status=ValidationStatus.FAIL,
                message="No properly attributed records found",
                recommendations=["Check New Relic attribution processor configuration"]
            )
        
        attributed_count = results[0].get('attributed_records', 0)
        
        if attributed_count > 0:
            status = ValidationStatus.PASS
            message = f"Proper attribution: {attributed_count} records"
        else:
            status = ValidationStatus.FAIL
            message = "No proper New Relic attribution found"
        
        return ValidationResult(
            module=module,
            test_name="nr_attribution",
            status=status,
            message=message,
            data_points=attributed_count
        )
    
    def validate_module_dependencies(self, module: str) -> ValidationResult:
        """Validate module federation dependencies"""
        dependency_map = {
            'anomaly-detector': ['core-metrics', 'sql-intelligence', 'wait-profiler'],
            'business-impact': ['sql-intelligence'],
            'performance-advisor': ['core-metrics', 'sql-intelligence', 'anomaly-detector'],
            'alert-manager': ['anomaly-detector', 'core-metrics', 'performance-advisor']
        }
        
        if module not in dependency_map:
            return ValidationResult(
                module=module,
                test_name="dependencies",
                status=ValidationStatus.INFO,
                message="Module has no dependencies"
            )
        
        dependencies = dependency_map[module]
        failed_dependencies = []
        
        for dep_module in dependencies:
            # Check if dependency module is sending data
            query = f"""
            SELECT count(*) as 'records'
            FROM Metric 
            WHERE service.name = '{dep_module}'
            SINCE 10 minutes ago
            """
            
            results = self.execute_nrql(query)
            if not results or len(results) == 0 or results[0].get('records', 0) == 0:
                failed_dependencies.append(dep_module)
        
        if len(failed_dependencies) == 0:
            status = ValidationStatus.PASS
            message = f"All dependencies healthy: {', '.join(dependencies)}"
        else:
            status = ValidationStatus.WARN
            message = f"Dependencies with issues: {', '.join(failed_dependencies)}"
        
        return ValidationResult(
            module=module,
            test_name="dependencies",
            status=status,
            message=message,
            recommendations=[f"Check {dep} module health" for dep in failed_dependencies]
        )
    
    def validate_all_modules(self, modules: Optional[List[str]] = None) -> List[ValidationResult]:
        """Run all validations for specified modules or all modules"""
        if modules is None:
            modules = list(self.modules.keys())
        
        all_results = []
        
        for module in modules:
            logger.info(f"Validating module: {module}")
            
            # Run all validation tests for this module
            tests = [
                self.validate_module_data_presence,
                self.validate_module_data_freshness,
                self.validate_module_attributes,
                self.validate_entity_synthesis,
                self.validate_new_relic_attribution,
                self.validate_module_dependencies
            ]
            
            for test in tests:
                try:
                    result = test(module)
                    all_results.append(result)
                    logger.info(f"{module}.{result.test_name}: {result.status.value} - {result.message}")
                except Exception as e:
                    logger.error(f"Test {test.__name__} failed for {module}: {e}")
                    all_results.append(ValidationResult(
                        module=module,
                        test_name=test.__name__,
                        status=ValidationStatus.FAIL,
                        message=f"Test execution failed: {e}"
                    ))
        
        self.results.extend(all_results)
        return all_results
    
    def generate_summary_report(self) -> Dict:
        """Generate comprehensive summary report"""
        if not self.results:
            return {"error": "No validation results available"}
        
        # Aggregate results by status
        status_counts = {status.value: 0 for status in ValidationStatus}
        module_summaries = {}
        
        for result in self.results:
            status_counts[result.status.value] += 1
            
            if result.module not in module_summaries:
                module_summaries[result.module] = {
                    'total_tests': 0,
                    'passed': 0,
                    'failed': 0,
                    'warnings': 0,
                    'info': 0,
                    'total_data_points': 0,
                    'issues': []
                }
            
            summary = module_summaries[result.module]
            summary['total_tests'] += 1
            summary['total_data_points'] += result.data_points
            
            if result.status == ValidationStatus.PASS:
                summary['passed'] += 1
            elif result.status == ValidationStatus.FAIL:
                summary['failed'] += 1
                summary['issues'].append(f"{result.test_name}: {result.message}")
            elif result.status == ValidationStatus.WARN:
                summary['warnings'] += 1
                summary['issues'].append(f"{result.test_name}: {result.message}")
            else:
                summary['info'] += 1
        
        # Calculate overall health score
        total_tests = len(self.results)
        health_score = (status_counts['PASS'] / total_tests * 100) if total_tests > 0 else 0
        
        return {
            'timestamp': datetime.now().isoformat(),
            'overall_health_score': round(health_score, 2),
            'total_tests': total_tests,
            'status_summary': status_counts,
            'module_summaries': module_summaries,
            'critical_issues': [
                result for result in self.results 
                if result.status == ValidationStatus.FAIL
            ],
            'recommendations': self._generate_recommendations()
        }
    
    def _generate_recommendations(self) -> List[str]:
        """Generate recommendations based on validation results"""
        recommendations = []
        
        failed_modules = set()
        for result in self.results:
            if result.status == ValidationStatus.FAIL:
                failed_modules.add(result.module)
                recommendations.extend(result.recommendations)
        
        if failed_modules:
            recommendations.append(f"Run detailed diagnostics: ./troubleshoot-missing-data.sh {' '.join(failed_modules)}")
        
        # Remove duplicates while preserving order
        return list(dict.fromkeys(recommendations))
    
    def export_results(self, filename: str, format: str = 'json'):
        """Export validation results to file"""
        summary = self.generate_summary_report()
        
        if format == 'json':
            with open(filename, 'w') as f:
                json.dump(summary, f, indent=2, default=str)
        elif format == 'html':
            self._export_html_report(filename, summary)
        else:
            raise ValueError(f"Unsupported format: {format}")
        
        logger.info(f"Results exported to {filename}")
    
    def _export_html_report(self, filename: str, summary: Dict):
        """Export HTML validation report"""
        html_content = f"""
        <!DOCTYPE html>
        <html>
        <head>
            <title>NRDB Validation Report</title>
            <style>
                body {{ font-family: Arial, sans-serif; margin: 20px; }}
                .header {{ background: #f4f4f4; padding: 20px; border-radius: 5px; }}
                .summary {{ display: flex; gap: 20px; margin: 20px 0; }}
                .metric {{ background: #e9f4ff; padding: 15px; border-radius: 5px; flex: 1; }}
                .module {{ margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }}
                .pass {{ color: green; }}
                .fail {{ color: red; }}
                .warn {{ color: orange; }}
                .issue {{ background: #ffe9e9; padding: 10px; margin: 5px 0; border-radius: 3px; }}
                .recommendation {{ background: #e9ffe9; padding: 10px; margin: 5px 0; border-radius: 3px; }}
            </style>
        </head>
        <body>
            <div class="header">
                <h1>NRDB Validation Report</h1>
                <p>Generated: {summary['timestamp']}</p>
                <p>Overall Health Score: <strong>{summary['overall_health_score']}%</strong></p>
            </div>
            
            <div class="summary">
                <div class="metric">
                    <h3>Total Tests</h3>
                    <p><strong>{summary['total_tests']}</strong></p>
                </div>
                <div class="metric">
                    <h3>Passed</h3>
                    <p class="pass"><strong>{summary['status_summary']['PASS']}</strong></p>
                </div>
                <div class="metric">
                    <h3>Failed</h3>
                    <p class="fail"><strong>{summary['status_summary']['FAIL']}</strong></p>
                </div>
                <div class="metric">
                    <h3>Warnings</h3>
                    <p class="warn"><strong>{summary['status_summary']['WARN']}</strong></p>
                </div>
            </div>
            
            <h2>Module Details</h2>
        """
        
        for module, details in summary['module_summaries'].items():
            html_content += f"""
            <div class="module">
                <h3>{module}</h3>
                <p>Tests: {details['total_tests']} | 
                   Passed: <span class="pass">{details['passed']}</span> | 
                   Failed: <span class="fail">{details['failed']}</span> | 
                   Warnings: <span class="warn">{details['warnings']}</span></p>
                <p>Data Points: {details['total_data_points']}</p>
                
                {f'<h4>Issues:</h4>' + ''.join(f'<div class="issue">{issue}</div>' for issue in details['issues']) if details['issues'] else ''}
            </div>
            """
        
        if summary['recommendations']:
            html_content += f"""
            <h2>Recommendations</h2>
            {''.join(f'<div class="recommendation">{rec}</div>' for rec in summary['recommendations'])}
            """
        
        html_content += """
        </body>
        </html>
        """
        
        with open(filename, 'w') as f:
            f.write(html_content)

def main():
    parser = argparse.ArgumentParser(description='NRDB Validation Framework')
    parser.add_argument('--api-key', help='New Relic API Key (overrides .env)')
    parser.add_argument('--account-id', help='New Relic Account ID (overrides .env)')
    parser.add_argument('--modules', nargs='+', help='Specific modules to validate')
    parser.add_argument('--validate-all', action='store_true', help='Validate all modules')
    parser.add_argument('--output', default='validation_report', help='Output file base name')
    parser.add_argument('--format', choices=['json', 'html'], default='json', help='Output format')
    parser.add_argument('--verbose', action='store_true', help='Verbose logging')
    
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
    
    # Initialize validator
    validator = NRDBValidator(api_key, account_id)
    
    # Determine modules to validate
    modules_to_validate = None
    if args.modules:
        modules_to_validate = args.modules
    elif args.validate_all:
        modules_to_validate = list(validator.modules.keys())
    else:
        print("Please specify --modules or --validate-all")
        sys.exit(1)
    
    # Run validation
    logger.info(f"Starting validation for modules: {modules_to_validate}")
    results = validator.validate_all_modules(modules_to_validate)
    
    # Generate and export report
    summary = validator.generate_summary_report()
    output_file = f"{args.output}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.{args.format}"
    validator.export_results(output_file, args.format)
    
    # Print summary to console
    print(f"\n{'='*60}")
    print(f"VALIDATION SUMMARY")
    print(f"{'='*60}")
    print(f"Overall Health Score: {summary['overall_health_score']}%")
    print(f"Total Tests: {summary['total_tests']}")
    print(f"Status: {summary['status_summary']}")
    
    if summary['critical_issues']:
        print(f"\nCritical Issues ({len(summary['critical_issues'])}):")
        for issue in summary['critical_issues'][:5]:  # Show first 5
            print(f"  - {issue.module}.{issue.test_name}: {issue.message}")
    
    if summary['recommendations']:
        print(f"\nTop Recommendations:")
        for rec in summary['recommendations'][:3]:  # Show first 3
            print(f"  - {rec}")
    
    print(f"\nDetailed report saved to: {output_file}")
    
    # Exit with appropriate code
    if summary['status_summary']['FAIL'] > 0:
        sys.exit(1)
    elif summary['status_summary']['WARN'] > 0:
        sys.exit(2)
    else:
        sys.exit(0)

if __name__ == '__main__':
    main()