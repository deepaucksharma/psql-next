#!/usr/bin/env python3
"""
Alert Validator for E2E Testing
Validates that alerts are properly configured and triggering as expected
"""

import time
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Set

from validation_framework import BaseValidator, ValidationResult


class AlertValidator(BaseValidator):
    """Validates alert configurations and functionality"""
    
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.expected_alerts = self._load_expected_alerts()
    
    def _load_expected_alerts(self) -> Dict[str, Dict[str, Any]]:
        """Load expected alerts configuration for the module"""
        default_alerts = {
            # Database availability alerts
            "database_down": {
                "name": "DatabaseDown",
                "severity": "critical",
                "required": True,
                "expression": 'mysql_up == 0',
                "for": "1m",
                "labels": {
                    "severity": "critical",
                    "service": "mysql"
                }
            },
            "high_connection_usage": {
                "name": "HighConnectionUsage",
                "severity": "warning",
                "required": True,
                "expression": 'mysql_global_status_threads_connected / mysql_global_variables_max_connections > 0.8',
                "for": "5m",
                "labels": {
                    "severity": "warning",
                    "service": "mysql"
                }
            },
            "slow_queries_increasing": {
                "name": "SlowQueriesIncreasing",
                "severity": "warning",
                "required": True,
                "expression": 'rate(mysql_global_status_slow_queries[5m]) > 1',
                "for": "10m",
                "labels": {
                    "severity": "warning",
                    "service": "mysql"
                }
            },
            "replication_lag": {
                "name": "ReplicationLag",
                "severity": "warning",
                "required": False,
                "expression": 'mysql_slave_lag_seconds > 30',
                "for": "5m",
                "labels": {
                    "severity": "warning",
                    "service": "mysql"
                }
            }
        }
        
        # Add module-specific alerts
        if self.module_config.name == "anomaly-detector":
            default_alerts.update({
                "anomaly_detected": {
                    "name": "AnomalyDetected",
                    "severity": "warning",
                    "required": True,
                    "expression": 'increase(anomalies_detected_total[5m]) > 0',
                    "for": "1m",
                    "labels": {
                        "severity": "warning",
                        "service": "anomaly-detector"
                    }
                },
                "anomaly_detection_failed": {
                    "name": "AnomalyDetectionFailed",
                    "severity": "critical",
                    "required": True,
                    "expression": 'anomaly_detection_enabled == 0',
                    "for": "5m",
                    "labels": {
                        "severity": "critical",
                        "service": "anomaly-detector"
                    }
                }
            })
        
        # Load from config if available
        if self.module_config.config_file:
            config = self._load_yaml_config(self.module_config.config_file)
            if config and "expected_alerts" in config:
                default_alerts.update(config["expected_alerts"])
        
        return default_alerts
    
    def validate(self) -> List[ValidationResult]:
        """Run alert validation tests"""
        self.results = []
        start_time = time.time()
        
        # Test 1: Prometheus rules availability
        prom_rules_available = self._test_prometheus_rules_availability()
        
        if not prom_rules_available:
            return self.results
        
        # Test 2: Alert rule configuration
        self._test_alert_rule_configuration()
        
        # Test 3: Alertmanager availability
        alertmanager_available = self._test_alertmanager_availability()
        
        if alertmanager_available:
            # Test 4: Alert routing configuration
            self._test_alert_routing()
            
            # Test 5: Active alerts
            self._test_active_alerts()
            
            # Test 6: Alert history
            self._test_alert_history()
        
        # Test 7: Alert simulation
        self._test_alert_simulation()
        
        return self.results
    
    def _test_prometheus_rules_availability(self) -> bool:
        """Test if Prometheus rules endpoint is available"""
        start = time.time()
        
        if not self.module_config.prometheus_url:
            self._record_result(
                "prometheus_rules_availability",
                "skipped",
                "No Prometheus URL configured",
                time.time() - start
            )
            return False
        
        response = self._make_request(f"{self.module_config.prometheus_url}/api/v1/rules")
        
        if response and response.status_code == 200:
            self._record_result(
                "prometheus_rules_availability",
                "passed",
                "Prometheus rules endpoint is available",
                time.time() - start
            )
            return True
        else:
            self._record_result(
                "prometheus_rules_availability",
                "failed",
                "Prometheus rules endpoint not available",
                time.time() - start
            )
            return False
    
    def _test_alert_rule_configuration(self) -> None:
        """Validate alert rule configurations"""
        start = time.time()
        
        response = self._make_request(f"{self.module_config.prometheus_url}/api/v1/rules")
        
        if not response:
            self._record_result(
                "alert_rule_configuration",
                "failed",
                "Failed to fetch alert rules from Prometheus",
                time.time() - start
            )
            return
        
        try:
            data = response.json()
            if data["status"] != "success":
                raise ValueError("Failed to get rules")
            
            # Extract all alert rules
            configured_alerts = {}
            for group in data["data"]["groups"]:
                for rule in group["rules"]:
                    if rule["type"] == "alerting":
                        alert_name = rule["name"]
                        configured_alerts[alert_name] = {
                            "query": rule["query"],
                            "duration": rule.get("duration", 0),
                            "labels": rule.get("labels", {}),
                            "annotations": rule.get("annotations", {}),
                            "state": rule.get("state", "inactive")
                        }
            
            # Check expected alerts
            missing_alerts = []
            misconfigured_alerts = []
            
            for alert_id, expected in self.expected_alerts.items():
                if not expected.get("required", False):
                    continue
                
                alert_name = expected["name"]
                if alert_name not in configured_alerts:
                    missing_alerts.append(alert_name)
                else:
                    # Validate configuration
                    configured = configured_alerts[alert_name]
                    
                    # Check expression (basic comparison)
                    if expected.get("expression"):
                        # Normalize expressions for comparison
                        expected_expr = expected["expression"].replace(" ", "")
                        configured_expr = configured["query"].replace(" ", "")
                        
                        if expected_expr not in configured_expr:
                            misconfigured_alerts.append(
                                f"{alert_name}: expression mismatch"
                            )
                    
                    # Check severity label
                    if expected.get("severity"):
                        if configured["labels"].get("severity") != expected["severity"]:
                            misconfigured_alerts.append(
                                f"{alert_name}: incorrect severity"
                            )
            
            if missing_alerts or misconfigured_alerts:
                issues = []
                if missing_alerts:
                    issues.append(f"Missing: {', '.join(missing_alerts)}")
                if misconfigured_alerts:
                    issues.append(f"Misconfigured: {', '.join(misconfigured_alerts)}")
                
                self._record_result(
                    "alert_rule_configuration",
                    "failed",
                    "; ".join(issues),
                    time.time() - start,
                    {"missing": missing_alerts, "misconfigured": misconfigured_alerts}
                )
            else:
                self._record_result(
                    "alert_rule_configuration",
                    "passed",
                    f"All required alert rules are properly configured",
                    time.time() - start,
                    {"configured_alerts": list(configured_alerts.keys())}
                )
        
        except Exception as e:
            self._record_result(
                "alert_rule_configuration",
                "failed",
                f"Failed to validate alert rules: {e}",
                time.time() - start
            )
    
    def _test_alertmanager_availability(self) -> bool:
        """Test if Alertmanager is available"""
        start = time.time()
        
        if not self.module_config.alertmanager_url:
            self._record_result(
                "alertmanager_availability",
                "skipped",
                "No Alertmanager URL configured",
                time.time() - start
            )
            return False
        
        response = self._make_request(f"{self.module_config.alertmanager_url}/-/healthy")
        
        if response and response.status_code == 200:
            self._record_result(
                "alertmanager_availability",
                "passed",
                f"Alertmanager is healthy at {self.module_config.alertmanager_url}",
                time.time() - start
            )
            return True
        else:
            self._record_result(
                "alertmanager_availability",
                "failed",
                f"Alertmanager not available at {self.module_config.alertmanager_url}",
                time.time() - start
            )
            return False
    
    def _test_alert_routing(self) -> None:
        """Test Alertmanager routing configuration"""
        start = time.time()
        
        response = self._make_request(f"{self.module_config.alertmanager_url}/api/v1/status")
        
        if not response:
            self._record_result(
                "alert_routing",
                "failed",
                "Failed to fetch Alertmanager status",
                time.time() - start
            )
            return
        
        try:
            data = response.json()
            config = data.get("data", {}).get("config", {})
            
            if not config:
                self._record_result(
                    "alert_routing",
                    "failed",
                    "No Alertmanager configuration found",
                    time.time() - start
                )
                return
            
            # Check for basic routing configuration
            route = config.get("route", {})
            receivers = config.get("receivers", [])
            
            issues = []
            
            if not route:
                issues.append("No routing configuration")
            elif not route.get("receiver"):
                issues.append("No default receiver configured")
            
            if not receivers:
                issues.append("No receivers configured")
            else:
                # Check for at least one non-null receiver
                active_receivers = [r for r in receivers if r.get("name") != "null"]
                if not active_receivers:
                    issues.append("No active receivers configured")
            
            if issues:
                self._record_result(
                    "alert_routing",
                    "failed",
                    f"Alert routing issues: {', '.join(issues)}",
                    time.time() - start,
                    {"issues": issues}
                )
            else:
                self._record_result(
                    "alert_routing",
                    "passed",
                    "Alert routing is properly configured",
                    time.time() - start,
                    {"receivers": [r["name"] for r in receivers]}
                )
        
        except Exception as e:
            self._record_result(
                "alert_routing",
                "failed",
                f"Failed to validate alert routing: {e}",
                time.time() - start
            )
    
    def _test_active_alerts(self) -> None:
        """Check currently active alerts"""
        start = time.time()
        
        # Get alerts from Prometheus
        response = self._make_request(f"{self.module_config.prometheus_url}/api/v1/alerts")
        
        if not response:
            self._record_result(
                "active_alerts",
                "failed",
                "Failed to fetch active alerts",
                time.time() - start
            )
            return
        
        try:
            data = response.json()
            if data["status"] != "success":
                raise ValueError("Failed to get alerts")
            
            alerts = data["data"]["alerts"]
            active_alerts = [a for a in alerts if a["state"] == "firing"]
            pending_alerts = [a for a in alerts if a["state"] == "pending"]
            
            # Check for critical alerts
            critical_alerts = [
                a for a in active_alerts 
                if a.get("labels", {}).get("severity") == "critical"
            ]
            
            details = {
                "active_count": len(active_alerts),
                "pending_count": len(pending_alerts),
                "critical_count": len(critical_alerts)
            }
            
            if critical_alerts:
                critical_names = [a.get("labels", {}).get("alertname", "unknown") for a in critical_alerts]
                self._record_result(
                    "active_alerts",
                    "failed",
                    f"Critical alerts firing: {', '.join(critical_names)}",
                    time.time() - start,
                    {**details, "critical_alerts": critical_names}
                )
            elif active_alerts:
                # Non-critical alerts are a warning
                alert_names = [a.get("labels", {}).get("alertname", "unknown") for a in active_alerts]
                self._record_result(
                    "active_alerts",
                    "warning",
                    f"{len(active_alerts)} non-critical alerts active",
                    time.time() - start,
                    {**details, "active_alerts": alert_names[:5]}  # Limit output
                )
            else:
                self._record_result(
                    "active_alerts",
                    "passed",
                    "No active alerts",
                    time.time() - start,
                    details
                )
        
        except Exception as e:
            self._record_result(
                "active_alerts",
                "failed",
                f"Failed to check active alerts: {e}",
                time.time() - start
            )
    
    def _test_alert_history(self) -> None:
        """Check alert history for recent activity"""
        start = time.time()
        
        # Query Alertmanager for recent alerts
        response = self._make_request(f"{self.module_config.alertmanager_url}/api/v1/alerts")
        
        if not response:
            self._record_result(
                "alert_history",
                "skipped",
                "Failed to fetch alert history",
                time.time() - start
            )
            return
        
        try:
            data = response.json()
            alerts = data.get("data", [])
            
            # Count alerts by state and time
            now = datetime.now()
            recent_alerts = []
            
            for alert in alerts:
                # Check if alert was active in last 24 hours
                starts_at = alert.get("startsAt")
                if starts_at:
                    try:
                        # Parse ISO format timestamp
                        alert_time = datetime.fromisoformat(starts_at.replace("Z", "+00:00"))
                        if (now - alert_time).total_seconds() < 86400:  # 24 hours
                            recent_alerts.append({
                                "name": alert.get("labels", {}).get("alertname", "unknown"),
                                "severity": alert.get("labels", {}).get("severity", "unknown"),
                                "status": alert.get("status", {}).get("state", "unknown")
                            })
                    except Exception:
                        pass
            
            if recent_alerts:
                # Group by severity
                by_severity = {}
                for alert in recent_alerts:
                    sev = alert["severity"]
                    if sev not in by_severity:
                        by_severity[sev] = 0
                    by_severity[sev] += 1
                
                self._record_result(
                    "alert_history",
                    "passed",
                    f"{len(recent_alerts)} alerts in last 24 hours",
                    time.time() - start,
                    {"recent_alerts": recent_alerts[:10], "by_severity": by_severity}
                )
            else:
                self._record_result(
                    "alert_history",
                    "warning",
                    "No alerts triggered in last 24 hours",
                    time.time() - start
                )
        
        except Exception as e:
            self._record_result(
                "alert_history",
                "failed",
                f"Failed to check alert history: {e}",
                time.time() - start
            )
    
    def _test_alert_simulation(self) -> None:
        """Simulate alert conditions to verify they trigger correctly"""
        start = time.time()
        
        # This is a placeholder for alert simulation
        # In a real implementation, you would:
        # 1. Create conditions that should trigger alerts
        # 2. Wait for the alert to fire
        # 3. Verify the alert was received
        
        self._record_result(
            "alert_simulation",
            "skipped",
            "Alert simulation not implemented - requires test environment setup",
            time.time() - start,
            {"note": "Implement with test data injection or mock metrics"}
        )