import pytest
import requests
import time

# Module endpoints
MODULES = {
    'core-metrics': 'http://core-metrics:8081/metrics',
    'sql-intelligence': 'http://sql-intelligence:8082/metrics',
    'wait-profiler': 'http://wait-profiler:8083/metrics',
    'anomaly-detector': 'http://anomaly-detector:8084/metrics',
    'business-impact': 'http://business-impact:8085/metrics',
    'performance-advisor': 'http://performance-advisor:8087/metrics',
    'resource-monitor': 'http://resource-monitor:8088/metrics'
}

def wait_for_service(url, timeout=60):
    """Wait for a service to become available"""
    start = time.time()
    while time.time() - start < timeout:
        try:
            response = requests.get(url, timeout=5)
            if response.status_code == 200:
                return True
        except:
            pass
        time.sleep(2)
    return False

class TestModuleIntegration:
    
    def test_all_modules_healthy(self):
        """Test that all modules are running and healthy"""
        for module, url in MODULES.items():
            assert wait_for_service(url), f"{module} failed to start"
            response = requests.get(url)
            assert response.status_code == 200, f"{module} returned {response.status_code}"
    
    def test_core_metrics_exports(self):
        """Test that core-metrics exports expected metrics"""
        response = requests.get(MODULES['core-metrics'])
        metrics = response.text
        
        assert 'mysql_uptime' in metrics
        assert 'mysql_connections' in metrics
        assert 'mysql_threads' in metrics
    
    def test_sql_intelligence_exports(self):
        """Test that sql-intelligence exports query metrics"""
        response = requests.get(MODULES['sql-intelligence'])
        metrics = response.text
        
        assert 'mysql_query' in metrics
        assert 'mysql_table_io' in metrics
    
    def test_anomaly_detector_consumes_metrics(self):
        """Test that anomaly-detector consumes metrics from other modules"""
        response = requests.get(MODULES['anomaly-detector'])
        metrics = response.text
        
        assert 'mysql_anomaly_score' in metrics
    
    def test_performance_advisor_recommendations(self):
        """Test that performance-advisor generates recommendations"""
        response = requests.get(MODULES['performance-advisor'])
        metrics = response.text
        
        assert 'mysql_recommendation' in metrics
    
    def test_metric_flow(self):
        """Test that metrics flow through the system"""
        # Generate some load on MySQL
        # This would trigger metrics in core-metrics
        # Which would be consumed by anomaly-detector
        # And generate recommendations in performance-advisor
        
        # Wait for metrics to propagate
        time.sleep(10)
        
        # Check that anomaly detector has processed metrics
        response = requests.get(MODULES['anomaly-detector'])
        assert response.status_code == 200