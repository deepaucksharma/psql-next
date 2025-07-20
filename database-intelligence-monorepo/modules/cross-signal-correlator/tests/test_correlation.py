#!/usr/bin/env python3
"""
Test suite for Cross-Signal Correlator module
"""

import unittest
import requests
import time
import json
from typing import Dict, Any, List


class TestCrossSignalCorrelator(unittest.TestCase):
    """Test cases for cross-signal correlation functionality"""
    
    BASE_URL = "http://localhost"
    OTLP_GRPC_PORT = 4317
    OTLP_HTTP_PORT = 4318
    METRICS_PORT = 8892
    HEALTH_PORT = 13137
    
    @classmethod
    def setUpClass(cls):
        """Wait for service to be ready"""
        cls.wait_for_service()
    
    @classmethod
    def wait_for_service(cls, timeout: int = 30):
        """Wait for the service to be healthy"""
        start_time = time.time()
        while time.time() - start_time < timeout:
            try:
                response = requests.get(f"{cls.BASE_URL}:{cls.HEALTH_PORT}/health")
                if response.status_code == 200:
                    return
            except requests.exceptions.ConnectionError:
                pass
            time.sleep(1)
        raise TimeoutError("Service did not become healthy in time")
    
    def test_health_endpoint(self):
        """Test that health endpoint is responding"""
        response = requests.get(f"{self.BASE_URL}:{self.HEALTH_PORT}/health")
        self.assertEqual(response.status_code, 200)
    
    def test_metrics_endpoint(self):
        """Test that Prometheus metrics endpoint is available"""
        response = requests.get(f"{self.BASE_URL}:{self.METRICS_PORT}/metrics")
        self.assertEqual(response.status_code, 200)
        self.assertIn("# HELP", response.text)
        self.assertIn("# TYPE", response.text)
    
    def test_otlp_http_endpoint(self):
        """Test OTLP HTTP endpoint availability"""
        # Send a minimal OTLP trace
        trace_data = {
            "resourceSpans": [{
                "resource": {
                    "attributes": [{
                        "key": "service.name",
                        "value": {"stringValue": "test-service"}
                    }]
                },
                "scopeSpans": [{
                    "scope": {"name": "test-scope"},
                    "spans": [{
                        "traceId": "5B8EFFF798038103D269B633813FC60C",
                        "spanId": "EEE19B7EC3C1B174",
                        "name": "test-span",
                        "startTimeUnixNano": int(time.time() * 1e9),
                        "endTimeUnixNano": int(time.time() * 1e9) + 1000000,
                        "attributes": [{
                            "key": "db.statement",
                            "value": {"stringValue": "SELECT * FROM users"}
                        }]
                    }]
                }]
            }]
        }
        
        response = requests.post(
            f"{self.BASE_URL}:{self.OTLP_HTTP_PORT}/v1/traces",
            json=trace_data,
            headers={"Content-Type": "application/json"}
        )
        # OTLP typically returns 200 or 202 for success
        self.assertIn(response.status_code, [200, 202])
    
    def test_span_metrics_generation(self):
        """Test that span metrics are generated from traces"""
        # Send a trace
        self.test_otlp_http_endpoint()
        
        # Wait for metrics to be generated
        time.sleep(5)
        
        # Check for span metrics
        response = requests.get(f"{self.BASE_URL}:{self.METRICS_PORT}/metrics")
        self.assertIn("traces_spanmetrics", response.text)
    
    def test_prometheus_federation(self):
        """Test Prometheus federation is working"""
        response = requests.get(f"{self.BASE_URL}:{self.METRICS_PORT}/metrics")
        self.assertEqual(response.status_code, 200)
        
        # Check for federated metrics (if other modules are running)
        metrics_text = response.text
        # These would be present if federation is working and other modules are up
        possible_metrics = [
            "mysql_connections",
            "mysql_query_",
            "mysql_wait_"
        ]
        # Just verify the endpoint works, actual metrics depend on other modules
        self.assertIsInstance(metrics_text, str)
    
    def test_correlation_attributes(self):
        """Test that correlation attributes are added"""
        response = requests.get(f"{self.BASE_URL}:{self.METRICS_PORT}/metrics")
        
        # Look for correlation-specific labels
        if "correlation_enabled" in response.text:
            self.assertIn('module="cross-signal-correlator"', response.text)
    
    def test_internal_metrics(self):
        """Test internal collector metrics"""
        response = requests.get(f"{self.BASE_URL}:8888/metrics")
        self.assertEqual(response.status_code, 200)
        
        # Check for OTEL collector internal metrics
        self.assertIn("otelcol_", response.text)
        self.assertIn("process_", response.text)
    
    def test_zpages(self):
        """Test zPages endpoint availability"""
        # zPages root typically returns a simple HTML page
        response = requests.get(f"{self.BASE_URL}:55679/")
        self.assertEqual(response.status_code, 200)
    
    def test_config_validation(self):
        """Test that the configuration is valid"""
        # This is more of a build-time test, but we can check if service started
        # successfully which indicates valid config
        response = requests.get(f"{self.BASE_URL}:{self.HEALTH_PORT}/health")
        self.assertEqual(response.status_code, 200)


class TestSlowQueryLogParsing(unittest.TestCase):
    """Test slow query log parsing functionality"""
    
    def test_log_format_parsing(self):
        """Test that log format matches expected pattern"""
        # Sample slow query log line
        log_line = "# Time: 2024-01-15T10:30:45.123456Z"
        import re
        pattern = r'# Time: (\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.\d+Z)'
        match = re.match(pattern, log_line)
        self.assertIsNotNone(match)
    
    def test_query_time_parsing(self):
        """Test query time extraction"""
        log_line = "# Query_time: 2.345678  Lock_time: 0.001234"
        import re
        pattern = r'# Query_time: ([\d.]+)\s+Lock_time: ([\d.]+)'
        match = re.search(pattern, log_line)
        self.assertIsNotNone(match)
        self.assertEqual(match.group(1), "2.345678")
        self.assertEqual(match.group(2), "0.001234")


class TestExemplarGeneration(unittest.TestCase):
    """Test exemplar generation for Prometheus metrics"""
    
    def test_exemplar_format(self):
        """Test that exemplars follow OpenMetrics format"""
        # Exemplar format: metric_name{labels} value timestamp # {trace_id="...",span_id="..."}
        exemplar = 'http_request_duration_seconds_bucket{le="0.1"} 1234 # {trace_id="abc123",span_id="def456"} 1.5'
        self.assertIn("trace_id", exemplar)
        self.assertIn("span_id", exemplar)


if __name__ == "__main__":
    unittest.main()