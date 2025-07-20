#!/usr/bin/env python3
"""
Enterprise Database Intelligence Validation Suite

This comprehensive validation framework tests all enterprise patterns including:
- Circuit breaker functionality
- Persistent queue operations
- Multi-pipeline architecture
- Entity synthesis standardization
- Dashboard integration
- End-to-end data flow
"""

import os
import sys
import json
import urllib.request
import urllib.parse
import urllib.error
import time
import logging
from datetime import datetime, timedelta
from dataclasses import dataclass
from typing import Dict, List, Optional, Tuple
from enum import Enum

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class ValidationStatus(Enum):
    PASS = "✅ PASS"
    FAIL = "❌ FAIL" 
    WARN = "⚠️ WARN"
    INFO = "ℹ️ INFO"

@dataclass
class ModuleConfig:
    name: str
    port: int
    entity_type: str
    expected_metrics: List[str]
    critical_metrics: List[str]
    health_endpoint: str = "13133"

@dataclass
class ValidationResult:
    module: str
    test_name: str
    status: ValidationStatus
    message: str
    details: Optional[Dict] = None

class EnterpriseValidationSuite:
    def __init__(self):
        self.load_environment()
        self.modules = self.define_modules()
        self.results: List[ValidationResult] = []

    def load_environment(self):
        """Load environment variables from .env file"""
        env_file = '.env'
        if os.path.exists(env_file):
            with open(env_file, 'r') as f:
                for line in f:
                    line = line.strip()
                    if line and not line.startswith('#') and '=' in line:
                        key, value = line.split('=', 1)
                        os.environ[key.strip()] = value.strip()

        self.api_key = os.getenv('NEW_RELIC_API_KEY')
        self.account_id = os.getenv('NEW_RELIC_ACCOUNT_ID')
        
        if not self.api_key or not self.account_id:
            raise Exception("Missing NEW_RELIC_API_KEY or NEW_RELIC_ACCOUNT_ID")

    def define_modules(self) -> List[ModuleConfig]:
        """Define all modules with their expected configurations"""
        return [
            ModuleConfig("core-metrics", 8081, "MYSQL_INSTANCE", 
                        ["mysql.connections.current", "mysql.threads.running", "mysql.buffer_pool.usage"],
                        ["mysql.connections.current", "mysql.threads.running"]),
            ModuleConfig("sql-intelligence", 8082, "MYSQL_INSTANCE", 
                        ["mysql.query.exec_total", "mysql.query.latency_ms", "mysql.query.rows_examined_total"],
                        ["mysql.query.latency_ms", "mysql.slow_queries.detected"]),
            ModuleConfig("wait-profiler", 8083, "MYSQL_INSTANCE", 
                        ["mysql.wait.count", "mysql.wait.time_ms", "mysql.wait.mutex.count"],
                        ["mysql.wait.time_ms", "mysql.wait.lock.detected"]),
            ModuleConfig("anomaly-detector", 8084, "APPLICATION", 
                        ["anomaly_score_cpu", "anomaly_score_memory", "anomaly_detected"],
                        ["anomaly_detected", "anomaly_score_critical"]),
            ModuleConfig("business-impact", 8085, "APPLICATION", 
                        ["business_impact_score", "revenue_impact_hourly", "sla_impact"],
                        ["revenue_impact_hourly", "sla_impact"]),
            ModuleConfig("replication-monitor", 8086, "MYSQL_INSTANCE", 
                        ["mysql_replica_lag", "mysql_replication_running", "mysql_gtid_executed"],
                        ["mysql_replica_lag", "mysql_replication_running"]),
            ModuleConfig("performance-advisor", 8087, "APPLICATION", 
                        ["db.performance.recommendation.missing_index", "db.performance.recommendation.slow_query"],
                        ["db.performance.recommendation.critical"]),
            ModuleConfig("resource-monitor", 8088, "HOST", 
                        ["system.cpu.utilization", "system.memory.usage", "system.disk.io.time"],
                        ["system.cpu.utilization", "system.memory.usage"])
        ]

    def validate_circuit_breaker_patterns(self, module: ModuleConfig) -> List[ValidationResult]:
        """Validate circuit breaker implementation for a module"""
        results = []
        
        # Test 1: Dual Exporter Configuration
        try:
            # Check for both standard and critical exporters in New Relic
            standard_query = f"""
            SELECT count(*) FROM Metric 
            WHERE module = '{module.name}' 
            AND instrumentation.name = 'mysql-otel-collector'
            SINCE 10 minutes ago
            """
            
            critical_query = f"""
            SELECT count(*) FROM Metric 
            WHERE module = '{module.name}' 
            AND instrumentation.name = 'mysql-otel-collector'
            AND nr.priority = 'high'
            SINCE 10 minutes ago
            """
            
            standard_count = self.execute_nrql(standard_query)
            critical_count = self.execute_nrql(critical_query)
            
            if standard_count > 0:
                results.append(ValidationResult(
                    module.name, "Circuit Breaker - Standard Pipeline",
                    ValidationStatus.PASS, f"Standard pipeline active: {standard_count} metrics"
                ))
            else:
                results.append(ValidationResult(
                    module.name, "Circuit Breaker - Standard Pipeline",
                    ValidationStatus.FAIL, "Standard pipeline not receiving data"
                ))
            
            if critical_count > 0:
                results.append(ValidationResult(
                    module.name, "Circuit Breaker - Critical Pipeline",
                    ValidationStatus.PASS, f"Critical pipeline active: {critical_count} metrics"
                ))
            else:
                results.append(ValidationResult(
                    module.name, "Circuit Breaker - Critical Pipeline",
                    ValidationStatus.WARN, "Critical pipeline not active (may be normal if no critical alerts)"
                ))
                
        except Exception as e:
            results.append(ValidationResult(
                module.name, "Circuit Breaker Validation",
                ValidationStatus.FAIL, f"Circuit breaker validation failed: {e}"
            ))
        
        return results

    def validate_persistent_queues(self, module: ModuleConfig) -> List[ValidationResult]:
        """Validate persistent queue functionality"""
        results = []
        
        try:
            # Test queue persistence by checking data continuity
            continuity_query = f"""
            SELECT count(*) FROM Metric 
            WHERE module = '{module.name}'
            SINCE 30 minutes ago
            TIMESERIES 5 minutes
            """
            
            data_points = self.execute_nrql_timeseries(continuity_query)
            
            # Check for data gaps (indicating queue issues)
            gaps = 0
            for i, point in enumerate(data_points):
                if i > 0 and point['count'] == 0:
                    gaps += 1
            
            if gaps == 0:
                results.append(ValidationResult(
                    module.name, "Persistent Queues - Data Continuity",
                    ValidationStatus.PASS, "No data gaps detected - queues functioning properly"
                ))
            elif gaps <= 2:
                results.append(ValidationResult(
                    module.name, "Persistent Queues - Data Continuity",
                    ValidationStatus.WARN, f"Minor data gaps detected: {gaps} intervals"
                ))
            else:
                results.append(ValidationResult(
                    module.name, "Persistent Queues - Data Continuity",
                    ValidationStatus.FAIL, f"Significant data gaps detected: {gaps} intervals"
                ))
                
        except Exception as e:
            results.append(ValidationResult(
                module.name, "Persistent Queue Validation",
                ValidationStatus.FAIL, f"Queue validation failed: {e}"
            ))
        
        return results

    def validate_entity_synthesis(self, module: ModuleConfig) -> List[ValidationResult]:
        """Validate New Relic entity synthesis standardization"""
        results = []
        
        try:
            # Check entity attributes
            entity_query = f"""
            SELECT latest(entity.type) as entity_type, 
                   latest(entity.guid) as entity_guid,
                   latest(entity.name) as entity_name,
                   latest(newrelic.entity.synthesis) as synthesis_enabled
            FROM Metric 
            WHERE module = '{module.name}'
            SINCE 10 minutes ago
            """
            
            entity_data = self.execute_nrql(entity_query)
            
            if entity_data:
                # Validate entity type
                if entity_data.get('entity_type') == module.entity_type:
                    results.append(ValidationResult(
                        module.name, "Entity Synthesis - Type",
                        ValidationStatus.PASS, f"Correct entity type: {module.entity_type}"
                    ))
                else:
                    results.append(ValidationResult(
                        module.name, "Entity Synthesis - Type",
                        ValidationStatus.FAIL, f"Incorrect entity type. Expected: {module.entity_type}, Got: {entity_data.get('entity_type')}"
                    ))
                
                # Validate GUID format
                entity_guid = entity_data.get('entity_guid', '')
                if entity_guid.startswith(f"{module.entity_type}|"):
                    results.append(ValidationResult(
                        module.name, "Entity Synthesis - GUID Format",
                        ValidationStatus.PASS, f"Correct GUID format: {entity_guid[:50]}..."
                    ))
                else:
                    results.append(ValidationResult(
                        module.name, "Entity Synthesis - GUID Format",
                        ValidationStatus.FAIL, f"Incorrect GUID format: {entity_guid}"
                    ))
                
                # Validate synthesis enabled
                if entity_data.get('synthesis_enabled') == 'true':
                    results.append(ValidationResult(
                        module.name, "Entity Synthesis - Enabled",
                        ValidationStatus.PASS, "Entity synthesis properly enabled"
                    ))
                else:
                    results.append(ValidationResult(
                        module.name, "Entity Synthesis - Enabled",
                        ValidationStatus.FAIL, "Entity synthesis not enabled"
                    ))
            else:
                results.append(ValidationResult(
                    module.name, "Entity Synthesis",
                    ValidationStatus.FAIL, "No entity data found"
                ))
                
        except Exception as e:
            results.append(ValidationResult(
                module.name, "Entity Synthesis Validation",
                ValidationStatus.FAIL, f"Entity validation failed: {e}"
            ))
        
        return results

    def validate_multi_pipeline_architecture(self, module: ModuleConfig) -> List[ValidationResult]:
        """Validate multi-pipeline architecture implementation"""
        results = []
        
        try:
            # Check for different pipeline indicators in the data
            pipelines = ['standard', 'critical', 'debug', 'federation']
            active_pipelines = []
            
            for pipeline in pipelines:
                pipeline_query = f"""
                SELECT count(*) FROM Metric 
                WHERE module = '{module.name}'
                AND (pipeline = '{pipeline}' OR instrumentation.pipeline = '{pipeline}')
                SINCE 10 minutes ago
                """
                
                count = self.execute_nrql(pipeline_query)
                if count > 0:
                    active_pipelines.append(pipeline)
            
            if len(active_pipelines) >= 2:
                results.append(ValidationResult(
                    module.name, "Multi-Pipeline Architecture",
                    ValidationStatus.PASS, f"Multiple pipelines active: {', '.join(active_pipelines)}"
                ))
            elif len(active_pipelines) == 1:
                results.append(ValidationResult(
                    module.name, "Multi-Pipeline Architecture",
                    ValidationStatus.WARN, f"Single pipeline active: {active_pipelines[0]}"
                ))
            else:
                # Check for general data flow as fallback
                general_query = f"""
                SELECT count(*) FROM Metric 
                WHERE module = '{module.name}'
                SINCE 10 minutes ago
                """
                
                general_count = self.execute_nrql(general_query)
                if general_count > 0:
                    results.append(ValidationResult(
                        module.name, "Multi-Pipeline Architecture",
                        ValidationStatus.WARN, "Data flowing but pipeline attribution missing"
                    ))
                else:
                    results.append(ValidationResult(
                        module.name, "Multi-Pipeline Architecture",
                        ValidationStatus.FAIL, "No pipeline activity detected"
                    ))
                    
        except Exception as e:
            results.append(ValidationResult(
                module.name, "Multi-Pipeline Validation",
                ValidationStatus.FAIL, f"Pipeline validation failed: {e}"
            ))
        
        return results

    def validate_health_monitoring(self, module: ModuleConfig) -> List[ValidationResult]:
        """Validate health monitoring and extensions"""
        results = []
        
        try:
            # Test health endpoint
            health_url = f"http://localhost:{module.health_endpoint}/"
            try:
                req = urllib.request.Request(health_url)
                with urllib.request.urlopen(req, timeout=10) as response:
                    health_data = json.loads(response.read().decode('utf-8'))
                    
                    if health_data.get('status') == 'Server available':
                        results.append(ValidationResult(
                            module.name, "Health Monitoring - Endpoint",
                            ValidationStatus.PASS, f"Health endpoint responding on port {module.health_endpoint}"
                        ))
                    else:
                        results.append(ValidationResult(
                            module.name, "Health Monitoring - Endpoint",
                            ValidationStatus.WARN, f"Health endpoint responding but status unclear: {health_data}"
                        ))
                        
            except Exception as e:
                results.append(ValidationResult(
                    module.name, "Health Monitoring - Endpoint",
                    ValidationStatus.FAIL, f"Health endpoint not accessible: {e}"
                ))
            
            # Test metrics endpoint
            metrics_url = f"http://localhost:{module.port}/metrics"
            try:
                req = urllib.request.Request(metrics_url)
                with urllib.request.urlopen(req, timeout=10) as response:
                    metrics_data = response.read().decode('utf-8')
                    
                    if len(metrics_data) > 100:  # Basic check for meaningful data
                        results.append(ValidationResult(
                            module.name, "Health Monitoring - Metrics",
                            ValidationStatus.PASS, f"Metrics endpoint active on port {module.port}"
                        ))
                    else:
                        results.append(ValidationResult(
                            module.name, "Health Monitoring - Metrics", 
                            ValidationStatus.WARN, "Metrics endpoint responding but limited data"
                        ))
                        
            except Exception as e:
                results.append(ValidationResult(
                    module.name, "Health Monitoring - Metrics",
                    ValidationStatus.FAIL, f"Metrics endpoint not accessible: {e}"
                ))
                
        except Exception as e:
            results.append(ValidationResult(
                module.name, "Health Monitoring",
                ValidationStatus.FAIL, f"Health monitoring validation failed: {e}"
            ))
        
        return results

    def validate_dashboard_integration(self) -> List[ValidationResult]:
        """Validate dashboard data integration"""
        results = []
        
        try:
            # Test executive dashboard data sources
            dashboard_queries = {
                "Business Impact": "SELECT count(*) FROM Metric WHERE module = 'business-impact' SINCE 1 hour ago",
                "Performance Metrics": "SELECT count(*) FROM Metric WHERE module IN ('core-metrics', 'sql-intelligence') SINCE 1 hour ago",
                "Anomaly Detection": "SELECT count(*) FROM Metric WHERE module = 'anomaly-detector' SINCE 1 hour ago",
                "Resource Monitoring": "SELECT count(*) FROM Metric WHERE module = 'resource-monitor' SINCE 1 hour ago"
            }
            
            for dashboard_component, query in dashboard_queries.items():
                count = self.execute_nrql(query)
                
                if count > 0:
                    results.append(ValidationResult(
                        "dashboard", f"Dashboard Integration - {dashboard_component}",
                        ValidationStatus.PASS, f"Data available for {dashboard_component}: {count} metrics"
                    ))
                else:
                    results.append(ValidationResult(
                        "dashboard", f"Dashboard Integration - {dashboard_component}",
                        ValidationStatus.WARN, f"No data available for {dashboard_component}"
                    ))
                    
        except Exception as e:
            results.append(ValidationResult(
                "dashboard", "Dashboard Integration",
                ValidationStatus.FAIL, f"Dashboard validation failed: {e}"
            ))
        
        return results

    def execute_nrql(self, query: str) -> int:
        """Execute NRQL query and return count result"""
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
        
        url = "https://api.newrelic.com/graphql"
        data = json.dumps(graphql_query).encode('utf-8')
        
        req = urllib.request.Request(url, data=data)
        req.add_header('Api-Key', self.api_key)
        req.add_header('Content-Type', 'application/json')
        
        with urllib.request.urlopen(req, timeout=30) as response:
            response_data = json.loads(response.read().decode('utf-8'))
            
            if 'errors' in response_data:
                raise Exception(f"NRQL Error: {response_data['errors']}")
            
            results = response_data.get('data', {}).get('actor', {}).get('account', {}).get('nrql', {}).get('results', [])
            
            if results:
                return results[0].get('count', 0)
            return 0

    def execute_nrql_timeseries(self, query: str) -> List[Dict]:
        """Execute NRQL timeseries query and return results"""
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
        
        url = "https://api.newrelic.com/graphql"
        data = json.dumps(graphql_query).encode('utf-8')
        
        req = urllib.request.Request(url, data=data)
        req.add_header('Api-Key', self.api_key)
        req.add_header('Content-Type', 'application/json')
        
        with urllib.request.urlopen(req, timeout=30) as response:
            response_data = json.loads(response.read().decode('utf-8'))
            
            if 'errors' in response_data:
                raise Exception(f"NRQL Error: {response_data['errors']}")
            
            return response_data.get('data', {}).get('actor', {}).get('account', {}).get('nrql', {}).get('results', [])

    def run_comprehensive_validation(self):
        """Run the complete enterprise validation suite"""
        print("🚀 Starting Enterprise Database Intelligence Validation Suite")
        print("=" * 80)
        
        start_time = datetime.now()
        
        # Validate each module
        for module in self.modules:
            print(f"\n📊 Validating {module.name} (Port {module.port})...")
            
            # Run all validation tests for this module
            self.results.extend(self.validate_circuit_breaker_patterns(module))
            self.results.extend(self.validate_persistent_queues(module))
            self.results.extend(self.validate_entity_synthesis(module))
            self.results.extend(self.validate_multi_pipeline_architecture(module))
            self.results.extend(self.validate_health_monitoring(module))
        
        # Validate cross-module functionality
        print(f"\n🎯 Validating Cross-Module Integration...")
        self.results.extend(self.validate_dashboard_integration())
        
        # Generate summary report
        self.generate_summary_report(start_time)

    def generate_summary_report(self, start_time: datetime):
        """Generate comprehensive validation summary"""
        end_time = datetime.now()
        duration = end_time - start_time
        
        # Count results by status
        status_counts = {status: 0 for status in ValidationStatus}
        for result in self.results:
            status_counts[result.status] += 1
        
        print("\n" + "=" * 80)
        print("📋 ENTERPRISE VALIDATION SUMMARY REPORT")
        print("=" * 80)
        
        print(f"🕒 Validation Duration: {duration.total_seconds():.2f} seconds")
        print(f"📊 Total Tests Executed: {len(self.results)}")
        print(f"🎯 Modules Tested: {len(self.modules)}")
        
        print(f"\n📈 Results Breakdown:")
        for status, count in status_counts.items():
            percentage = (count / len(self.results)) * 100 if self.results else 0
            print(f"  {status.value}: {count} tests ({percentage:.1f}%)")
        
        # Calculate overall health score
        passing_tests = status_counts[ValidationStatus.PASS]
        total_tests = len(self.results)
        health_score = (passing_tests / total_tests) * 100 if total_tests > 0 else 0
        
        print(f"\n🏆 Overall Health Score: {health_score:.1f}%")
        
        if health_score >= 95:
            print("✅ EXCELLENT: System is production-ready with enterprise-grade patterns")
        elif health_score >= 85:
            print("✅ GOOD: System is largely production-ready with minor improvements needed")
        elif health_score >= 70:
            print("⚠️ ACCEPTABLE: System needs some improvements before production deployment")
        else:
            print("❌ NEEDS WORK: Significant improvements required before production deployment")
        
        # Detailed results by module
        print(f"\n📋 Detailed Results by Module:")
        module_results = {}
        for result in self.results:
            if result.module not in module_results:
                module_results[result.module] = []
            module_results[result.module].append(result)
        
        for module_name, results in module_results.items():
            module_pass_count = sum(1 for r in results if r.status == ValidationStatus.PASS)
            module_total = len(results)
            module_score = (module_pass_count / module_total) * 100 if module_total > 0 else 0
            
            print(f"\n  📦 {module_name.upper()} - Score: {module_score:.1f}% ({module_pass_count}/{module_total})")
            
            for result in results:
                print(f"    {result.status.value} {result.test_name}: {result.message}")
        
        # Recommendations
        print(f"\n💡 Recommendations:")
        
        if status_counts[ValidationStatus.FAIL] > 0:
            print("  🚨 HIGH PRIORITY: Address failed validations immediately")
            print("    - Review module configurations and restart affected services")
            print("    - Check network connectivity and New Relic credentials")
            print("    - Verify persistent queue storage and permissions")
        
        if status_counts[ValidationStatus.WARN] > 0:
            print("  ⚠️ MEDIUM PRIORITY: Investigate warnings for optimization opportunities")
            print("    - Consider enabling missing pipeline features")
            print("    - Review health check configurations")
            print("    - Optimize data flow patterns")
        
        if health_score >= 90:
            print("  🎯 ENHANCEMENT: Consider advanced features")
            print("    - Implement ML-based anomaly detection")
            print("    - Add business impact analysis")
            print("    - Deploy executive dashboards")
        
        print(f"\n🔗 Next Steps:")
        print("  1. Address any failing validations")
        print("  2. Review and deploy missing enterprise patterns")
        print("  3. Configure executive dashboards")
        print("  4. Set up automated monitoring and alerting")
        
        print("\n" + "=" * 80)

def main():
    try:
        validator = EnterpriseValidationSuite()
        validator.run_comprehensive_validation()
        
        # Exit with appropriate code
        fail_count = sum(1 for r in validator.results if r.status == ValidationStatus.FAIL)
        if fail_count > 0:
            sys.exit(1)
        else:
            sys.exit(0)
            
    except Exception as e:
        print(f"❌ Validation suite failed: {e}")
        sys.exit(1)

if __name__ == '__main__':
    main()