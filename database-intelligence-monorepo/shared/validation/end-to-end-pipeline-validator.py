#!/usr/bin/env python3
"""
End-to-End Data Flow Pipeline Validator

This tool validates the complete data flow pipeline from MySQL source through
OpenTelemetry collectors to New Relic backend, ensuring data integrity and
proper flow at each stage.

Pipeline Stages:
1. MySQL Source Data Validation
2. OpenTelemetry Collector Health Check
3. Prometheus Metrics Endpoint Validation  
4. New Relic Data Ingestion Validation
5. Cross-Module Federation Validation
6. Data Consistency and Latency Validation

Usage:
    python3 end-to-end-pipeline-validator.py
    python3 end-to-end-pipeline-validator.py --stage mysql --stage otel --stage newrelic
    python3 end-to-end-pipeline-validator.py --modules core-metrics sql-intelligence
"""

import os
import sys
import json
import time
import requests
import subprocess
import argparse
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Tuple, Any
import logging
from dataclasses import dataclass
from enum import Enum
from concurrent.futures import ThreadPoolExecutor, as_completed
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class ValidationStage(Enum):
    MYSQL = "mysql"
    OTEL = "otel"
    PROMETHEUS = "prometheus"
    NEWRELIC = "newrelic"
    FEDERATION = "federation"
    CONSISTENCY = "consistency"

class PipelineStatus(Enum):
    HEALTHY = "HEALTHY"
    DEGRADED = "DEGRADED"
    FAILED = "FAILED"
    UNKNOWN = "UNKNOWN"

@dataclass
class StageResult:
    stage: ValidationStage
    status: PipelineStatus
    message: str
    details: Dict[str, Any]
    duration_ms: float
    timestamp: datetime
    issues: List[str] = None
    recommendations: List[str] = None
    
    def __post_init__(self):
        if self.issues is None:
            self.issues = []
        if self.recommendations is None:
            self.recommendations = []

class EndToEndPipelineValidator:
    def __init__(self):
        self.modules = {
            'core-metrics': {'port': 8081, 'health_port': 13133},
            'sql-intelligence': {'port': 8082, 'health_port': 13133},
            'wait-profiler': {'port': 8083, 'health_port': 13133},
            'anomaly-detector': {'port': 8084, 'health_port': 13133},
            'business-impact': {'port': 8085, 'health_port': 13133},
            'replication-monitor': {'port': 8086, 'health_port': 13133},
            'performance-advisor': {'port': 8087, 'health_port': 13133},
            'resource-monitor': {'port': 8088, 'health_port': 13135},
            'alert-manager': {'port': 8089, 'health_port': 13134},
            'canary-tester': {'port': 8090, 'health_port': 13133},
            'cross-signal-correlator': {'port': 8099, 'health_port': 13137}
        }
        
        # New Relic configuration
        self.nr_api_key = os.getenv('NEW_RELIC_API_KEY')
        self.nr_account_id = os.getenv('NEW_RELIC_ACCOUNT_ID')
        self.nr_otlp_endpoint = os.getenv('NEW_RELIC_OTLP_ENDPOINT')
        
        if not all([self.nr_api_key, self.nr_account_id]):
            raise ValueError("NEW_RELIC_API_KEY and NEW_RELIC_ACCOUNT_ID must be set")
        
        # Setup HTTP session for New Relic API
        self.session = requests.Session()
        self.session.headers.update({
            'Api-Key': self.nr_api_key,
            'Content-Type': 'application/json'
        })
        
        # MySQL configuration
        self.mysql_endpoint = os.getenv('MYSQL_ENDPOINT', 'localhost:3306')
        self.mysql_user = os.getenv('MYSQL_USER', 'root')
        self.mysql_password = os.getenv('MYSQL_PASSWORD', 'test')
        self.mysql_database = os.getenv('MYSQL_DATABASE', 'testdb')
        
        self.validation_results = []

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

    def validate_mysql_source(self, modules_to_test: List[str] = None) -> StageResult:
        """Validate MySQL source connectivity and data availability"""
        start_time = time.time()
        logger.info("Validating MySQL source connectivity...")
        
        result = StageResult(
            stage=ValidationStage.MYSQL,
            status=PipelineStatus.UNKNOWN,
            message="",
            details={},
            duration_ms=0,
            timestamp=datetime.now()
        )
        
        if modules_to_test is None:
            modules_to_test = list(self.modules.keys())
        
        mysql_tests = {
            'connection': False,
            'performance_schema': False,
            'binlog_enabled': False,
            'required_tables': False
        }
        
        try:
            # Test basic MySQL connectivity
            mysql_cmd = [
                'mysql',
                '-h', self.mysql_endpoint.split(':')[0],
                '-P', self.mysql_endpoint.split(':')[1] if ':' in self.mysql_endpoint else '3306',
                '-u', self.mysql_user,
                f'-p{self.mysql_password}',
                '-e', 'SELECT 1'
            ]
            
            mysql_result = subprocess.run(mysql_cmd, capture_output=True, text=True, timeout=10)
            
            if mysql_result.returncode == 0:
                mysql_tests['connection'] = True
                logger.info("✓ MySQL connection successful")
            else:
                result.issues.append(f"MySQL connection failed: {mysql_result.stderr}")
                result.recommendations.append("Check MySQL connectivity and credentials")
                
        except subprocess.TimeoutExpired:
            result.issues.append("MySQL connection timeout")
            result.recommendations.append("Check MySQL server availability and network connectivity")
        except FileNotFoundError:
            result.issues.append("MySQL client not found")
            result.recommendations.append("Install MySQL client or use Docker to test connectivity")
        except Exception as e:
            result.issues.append(f"MySQL test error: {e}")
        
        # Test performance_schema availability
        if mysql_tests['connection']:
            try:
                perf_schema_cmd = [
                    'mysql',
                    '-h', self.mysql_endpoint.split(':')[0],
                    '-P', self.mysql_endpoint.split(':')[1] if ':' in self.mysql_endpoint else '3306',
                    '-u', self.mysql_user,
                    f'-p{self.mysql_password}',
                    '-e', 'SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = "performance_schema"'
                ]
                
                perf_result = subprocess.run(perf_schema_cmd, capture_output=True, text=True, timeout=10)
                
                if perf_result.returncode == 0 and '1' in perf_result.stdout:
                    mysql_tests['performance_schema'] = True
                    logger.info("✓ Performance schema available")
                else:
                    result.issues.append("Performance schema not available")
                    result.recommendations.append("Enable performance_schema in MySQL configuration")
                    
            except Exception as e:
                result.issues.append(f"Performance schema check failed: {e}")
        
        # Check for key tables needed by modules
        key_tables_to_check = [
            'performance_schema.events_statements_summary_by_digest',
            'performance_schema.events_waits_summary_global_by_event_name',
            'performance_schema.table_io_waits_summary_by_table'
        ]
        
        if mysql_tests['performance_schema']:
            tables_found = 0
            for table in key_tables_to_check:
                try:
                    table_cmd = [
                        'mysql',
                        '-h', self.mysql_endpoint.split(':')[0],
                        '-P', self.mysql_endpoint.split(':')[1] if ':' in self.mysql_endpoint else '3306',
                        '-u', self.mysql_user,
                        f'-p{self.mysql_password}',
                        '-e', f'SELECT COUNT(*) FROM {table} LIMIT 1'
                    ]
                    
                    table_result = subprocess.run(table_cmd, capture_output=True, text=True, timeout=5)
                    if table_result.returncode == 0:
                        tables_found += 1
                        
                except Exception:
                    pass
            
            if tables_found >= len(key_tables_to_check) * 0.8:  # 80% of tables available
                mysql_tests['required_tables'] = True
                logger.info(f"✓ Required tables available ({tables_found}/{len(key_tables_to_check)})")
            else:
                result.issues.append(f"Only {tables_found}/{len(key_tables_to_check)} required tables available")
                result.recommendations.append("Enable required performance_schema instruments")
        
        # Determine overall status
        healthy_tests = sum(mysql_tests.values())
        total_tests = len(mysql_tests)
        
        result.details = {
            'tests_performed': mysql_tests,
            'health_score': healthy_tests / total_tests,
            'mysql_endpoint': self.mysql_endpoint
        }
        
        if healthy_tests == total_tests:
            result.status = PipelineStatus.HEALTHY
            result.message = f"MySQL source fully operational ({healthy_tests}/{total_tests} checks passed)"
        elif healthy_tests >= total_tests * 0.7:  # 70% threshold
            result.status = PipelineStatus.DEGRADED
            result.message = f"MySQL source partially operational ({healthy_tests}/{total_tests} checks passed)"
        else:
            result.status = PipelineStatus.FAILED
            result.message = f"MySQL source critical issues ({healthy_tests}/{total_tests} checks passed)"
        
        result.duration_ms = (time.time() - start_time) * 1000
        return result

    def validate_otel_collectors(self, modules_to_test: List[str] = None) -> StageResult:
        """Validate OpenTelemetry collector health and configuration"""
        start_time = time.time()
        logger.info("Validating OpenTelemetry collectors...")
        
        result = StageResult(
            stage=ValidationStage.OTEL,
            status=PipelineStatus.UNKNOWN,
            message="",
            details={},
            duration_ms=0,
            timestamp=datetime.now()
        )
        
        if modules_to_test is None:
            modules_to_test = list(self.modules.keys())
        
        collector_health = {}
        healthy_collectors = 0
        
        for module in modules_to_test:
            module_config = self.modules.get(module)
            if not module_config:
                continue
                
            health_port = module_config['health_port']
            module_health = {
                'health_endpoint': False,
                'config_valid': False,
                'container_running': False
            }
            
            # Check health endpoint
            try:
                health_url = f"http://localhost:{health_port}/"
                response = requests.get(health_url, timeout=5)
                if response.status_code == 200:
                    module_health['health_endpoint'] = True
                    logger.info(f"✓ {module} health endpoint responding")
                else:
                    result.issues.append(f"{module} health endpoint returned {response.status_code}")
                    
            except requests.exceptions.ConnectionError:
                result.issues.append(f"{module} health endpoint not accessible")
                result.recommendations.append(f"Check if {module} collector is running")
            except Exception as e:
                result.issues.append(f"{module} health check failed: {e}")
            
            # Check if container is running
            try:
                container_name = f"{module}-otel-collector"
                docker_cmd = ['docker', 'ps', '--filter', f'name={container_name}', '--format', '{{.Names}}']
                docker_result = subprocess.run(docker_cmd, capture_output=True, text=True, timeout=5)
                
                if container_name in docker_result.stdout:
                    module_health['container_running'] = True
                    logger.info(f"✓ {module} container running")
                else:
                    result.issues.append(f"{module} container not running")
                    result.recommendations.append(f"Start {module} with: cd modules/{module} && docker-compose up -d")
                    
            except Exception as e:
                result.issues.append(f"{module} container check failed: {e}")
            
            # Check configuration file exists and is valid
            config_path = f"modules/{module}/config/collector.yaml"
            if os.path.exists(config_path):
                try:
                    import yaml
                    with open(config_path, 'r') as f:
                        config = yaml.safe_load(f)
                    
                    # Basic configuration validation
                    required_sections = ['receivers', 'processors', 'exporters', 'service']
                    if all(section in config for section in required_sections):
                        module_health['config_valid'] = True
                        logger.info(f"✓ {module} configuration valid")
                    else:
                        result.issues.append(f"{module} configuration missing required sections")
                        
                except Exception as e:
                    result.issues.append(f"{module} configuration validation failed: {e}")
            else:
                result.issues.append(f"{module} configuration file not found")
            
            # Calculate module health score
            module_score = sum(module_health.values()) / len(module_health)
            if module_score >= 0.7:  # 70% threshold
                healthy_collectors += 1
                
            collector_health[module] = module_health
        
        # Determine overall status
        total_modules = len(modules_to_test)
        health_percentage = healthy_collectors / total_modules if total_modules > 0 else 0
        
        result.details = {
            'collector_health': collector_health,
            'healthy_collectors': healthy_collectors,
            'total_collectors': total_modules,
            'health_percentage': health_percentage
        }
        
        if health_percentage >= 0.9:  # 90% healthy
            result.status = PipelineStatus.HEALTHY
            result.message = f"OpenTelemetry collectors healthy ({healthy_collectors}/{total_modules})"
        elif health_percentage >= 0.7:  # 70% healthy
            result.status = PipelineStatus.DEGRADED
            result.message = f"OpenTelemetry collectors partially healthy ({healthy_collectors}/{total_modules})"
        else:
            result.status = PipelineStatus.FAILED
            result.message = f"OpenTelemetry collectors critical issues ({healthy_collectors}/{total_modules})"
        
        result.duration_ms = (time.time() - start_time) * 1000
        return result

    def validate_prometheus_endpoints(self, modules_to_test: List[str] = None) -> StageResult:
        """Validate Prometheus metrics endpoints are accessible and serving data"""
        start_time = time.time()
        logger.info("Validating Prometheus metrics endpoints...")
        
        result = StageResult(
            stage=ValidationStage.PROMETHEUS,
            status=PipelineStatus.UNKNOWN,
            message="",
            details={},
            duration_ms=0,
            timestamp=datetime.now()
        )
        
        if modules_to_test is None:
            modules_to_test = list(self.modules.keys())
        
        metrics_health = {}
        healthy_endpoints = 0
        
        for module in modules_to_test:
            module_config = self.modules.get(module)
            if not module_config:
                continue
                
            metrics_port = module_config['port']
            endpoint_health = {
                'endpoint_accessible': False,
                'metrics_available': False,
                'metric_count': 0
            }
            
            try:
                metrics_url = f"http://localhost:{metrics_port}/metrics"
                response = requests.get(metrics_url, timeout=10)
                
                if response.status_code == 200:
                    endpoint_health['endpoint_accessible'] = True
                    
                    # Count metrics in the response
                    metrics_text = response.text
                    metric_lines = [line for line in metrics_text.split('\n') 
                                  if line and not line.startswith('#')]
                    
                    endpoint_health['metric_count'] = len(metric_lines)
                    
                    if metric_lines:
                        endpoint_health['metrics_available'] = True
                        logger.info(f"✓ {module} serving {len(metric_lines)} metrics")
                    else:
                        result.issues.append(f"{module} endpoint accessible but no metrics")
                        result.recommendations.append(f"Check {module} data collection configuration")
                else:
                    result.issues.append(f"{module} metrics endpoint returned {response.status_code}")
                    
            except requests.exceptions.ConnectionError:
                result.issues.append(f"{module} metrics endpoint not accessible")
                result.recommendations.append(f"Check if {module} is running on port {metrics_port}")
            except Exception as e:
                result.issues.append(f"{module} metrics validation failed: {e}")
            
            # Calculate endpoint health
            endpoint_score = sum(v for v in endpoint_health.values() if isinstance(v, bool)) / 2
            if endpoint_score >= 1.0:  # Both accessible and has metrics
                healthy_endpoints += 1
                
            metrics_health[module] = endpoint_health
        
        # Determine overall status
        total_modules = len(modules_to_test)
        health_percentage = healthy_endpoints / total_modules if total_modules > 0 else 0
        
        result.details = {
            'metrics_health': metrics_health,
            'healthy_endpoints': healthy_endpoints,
            'total_endpoints': total_modules,
            'health_percentage': health_percentage
        }
        
        if health_percentage >= 0.9:
            result.status = PipelineStatus.HEALTHY
            result.message = f"Prometheus endpoints healthy ({healthy_endpoints}/{total_modules})"
        elif health_percentage >= 0.7:
            result.status = PipelineStatus.DEGRADED
            result.message = f"Prometheus endpoints partially healthy ({healthy_endpoints}/{total_modules})"
        else:
            result.status = PipelineStatus.FAILED
            result.message = f"Prometheus endpoints critical issues ({healthy_endpoints}/{total_modules})"
        
        result.duration_ms = (time.time() - start_time) * 1000
        return result

    def validate_newrelic_ingestion(self, modules_to_test: List[str] = None) -> StageResult:
        """Validate data is being ingested into New Relic successfully"""
        start_time = time.time()
        logger.info("Validating New Relic data ingestion...")
        
        result = StageResult(
            stage=ValidationStage.NEWRELIC,
            status=PipelineStatus.UNKNOWN,
            message="",
            details={},
            duration_ms=0,
            timestamp=datetime.now()
        )
        
        if modules_to_test is None:
            modules_to_test = list(self.modules.keys())
        
        ingestion_health = {}
        healthy_ingestion = 0
        
        for module in modules_to_test:
            module_health = {
                'data_present': False,
                'data_fresh': False,
                'record_count': 0,
                'last_ingestion': None
            }
            
            # Check for recent data in New Relic
            query = f"""
            SELECT count(*) as 'record_count',
                   latest(timestamp) as 'last_timestamp'
            FROM Metric 
            WHERE service.name = '{module}'
            SINCE 1 hour ago
            """
            
            try:
                data = self.execute_nrql(query)
                if data and len(data) > 0:
                    record_count = data[0].get('record_count', 0)
                    last_timestamp = data[0].get('last_timestamp')
                    
                    module_health['record_count'] = record_count
                    module_health['last_ingestion'] = last_timestamp
                    
                    if record_count > 0:
                        module_health['data_present'] = True
                        
                        # Check data freshness (within last 10 minutes)
                        if last_timestamp:
                            try:
                                last_time = datetime.fromisoformat(last_timestamp.replace('Z', '+00:00'))
                                time_diff = datetime.now().astimezone() - last_time
                                
                                if time_diff.total_seconds() < 600:  # 10 minutes
                                    module_health['data_fresh'] = True
                                    logger.info(f"✓ {module} fresh data ({record_count} records)")
                                else:
                                    result.issues.append(f"{module} data is stale ({time_diff.total_seconds():.0f}s old)")
                            except Exception:
                                result.issues.append(f"{module} timestamp parsing failed")
                    else:
                        result.issues.append(f"{module} no data in New Relic")
                        result.recommendations.append(f"Check {module} OTLP export configuration")
                else:
                    result.issues.append(f"{module} NRQL query failed")
                    
            except Exception as e:
                result.issues.append(f"{module} New Relic validation failed: {e}")
            
            # Calculate module health
            health_score = sum(v for v in module_health.values() if isinstance(v, bool)) / 2
            if health_score >= 1.0:  # Both present and fresh
                healthy_ingestion += 1
                
            ingestion_health[module] = module_health
        
        # Additional check for overall data flow
        total_query = """
        SELECT count(*) as 'total_records'
        FROM Metric 
        WHERE service.name IN ('core-metrics', 'sql-intelligence', 'wait-profiler', 'anomaly-detector',
                              'business-impact', 'replication-monitor', 'performance-advisor', 
                              'resource-monitor', 'alert-manager', 'canary-tester', 'cross-signal-correlator')
        SINCE 1 hour ago
        """
        
        total_records = 0
        try:
            total_data = self.execute_nrql(total_query)
            if total_data and len(total_data) > 0:
                total_records = total_data[0].get('total_records', 0)
        except Exception as e:
            result.issues.append(f"Total records query failed: {e}")
        
        # Determine overall status
        total_modules = len(modules_to_test)
        health_percentage = healthy_ingestion / total_modules if total_modules > 0 else 0
        
        result.details = {
            'ingestion_health': ingestion_health,
            'healthy_ingestion': healthy_ingestion,
            'total_modules': total_modules,
            'health_percentage': health_percentage,
            'total_records_1h': total_records
        }
        
        if health_percentage >= 0.8 and total_records > 0:
            result.status = PipelineStatus.HEALTHY
            result.message = f"New Relic ingestion healthy ({healthy_ingestion}/{total_modules}, {total_records} total records)"
        elif health_percentage >= 0.6 or total_records > 0:
            result.status = PipelineStatus.DEGRADED
            result.message = f"New Relic ingestion partially healthy ({healthy_ingestion}/{total_modules}, {total_records} total records)"
        else:
            result.status = PipelineStatus.FAILED
            result.message = f"New Relic ingestion critical issues ({healthy_ingestion}/{total_modules}, {total_records} total records)"
        
        result.duration_ms = (time.time() - start_time) * 1000
        return result

    def validate_federation_flow(self, modules_to_test: List[str] = None) -> StageResult:
        """Validate cross-module federation and data dependencies"""
        start_time = time.time()
        logger.info("Validating federation data flow...")
        
        result = StageResult(
            stage=ValidationStage.FEDERATION,
            status=PipelineStatus.UNKNOWN,
            message="",
            details={},
            duration_ms=0,
            timestamp=datetime.now()
        )
        
        # Federation dependencies
        federation_dependencies = {
            'anomaly-detector': ['core-metrics', 'sql-intelligence', 'wait-profiler'],
            'business-impact': ['sql-intelligence'],
            'performance-advisor': ['core-metrics', 'sql-intelligence', 'anomaly-detector'],
            'alert-manager': ['anomaly-detector', 'core-metrics', 'performance-advisor']
        }
        
        federation_health = {}
        healthy_federations = 0
        
        for consumer, providers in federation_dependencies.items():
            if modules_to_test and consumer not in modules_to_test:
                continue
                
            consumer_health = {
                'consumer_active': False,
                'providers_available': 0,
                'federated_data_present': False
            }
            
            # Check if consumer is active
            consumer_query = f"""
            SELECT count(*) as 'records'
            FROM Metric 
            WHERE service.name = '{consumer}'
            SINCE 30 minutes ago
            """
            
            try:
                consumer_data = self.execute_nrql(consumer_query)
                if consumer_data and consumer_data[0].get('records', 0) > 0:
                    consumer_health['consumer_active'] = True
            except Exception as e:
                result.issues.append(f"Consumer {consumer} check failed: {e}")
            
            # Check provider availability
            available_providers = 0
            for provider in providers:
                provider_query = f"""
                SELECT count(*) as 'records'
                FROM Metric 
                WHERE service.name = '{provider}'
                SINCE 30 minutes ago
                """
                
                try:
                    provider_data = self.execute_nrql(provider_query)
                    if provider_data and provider_data[0].get('records', 0) > 0:
                        available_providers += 1
                except Exception:
                    pass
            
            consumer_health['providers_available'] = available_providers
            
            if available_providers < len(providers):
                missing_providers = len(providers) - available_providers
                result.issues.append(f"{consumer} missing {missing_providers}/{len(providers)} federation providers")
                result.recommendations.append(f"Ensure all {consumer} dependencies are running: {', '.join(providers)}")
            
            # Check for federated data in consumer
            if consumer_health['consumer_active']:
                federated_query = f"""
                SELECT count(*) as 'federated_records'
                FROM Metric 
                WHERE service.name = '{consumer}'
                  AND federated_from IS NOT NULL
                SINCE 30 minutes ago
                """
                
                try:
                    federated_data = self.execute_nrql(federated_query)
                    if federated_data and federated_data[0].get('federated_records', 0) > 0:
                        consumer_health['federated_data_present'] = True
                        logger.info(f"✓ {consumer} receiving federated data")
                    else:
                        result.issues.append(f"{consumer} not receiving federated data")
                        result.recommendations.append(f"Check {consumer} Prometheus federation configuration")
                except Exception as e:
                    result.issues.append(f"Federated data check for {consumer} failed: {e}")
            
            # Calculate federation health
            health_score = (
                consumer_health['consumer_active'] +
                (consumer_health['providers_available'] / len(providers)) +
                consumer_health['federated_data_present']
            ) / 3
            
            if health_score >= 0.8:
                healthy_federations += 1
                
            federation_health[consumer] = consumer_health
        
        # Determine overall status
        total_federations = len(federation_dependencies)
        if modules_to_test:
            total_federations = len([c for c in federation_dependencies.keys() if c in modules_to_test])
        
        health_percentage = healthy_federations / total_federations if total_federations > 0 else 1.0
        
        result.details = {
            'federation_health': federation_health,
            'healthy_federations': healthy_federations,
            'total_federations': total_federations,
            'health_percentage': health_percentage
        }
        
        if health_percentage >= 0.8:
            result.status = PipelineStatus.HEALTHY
            result.message = f"Federation flow healthy ({healthy_federations}/{total_federations})"
        elif health_percentage >= 0.6:
            result.status = PipelineStatus.DEGRADED
            result.message = f"Federation flow partially healthy ({healthy_federations}/{total_federations})"
        else:
            result.status = PipelineStatus.FAILED
            result.message = f"Federation flow critical issues ({healthy_federations}/{total_federations})"
        
        result.duration_ms = (time.time() - start_time) * 1000
        return result

    def validate_data_consistency(self, modules_to_test: List[str] = None) -> StageResult:
        """Validate data consistency and latency across the pipeline"""
        start_time = time.time()
        logger.info("Validating data consistency and latency...")
        
        result = StageResult(
            stage=ValidationStage.CONSISTENCY,
            status=PipelineStatus.UNKNOWN,
            message="",
            details={},
            duration_ms=0,
            timestamp=datetime.now()
        )
        
        if modules_to_test is None:
            modules_to_test = list(self.modules.keys())
        
        consistency_metrics = {
            'data_gaps': 0,
            'high_latency_modules': 0,
            'inconsistent_timestamps': 0,
            'average_latency_minutes': 0
        }
        
        # Check for data gaps (missing data points in time series)
        for module in modules_to_test:
            gap_query = f"""
            SELECT count(*) as 'data_points'
            FROM Metric 
            WHERE service.name = '{module}'
            SINCE 1 hour ago
            FACET timestamp
            LIMIT 100
            """
            
            try:
                gap_data = self.execute_nrql(gap_query)
                if gap_data:
                    # Simple gap detection based on expected data frequency
                    expected_points_per_hour = 360  # Every 10 seconds
                    actual_points = len(gap_data)
                    
                    if actual_points < expected_points_per_hour * 0.8:  # 80% threshold
                        consistency_metrics['data_gaps'] += 1
                        result.issues.append(f"{module} has data gaps ({actual_points}/{expected_points_per_hour} expected points)")
            except Exception as e:
                result.issues.append(f"Gap analysis for {module} failed: {e}")
        
        # Check data freshness and latency
        latency_query = """
        SELECT service.name,
               (now() - latest(timestamp))/1000/60 as 'latency_minutes'
        FROM Metric 
        WHERE service.name IN ('core-metrics', 'sql-intelligence', 'wait-profiler', 'anomaly-detector',
                              'business-impact', 'replication-monitor', 'performance-advisor', 
                              'resource-monitor', 'alert-manager', 'canary-tester', 'cross-signal-correlator')
        FACET service.name
        SINCE 1 hour ago
        """
        
        try:
            latency_data = self.execute_nrql(latency_query)
            if latency_data:
                latencies = []
                for item in latency_data:
                    latency = item.get('latency_minutes', 0)
                    module_name = item.get('service.name', 'unknown')
                    latencies.append(latency)
                    
                    if latency > 10:  # 10 minutes threshold
                        consistency_metrics['high_latency_modules'] += 1
                        result.issues.append(f"{module_name} high latency: {latency:.1f} minutes")
                
                if latencies:
                    consistency_metrics['average_latency_minutes'] = sum(latencies) / len(latencies)
        except Exception as e:
            result.issues.append(f"Latency analysis failed: {e}")
        
        # Check timestamp consistency
        timestamp_query = """
        SELECT count(*) as 'inconsistent_records'
        FROM Metric 
        WHERE timestamp > (now() + 5*60*1000)  -- Future timestamps
           OR timestamp < (now() - 24*60*60*1000)  -- Very old timestamps
        SINCE 1 hour ago
        """
        
        try:
            timestamp_data = self.execute_nrql(timestamp_query)
            if timestamp_data:
                inconsistent_count = timestamp_data[0].get('inconsistent_records', 0)
                consistency_metrics['inconsistent_timestamps'] = inconsistent_count
                
                if inconsistent_count > 0:
                    result.issues.append(f"{inconsistent_count} records with inconsistent timestamps")
                    result.recommendations.append("Check system clock synchronization across collectors")
        except Exception as e:
            result.issues.append(f"Timestamp consistency check failed: {e}")
        
        # Determine overall status
        issues_found = (
            consistency_metrics['data_gaps'] +
            consistency_metrics['high_latency_modules'] +
            (1 if consistency_metrics['inconsistent_timestamps'] > 0 else 0)
        )
        
        avg_latency = consistency_metrics['average_latency_minutes']
        
        result.details = {
            'consistency_metrics': consistency_metrics,
            'issues_found': issues_found,
            'modules_tested': len(modules_to_test)
        }
        
        if issues_found == 0 and avg_latency < 5:
            result.status = PipelineStatus.HEALTHY
            result.message = f"Data consistency excellent (avg latency: {avg_latency:.1f}min)"
        elif issues_found <= 2 and avg_latency < 10:
            result.status = PipelineStatus.DEGRADED
            result.message = f"Data consistency acceptable ({issues_found} issues, avg latency: {avg_latency:.1f}min)"
        else:
            result.status = PipelineStatus.FAILED
            result.message = f"Data consistency problems ({issues_found} issues, avg latency: {avg_latency:.1f}min)"
        
        result.duration_ms = (time.time() - start_time) * 1000
        return result

    def run_full_pipeline_validation(self, 
                                   stages: List[ValidationStage] = None,
                                   modules: List[str] = None,
                                   parallel: bool = False) -> Dict:
        """Run complete end-to-end pipeline validation"""
        logger.info("Starting end-to-end pipeline validation...")
        
        if stages is None:
            stages = list(ValidationStage)
        
        validation_start = time.time()
        stage_results = []
        
        # Stage validation functions
        stage_validators = {
            ValidationStage.MYSQL: self.validate_mysql_source,
            ValidationStage.OTEL: self.validate_otel_collectors,
            ValidationStage.PROMETHEUS: self.validate_prometheus_endpoints,
            ValidationStage.NEWRELIC: self.validate_newrelic_ingestion,
            ValidationStage.FEDERATION: self.validate_federation_flow,
            ValidationStage.CONSISTENCY: self.validate_data_consistency
        }
        
        if parallel:
            # Run validations in parallel (except MySQL which should be first)
            mysql_result = None
            if ValidationStage.MYSQL in stages:
                mysql_result = self.validate_mysql_source(modules)
                stage_results.append(mysql_result)
                stages = [s for s in stages if s != ValidationStage.MYSQL]
            
            with ThreadPoolExecutor(max_workers=3) as executor:
                future_to_stage = {
                    executor.submit(stage_validators[stage], modules): stage
                    for stage in stages if stage in stage_validators
                }
                
                for future in as_completed(future_to_stage):
                    stage = future_to_stage[future]
                    try:
                        result = future.result()
                        stage_results.append(result)
                    except Exception as e:
                        error_result = StageResult(
                            stage=stage,
                            status=PipelineStatus.FAILED,
                            message=f"Validation failed: {e}",
                            details={'error': str(e)},
                            duration_ms=0,
                            timestamp=datetime.now()
                        )
                        stage_results.append(error_result)
        else:
            # Run validations sequentially
            for stage in stages:
                if stage in stage_validators:
                    try:
                        result = stage_validators[stage](modules)
                        stage_results.append(result)
                    except Exception as e:
                        error_result = StageResult(
                            stage=stage,
                            status=PipelineStatus.FAILED,
                            message=f"Validation failed: {e}",
                            details={'error': str(e)},
                            duration_ms=0,
                            timestamp=datetime.now()
                        )
                        stage_results.append(error_result)
        
        # Sort results by stage order
        stage_order = {stage: i for i, stage in enumerate(ValidationStage)}
        stage_results.sort(key=lambda r: stage_order.get(r.stage, 999))
        
        # Calculate overall pipeline health
        total_stages = len(stage_results)
        healthy_stages = len([r for r in stage_results if r.status == PipelineStatus.HEALTHY])
        degraded_stages = len([r for r in stage_results if r.status == PipelineStatus.DEGRADED])
        failed_stages = len([r for r in stage_results if r.status == PipelineStatus.FAILED])
        
        # Determine overall status
        if failed_stages > 0:
            overall_status = PipelineStatus.FAILED
        elif degraded_stages > 0:
            overall_status = PipelineStatus.DEGRADED
        else:
            overall_status = PipelineStatus.HEALTHY
        
        # Aggregate all issues and recommendations
        all_issues = []
        all_recommendations = []
        for result in stage_results:
            all_issues.extend(result.issues)
            all_recommendations.extend(result.recommendations)
        
        # Remove duplicates
        all_issues = list(set(all_issues))
        all_recommendations = list(set(all_recommendations))
        
        total_duration = time.time() - validation_start
        
        return {
            'pipeline_validation': {
                'timestamp': datetime.now().isoformat(),
                'overall_status': overall_status.value,
                'total_duration_seconds': total_duration,
                'stages_validated': len(stage_results),
                'pipeline_health_score': (healthy_stages + 0.5 * degraded_stages) / total_stages * 100
            },
            'stage_results': [
                {
                    'stage': result.stage.value,
                    'status': result.status.value,
                    'message': result.message,
                    'duration_ms': result.duration_ms,
                    'timestamp': result.timestamp.isoformat(),
                    'details': result.details,
                    'issues': result.issues,
                    'recommendations': result.recommendations
                }
                for result in stage_results
            ],
            'summary': {
                'healthy_stages': healthy_stages,
                'degraded_stages': degraded_stages,
                'failed_stages': failed_stages,
                'total_stages': total_stages
            },
            'critical_issues': all_issues,
            'recommendations': all_recommendations,
            'modules_tested': modules or list(self.modules.keys())
        }

def main():
    parser = argparse.ArgumentParser(description='End-to-End Pipeline Validator')
    parser.add_argument('--stages', nargs='+', 
                       choices=[s.value for s in ValidationStage],
                       help='Specific validation stages to run')
    parser.add_argument('--modules', nargs='+', help='Specific modules to validate')
    parser.add_argument('--parallel', action='store_true', help='Run validations in parallel')
    parser.add_argument('--output', help='Output file for results (JSON format)')
    parser.add_argument('--verbose', action='store_true', help='Verbose output')
    
    args = parser.parse_args()
    
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    
    try:
        validator = EndToEndPipelineValidator()
    except ValueError as e:
        print(f"Configuration error: {e}")
        sys.exit(1)
    
    # Parse stages
    stages = None
    if args.stages:
        stages = [ValidationStage(s) for s in args.stages]
    
    # Run validation
    results = validator.run_full_pipeline_validation(
        stages=stages,
        modules=args.modules,
        parallel=args.parallel
    )
    
    # Output results
    if args.output:
        with open(args.output, 'w') as f:
            json.dump(results, f, indent=2, default=str)
        print(f"Results saved to {args.output}")
    else:
        print(json.dumps(results, indent=2, default=str))
    
    # Print summary
    pipeline_health = results['pipeline_validation']
    print(f"\n{'='*60}")
    print(f"END-TO-END PIPELINE VALIDATION")
    print(f"{'='*60}")
    print(f"Overall Status: {pipeline_health['overall_status']}")
    print(f"Pipeline Health: {pipeline_health['pipeline_health_score']:.1f}%")
    print(f"Total Duration: {pipeline_health['total_duration_seconds']:.1f}s")
    
    summary = results['summary']
    print(f"\nStage Summary:")
    print(f"  ✅ Healthy: {summary['healthy_stages']}")
    print(f"  ⚠️  Degraded: {summary['degraded_stages']}")
    print(f"  ❌ Failed: {summary['failed_stages']}")
    
    if results['critical_issues']:
        print(f"\nCritical Issues ({len(results['critical_issues'])}):")
        for issue in results['critical_issues'][:5]:
            print(f"  - {issue}")
        if len(results['critical_issues']) > 5:
            print(f"  ... and {len(results['critical_issues']) - 5} more")
    
    # Exit with appropriate code
    if pipeline_health['overall_status'] == 'FAILED':
        sys.exit(1)
    elif pipeline_health['overall_status'] == 'DEGRADED':
        sys.exit(2)
    else:
        sys.exit(0)

if __name__ == '__main__':
    main()