#!/usr/bin/env python3
"""
Dashboard Validator for E2E Testing
Validates that Grafana dashboards are properly configured and rendering
"""

import json
import time
from typing import Any, Dict, List, Optional, Set

from validation_framework import BaseValidator, ValidationResult


class DashboardValidator(BaseValidator):
    """Validates Grafana dashboards configuration and rendering"""
    
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.expected_dashboards = self._load_expected_dashboards()
    
    def _load_expected_dashboards(self) -> Dict[str, Dict[str, Any]]:
        """Load expected dashboards configuration for the module"""
        default_dashboards = {
            "database-overview": {
                "title": "Database Overview",
                "required": True,
                "panels": [
                    "Database Status",
                    "Connection Metrics",
                    "Query Performance",
                    "Resource Utilization"
                ]
            },
            "performance-insights": {
                "title": "Performance Insights",
                "required": True,
                "panels": [
                    "Slow Query Analysis",
                    "Table I/O Statistics",
                    "Lock Wait Analysis"
                ]
            }
        }
        
        # Add module-specific dashboards
        if self.module_config.name == "anomaly-detector":
            default_dashboards["anomaly-detection"] = {
                "title": "Anomaly Detection",
                "required": True,
                "panels": [
                    "Anomaly Timeline",
                    "Detection Status",
                    "Anomaly Types Distribution",
                    "Alert History"
                ]
            }
        elif self.module_config.name == "query-insights":
            default_dashboards["query-insights"] = {
                "title": "Query Insights",
                "required": True,
                "panels": [
                    "Top Queries by Execution Time",
                    "Query Frequency Analysis",
                    "Query Plan Statistics"
                ]
            }
        
        # Load from config if available
        if self.module_config.config_file:
            config = self._load_yaml_config(self.module_config.config_file)
            if config and "expected_dashboards" in config:
                default_dashboards.update(config["expected_dashboards"])
        
        return default_dashboards
    
    def validate(self) -> List[ValidationResult]:
        """Run dashboard validation tests"""
        self.results = []
        start_time = time.time()
        
        # Test 1: Grafana availability
        grafana_available = self._test_grafana_availability()
        
        if not grafana_available:
            return self.results
        
        # Test 2: Check dashboard existence
        self._test_dashboard_existence()
        
        # Test 3: Validate dashboard configurations
        self._test_dashboard_configurations()
        
        # Test 4: Check panel data sources
        self._test_panel_datasources()
        
        # Test 5: Validate dashboard queries
        self._test_dashboard_queries()
        
        # Test 6: Check dashboard annotations
        self._test_dashboard_annotations()
        
        return self.results
    
    def _test_grafana_availability(self) -> bool:
        """Test if Grafana is available and accessible"""
        start = time.time()
        
        if not self.module_config.grafana_url:
            self._record_result(
                "grafana_availability",
                "skipped",
                "No Grafana URL configured",
                time.time() - start
            )
            return False
        
        # Check health endpoint
        response = self._make_request(f"{self.module_config.grafana_url}/api/health")
        
        if response and response.status_code == 200:
            try:
                health_data = response.json()
                if health_data.get("database") == "ok":
                    self._record_result(
                        "grafana_availability",
                        "passed",
                        f"Grafana is healthy at {self.module_config.grafana_url}",
                        time.time() - start
                    )
                    return True
            except Exception:
                pass
        
        self._record_result(
            "grafana_availability",
            "failed",
            f"Grafana not available at {self.module_config.grafana_url}",
            time.time() - start
        )
        return False
    
    def _test_dashboard_existence(self) -> None:
        """Check if expected dashboards exist"""
        start = time.time()
        
        # Get all dashboards
        response = self._make_request(f"{self.module_config.grafana_url}/api/search?type=dash-db")
        
        if not response:
            self._record_result(
                "dashboard_existence",
                "failed",
                "Failed to fetch dashboard list from Grafana",
                time.time() - start
            )
            return
        
        try:
            dashboards = response.json()
            dashboard_uids = {d.get("uid", ""): d.get("title", "") for d in dashboards}
            
            missing_dashboards = []
            found_dashboards = []
            
            for uid, config in self.expected_dashboards.items():
                if config.get("required", False):
                    # Check by UID or title
                    if uid in dashboard_uids or config["title"] in dashboard_uids.values():
                        found_dashboards.append(config["title"])
                    else:
                        missing_dashboards.append(config["title"])
            
            if missing_dashboards:
                self._record_result(
                    "dashboard_existence",
                    "failed",
                    f"Missing required dashboards: {', '.join(missing_dashboards)}",
                    time.time() - start,
                    {"missing": missing_dashboards, "found": found_dashboards}
                )
            else:
                self._record_result(
                    "dashboard_existence",
                    "passed",
                    f"All {len(found_dashboards)} required dashboards exist",
                    time.time() - start,
                    {"found": found_dashboards}
                )
        except Exception as e:
            self._record_result(
                "dashboard_existence",
                "failed",
                f"Failed to parse dashboard list: {e}",
                time.time() - start
            )
    
    def _test_dashboard_configurations(self) -> None:
        """Validate dashboard configurations and panels"""
        start = time.time()
        invalid_configs = []
        
        # Get dashboard list
        response = self._make_request(f"{self.module_config.grafana_url}/api/search?type=dash-db")
        if not response:
            return
        
        try:
            dashboards = response.json()
            
            for dashboard in dashboards:
                uid = dashboard.get("uid")
                title = dashboard.get("title", "")
                
                # Find matching expected dashboard
                expected = None
                for exp_uid, exp_config in self.expected_dashboards.items():
                    if exp_uid == uid or exp_config["title"] == title:
                        expected = exp_config
                        break
                
                if not expected or not expected.get("required", False):
                    continue
                
                # Get full dashboard config
                dash_response = self._make_request(
                    f"{self.module_config.grafana_url}/api/dashboards/uid/{uid}"
                )
                
                if dash_response:
                    dash_data = dash_response.json()
                    dashboard_config = dash_data.get("dashboard", {})
                    
                    # Check required panels
                    panels = dashboard_config.get("panels", [])
                    panel_titles = [p.get("title", "") for p in panels]
                    
                    missing_panels = []
                    for required_panel in expected.get("panels", []):
                        if required_panel not in panel_titles:
                            missing_panels.append(required_panel)
                    
                    if missing_panels:
                        invalid_configs.append(
                            f"{title}: missing panels {missing_panels}"
                        )
                    
                    # Check dashboard settings
                    if not dashboard_config.get("time"):
                        invalid_configs.append(f"{title}: no time range configured")
                    
                    if not dashboard_config.get("refresh"):
                        invalid_configs.append(f"{title}: no auto-refresh configured")
                    
                    # Check for empty panels
                    empty_panels = [p["title"] for p in panels if not p.get("targets")]
                    if empty_panels:
                        invalid_configs.append(f"{title}: empty panels {empty_panels}")
        
        except Exception as e:
            self.logger.error(f"Failed to validate dashboard configs: {e}")
        
        if invalid_configs:
            self._record_result(
                "dashboard_configurations",
                "failed",
                f"Invalid dashboard configurations found",
                time.time() - start,
                {"issues": invalid_configs}
            )
        else:
            self._record_result(
                "dashboard_configurations",
                "passed",
                "All dashboard configurations are valid",
                time.time() - start
            )
    
    def _test_panel_datasources(self) -> None:
        """Check that all panels have valid data sources"""
        start = time.time()
        invalid_datasources = []
        
        # Get available data sources
        ds_response = self._make_request(f"{self.module_config.grafana_url}/api/datasources")
        if not ds_response:
            return
        
        try:
            datasources = ds_response.json()
            ds_names = {ds["name"] for ds in datasources}
            ds_uids = {ds["uid"] for ds in datasources}
            
            # Check each dashboard
            dash_response = self._make_request(f"{self.module_config.grafana_url}/api/search?type=dash-db")
            if dash_response:
                dashboards = dash_response.json()
                
                for dashboard in dashboards:
                    uid = dashboard.get("uid")
                    title = dashboard.get("title", "")
                    
                    # Get full dashboard
                    full_dash = self._make_request(
                        f"{self.module_config.grafana_url}/api/dashboards/uid/{uid}"
                    )
                    
                    if full_dash:
                        dash_data = full_dash.json()
                        panels = dash_data.get("dashboard", {}).get("panels", [])
                        
                        for panel in panels:
                            panel_title = panel.get("title", "Untitled")
                            
                            # Check panel datasource
                            datasource = panel.get("datasource")
                            if datasource:
                                ds_uid = datasource.get("uid", "")
                                ds_name = datasource.get("type", "")
                                
                                if ds_uid not in ds_uids and ds_name not in ds_names:
                                    invalid_datasources.append(
                                        f"{title}/{panel_title}: invalid datasource"
                                    )
                            
                            # Check target datasources
                            for target in panel.get("targets", []):
                                target_ds = target.get("datasource")
                                if target_ds and isinstance(target_ds, dict):
                                    ds_uid = target_ds.get("uid", "")
                                    if ds_uid and ds_uid not in ds_uids:
                                        invalid_datasources.append(
                                            f"{title}/{panel_title}: invalid target datasource"
                                        )
        
        except Exception as e:
            self.logger.error(f"Failed to validate datasources: {e}")
        
        if invalid_datasources:
            self._record_result(
                "panel_datasources",
                "failed",
                "Panels with invalid data sources detected",
                time.time() - start,
                {"invalid": invalid_datasources[:10]}  # Limit output
            )
        else:
            self._record_result(
                "panel_datasources",
                "passed",
                "All panels have valid data sources",
                time.time() - start
            )
    
    def _test_dashboard_queries(self) -> None:
        """Validate that dashboard queries are properly formed"""
        start = time.time()
        invalid_queries = []
        
        try:
            # Get dashboard list
            response = self._make_request(f"{self.module_config.grafana_url}/api/search?type=dash-db")
            if not response:
                return
            
            dashboards = response.json()
            
            for dashboard in dashboards[:5]:  # Limit to first 5 dashboards for performance
                uid = dashboard.get("uid")
                title = dashboard.get("title", "")
                
                # Get full dashboard
                dash_response = self._make_request(
                    f"{self.module_config.grafana_url}/api/dashboards/uid/{uid}"
                )
                
                if dash_response:
                    dash_data = dash_response.json()
                    panels = dash_data.get("dashboard", {}).get("panels", [])
                    
                    for panel in panels:
                        panel_title = panel.get("title", "Untitled")
                        panel_type = panel.get("type", "")
                        
                        # Skip non-query panels
                        if panel_type in ["text", "row"]:
                            continue
                        
                        targets = panel.get("targets", [])
                        if not targets:
                            invalid_queries.append(f"{title}/{panel_title}: no queries defined")
                            continue
                        
                        for i, target in enumerate(targets):
                            # Check Prometheus queries
                            if "expr" in target:
                                expr = target.get("expr", "")
                                if not expr or expr == "":
                                    invalid_queries.append(
                                        f"{title}/{panel_title}: empty query {i+1}"
                                    )
                                elif "$" in expr and "{{" not in expr:
                                    # Check for unresolved variables
                                    invalid_queries.append(
                                        f"{title}/{panel_title}: unresolved variable in query {i+1}"
                                    )
                            
                            # Check SQL queries
                            elif "rawSql" in target:
                                sql = target.get("rawSql", "")
                                if not sql or sql == "":
                                    invalid_queries.append(
                                        f"{title}/{panel_title}: empty SQL query {i+1}"
                                    )
        
        except Exception as e:
            self.logger.error(f"Failed to validate queries: {e}")
        
        if invalid_queries:
            self._record_result(
                "dashboard_queries",
                "failed",
                "Invalid queries found in dashboards",
                time.time() - start,
                {"invalid": invalid_queries[:10]}  # Limit output
            )
        else:
            self._record_result(
                "dashboard_queries",
                "passed",
                "All dashboard queries are properly formed",
                time.time() - start
            )
    
    def _test_dashboard_annotations(self) -> None:
        """Check if dashboards have proper annotations configured"""
        start = time.time()
        dashboards_without_annotations = []
        
        try:
            # Get dashboard list
            response = self._make_request(f"{self.module_config.grafana_url}/api/search?type=dash-db")
            if not response:
                return
            
            dashboards = response.json()
            
            for dashboard in dashboards:
                uid = dashboard.get("uid")
                title = dashboard.get("title", "")
                
                # Skip if not a required dashboard
                is_required = False
                for exp_uid, exp_config in self.expected_dashboards.items():
                    if (exp_uid == uid or exp_config["title"] == title) and exp_config.get("required", False):
                        is_required = True
                        break
                
                if not is_required:
                    continue
                
                # Get full dashboard
                dash_response = self._make_request(
                    f"{self.module_config.grafana_url}/api/dashboards/uid/{uid}"
                )
                
                if dash_response:
                    dash_data = dash_response.json()
                    annotations = dash_data.get("dashboard", {}).get("annotations", {}).get("list", [])
                    
                    # Check for at least one enabled annotation
                    enabled_annotations = [a for a in annotations if a.get("enable", False)]
                    
                    if not enabled_annotations:
                        dashboards_without_annotations.append(title)
        
        except Exception as e:
            self.logger.error(f"Failed to check annotations: {e}")
        
        if dashboards_without_annotations:
            self._record_result(
                "dashboard_annotations",
                "warning",
                f"Dashboards without annotations: {', '.join(dashboards_without_annotations)}",
                time.time() - start,
                {"missing_annotations": dashboards_without_annotations}
            )
        else:
            self._record_result(
                "dashboard_annotations",
                "passed",
                "All required dashboards have annotations configured",
                time.time() - start
            )