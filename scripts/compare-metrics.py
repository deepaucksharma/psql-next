#!/usr/bin/env python3

import os
import json
import requests
from datetime import datetime, timedelta
from typing import Dict, List, Any, Optional
from dotenv import load_dotenv
import argparse
import pandas as pd
from tabulate import tabulate

load_dotenv()

class MetricsComparator:
    def __init__(self):
        self.account_id = os.getenv('NEW_RELIC_ACCOUNT_ID')
        self.api_key = os.getenv('NEW_RELIC_API_KEY')
        self.graphql_endpoint = os.getenv('NEW_RELIC_GRAPHQL_ENDPOINT', 'https://api.newrelic.com/graphql')
        
        if not self.account_id or not self.api_key:
            raise ValueError("NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY must be set in .env file")
        
        self.headers = {
            'Content-Type': 'application/json',
            'API-Key': self.api_key
        }
        
        # Define metric equivalencies
        self.metric_mappings = {
            'system': {
                'cpu': {
                    'event': {'type': 'SystemSample', 'attribute': 'cpuPercent'},
                    'otel': {'name': 'system.cpu.utilization', 'unit': '1'}
                },
                'memory': {
                    'event': {'type': 'SystemSample', 'attribute': 'memoryUsedPercent'},
                    'otel': {'name': 'system.memory.utilization', 'unit': '1'}
                },
                'disk': {
                    'event': {'type': 'SystemSample', 'attribute': 'diskUsedPercent'},
                    'otel': {'name': 'system.filesystem.utilization', 'unit': '1'}
                }
            },
            'process': {
                'cpu': {
                    'event': {'type': 'ProcessSample', 'attribute': 'cpuPercent'},
                    'otel': {'name': 'process.cpu.utilization', 'unit': '1'}
                },
                'memory': {
                    'event': {'type': 'ProcessSample', 'attribute': 'memoryResidentSizeBytes'},
                    'otel': {'name': 'process.memory.usage', 'unit': 'By'}
                }
            },
            'database': {
                'connections': {
                    'event': {'type': 'DatabaseSample', 'attribute': 'db.connectionCount'},
                    'otel': {'name': 'db.client.connections.usage', 'unit': '{connections}'}
                },
                'query_duration': {
                    'event': {'type': 'DatastoreSample', 'attribute': 'query.averageDuration'},
                    'otel': {'name': 'db.query.duration', 'unit': 'ms'}
                }
            }
        }
    
    def execute_nrql(self, query: str) -> Dict[str, Any]:
        """Execute NRQL query via GraphQL API"""
        graphql_query = {
            'query': f'''
            {{
                actor {{
                    account(id: {self.account_id}) {{
                        nrql(query: "{query}") {{
                            results
                            totalResult
                        }}
                    }}
                }}
            }}
            '''
        }
        
        response = requests.post(self.graphql_endpoint, headers=self.headers, json=graphql_query)
        response.raise_for_status()
        
        data = response.json()
        if 'errors' in data:
            raise Exception(f"GraphQL errors: {data['errors']}")
        
        return data['data']['actor']['account']['nrql']
    
    def get_event_value(self, event_type: str, attribute: str, time_range: str = '5 minutes') -> Optional[float]:
        """Get average value from event"""
        query = f"SELECT average({attribute}) FROM {event_type} SINCE {time_range} ago"
        try:
            result = self.execute_nrql(query)
            if result['results'] and len(result['results']) > 0:
                return result['results'][0].get(f'average.{attribute}')
        except Exception as e:
            print(f"Error querying {event_type}.{attribute}: {e}")
        return None
    
    def get_metric_value(self, metric_name: str, time_range: str = '5 minutes') -> Optional[float]:
        """Get average value from OTel metric"""
        query = f"SELECT average(value) FROM Metric WHERE metricName = '{metric_name}' SINCE {time_range} ago"
        try:
            result = self.execute_nrql(query)
            if result['results'] and len(result['results']) > 0:
                return result['results'][0].get('average.value')
        except Exception as e:
            print(f"Error querying metric {metric_name}: {e}")
        return None
    
    def compare_single_metric(self, category: str, metric: str, time_range: str = '5 minutes') -> Dict:
        """Compare a single metric between event and OTel"""
        mapping = self.metric_mappings.get(category, {}).get(metric)
        if not mapping:
            return {'error': f'No mapping found for {category}.{metric}'}
        
        event_info = mapping['event']
        otel_info = mapping['otel']
        
        event_value = self.get_event_value(event_info['type'], event_info['attribute'], time_range)
        otel_value = self.get_metric_value(otel_info['name'], time_range)
        
        result = {
            'category': category,
            'metric': metric,
            'event': {
                'type': event_info['type'],
                'attribute': event_info['attribute'],
                'value': event_value
            },
            'otel': {
                'name': otel_info['name'],
                'unit': otel_info['unit'],
                'value': otel_value
            },
            'difference': None,
            'difference_percent': None
        }
        
        if event_value is not None and otel_value is not None:
            result['difference'] = abs(event_value - otel_value)
            if event_value != 0:
                result['difference_percent'] = (result['difference'] / event_value) * 100
        
        return result
    
    def compare_all_metrics(self, time_range: str = '5 minutes') -> List[Dict]:
        """Compare all defined metrics"""
        results = []
        
        for category, metrics in self.metric_mappings.items():
            for metric_name in metrics:
                result = self.compare_single_metric(category, metric_name, time_range)
                results.append(result)
        
        return results
    
    def generate_comparison_report(self, time_range: str = '5 minutes', output_format: str = 'table'):
        """Generate comprehensive comparison report"""
        print(f"Comparing metrics over the last {time_range}...\n")
        
        comparisons = self.compare_all_metrics(time_range)
        
        if output_format == 'table':
            self.print_table_report(comparisons)
        elif output_format == 'json':
            print(json.dumps(comparisons, indent=2))
        elif output_format == 'csv':
            self.save_csv_report(comparisons)
        
        return comparisons
    
    def print_table_report(self, comparisons: List[Dict]):
        """Print comparison results as table"""
        table_data = []
        
        for comp in comparisons:
            if 'error' in comp:
                continue
            
            event_val = comp['event']['value']
            otel_val = comp['otel']['value']
            diff_pct = comp['difference_percent']
            
            event_str = f"{event_val:.2f}" if event_val is not None else "N/A"
            otel_str = f"{otel_val:.2f}" if otel_val is not None else "N/A"
            diff_str = f"{diff_pct:.1f}%" if diff_pct is not None else "N/A"
            
            status = "✓" if diff_pct is not None and diff_pct < 5 else "⚠"
            
            table_data.append([
                f"{comp['category']}.{comp['metric']}",
                comp['event']['type'],
                comp['event']['attribute'],
                event_str,
                comp['otel']['name'],
                otel_str,
                diff_str,
                status
            ])
        
        headers = ['Metric', 'Event Type', 'Event Attr', 'Event Val', 'OTel Metric', 'OTel Val', 'Diff %', 'Status']
        print(tabulate(table_data, headers=headers, tablefmt='grid'))
        
        # Print summary
        print("\nSummary:")
        matching = sum(1 for row in table_data if row[7] == "✓")
        total = len(table_data)
        print(f"  Matching metrics (< 5% difference): {matching}/{total}")
        print(f"  Metrics with discrepancies: {total - matching}/{total}")
    
    def save_csv_report(self, comparisons: List[Dict], filename: str = 'metric-comparison.csv'):
        """Save comparison results as CSV"""
        data = []
        
        for comp in comparisons:
            if 'error' in comp:
                continue
            
            data.append({
                'Category': comp['category'],
                'Metric': comp['metric'],
                'Event Type': comp['event']['type'],
                'Event Attribute': comp['event']['attribute'],
                'Event Value': comp['event']['value'],
                'OTel Metric': comp['otel']['name'],
                'OTel Unit': comp['otel']['unit'],
                'OTel Value': comp['otel']['value'],
                'Difference': comp['difference'],
                'Difference %': comp['difference_percent']
            })
        
        df = pd.DataFrame(data)
        df.to_csv(filename, index=False)
        print(f"CSV report saved to {filename}")
    
    def validate_metric_availability(self) -> Dict[str, List[str]]:
        """Check which metrics are actually available"""
        available = {'events': [], 'otel_metrics': []}
        
        # Check events
        for category, metrics in self.metric_mappings.items():
            for metric_name, mapping in metrics.items():
                event_type = mapping['event']['type']
                if event_type not in available['events']:
                    query = f"SELECT count(*) FROM {event_type} SINCE 1 hour ago LIMIT 1"
                    try:
                        result = self.execute_nrql(query)
                        if result['results'] and result['results'][0].get('count', 0) > 0:
                            available['events'].append(event_type)
                    except:
                        pass
        
        # Check OTel metrics
        for category, metrics in self.metric_mappings.items():
            for metric_name, mapping in metrics.items():
                otel_name = mapping['otel']['name']
                query = f"SELECT count(*) FROM Metric WHERE metricName = '{otel_name}' SINCE 1 hour ago LIMIT 1"
                try:
                    result = self.execute_nrql(query)
                    if result['results'] and result['results'][0].get('count', 0) > 0:
                        available['otel_metrics'].append(otel_name)
                except:
                    pass
        
        return available

def main():
    parser = argparse.ArgumentParser(description='Compare New Relic events with OTel metrics')
    parser.add_argument('--time-range', '-t', default='5 minutes', help='Time range for comparison (e.g., "5 minutes", "1 hour")')
    parser.add_argument('--format', '-f', choices=['table', 'json', 'csv'], default='table', help='Output format')
    parser.add_argument('--category', '-c', help='Compare specific category (system, process, database)')
    parser.add_argument('--metric', '-m', help='Compare specific metric within category')
    parser.add_argument('--validate', action='store_true', help='Validate metric availability')
    
    args = parser.parse_args()
    
    try:
        comparator = MetricsComparator()
        
        if args.validate:
            available = comparator.validate_metric_availability()
            print("Available data sources:")
            print(f"  Events: {', '.join(available['events'])}")
            print(f"  OTel Metrics: {', '.join(available['otel_metrics'])}")
        
        elif args.category and args.metric:
            result = comparator.compare_single_metric(args.category, args.metric, args.time_range)
            print(json.dumps(result, indent=2))
        
        else:
            comparator.generate_comparison_report(args.time_range, args.format)
            
    except Exception as e:
        print(f"Error: {e}")
        return 1
    
    return 0

if __name__ == '__main__':
    exit(main())