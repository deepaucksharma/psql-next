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


def main():
    """Main function to generate dashboards based on command line arguments."""
    parser = argparse.ArgumentParser(description='Generate module-specific Grafana dashboards')
    parser.add_argument('module', choices=['anomaly-detector', 'performance-tuner', 'query-optimizer', 'all'],
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
        'query-optimizer': create_query_optimizer_dashboard
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