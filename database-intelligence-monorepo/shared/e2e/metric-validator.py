#!/usr/bin/env python3
"""
Metric Validator for E2E Testing
Validates that expected metrics are being collected and exported
"""

import re
import time
from typing import Any, Dict, List, Optional, Set

from prometheus_client.parser import text_string_to_metric_families

from validation_framework import BaseValidator, ValidationResult


class MetricValidator(BaseValidator):
    """Validates metrics collection and export"""
    
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.expected_metrics = self._load_expected_metrics()
    
    def _load_expected_metrics(self) -> Dict[str, Dict[str, Any]]:
        """Load expected metrics configuration for the module"""
        default_metrics = {
            # Database metrics
            "mysql_up": {
                "type": "gauge",
                "description": "Database availability",
                "required": True,
                "min_value": 0,
                "max_value": 1
            },
            "mysql_global_status_connections": {
                "type": "counter",
                "description": "Total number of connection attempts",
                "required": True
            },
            "mysql_global_status_threads_connected": {
                "type": "gauge",
                "description": "Currently open connections",
                "required": True,
                "min_value": 0
            },
            "mysql_global_status_queries": {
                "type": "counter",
                "description": "Total number of queries",
                "required": True
            },
            "mysql_global_status_slow_queries": {
                "type": "counter",
                "description": "Number of slow queries",
                "required": False
            },
            
            # Performance schema metrics
            "mysql_perf_schema_table_io_waits_total": {
                "type": "counter",
                "description": "Table I/O wait events",
                "required": False
            },
            "mysql_perf_schema_index_io_waits_total": {
                "type": "counter",
                "description": "Index I/O wait events",
                "required": False
            },
            
            # Custom metrics based on module
            "anomaly_detection_enabled": {
                "type": "gauge",
                "description": "Anomaly detection status",
                "required": self.module_config.name == "anomaly-detector"
            },
            "anomalies_detected_total": {
                "type": "counter",
                "description": "Total anomalies detected",
                "required": self.module_config.name == "anomaly-detector"
            }
        }
        
        # Load module-specific metrics if config exists
        if self.module_config.config_file:
            config = self._load_yaml_config(self.module_config.config_file)
            if config and "expected_metrics" in config:
                default_metrics.update(config["expected_metrics"])
        
        return default_metrics
    
    def validate(self) -> List[ValidationResult]:
        """Run metric validation tests"""
        self.results = []
        start_time = time.time()
        
        # Test 1: Prometheus endpoint availability
        prom_available = self._test_prometheus_availability()
        
        if not prom_available:
            return self.results
        
        # Test 2: Fetch and validate metrics
        self._test_metrics_collection()
        
        # Test 3: Validate metric values
        self._test_metric_values()
        
        # Test 4: Check metric freshness
        self._test_metric_freshness()
        
        # Test 5: Validate custom module metrics
        self._test_custom_metrics()
        
        return self.results
    
    def _test_prometheus_availability(self) -> bool:
        """Test if Prometheus endpoint is available"""
        start = time.time()
        
        if not self.module_config.prometheus_url:
            self._record_result(
                "prometheus_availability",
                "skipped",
                "No Prometheus URL configured",
                time.time() - start
            )
            return False
        
        response = self._make_request(f"{self.module_config.prometheus_url}/api/v1/query?query=up")
        
        if response and response.status_code == 200:
            self._record_result(
                "prometheus_availability",
                "passed",
                f"Prometheus endpoint is available at {self.module_config.prometheus_url}",
                time.time() - start
            )
            return True
        else:
            self._record_result(
                "prometheus_availability",
                "failed",
                f"Prometheus endpoint not available at {self.module_config.prometheus_url}",
                time.time() - start
            )
            return False
    
    def _test_metrics_collection(self) -> None:
        """Test that expected metrics are being collected"""
        start = time.time()
        
        # Fetch all metrics
        metrics_response = self._make_request(f"{self.module_config.prometheus_url}/metrics")
        
        if not metrics_response:
            self._record_result(
                "metrics_collection",
                "failed",
                "Failed to fetch metrics from Prometheus",
                time.time() - start
            )
            return
        
        # Parse metrics
        collected_metrics = set()
        try:
            for family in text_string_to_metric_families(metrics_response.text):
                collected_metrics.add(family.name)
        except Exception as e:
            self._record_result(
                "metrics_collection",
                "failed",
                f"Failed to parse metrics: {e}",
                time.time() - start
            )
            return
        
        # Check required metrics
        missing_required = []
        found_metrics = []
        
        for metric_name, config in self.expected_metrics.items():
            if config.get("required", False):
                if metric_name in collected_metrics:
                    found_metrics.append(metric_name)
                else:
                    # Check with wildcards
                    pattern = metric_name.replace("*", ".*")
                    if any(re.match(pattern, m) for m in collected_metrics):
                        found_metrics.append(metric_name)
                    else:
                        missing_required.append(metric_name)
        
        if missing_required:
            self._record_result(
                "metrics_collection",
                "failed",
                f"Missing required metrics: {', '.join(missing_required)}",
                time.time() - start,
                {"missing": missing_required, "found": found_metrics}
            )
        else:
            self._record_result(
                "metrics_collection",
                "passed",
                f"All {len(found_metrics)} required metrics are being collected",
                time.time() - start,
                {"found": found_metrics}
            )
    
    def _test_metric_values(self) -> None:
        """Validate that metric values are within expected ranges"""
        start = time.time()
        invalid_metrics = []
        
        for metric_name, config in self.expected_metrics.items():
            if not config.get("required", False):
                continue
            
            # Query metric value
            query_response = self._make_request(
                f"{self.module_config.prometheus_url}/api/v1/query?query={metric_name}"
            )
            
            if not query_response:
                continue
            
            try:
                data = query_response.json()
                if data["status"] == "success" and data["data"]["result"]:
                    value = float(data["data"]["result"][0]["value"][1])
                    
                    # Check min/max bounds if specified
                    min_val = config.get("min_value")
                    max_val = config.get("max_value")
                    
                    if min_val is not None and value < min_val:
                        invalid_metrics.append(f"{metric_name} ({value} < {min_val})")
                    elif max_val is not None and value > max_val:
                        invalid_metrics.append(f"{metric_name} ({value} > {max_val})")
            except Exception as e:
                self.logger.debug(f"Failed to validate {metric_name}: {e}")
        
        if invalid_metrics:
            self._record_result(
                "metric_values",
                "failed",
                f"Metrics with invalid values: {', '.join(invalid_metrics)}",
                time.time() - start,
                {"invalid": invalid_metrics}
            )
        else:
            self._record_result(
                "metric_values",
                "passed",
                "All metric values are within expected ranges",
                time.time() - start
            )
    
    def _test_metric_freshness(self) -> None:
        """Check that metrics are being updated regularly"""
        start = time.time()
        stale_metrics = []
        
        # Check mysql_up metric for freshness (should be updated every scrape)
        query = 'mysql_up'
        response = self._make_request(
            f"{self.module_config.prometheus_url}/api/v1/query?query={query}"
        )
        
        if response:
            try:
                data = response.json()
                if data["status"] == "success" and data["data"]["result"]:
                    # Check timestamp
                    metric_timestamp = float(data["data"]["result"][0]["value"][0])
                    current_time = time.time()
                    age_seconds = current_time - metric_timestamp
                    
                    # Metrics older than 5 minutes are considered stale
                    if age_seconds > 300:
                        stale_metrics.append(f"{query} (age: {age_seconds:.0f}s)")
            except Exception as e:
                self.logger.debug(f"Failed to check freshness for {query}: {e}")
        
        # Check a few more critical metrics
        critical_metrics = ["mysql_global_status_connections", "mysql_global_status_threads_connected"]
        for metric in critical_metrics:
            response = self._make_request(
                f"{self.module_config.prometheus_url}/api/v1/query?query=rate({metric}[5m])"
            )
            if response:
                try:
                    data = response.json()
                    if data["status"] == "success" and not data["data"]["result"]:
                        stale_metrics.append(f"{metric} (no recent data)")
                except Exception:
                    pass
        
        if stale_metrics:
            self._record_result(
                "metric_freshness",
                "failed",
                f"Stale metrics detected: {', '.join(stale_metrics)}",
                time.time() - start,
                {"stale": stale_metrics}
            )
        else:
            self._record_result(
                "metric_freshness",
                "passed",
                "All metrics are being updated regularly",
                time.time() - start
            )
    
    def _test_custom_metrics(self) -> None:
        """Test module-specific custom metrics"""
        start = time.time()
        
        if self.module_config.name == "anomaly-detector":
            # Check anomaly detection metrics
            anomaly_metrics = [
                "anomaly_detection_enabled",
                "anomalies_detected_total",
                "anomaly_detection_last_run_timestamp"
            ]
            
            found = []
            missing = []
            
            for metric in anomaly_metrics:
                response = self._make_request(
                    f"{self.module_config.prometheus_url}/api/v1/query?query={metric}"
                )
                if response:
                    try:
                        data = response.json()
                        if data["status"] == "success" and data["data"]["result"]:
                            found.append(metric)
                        else:
                            missing.append(metric)
                    except Exception:
                        missing.append(metric)
                else:
                    missing.append(metric)
            
            if missing:
                self._record_result(
                    "custom_metrics",
                    "failed",
                    f"Missing custom metrics for {self.module_config.name}: {', '.join(missing)}",
                    time.time() - start,
                    {"missing": missing, "found": found}
                )
            else:
                self._record_result(
                    "custom_metrics",
                    "passed",
                    f"All custom metrics for {self.module_config.name} are present",
                    time.time() - start,
                    {"found": found}
                )
        else:
            self._record_result(
                "custom_metrics",
                "skipped",
                f"No custom metrics defined for module {self.module_config.name}",
                time.time() - start
            )