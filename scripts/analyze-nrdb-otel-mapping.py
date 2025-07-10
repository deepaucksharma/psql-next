#!/usr/bin/env python3

import os
import json
import requests
from datetime import datetime, timedelta
from typing import Dict, List, Any
from dotenv import load_dotenv
import argparse
import pandas as pd
from collections import defaultdict

load_dotenv()

class NRDBAnalyzer:
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
                            metadata {{
                                eventTypes
                                facets
                                messages
                                timeWindow {{
                                    begin
                                    end
                                }}
                            }}
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
    
    def get_event_samples(self, event_type: str, limit: int = 100) -> List[Dict]:
        """Get sample events from NRDB"""
        query = f"SELECT * FROM {event_type} SINCE 1 hour ago LIMIT {limit}"
        result = self.execute_nrql(query)
        return result['results']
    
    def get_otel_metrics(self, limit: int = 100) -> List[Dict]:
        """Get OTel metrics from NRDB"""
        query = f"SELECT * FROM Metric WHERE otel.library.name IS NOT NULL SINCE 1 hour ago LIMIT {limit}"
        result = self.execute_nrql(query)
        return result['results']
    
    def get_metric_names(self) -> List[str]:
        """Get unique metric names"""
        query = "SELECT uniques(metricName) FROM Metric SINCE 1 hour ago LIMIT 1000"
        result = self.execute_nrql(query)
        if result['results'] and 'uniques.metricName' in result['results'][0]:
            return result['results'][0]['uniques.metricName']
        return []
    
    def get_event_types(self) -> List[str]:
        """Get available event types"""
        query = "SHOW EVENT TYPES SINCE 1 hour ago"
        result = self.execute_nrql(query)
        return [r['eventType'] for r in result['results'] if 'eventType' in r]
    
    def analyze_metric_mapping(self, metric_name: str) -> Dict:
        """Analyze how a specific OTel metric maps to New Relic"""
        query = f"SELECT * FROM Metric WHERE metricName = '{metric_name}' SINCE 1 hour ago LIMIT 10"
        result = self.execute_nrql(query)
        
        if not result['results']:
            return {'error': f'No data found for metric {metric_name}'}
        
        sample = result['results'][0]
        mapping = {
            'metric_name': metric_name,
            'new_relic_attributes': {},
            'otel_attributes': {},
            'metric_type': sample.get('type', 'unknown'),
            'unit': sample.get('unit', 'none')
        }
        
        for key, value in sample.items():
            if key.startswith('otel.'):
                mapping['otel_attributes'][key] = value
            elif key not in ['timestamp', 'metricName', 'type', 'unit']:
                mapping['new_relic_attributes'][key] = value
        
        return mapping
    
    def compare_events_to_metrics(self) -> Dict:
        """Compare traditional New Relic events to OTel metrics"""
        comparison = {
            'timestamp': datetime.now().isoformat(),
            'event_types': {},
            'otel_metrics': {},
            'mappings': []
        }
        
        # Get event types and samples
        event_types = self.get_event_types()
        for event_type in event_types[:10]:  # Limit to first 10 for analysis
            try:
                samples = self.get_event_samples(event_type, 5)
                if samples:
                    comparison['event_types'][event_type] = {
                        'sample_count': len(samples),
                        'attributes': list(samples[0].keys()) if samples else []
                    }
            except Exception as e:
                print(f"Error getting samples for {event_type}: {e}")
        
        # Get OTel metrics
        metric_names = self.get_metric_names()
        for metric_name in metric_names[:20]:  # Limit to first 20 for analysis
            try:
                mapping = self.analyze_metric_mapping(metric_name)
                comparison['otel_metrics'][metric_name] = mapping
                
                # Try to find equivalent event type
                if 'database' in metric_name.lower():
                    comparison['mappings'].append({
                        'otel_metric': metric_name,
                        'possible_event_types': ['DatabaseSample', 'DatastoreSample'],
                        'confidence': 'high' if 'query' in metric_name.lower() else 'medium'
                    })
            except Exception as e:
                print(f"Error analyzing metric {metric_name}: {e}")
        
        return comparison
    
    def generate_mapping_report(self, output_file: str = 'nrdb-otel-mapping.json'):
        """Generate comprehensive mapping report"""
        print("Analyzing NRDB event samples and OTel metrics...")
        
        comparison = self.compare_events_to_metrics()
        
        # Add common mappings based on New Relic conventions
        common_mappings = {
            'SystemSample': {
                'otel_equivalents': [
                    'system.cpu.utilization',
                    'system.memory.usage',
                    'system.disk.io',
                    'system.network.io'
                ],
                'attributes_mapping': {
                    'cpuPercent': 'system.cpu.utilization',
                    'memoryUsedPercent': 'system.memory.utilization',
                    'diskUsedPercent': 'system.filesystem.utilization'
                }
            },
            'ProcessSample': {
                'otel_equivalents': [
                    'process.cpu.utilization',
                    'process.memory.usage',
                    'process.threads'
                ],
                'attributes_mapping': {
                    'cpuPercent': 'process.cpu.utilization',
                    'memoryResidentSizeBytes': 'process.memory.usage'
                }
            },
            'DatabaseSample': {
                'otel_equivalents': [
                    'db.client.connections.usage',
                    'db.client.connections.max',
                    'db.query.duration'
                ],
                'attributes_mapping': {
                    'db.connectionCount': 'db.client.connections.usage',
                    'db.queryDuration': 'db.query.duration'
                }
            }
        }
        
        comparison['common_mappings'] = common_mappings
        
        # Save report
        with open(output_file, 'w') as f:
            json.dump(comparison, f, indent=2)
        
        print(f"Mapping report saved to {output_file}")
        
        # Generate summary
        self.print_summary(comparison)
    
    def print_summary(self, comparison: Dict):
        """Print analysis summary"""
        print("\n=== NRDB Event to OTel Metrics Mapping Summary ===\n")
        
        print(f"Event Types Found: {len(comparison['event_types'])}")
        for event_type, info in list(comparison['event_types'].items())[:5]:
            print(f"  - {event_type}: {info['sample_count']} samples, {len(info['attributes'])} attributes")
        
        print(f"\nOTel Metrics Found: {len(comparison['otel_metrics'])}")
        for metric_name, info in list(comparison['otel_metrics'].items())[:5]:
            if 'error' not in info:
                print(f"  - {metric_name}: {info['metric_type']} type")
        
        print("\nKey Mappings:")
        for mapping in comparison['mappings'][:5]:
            print(f"  - {mapping['otel_metric']} → {', '.join(mapping['possible_event_types'])} ({mapping['confidence']} confidence)")
        
        print("\nCommon Event to Metric Mappings:")
        for event_type, mapping in comparison['common_mappings'].items():
            print(f"  - {event_type}:")
            for nr_attr, otel_metric in list(mapping['attributes_mapping'].items())[:3]:
                print(f"    • {nr_attr} → {otel_metric}")

def main():
    parser = argparse.ArgumentParser(description='Analyze NRDB events and OTel metrics mapping')
    parser.add_argument('--output', '-o', default='nrdb-otel-mapping.json', help='Output file for mapping report')
    parser.add_argument('--event-type', help='Analyze specific event type')
    parser.add_argument('--metric-name', help='Analyze specific metric name')
    
    args = parser.parse_args()
    
    try:
        analyzer = NRDBAnalyzer()
        
        if args.event_type:
            samples = analyzer.get_event_samples(args.event_type)
            print(f"\nSamples for {args.event_type}:")
            print(json.dumps(samples[:2], indent=2))
        
        elif args.metric_name:
            mapping = analyzer.analyze_metric_mapping(args.metric_name)
            print(f"\nMapping for {args.metric_name}:")
            print(json.dumps(mapping, indent=2))
        
        else:
            analyzer.generate_mapping_report(args.output)
            
    except Exception as e:
        print(f"Error: {e}")
        return 1
    
    return 0

if __name__ == '__main__':
    exit(main())