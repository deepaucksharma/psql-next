#!/usr/bin/env python3
"""
Anomaly Detector Module Specific Validation Tool

This tool provides deep validation for the anomaly-detector module (port 8084).
It validates anomaly detection algorithms, baseline tracking, and alert generation.

Usage:
    python3 validate-anomaly-detector.py --api-key YOUR_API_KEY --account-id YOUR_ACCOUNT_ID
    python3 validate-anomaly-detector.py --check-baselines --check-anomalies --check-federation
"""

import argparse
import json
import requests
import sys
import time
import os
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Tuple
import logging
import statistics
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class AnomalyDetectorValidator:
    def __init__(self, api_key: str, account_id: str):
        self.api_key = api_key
        self.account_id = account_id
        self.module_name = "anomaly-detector"
        self.module_port = 8084
        self.session = requests.Session()
        self.session.headers.update({
            'Api-Key': api_key,
            'Content-Type': 'application/json'
        })
        
        # Anomaly Detector expected metrics
        self.expected_metrics = {
            'anomaly_scores': [
                'anomaly_score_cpu',
                'anomaly_score_memory',
                'anomaly_score_connections',
                'anomaly_score_query_latency',
                'anomaly_score_buffer_pool'
            ],
            'detection_results': [
                'anomaly_detected',
                'anomaly_severity',
                'anomaly_confidence'
            ],
            'baseline_metrics': [
                'baseline_mean_cpu',
                'baseline_stddev_cpu',
                'baseline_mean_memory',
                'baseline_stddev_memory',
                'baseline_sample_count'
            ],
            'federated_data': [
                'mysql_connections_current',
                'mysql_threads_running',
                'mysql_buffer_pool_pages_dirty',
                'mysql.query.latency_avg_ms'
            ]
        }
        
        # Validation thresholds
        self.thresholds = {
            'anomaly_score_min': 0.0,
            'anomaly_score_max': 1.0,
            'anomaly_score_critical': 0.8,
            'anomaly_score_warning': 0.6,
            'baseline_min_samples': 50,
            'baseline_staleness_hours': 24,
            'detection_sensitivity': 0.05,  # 5% detection rate expected
            'confidence_threshold': 0.7,
            'federation_data_freshness_minutes': 5
        }
        
        # Dependencies for federation
        self.federation_sources = [
            'core-metrics',
            'sql-intelligence', 
            'wait-profiler'
        ]

    def execute_nrql(self, query: str) -> Optional[List[Dict]]:
        """Execute NRQL query against New Relic"""
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
        
        try:
            response = self.session.post(
                "https://api.newrelic.com/graphql",
                json=graphql_query,
                timeout=30
            )
            response.raise_for_status()
            
            data = response.json()
            if 'errors' in data:
                logger.error(f"GraphQL errors: {data['errors']}")
                return None
                
            return data['data']['actor']['account']['nrql']['results']
            
        except Exception as e:
            logger.error(f"Failed to execute NRQL: {e}")
            return None

    def validate_baseline_tracking(self) -> Dict:
        """Validate baseline calculation and tracking"""
        logger.info("Validating baseline tracking...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check baseline data presence
        baseline_query = """
        SELECT latest(baseline_mean_cpu) as 'cpu_baseline_mean',
               latest(baseline_stddev_cpu) as 'cpu_baseline_stddev',
               latest(baseline_mean_memory) as 'memory_baseline_mean',
               latest(baseline_stddev_memory) as 'memory_baseline_stddev',
               latest(baseline_sample_count) as 'sample_count',
               latest(timestamp) as 'last_update'
        FROM Metric 
        WHERE service.name = 'anomaly-detector'
          AND metricName IN ('baseline_mean_cpu', 'baseline_stddev_cpu', 'baseline_mean_memory', 'baseline_stddev_memory', 'baseline_sample_count')
        SINCE 24 hours ago
        """
        
        data = self.execute_nrql(baseline_query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No baseline tracking data found")
            results['recommendations'].append("Verify baseline calculation processor is enabled")
            return results
        
        metrics = data[0]
        cpu_mean = metrics.get('cpu_baseline_mean')
        cpu_stddev = metrics.get('cpu_baseline_stddev') 
        memory_mean = metrics.get('memory_baseline_mean')
        memory_stddev = metrics.get('memory_baseline_stddev')
        sample_count = metrics.get('sample_count', 0)
        last_update = metrics.get('last_update')
        
        results['metrics'] = {
            'cpu_baseline_mean': cpu_mean,
            'cpu_baseline_stddev': cpu_stddev,
            'memory_baseline_mean': memory_mean,
            'memory_baseline_stddev': memory_stddev,
            'baseline_sample_count': sample_count,
            'last_baseline_update': last_update
        }
        
        # Validate baseline completeness
        if cpu_mean is None or cpu_stddev is None:
            results['status'] = 'FAIL'
            results['issues'].append("CPU baseline data missing")
            results['recommendations'].append("Check CPU metrics collection and baseline processor")
        
        if memory_mean is None or memory_stddev is None:
            results['status'] = 'FAIL' if results['status'] != 'FAIL' else results['status']
            results['issues'].append("Memory baseline data missing") 
            results['recommendations'].append("Check memory metrics collection and baseline processor")
        
        # Validate sample count
        if sample_count < self.thresholds['baseline_min_samples']:
            results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
            results['issues'].append(f"Insufficient baseline samples: {sample_count} < {self.thresholds['baseline_min_samples']}")
            results['recommendations'].append("Allow more time for baseline calculation or reduce baseline window")
        
        # Check baseline freshness
        if last_update:
            try:
                last_update_time = datetime.fromisoformat(last_update.replace('Z', '+00:00'))
                hours_since_update = (datetime.now().astimezone() - last_update_time).total_seconds() / 3600
                results['metrics']['hours_since_baseline_update'] = hours_since_update
                
                if hours_since_update > self.thresholds['baseline_staleness_hours']:
                    results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                    results['issues'].append(f"Baseline data is stale: {hours_since_update:.1f} hours old")
                    results['recommendations'].append("Check baseline update frequency and data ingestion")
            except Exception as e:
                logger.warning(f"Could not parse baseline timestamp: {e}")
        
        return results

    def validate_anomaly_scores(self) -> Dict:
        """Validate anomaly score calculation and ranges"""
        logger.info("Validating anomaly scores...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check anomaly scores
        scores_query = """
        SELECT latest(anomaly_score_cpu) as 'cpu_score',
               latest(anomaly_score_memory) as 'memory_score',
               latest(anomaly_score_connections) as 'connections_score',
               latest(anomaly_score_query_latency) as 'latency_score',
               average(anomaly_score_cpu) as 'avg_cpu_score',
               max(anomaly_score_cpu) as 'max_cpu_score',
               percentile(anomaly_score_cpu, 95) as 'p95_cpu_score'
        FROM Metric 
        WHERE service.name = 'anomaly-detector'
          AND metricName IN ('anomaly_score_cpu', 'anomaly_score_memory', 'anomaly_score_connections', 'anomaly_score_query_latency')
        SINCE 30 minutes ago
        """
        
        data = self.execute_nrql(scores_query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No anomaly scores found")
            results['recommendations'].append("Verify anomaly detection processor is running")
            return results
        
        metrics = data[0]
        cpu_score = metrics.get('cpu_score')
        memory_score = metrics.get('memory_score')
        connections_score = metrics.get('connections_score')
        latency_score = metrics.get('latency_score')
        avg_cpu_score = metrics.get('avg_cpu_score', 0)
        max_cpu_score = metrics.get('max_cpu_score', 0)
        p95_cpu_score = metrics.get('p95_cpu_score', 0)
        
        results['metrics'] = {
            'latest_cpu_score': cpu_score,
            'latest_memory_score': memory_score,
            'latest_connections_score': connections_score,
            'latest_latency_score': latency_score,
            'avg_cpu_score': avg_cpu_score,
            'max_cpu_score': max_cpu_score,
            'p95_cpu_score': p95_cpu_score
        }
        
        # Validate score ranges
        scores_to_check = [
            ('CPU', cpu_score),
            ('Memory', memory_score), 
            ('Connections', connections_score),
            ('Latency', latency_score)
        ]
        
        for score_name, score_value in scores_to_check:
            if score_value is not None:
                if score_value < self.thresholds['anomaly_score_min'] or score_value > self.thresholds['anomaly_score_max']:
                    results['status'] = 'FAIL'
                    results['issues'].append(f"{score_name} anomaly score out of range: {score_value}")
                    results['recommendations'].append("Check anomaly score calculation algorithm")
                elif score_value >= self.thresholds['anomaly_score_critical']:
                    results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                    results['issues'].append(f"Critical {score_name} anomaly detected: score {score_value:.2f}")
                    results['recommendations'].append(f"Investigate {score_name} anomaly immediately")
        
        # Check score distribution
        if max_cpu_score > 0:
            score_variance = max_cpu_score - avg_cpu_score
            results['metrics']['cpu_score_variance'] = score_variance
            
            if score_variance < 0.1:  # Very low variance
                results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                results['issues'].append("Low anomaly score variance - detection may be insensitive")
                results['recommendations'].append("Review anomaly detection sensitivity settings")
        
        return results

    def validate_anomaly_detection(self) -> Dict:
        """Validate anomaly detection and alerting"""
        logger.info("Validating anomaly detection...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check detection events
        detection_query = """
        SELECT count(*) as 'total_detections',
               count(*) filter(WHERE anomaly_severity = 'critical') as 'critical_detections',
               count(*) filter(WHERE anomaly_severity = 'warning') as 'warning_detections',
               average(anomaly_confidence) as 'avg_confidence',
               latest(anomaly_detected) as 'latest_detection'
        FROM Metric 
        WHERE service.name = 'anomaly-detector'
          AND metricName IN ('anomaly_detected', 'anomaly_severity', 'anomaly_confidence')
          AND anomaly_detected = 1
        SINCE 1 hour ago
        """
        
        data = self.execute_nrql(detection_query)
        if not data or len(data) == 0:
            # Check if any detection metrics exist at all
            check_query = """
            SELECT count(*) as 'detection_metric_count'
            FROM Metric 
            WHERE service.name = 'anomaly-detector'
              AND metricName = 'anomaly_detected'
            SINCE 1 hour ago
            """
            check_data = self.execute_nrql(check_query)
            
            if check_data and check_data[0].get('detection_metric_count', 0) > 0:
                results['status'] = 'PASS'
                results['metrics']['total_detections'] = 0
                results['metrics']['status'] = 'No anomalies detected (normal)'
            else:
                results['status'] = 'FAIL'
                results['issues'].append("No anomaly detection metrics found")
                results['recommendations'].append("Verify anomaly detection processor is enabled")
            return results
        
        metrics = data[0]
        total_detections = metrics.get('total_detections', 0)
        critical_detections = metrics.get('critical_detections', 0)
        warning_detections = metrics.get('warning_detections', 0)
        avg_confidence = metrics.get('avg_confidence', 0)
        latest_detection = metrics.get('latest_detection')
        
        results['metrics'] = {
            'total_detections': total_detections,
            'critical_detections': critical_detections,
            'warning_detections': warning_detections,
            'avg_detection_confidence': avg_confidence,
            'latest_detection_time': latest_detection
        }
        
        # Validate confidence levels
        if avg_confidence < self.thresholds['confidence_threshold']:
            results['status'] = 'WARN'
            results['issues'].append(f"Low average detection confidence: {avg_confidence:.2f}")
            results['recommendations'].append("Review detection algorithm parameters")
        
        # Check detection rate reasonableness
        total_measurements_query = """
        SELECT count(*) as 'total_measurements'
        FROM Metric 
        WHERE service.name = 'anomaly-detector'
          AND metricName = 'anomaly_score_cpu'
        SINCE 1 hour ago
        """
        
        measurements_data = self.execute_nrql(total_measurements_query)
        if measurements_data and len(measurements_data) > 0:
            total_measurements = measurements_data[0].get('total_measurements', 0)
            if total_measurements > 0:
                detection_rate = total_detections / total_measurements
                results['metrics']['detection_rate'] = detection_rate
                
                if detection_rate > 0.5:  # More than 50% detections
                    results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                    results['issues'].append(f"Very high detection rate: {detection_rate:.1%}")
                    results['recommendations'].append("Review detection sensitivity - may be too aggressive")
                elif detection_rate < 0.001 and total_measurements > 100:  # Less than 0.1% with sufficient data
                    results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                    results['issues'].append(f"Very low detection rate: {detection_rate:.2%}")
                    results['recommendations'].append("Review detection sensitivity - may be too conservative")
        
        # Check recent critical detections
        if critical_detections > 0:
            results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
            results['issues'].append(f"{critical_detections} critical anomalies detected in last hour")
            results['recommendations'].append("Investigate critical anomalies immediately")
        
        return results

    def validate_federation_data(self) -> Dict:
        """Validate federated data from other modules"""
        logger.info("Validating federation data...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check federated metrics presence
        federation_query = """
        SELECT count(*) as 'federated_metrics_count'
        FROM Metric 
        WHERE service.name = 'anomaly-detector'
          AND metricName IN ('mysql_connections_current', 'mysql_threads_running', 'mysql.query.latency_avg_ms')
          AND federated_from IS NOT NULL
        SINCE 30 minutes ago
        """
        
        data = self.execute_nrql(federation_query)
        if not data or len(data) == 0 or data[0].get('federated_metrics_count', 0) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No federated data found")
            results['recommendations'].append("Check Prometheus federation configuration and source module availability")
            return results
        
        federated_count = data[0]['federated_metrics_count']
        results['metrics']['federated_metrics_count'] = federated_count
        
        # Check data freshness from each source
        for source in self.federation_sources:
            freshness_query = f"""
            SELECT (now() - latest(timestamp))/1000/60 as 'minutes_since_last_data'
            FROM Metric 
            WHERE service.name = 'anomaly-detector'
              AND federated_from = '{source}'
            SINCE 1 hour ago
            """
            
            freshness_data = self.execute_nrql(freshness_query)
            if freshness_data and len(freshness_data) > 0:
                minutes_old = freshness_data[0].get('minutes_since_last_data', float('inf'))
                results['metrics'][f'{source}_data_age_minutes'] = minutes_old
                
                if minutes_old > self.thresholds['federation_data_freshness_minutes']:
                    results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                    results['issues'].append(f"Stale federated data from {source}: {minutes_old:.1f} minutes old")
                    results['recommendations'].append(f"Check {source} module health and federation connectivity")
            else:
                results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                results['issues'].append(f"No federated data from {source}")
                results['recommendations'].append(f"Verify {source} module is running and federation is configured")
        
        # Check federation data quality
        quality_query = """
        SELECT count(DISTINCT federated_from) as 'unique_sources',
               sum(case when federated_from = 'core-metrics' then 1 else 0 end) as 'core_metrics_count',
               sum(case when federated_from = 'sql-intelligence' then 1 else 0 end) as 'sql_intelligence_count'
        FROM Metric 
        WHERE service.name = 'anomaly-detector'
          AND federated_from IS NOT NULL
        SINCE 30 minutes ago
        """
        
        quality_data = self.execute_nrql(quality_query)
        if quality_data and len(quality_data) > 0:
            unique_sources = quality_data[0].get('unique_sources', 0)
            core_metrics_count = quality_data[0].get('core_metrics_count', 0)
            sql_intelligence_count = quality_data[0].get('sql_intelligence_count', 0)
            
            results['metrics']['unique_federation_sources'] = unique_sources
            results['metrics']['core_metrics_federated_count'] = core_metrics_count
            results['metrics']['sql_intelligence_federated_count'] = sql_intelligence_count
            
            expected_sources = len(self.federation_sources)
            if unique_sources < expected_sources:
                results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                results['issues'].append(f"Only {unique_sources}/{expected_sources} federation sources active")
                results['recommendations'].append("Check all dependent modules are running and accessible")
        
        return results

    def validate_data_pipeline(self) -> Dict:
        """Validate the complete anomaly detection data pipeline"""
        logger.info("Validating data pipeline...")
        
        results = {
            'status': 'PASS',
            'issues': [],
            'recommendations': [],
            'metrics': {}
        }
        
        # Check pipeline completeness: Raw Data -> Baseline -> Scores -> Detection
        pipeline_query = """
        SELECT count(*) filter(WHERE metricName LIKE 'mysql_%') as 'raw_data_points',
               count(*) filter(WHERE metricName LIKE 'baseline_%') as 'baseline_points',
               count(*) filter(WHERE metricName LIKE 'anomaly_score_%') as 'score_points',
               count(*) filter(WHERE metricName = 'anomaly_detected') as 'detection_points'
        FROM Metric 
        WHERE service.name = 'anomaly-detector'
        SINCE 30 minutes ago
        """
        
        data = self.execute_nrql(pipeline_query)
        if not data or len(data) == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No pipeline data found")
            return results
        
        metrics = data[0]
        raw_data = metrics.get('raw_data_points', 0)
        baseline_data = metrics.get('baseline_points', 0)
        score_data = metrics.get('score_points', 0)
        detection_data = metrics.get('detection_points', 0)
        
        results['metrics'] = {
            'raw_data_points': raw_data,
            'baseline_data_points': baseline_data,
            'score_data_points': score_data,
            'detection_data_points': detection_data
        }
        
        # Validate pipeline flow
        if raw_data == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No raw data in pipeline")
            results['recommendations'].append("Check federation and data ingestion")
        elif baseline_data == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No baseline calculation")
            results['recommendations'].append("Check baseline calculation processor")
        elif score_data == 0:
            results['status'] = 'FAIL'
            results['issues'].append("No anomaly scores generated")
            results['recommendations'].append("Check anomaly scoring processor")
        elif detection_data == 0:
            results['status'] = 'WARN'
            results['issues'].append("No detection events generated")
            results['recommendations'].append("Verify detection thresholds are appropriate")
        
        # Check data flow ratios
        if raw_data > 0 and score_data > 0:
            processing_ratio = score_data / raw_data
            results['metrics']['data_processing_ratio'] = processing_ratio
            
            if processing_ratio < 0.5:  # Less than 50% of raw data processed
                results['status'] = 'WARN' if results['status'] == 'PASS' else results['status']
                results['issues'].append(f"Low data processing ratio: {processing_ratio:.1%}")
                results['recommendations'].append("Check for processing bottlenecks or errors")
        
        return results

    def run_comprehensive_validation(self) -> Dict:
        """Run all anomaly detector validations"""
        logger.info("Starting comprehensive anomaly-detector validation...")
        
        validation_results = {
            'module': self.module_name,
            'timestamp': datetime.now().isoformat(),
            'overall_status': 'PASS',
            'validations': {},
            'summary': {
                'total_checks': 0,
                'passed': 0,
                'warnings': 0,
                'failures': 0
            },
            'recommendations': []
        }
        
        # Run all validations
        validations = {
            'baseline_tracking': self.validate_baseline_tracking,
            'anomaly_scores': self.validate_anomaly_scores,
            'anomaly_detection': self.validate_anomaly_detection,
            'federation_data': self.validate_federation_data,
            'data_pipeline': self.validate_data_pipeline
        }
        
        for validation_name, validation_func in validations.items():
            try:
                result = validation_func()
                validation_results['validations'][validation_name] = result
                validation_results['summary']['total_checks'] += 1
                
                if result['status'] == 'PASS':
                    validation_results['summary']['passed'] += 1
                elif result['status'] == 'WARN':
                    validation_results['summary']['warnings'] += 1
                    validation_results['overall_status'] = 'WARN' if validation_results['overall_status'] == 'PASS' else validation_results['overall_status']
                else:
                    validation_results['summary']['failures'] += 1
                    validation_results['overall_status'] = 'FAIL'
                
                validation_results['recommendations'].extend(result['recommendations'])
                
            except Exception as e:
                logger.error(f"Validation {validation_name} failed: {e}")
                validation_results['validations'][validation_name] = {
                    'status': 'FAIL',
                    'error': str(e)
                }
                validation_results['summary']['failures'] += 1
                validation_results['overall_status'] = 'FAIL'
        
        # Remove duplicate recommendations
        validation_results['recommendations'] = list(set(validation_results['recommendations']))
        
        return validation_results

def main():
    parser = argparse.ArgumentParser(description='Anomaly Detector Module Validation')
    parser.add_argument('--api-key', help='New Relic API Key (overrides .env)')
    parser.add_argument('--account-id', help='New Relic Account ID (overrides .env)')
    parser.add_argument('--check-baselines', action='store_true', help='Check baseline tracking only')
    parser.add_argument('--check-anomalies', action='store_true', help='Check anomaly detection only')
    parser.add_argument('--check-federation', action='store_true', help='Check federation data only')
    parser.add_argument('--check-pipeline', action='store_true', help='Check data pipeline only')
    parser.add_argument('--output', help='Output file for results (JSON format)')
    parser.add_argument('--verbose', action='store_true', help='Verbose output')
    
    args = parser.parse_args()
    
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    
    # Get API credentials from arguments or environment
    api_key = args.api_key or os.getenv('NEW_RELIC_API_KEY')
    account_id = args.account_id or os.getenv('NEW_RELIC_ACCOUNT_ID')
    
    if not api_key:
        print("Error: NEW_RELIC_API_KEY not found in environment or arguments")
        sys.exit(1)
    if not account_id:
        print("Error: NEW_RELIC_ACCOUNT_ID not found in environment or arguments")
        sys.exit(1)
    
    validator = AnomalyDetectorValidator(api_key, account_id)
    
    # Run specific validations if requested
    if any([args.check_baselines, args.check_anomalies, args.check_federation, args.check_pipeline]):
        results = {'validations': {}}
        if args.check_baselines:
            results['validations']['baseline_tracking'] = validator.validate_baseline_tracking()
        if args.check_anomalies:
            results['validations']['anomaly_detection'] = validator.validate_anomaly_detection()
        if args.check_federation:
            results['validations']['federation_data'] = validator.validate_federation_data()
        if args.check_pipeline:
            results['validations']['data_pipeline'] = validator.validate_data_pipeline()
    else:
        results = validator.run_comprehensive_validation()
    
    # Output results
    if args.output:
        with open(args.output, 'w') as f:
            json.dump(results, f, indent=2, default=str)
        print(f"Results saved to {args.output}")
    else:
        print(json.dumps(results, indent=2, default=str))
    
    # Exit with appropriate code
    if 'overall_status' in results:
        if results['overall_status'] == 'FAIL':
            sys.exit(1)
        elif results['overall_status'] == 'WARN':
            sys.exit(2)
    
    sys.exit(0)

if __name__ == '__main__':
    main()