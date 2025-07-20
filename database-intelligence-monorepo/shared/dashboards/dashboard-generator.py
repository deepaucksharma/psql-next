#!/usr/bin/env python3
"""
Dashboard Generator for Database Intelligence Modules

This script generates module-specific Grafana dashboards based on the base dashboard template.
It allows customization of panels, variables, and module-specific metrics.
"""

import json
import argparse
import copy
from pathlib import Path
from typing import Dict, List, Any, Optional


class DashboardGenerator:
    """Generate module-specific dashboards from base template."""
    
    def __init__(self, base_dashboard_path: str = "base-dashboard.json"):
        """Initialize the generator with base dashboard template."""
        self.base_path = Path(base_dashboard_path)
        with open(self.base_path, 'r') as f:
            self.base_dashboard = json.load(f)
    
    def create_panel(self, panel_config: Dict[str, Any], grid_position: Dict[str, int]) -> Dict[str, Any]:
        """Create a panel from the panels library or custom configuration."""
        if 'panel_ref' in panel_config:
            # Use a panel from the library
            panel_ref = panel_config['panel_ref']
            if panel_ref in self.base_dashboard.get('panels_library', {}):
                panel = copy.deepcopy(self.base_dashboard['panels_library'][panel_ref])
                # Override with custom settings
                if 'overrides' in panel_config:
                    self._deep_merge(panel, panel_config['overrides'])
            else:
                raise ValueError(f"Panel reference '{panel_ref}' not found in panels library")
        else:
            # Create custom panel
            panel = panel_config
        
        # Set grid position
        panel['gridPos'] = grid_position
        
        # Ensure panel has an ID
        if 'id' not in panel:
            panel['id'] = len(self.base_dashboard.get('panels', [])) + 1
        
        return panel
    
    def _deep_merge(self, target: Dict, source: Dict) -> Dict:
        """Deep merge source dictionary into target dictionary."""
        for key, value in source.items():
            if key in target and isinstance(target[key], dict) and isinstance(value, dict):
                self._deep_merge(target[key], value)
            else:
                target[key] = value
        return target
    
    def generate_dashboard(
        self,
        module_name: str,
        title: str,
        description: str,
        panels: List[Dict[str, Any]],
        custom_variables: Optional[List[Dict[str, Any]]] = None,
        tags: Optional[List[str]] = None,
        uid_prefix: str = "db-intel"
    ) -> Dict[str, Any]:
        """Generate a module-specific dashboard."""
        dashboard = copy.deepcopy(self.base_dashboard)
        
        # Set basic properties
        dashboard['title'] = title
        dashboard['description'] = description
        dashboard['uid'] = f"{uid_prefix}-{module_name}"
        
        # Add module-specific tags
        if tags:
            dashboard['tags'] = list(set(dashboard.get('tags', []) + tags))
        
        # Add custom template variables
        if custom_variables:
            dashboard['templating']['list'].extend(custom_variables)
        
        # Generate panels with automatic grid positioning
        dashboard['panels'] = []
        row_height = 8
        panels_per_row = 2
        
        for i, panel_config in enumerate(panels):
            row = i // panels_per_row
            col = i % panels_per_row
            
            grid_pos = {
                'x': col * 12,
                'y': row * row_height,
                'w': 12 if len(panels) > 1 else 24,
                'h': row_height
            }
            
            # Allow custom grid positioning
            if 'gridPos' in panel_config:
                grid_pos.update(panel_config['gridPos'])
                del panel_config['gridPos']
            
            panel = self.create_panel(panel_config, grid_pos)
            dashboard['panels'].append(panel)
        
        return dashboard
    
    def save_dashboard(self, dashboard: Dict[str, Any], output_path: str):
        """Save dashboard to JSON file."""
        with open(output_path, 'w') as f:
            json.dump(dashboard, f, indent=2)
        print(f"Dashboard saved to: {output_path}")


def create_anomaly_detector_dashboard():
    """Create dashboard for the anomaly-detector module."""
    generator = DashboardGenerator()
    
    panels = [
        {'panel_ref': 'cpu_usage'},
        {'panel_ref': 'memory_usage'},
        {
            'panel_ref': 'query_rate',
            'overrides': {
                'title': 'Query Rate with Anomaly Detection',
                'targets': [
                    {
                        'expr': 'rate(mysql_global_status_queries{instance=~"${instance}"}[${interval}])',
                        'refId': 'A',
                        'legendFormat': 'Actual'
                    },
                    {
                        'expr': 'anomaly_detection_threshold{metric="query_rate",instance=~"${instance}"}',
                        'refId': 'B',
                        'legendFormat': 'Threshold'
                    }
                ]
            }
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'thresholds'},
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'yellow', 'value': 5},
                            {'color': 'red', 'value': 10}
                        ]
                    },
                    'unit': 'short'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 10,
            'options': {
                'colorMode': 'value',
                'graphMode': 'area',
                'justifyMode': 'auto',
                'orientation': 'auto',
                'reduceOptions': {
                    'values': False,
                    'calcs': ['lastNotNull'],
                    'fields': ''
                },
                'textMode': 'auto'
            },
            'targets': [
                {
                    'expr': 'sum(increase(anomaly_detection_count{instance=~"${instance}"}[${interval}]))',
                    'refId': 'A'
                }
            ],
            'title': 'Anomalies Detected',
            'type': 'stat'
        }
    ]
    
    custom_variables = [
        {
            'name': 'anomaly_type',
            'label': 'Anomaly Type',
            'type': 'custom',
            'options': [
                {'text': 'All', 'value': '.*'},
                {'text': 'CPU', 'value': 'cpu'},
                {'text': 'Memory', 'value': 'memory'},
                {'text': 'Query Rate', 'value': 'query_rate'},
                {'text': 'Connections', 'value': 'connections'}
            ],
            'current': {'text': 'All', 'value': '.*'},
            'multi': False
        }
    ]
    
    dashboard = generator.generate_dashboard(
        module_name='anomaly-detector',
        title='Database Anomaly Detection Dashboard',
        description='Monitor database performance metrics and detect anomalies in real-time',
        panels=panels,
        custom_variables=custom_variables,
        tags=['anomaly', 'detection', 'mysql']
    )
    
    return dashboard


def create_performance_tuner_dashboard():
    """Create dashboard for the performance-tuner module."""
    generator = DashboardGenerator()
    
    panels = [
        {'panel_ref': 'cpu_usage'},
        {'panel_ref': 'memory_usage'},
        {'panel_ref': 'query_rate'},
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False}
                    },
                    'mappings': [],
                    'unit': 'ms'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 20,
            'options': {
                'legend': {'displayMode': 'list', 'placement': 'bottom'},
                'pieType': 'pie',
                'tooltip': {'mode': 'single'}
            },
            'targets': [
                {
                    'expr': 'topk(10, avg by (query) (mysql_perf_schema_events_statements_seconds_total{instance=~"${instance}"}))',
                    'refId': 'A'
                }
            ],
            'title': 'Top 10 Slowest Queries',
            'type': 'piechart'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'custom': {
                        'align': 'auto',
                        'displayMode': 'auto'
                    },
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [{'color': 'green', 'value': None}]
                    }
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 21,
            'options': {
                'showHeader': True
            },
            'targets': [
                {
                    'expr': 'performance_recommendations{instance=~"${instance}"}',
                    'format': 'table',
                    'refId': 'A'
                }
            ],
            'title': 'Performance Recommendations',
            'type': 'table'
        }
    ]
    
    dashboard = generator.generate_dashboard(
        module_name='performance-tuner',
        title='Database Performance Tuning Dashboard',
        description='Analyze database performance and get tuning recommendations',
        panels=panels,
        tags=['performance', 'tuning', 'optimization']
    )
    
    return dashboard


def create_query_optimizer_dashboard():
    """Create dashboard for the query-optimizer module."""
    generator = DashboardGenerator()
    
    panels = [
        {'panel_ref': 'query_rate'},
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'axisCenteredZero': False,
                        'axisColorMode': 'text',
                        'axisLabel': '',
                        'axisPlacement': 'auto',
                        'barAlignment': 0,
                        'drawStyle': 'bars',
                        'fillOpacity': 100,
                        'gradientMode': 'none',
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False},
                        'lineInterpolation': 'linear',
                        'lineWidth': 1,
                        'pointSize': 5,
                        'scaleDistribution': {'type': 'linear'},
                        'showPoints': 'never',
                        'spanNulls': False,
                        'stacking': {'group': 'A', 'mode': 'none'},
                        'thresholdsStyle': {'mode': 'off'}
                    },
                    'mappings': [],
                    'unit': 'short'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 30,
            'options': {
                'legend': {'calcs': [], 'displayMode': 'list', 'placement': 'bottom'},
                'tooltip': {'mode': 'single'}
            },
            'targets': [
                {
                    'expr': 'query_optimization_score{instance=~"${instance}"}',
                    'refId': 'A'
                }
            ],
            'title': 'Query Optimization Scores',
            'type': 'timeseries'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'custom': {
                        'align': 'auto',
                        'displayMode': 'auto'
                    },
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [{'color': 'green', 'value': None}]
                    }
                }
            },
            'gridPos': {'h': 10, 'w': 24},
            'id': 31,
            'options': {
                'showHeader': True,
                'sortBy': [{'displayName': 'Execution Time', 'desc': True}]
            },
            'targets': [
                {
                    'expr': 'query_execution_plan{instance=~"${instance}"}',
                    'format': 'table',
                    'refId': 'A'
                }
            ],
            'title': 'Query Execution Plans',
            'type': 'table'
        }
    ]
    
    dashboard = generator.generate_dashboard(
        module_name='query-optimizer',
        title='Database Query Optimization Dashboard',
        description='Analyze and optimize database queries for better performance',
        panels=panels,
        tags=['query', 'optimization', 'analysis']
    )
    
    return dashboard


def create_sql_intelligence_dashboard():
    """Create dashboard for the sql-intelligence module - Query performance analysis."""
    generator = DashboardGenerator()
    
    panels = [
        {'panel_ref': 'query_rate'},
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'axisCenteredZero': False,
                        'axisColorMode': 'text',
                        'axisLabel': '',
                        'axisPlacement': 'auto',
                        'barAlignment': 0,
                        'drawStyle': 'line',
                        'fillOpacity': 10,
                        'gradientMode': 'none',
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False},
                        'lineInterpolation': 'linear',
                        'lineWidth': 1,
                        'pointSize': 5,
                        'scaleDistribution': {'type': 'linear'},
                        'showPoints': 'never',
                        'spanNulls': False,
                        'stacking': {'group': 'A', 'mode': 'none'},
                        'thresholdsStyle': {'mode': 'off'}
                    },
                    'mappings': [],
                    'unit': 'ms'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 40,
            'options': {
                'legend': {'calcs': [], 'displayMode': 'list', 'placement': 'bottom'},
                'tooltip': {'mode': 'single'}
            },
            'targets': [
                {
                    'expr': 'histogram_quantile(0.95, sum(rate(mysql_perf_schema_events_statements_seconds_bucket{instance=~"${instance}"}[${interval}])) by (le))',
                    'refId': 'A',
                    'legendFormat': 'p95'
                },
                {
                    'expr': 'histogram_quantile(0.99, sum(rate(mysql_perf_schema_events_statements_seconds_bucket{instance=~"${instance}"}[${interval}])) by (le))',
                    'refId': 'B',
                    'legendFormat': 'p99'
                }
            ],
            'title': 'Query Latency Percentiles',
            'type': 'timeseries'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False}
                    },
                    'mappings': [],
                    'unit': 'short'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 41,
            'options': {
                'legend': {'displayMode': 'list', 'placement': 'right'},
                'pieType': 'donut',
                'tooltip': {'mode': 'single'}
            },
            'targets': [
                {
                    'expr': 'topk(5, sum by (digest_text) (rate(mysql_perf_schema_events_statements_total{instance=~"${instance}"}[${interval}])))',
                    'refId': 'A',
                    'legendFormat': '{{digest_text}}'
                }
            ],
            'title': 'Top 5 Query Types',
            'type': 'piechart'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'custom': {
                        'align': 'auto',
                        'displayMode': 'auto'
                    },
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'yellow', 'value': 1000},
                            {'color': 'red', 'value': 5000}
                        ]
                    },
                    'unit': 'ms'
                }
            },
            'gridPos': {'h': 10, 'w': 24},
            'id': 42,
            'options': {
                'showHeader': True,
                'sortBy': [{'displayName': 'Avg Execution Time', 'desc': True}]
            },
            'targets': [
                {
                    'expr': 'topk(10, avg by (digest_text) (mysql_perf_schema_events_statements_seconds_total{instance=~"${instance}"}))',
                    'format': 'table',
                    'refId': 'A'
                }
            ],
            'title': 'Slow Query Analysis',
            'type': 'table'
        }
    ]
    
    custom_variables = [
        {
            'name': 'query_digest',
            'label': 'Query Digest',
            'type': 'query',
            'query': 'label_values(mysql_perf_schema_events_statements_total, digest_text)',
            'multi': True,
            'includeAll': True,
            'current': {'text': 'All', 'value': '$__all'}
        }
    ]
    
    dashboard = generator.generate_dashboard(
        module_name='sql-intelligence',
        title='SQL Intelligence Dashboard',
        description='Advanced query performance analysis and insights',
        panels=panels,
        custom_variables=custom_variables,
        tags=['sql', 'query', 'performance', 'intelligence']
    )
    
    return dashboard


def create_wait_profiler_dashboard():
    """Create dashboard for the wait-profiler module - Wait event visualization."""
    generator = DashboardGenerator()
    
    panels = [
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'axisCenteredZero': False,
                        'axisColorMode': 'text',
                        'axisLabel': '',
                        'axisPlacement': 'auto',
                        'barAlignment': 0,
                        'drawStyle': 'line',
                        'fillOpacity': 10,
                        'gradientMode': 'none',
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False},
                        'lineInterpolation': 'linear',
                        'lineWidth': 1,
                        'pointSize': 5,
                        'scaleDistribution': {'type': 'linear'},
                        'showPoints': 'never',
                        'spanNulls': False,
                        'stacking': {'group': 'A', 'mode': 'normal'},
                        'thresholdsStyle': {'mode': 'off'}
                    },
                    'mappings': [],
                    'unit': 'µs'
                }
            },
            'gridPos': {'h': 10, 'w': 24},
            'id': 50,
            'options': {
                'legend': {'calcs': [], 'displayMode': 'list', 'placement': 'right'},
                'tooltip': {'mode': 'multi'}
            },
            'targets': [
                {
                    'expr': 'topk(10, sum by (event_name) (rate(mysql_perf_schema_events_waits_seconds_total{instance=~"${instance}"}[${interval}])))',
                    'refId': 'A',
                    'legendFormat': '{{event_name}}'
                }
            ],
            'title': 'Wait Events by Type',
            'type': 'timeseries'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'thresholds'},
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'yellow', 'value': 1000},
                            {'color': 'red', 'value': 5000}
                        ]
                    },
                    'unit': 'µs'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 51,
            'options': {
                'colorMode': 'value',
                'graphMode': 'area',
                'justifyMode': 'auto',
                'orientation': 'auto',
                'reduceOptions': {
                    'values': False,
                    'calcs': ['mean'],
                    'fields': ''
                },
                'textMode': 'auto'
            },
            'targets': [
                {
                    'expr': 'avg(mysql_perf_schema_events_waits_seconds_total{instance=~"${instance}"})',
                    'refId': 'A'
                }
            ],
            'title': 'Average Wait Time',
            'type': 'stat'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'continuous-GrYlRd'},
                    'custom': {
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False}
                    },
                    'mappings': [],
                    'unit': 'short'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 52,
            'options': {
                'calculate': True,
                'cellGap': 1,
                'cellValues': {
                    'unit': 'µs'
                },
                'color': {
                    'scheme': 'Spectral',
                    'steps': 64
                },
                'exemplars': False,
                'filterValues': {'le': 1e-09},
                'legend': {'show': True},
                'rowsFrame': {'layout': 'auto'},
                'tooltip': {'show': True, 'yHistogram': False},
                'yAxis': {'axisPlacement': 'left', 'reverse': False}
            },
            'targets': [
                {
                    'expr': 'sum by (event_name, object_name) (rate(mysql_perf_schema_events_waits_total{instance=~"${instance}"}[${interval}]))',
                    'refId': 'A'
                }
            ],
            'title': 'Wait Event Heatmap',
            'type': 'heatmap'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'custom': {
                        'align': 'auto',
                        'displayMode': 'auto'
                    },
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [{'color': 'green', 'value': None}]
                    },
                    'unit': 'µs'
                }
            },
            'gridPos': {'h': 10, 'w': 24},
            'id': 53,
            'options': {
                'showHeader': True,
                'sortBy': [{'displayName': 'Total Wait Time', 'desc': True}]
            },
            'targets': [
                {
                    'expr': 'topk(20, sum by (event_name, object_name, object_schema) (mysql_perf_schema_events_waits_seconds_total{instance=~"${instance}"}))',
                    'format': 'table',
                    'refId': 'A'
                }
            ],
            'title': 'Top Wait Events Detail',
            'type': 'table'
        }
    ]
    
    custom_variables = [
        {
            'name': 'wait_class',
            'label': 'Wait Class',
            'type': 'custom',
            'options': [
                {'text': 'All', 'value': '.*'},
                {'text': 'I/O', 'value': 'wait/io/.*'},
                {'text': 'Lock', 'value': 'wait/lock/.*'},
                {'text': 'Synch', 'value': 'wait/synch/.*'}
            ],
            'current': {'text': 'All', 'value': '.*'},
            'multi': False
        }
    ]
    
    dashboard = generator.generate_dashboard(
        module_name='wait-profiler',
        title='Wait Event Profiler Dashboard',
        description='Analyze database wait events and identify performance bottlenecks',
        panels=panels,
        custom_variables=custom_variables,
        tags=['wait', 'events', 'profiler', 'performance']
    )
    
    return dashboard


def create_business_impact_dashboard():
    """Create dashboard for the business-impact module - Business metrics and impact."""
    generator = DashboardGenerator()
    
    panels = [
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'thresholds'},
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'red', 'value': None},
                            {'color': 'yellow', 'value': 95},
                            {'color': 'green', 'value': 99}
                        ]
                    },
                    'unit': 'percent'
                }
            },
            'gridPos': {'h': 8, 'w': 6},
            'id': 60,
            'options': {
                'orientation': 'auto',
                'reduceOptions': {
                    'values': False,
                    'calcs': ['lastNotNull'],
                    'fields': ''
                },
                'showThresholdLabels': False,
                'showThresholdMarkers': True
            },
            'targets': [
                {
                    'expr': '(1 - (rate(mysql_global_status_aborted_connects{instance=~"${instance}"}[${interval}]) / rate(mysql_global_status_connections{instance=~"${instance}"}[${interval}]))) * 100',
                    'refId': 'A'
                }
            ],
            'title': 'Service Availability',
            'type': 'gauge'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'axisCenteredZero': False,
                        'axisColorMode': 'text',
                        'axisLabel': '',
                        'axisPlacement': 'auto',
                        'barAlignment': 0,
                        'drawStyle': 'line',
                        'fillOpacity': 10,
                        'gradientMode': 'none',
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False},
                        'lineInterpolation': 'linear',
                        'lineWidth': 2,
                        'pointSize': 5,
                        'scaleDistribution': {'type': 'linear'},
                        'showPoints': 'never',
                        'spanNulls': False,
                        'stacking': {'group': 'A', 'mode': 'none'},
                        'thresholdsStyle': {'mode': 'line'}
                    },
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'red', 'value': 1000}
                        ]
                    },
                    'unit': 'ms'
                }
            },
            'gridPos': {'h': 8, 'w': 18},
            'id': 61,
            'options': {
                'legend': {'calcs': ['mean', 'max'], 'displayMode': 'table', 'placement': 'right'},
                'tooltip': {'mode': 'multi'}
            },
            'targets': [
                {
                    'expr': 'histogram_quantile(0.95, sum(rate(mysql_perf_schema_events_transactions_seconds_bucket{instance=~"${instance}"}[${interval}])) by (le)) * 1000',
                    'refId': 'A',
                    'legendFormat': 'Transaction Time (p95)'
                }
            ],
            'title': 'Business Transaction Performance',
            'type': 'timeseries'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'thresholds'},
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'yellow', 'value': 50},
                            {'color': 'red', 'value': 100}
                        ]
                    },
                    'unit': 'currencyUSD'
                }
            },
            'gridPos': {'h': 8, 'w': 8},
            'id': 62,
            'options': {
                'colorMode': 'value',
                'graphMode': 'area',
                'justifyMode': 'auto',
                'orientation': 'auto',
                'reduceOptions': {
                    'values': False,
                    'calcs': ['lastNotNull'],
                    'fields': ''
                },
                'textMode': 'auto'
            },
            'targets': [
                {
                    'expr': 'sum(increase(business_revenue_impact{instance=~"${instance}"}[${interval}]))',
                    'refId': 'A'
                }
            ],
            'title': 'Revenue Impact',
            'type': 'stat'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'thresholds'},
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'yellow', 'value': 100},
                            {'color': 'red', 'value': 500}
                        ]
                    },
                    'unit': 'short'
                }
            },
            'gridPos': {'h': 8, 'w': 8},
            'id': 63,
            'options': {
                'colorMode': 'value',
                'graphMode': 'area',
                'justifyMode': 'auto',
                'orientation': 'auto',
                'reduceOptions': {
                    'values': False,
                    'calcs': ['lastNotNull'],
                    'fields': ''
                },
                'textMode': 'auto'
            },
            'targets': [
                {
                    'expr': 'sum(increase(business_users_affected{instance=~"${instance}"}[${interval}]))',
                    'refId': 'A'
                }
            ],
            'title': 'Users Affected',
            'type': 'stat'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'axisCenteredZero': False,
                        'axisColorMode': 'text',
                        'axisLabel': '',
                        'axisPlacement': 'auto',
                        'barAlignment': 0,
                        'drawStyle': 'bars',
                        'fillOpacity': 100,
                        'gradientMode': 'none',
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False},
                        'lineInterpolation': 'linear',
                        'lineWidth': 1,
                        'pointSize': 5,
                        'scaleDistribution': {'type': 'linear'},
                        'showPoints': 'never',
                        'spanNulls': False,
                        'stacking': {'group': 'A', 'mode': 'normal'},
                        'thresholdsStyle': {'mode': 'off'}
                    },
                    'mappings': [],
                    'unit': 'short'
                }
            },
            'gridPos': {'h': 8, 'w': 8},
            'id': 64,
            'options': {
                'legend': {'calcs': [], 'displayMode': 'list', 'placement': 'bottom'},
                'tooltip': {'mode': 'multi'}
            },
            'targets': [
                {
                    'expr': 'sum by (severity) (business_incident_count{instance=~"${instance}"})',
                    'refId': 'A',
                    'legendFormat': '{{severity}}'
                }
            ],
            'title': 'Business Incidents by Severity',
            'type': 'timeseries'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'custom': {
                        'align': 'auto',
                        'displayMode': 'auto'
                    },
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [{'color': 'green', 'value': None}]
                    }
                }
            },
            'gridPos': {'h': 10, 'w': 24},
            'id': 65,
            'options': {
                'showHeader': True,
                'sortBy': [{'displayName': 'Impact Score', 'desc': True}]
            },
            'targets': [
                {
                    'expr': 'business_impact_analysis{instance=~"${instance}"}',
                    'format': 'table',
                    'refId': 'A'
                }
            ],
            'title': 'Business Impact Analysis',
            'type': 'table'
        }
    ]
    
    custom_variables = [
        {
            'name': 'business_unit',
            'label': 'Business Unit',
            'type': 'query',
            'query': 'label_values(business_impact_analysis, business_unit)',
            'multi': True,
            'includeAll': True,
            'current': {'text': 'All', 'value': '$__all'}
        }
    ]
    
    dashboard = generator.generate_dashboard(
        module_name='business-impact',
        title='Business Impact Dashboard',
        description='Monitor database performance impact on business metrics and KPIs',
        panels=panels,
        custom_variables=custom_variables,
        tags=['business', 'impact', 'revenue', 'sla']
    )
    
    return dashboard


def create_resource_monitor_dashboard():
    """Create dashboard for the resource-monitor module - System resource usage."""
    generator = DashboardGenerator()
    
    panels = [
        {'panel_ref': 'cpu_usage'},
        {'panel_ref': 'memory_usage'},
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'axisCenteredZero': False,
                        'axisColorMode': 'text',
                        'axisLabel': '',
                        'axisPlacement': 'auto',
                        'barAlignment': 0,
                        'drawStyle': 'line',
                        'fillOpacity': 10,
                        'gradientMode': 'none',
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False},
                        'lineInterpolation': 'linear',
                        'lineWidth': 1,
                        'pointSize': 5,
                        'scaleDistribution': {'type': 'linear'},
                        'showPoints': 'never',
                        'spanNulls': False,
                        'stacking': {'group': 'A', 'mode': 'none'},
                        'thresholdsStyle': {'mode': 'off'}
                    },
                    'mappings': [],
                    'unit': 'binBps'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 70,
            'options': {
                'legend': {'calcs': [], 'displayMode': 'list', 'placement': 'bottom'},
                'tooltip': {'mode': 'multi'}
            },
            'targets': [
                {
                    'expr': 'rate(node_disk_read_bytes_total{instance=~"${instance}"}[${interval}])',
                    'refId': 'A',
                    'legendFormat': 'Read {{device}}'
                },
                {
                    'expr': 'rate(node_disk_written_bytes_total{instance=~"${instance}"}[${interval}])',
                    'refId': 'B',
                    'legendFormat': 'Write {{device}}'
                }
            ],
            'title': 'Disk I/O',
            'type': 'timeseries'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'palette-classic'},
                    'custom': {
                        'axisCenteredZero': False,
                        'axisColorMode': 'text',
                        'axisLabel': '',
                        'axisPlacement': 'auto',
                        'barAlignment': 0,
                        'drawStyle': 'line',
                        'fillOpacity': 10,
                        'gradientMode': 'none',
                        'hideFrom': {'tooltip': False, 'viz': False, 'legend': False},
                        'lineInterpolation': 'linear',
                        'lineWidth': 1,
                        'pointSize': 5,
                        'scaleDistribution': {'type': 'linear'},
                        'showPoints': 'never',
                        'spanNulls': False,
                        'stacking': {'group': 'A', 'mode': 'none'},
                        'thresholdsStyle': {'mode': 'off'}
                    },
                    'mappings': [],
                    'unit': 'binBps'
                }
            },
            'gridPos': {'h': 8, 'w': 12},
            'id': 71,
            'options': {
                'legend': {'calcs': [], 'displayMode': 'list', 'placement': 'bottom'},
                'tooltip': {'mode': 'multi'}
            },
            'targets': [
                {
                    'expr': 'rate(node_network_receive_bytes_total{instance=~"${instance}"}[${interval}])',
                    'refId': 'A',
                    'legendFormat': 'Receive {{device}}'
                },
                {
                    'expr': 'rate(node_network_transmit_bytes_total{instance=~"${instance}"}[${interval}])',
                    'refId': 'B',
                    'legendFormat': 'Transmit {{device}}'
                }
            ],
            'title': 'Network I/O',
            'type': 'timeseries'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'thresholds'},
                    'mappings': [],
                    'thresholds': {
                        'mode': 'percentage',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'yellow', 'value': 70},
                            {'color': 'red', 'value': 90}
                        ]
                    },
                    'unit': 'percent'
                }
            },
            'gridPos': {'h': 8, 'w': 8},
            'id': 72,
            'options': {
                'orientation': 'auto',
                'reduceOptions': {
                    'values': False,
                    'calcs': ['lastNotNull'],
                    'fields': ''
                },
                'showThresholdLabels': False,
                'showThresholdMarkers': True
            },
            'targets': [
                {
                    'expr': '100 - ((node_filesystem_avail_bytes{mountpoint="/",instance=~"${instance}"} * 100) / node_filesystem_size_bytes{mountpoint="/",instance=~"${instance}"})',
                    'refId': 'A'
                }
            ],
            'title': 'Disk Usage',
            'type': 'gauge'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'thresholds'},
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'yellow', 'value': 100},
                            {'color': 'red', 'value': 500}
                        ]
                    },
                    'unit': 'short'
                }
            },
            'gridPos': {'h': 8, 'w': 8},
            'id': 73,
            'options': {
                'colorMode': 'value',
                'graphMode': 'area',
                'justifyMode': 'auto',
                'orientation': 'auto',
                'reduceOptions': {
                    'values': False,
                    'calcs': ['lastNotNull'],
                    'fields': ''
                },
                'textMode': 'auto'
            },
            'targets': [
                {
                    'expr': 'mysql_global_status_threads_connected{instance=~"${instance}"}',
                    'refId': 'A'
                }
            ],
            'title': 'Database Connections',
            'type': 'stat'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'color': {'mode': 'thresholds'},
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [
                            {'color': 'green', 'value': None},
                            {'color': 'yellow', 'value': 10},
                            {'color': 'red', 'value': 50}
                        ]
                    },
                    'unit': 'short'
                }
            },
            'gridPos': {'h': 8, 'w': 8},
            'id': 74,
            'options': {
                'colorMode': 'value',
                'graphMode': 'area',
                'justifyMode': 'auto',
                'orientation': 'auto',
                'reduceOptions': {
                    'values': False,
                    'calcs': ['lastNotNull'],
                    'fields': ''
                },
                'textMode': 'auto'
            },
            'targets': [
                {
                    'expr': 'mysql_global_status_threads_running{instance=~"${instance}"}',
                    'refId': 'A'
                }
            ],
            'title': 'Active Threads',
            'type': 'stat'
        },
        {
            'datasource': {'type': 'prometheus', 'uid': '${datasource}'},
            'fieldConfig': {
                'defaults': {
                    'custom': {
                        'align': 'auto',
                        'displayMode': 'auto'
                    },
                    'mappings': [],
                    'thresholds': {
                        'mode': 'absolute',
                        'steps': [{'color': 'green', 'value': None}]
                    },
                    'unit': 'percent'
                }
            },
            'gridPos': {'h': 10, 'w': 24},
            'id': 75,
            'options': {
                'showHeader': True,
                'sortBy': [{'displayName': 'Usage %', 'desc': True}]
            },
            'targets': [
                {
                    'expr': 'topk(10, (node_filesystem_size_bytes{instance=~"${instance}"} - node_filesystem_avail_bytes{instance=~"${instance}"}) / node_filesystem_size_bytes{instance=~"${instance}"} * 100)',
                    'format': 'table',
                    'refId': 'A'
                }
            ],
            'title': 'Filesystem Usage Details',
            'type': 'table'
        }
    ]
    
    custom_variables = [
        {
            'name': 'device',
            'label': 'Device',
            'type': 'query',
            'query': 'label_values(node_disk_io_time_seconds_total, device)',
            'multi': True,
            'includeAll': True,
            'current': {'text': 'All', 'value': '$__all'}
        }
    ]
    
    dashboard = generator.generate_dashboard(
        module_name='resource-monitor',
        title='Resource Monitor Dashboard',
        description='Monitor system resources including CPU, memory, disk, and network usage',
        panels=panels,
        custom_variables=custom_variables,
        tags=['resource', 'system', 'monitoring', 'infrastructure']
    )
    
    return dashboard


def main():
    """Main function to generate dashboards based on command line arguments."""
    parser = argparse.ArgumentParser(description='Generate module-specific Grafana dashboards')
    parser.add_argument('module', choices=['anomaly-detector', 'performance-tuner', 'query-optimizer', 
                                         'sql-intelligence', 'wait-profiler', 'business-impact', 
                                         'resource-monitor', 'all'],
                       help='Module to generate dashboard for')
    parser.add_argument('-o', '--output-dir', default='.',
                       help='Output directory for generated dashboards')
    parser.add_argument('-b', '--base-dashboard', default='base-dashboard.json',
                       help='Path to base dashboard template')
    
    args = parser.parse_args()
    
    # Create output directory if it doesn't exist
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # Generate dashboards
    generators = {
        'anomaly-detector': create_anomaly_detector_dashboard,
        'performance-tuner': create_performance_tuner_dashboard,
        'query-optimizer': create_query_optimizer_dashboard,
        'sql-intelligence': create_sql_intelligence_dashboard,
        'wait-profiler': create_wait_profiler_dashboard,
        'business-impact': create_business_impact_dashboard,
        'resource-monitor': create_resource_monitor_dashboard
    }
    
    if args.module == 'all':
        for module_name, generator_func in generators.items():
            dashboard = generator_func()
            output_path = output_dir / f'{module_name}-dashboard.json'
            gen = DashboardGenerator(args.base_dashboard)
            gen.save_dashboard(dashboard, str(output_path))
    else:
        dashboard = generators[args.module]()
        output_path = output_dir / f'{args.module}-dashboard.json'
        gen = DashboardGenerator(args.base_dashboard)
        gen.save_dashboard(dashboard, str(output_path))


if __name__ == '__main__':
    main()