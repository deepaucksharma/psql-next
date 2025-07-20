#!/usr/bin/env python3
"""
Module-Specific Validation Orchestrator

This tool orchestrates deep validation for all modules in the database intelligence monorepo.
It runs module-specific validation tools and aggregates results.

Usage:
    python3 validate-all-modules.py --api-key YOUR_API_KEY --account-id YOUR_ACCOUNT_ID
    python3 validate-all-modules.py --modules core-metrics sql-intelligence anomaly-detector
    python3 validate-all-modules.py --parallel --timeout 300
"""

import argparse
import json
import subprocess
import sys
import time
import os
from datetime import datetime
from typing import Dict, List, Optional
import logging
from concurrent.futures import ThreadPoolExecutor, TimeoutError, as_completed
import importlib.util
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class ModuleValidationOrchestrator:
    def __init__(self, api_key: str, account_id: str):
        self.api_key = api_key
        self.account_id = account_id
        
        # Module validation mapping
        self.module_validators = {
            'core-metrics': {
                'script': 'validate-core-metrics.py',
                'port': 8081,
                'description': 'Basic MySQL metrics validation',
                'priority': 'high'
            },
            'sql-intelligence': {
                'script': 'validate-sql-intelligence.py', 
                'port': 8082,
                'description': 'Query analysis and performance validation',
                'priority': 'high'
            },
            'wait-profiler': {
                'script': 'validate-wait-profiler.py',
                'port': 8083,
                'description': 'Wait event and mutex validation',
                'priority': 'medium'
            },
            'anomaly-detector': {
                'script': 'validate-anomaly-detector.py',
                'port': 8084,
                'description': 'Anomaly detection and baseline validation',
                'priority': 'high'
            },
            'business-impact': {
                'script': 'validate-business-impact.py',
                'port': 8085,
                'description': 'Business scoring and impact validation',
                'priority': 'medium'
            },
            'replication-monitor': {
                'script': 'validate-replication-monitor.py',
                'port': 8086,
                'description': 'Replication health and lag validation',
                'priority': 'high'
            },
            'performance-advisor': {
                'script': 'validate-performance-advisor.py',
                'port': 8087,
                'description': 'Performance recommendations validation',
                'priority': 'medium'
            },
            'resource-monitor': {
                'script': 'validate-resource-monitor.py',
                'port': 8088,
                'description': 'System resource metrics validation',
                'priority': 'medium'
            },
            'alert-manager': {
                'script': 'validate-alert-manager.py',
                'port': 8089,
                'description': 'Alert processing and management validation',
                'priority': 'medium'
            },
            'canary-tester': {
                'script': 'validate-canary-tester.py',
                'port': 8090,
                'description': 'Canary testing and health validation',
                'priority': 'low'
            },
            'cross-signal-correlator': {
                'script': 'validate-cross-signal-correlator.py',
                'port': 8099,
                'description': 'Cross-signal correlation validation',
                'priority': 'low'
            }
        }
        
        self.script_dir = os.path.dirname(os.path.abspath(__file__))
        
    def check_validator_exists(self, module: str) -> bool:
        """Check if validator script exists for module"""
        if module not in self.module_validators:
            return False
            
        script_path = os.path.join(self.script_dir, self.module_validators[module]['script'])
        return os.path.exists(script_path)
    
    def run_module_validation(self, module: str, timeout: int = 300) -> Dict:
        """Run validation for a specific module"""
        logger.info(f"Running validation for {module}...")
        
        result = {
            'module': module,
            'status': 'UNKNOWN',
            'start_time': datetime.now().isoformat(),
            'duration_seconds': 0,
            'validation_results': {},
            'error': None
        }
        
        start_time = time.time()
        
        try:
            if not self.check_validator_exists(module):
                # If specific validator doesn't exist, create a basic validation result
                result['status'] = 'SKIPPED'
                result['error'] = f"No specific validator found for {module}"
                result['validation_results'] = self._create_basic_validation(module)
                return result
            
            script_path = os.path.join(self.script_dir, self.module_validators[module]['script'])
            
            # Run the module-specific validator (credentials come from .env)
            cmd = [
                'python3', script_path,
                '--output', f'/tmp/{module}_validation_result.json'
            ]
            
            process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )
            
            try:
                stdout, stderr = process.communicate(timeout=timeout)
                
                # Load results from output file
                output_file = f'/tmp/{module}_validation_result.json'
                if os.path.exists(output_file):
                    with open(output_file, 'r') as f:
                        result['validation_results'] = json.load(f)
                    os.remove(output_file)  # Clean up
                else:
                    # Parse stdout if no file
                    try:
                        result['validation_results'] = json.loads(stdout)
                    except json.JSONDecodeError:
                        result['validation_results'] = {'raw_output': stdout}
                
                # Determine status from exit code
                if process.returncode == 0:
                    result['status'] = 'PASS'
                elif process.returncode == 2:
                    result['status'] = 'WARN'
                else:
                    result['status'] = 'FAIL'
                    result['error'] = stderr if stderr else 'Validation failed'
                
            except subprocess.TimeoutExpired:
                process.kill()
                result['status'] = 'TIMEOUT'
                result['error'] = f'Validation timed out after {timeout} seconds'
                
        except Exception as e:
            result['status'] = 'ERROR'
            result['error'] = str(e)
            
        finally:
            result['duration_seconds'] = time.time() - start_time
            result['end_time'] = datetime.now().isoformat()
            
        logger.info(f"{module} validation completed: {result['status']} in {result['duration_seconds']:.1f}s")
        return result
    
    def _create_basic_validation(self, module: str) -> Dict:
        """Create basic validation result for modules without specific validators"""
        return {
            'module': module,
            'timestamp': datetime.now().isoformat(),
            'overall_status': 'SKIPPED',
            'message': f'No specific validator available for {module}',
            'validations': {},
            'summary': {
                'total_checks': 0,
                'passed': 0,
                'warnings': 0,
                'failures': 0
            },
            'recommendations': [f'Create specific validator for {module} module']
        }
    
    def run_parallel_validations(self, modules: List[str], max_workers: int = 3, timeout: int = 300) -> Dict:
        """Run multiple module validations in parallel"""
        logger.info(f"Running parallel validations for {len(modules)} modules...")
        
        results = {
            'timestamp': datetime.now().isoformat(),
            'total_modules': len(modules),
            'completed_modules': 0,
            'module_results': {},
            'summary': {
                'passed': 0,
                'warnings': 0,
                'failures': 0,
                'errors': 0,
                'skipped': 0,
                'timeouts': 0
            },
            'duration_seconds': 0
        }
        
        start_time = time.time()
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            # Submit all validation tasks
            future_to_module = {
                executor.submit(self.run_module_validation, module, timeout): module
                for module in modules
            }
            
            # Collect results as they complete
            for future in as_completed(future_to_module, timeout=timeout * len(modules)):
                module = future_to_module[future]
                try:
                    result = future.result()
                    results['module_results'][module] = result
                    results['completed_modules'] += 1
                    
                    # Update summary
                    status = result['status']
                    if status == 'PASS':
                        results['summary']['passed'] += 1
                    elif status == 'WARN':
                        results['summary']['warnings'] += 1
                    elif status == 'FAIL':
                        results['summary']['failures'] += 1
                    elif status == 'ERROR':
                        results['summary']['errors'] += 1
                    elif status == 'SKIPPED':
                        results['summary']['skipped'] += 1
                    elif status == 'TIMEOUT':
                        results['summary']['timeouts'] += 1
                        
                except Exception as e:
                    logger.error(f"Failed to get result for {module}: {e}")
                    results['module_results'][module] = {
                        'module': module,
                        'status': 'ERROR',
                        'error': str(e)
                    }
                    results['summary']['errors'] += 1
        
        results['duration_seconds'] = time.time() - start_time
        return results
    
    def run_sequential_validations(self, modules: List[str], timeout: int = 300) -> Dict:
        """Run module validations sequentially"""
        logger.info(f"Running sequential validations for {len(modules)} modules...")
        
        results = {
            'timestamp': datetime.now().isoformat(),
            'total_modules': len(modules),
            'completed_modules': 0,
            'module_results': {},
            'summary': {
                'passed': 0,
                'warnings': 0,
                'failures': 0,
                'errors': 0,
                'skipped': 0,
                'timeouts': 0
            },
            'duration_seconds': 0
        }
        
        start_time = time.time()
        
        for module in modules:
            result = self.run_module_validation(module, timeout)
            results['module_results'][module] = result
            results['completed_modules'] += 1
            
            # Update summary
            status = result['status']
            if status == 'PASS':
                results['summary']['passed'] += 1
            elif status == 'WARN':
                results['summary']['warnings'] += 1
            elif status == 'FAIL':
                results['summary']['failures'] += 1
            elif status == 'ERROR':
                results['summary']['errors'] += 1
            elif status == 'SKIPPED':
                results['summary']['skipped'] += 1
            elif status == 'TIMEOUT':
                results['summary']['timeouts'] += 1
        
        results['duration_seconds'] = time.time() - start_time
        return results
    
    def generate_comprehensive_report(self, results: Dict) -> Dict:
        """Generate comprehensive report from validation results"""
        report = {
            'validation_summary': {
                'timestamp': results['timestamp'],
                'total_modules_tested': results['total_modules'],
                'overall_status': 'UNKNOWN',
                'health_score': 0.0,
                'total_duration_seconds': results['duration_seconds']
            },
            'module_status_summary': results['summary'],
            'critical_issues': [],
            'warnings': [],
            'recommendations': [],
            'module_details': {}
        }
        
        # Calculate overall status and health score
        total_modules = results['total_modules']
        if total_modules > 0:
            passed = results['summary']['passed']
            warnings = results['summary']['warnings']
            failures = results['summary']['failures']
            errors = results['summary']['errors']
            
            # Health score calculation (passed modules + 0.5 * warning modules) / total
            health_score = (passed + 0.5 * warnings) / total_modules * 100
            report['validation_summary']['health_score'] = round(health_score, 1)
            
            # Overall status determination
            if failures > 0 or errors > 0:
                report['validation_summary']['overall_status'] = 'FAIL'
            elif warnings > 0:
                report['validation_summary']['overall_status'] = 'WARN'
            else:
                report['validation_summary']['overall_status'] = 'PASS'
        
        # Extract issues and recommendations
        for module, module_result in results['module_results'].items():
            module_detail = {
                'status': module_result['status'],
                'duration_seconds': module_result.get('duration_seconds', 0),
                'port': self.module_validators.get(module, {}).get('port', 'unknown'),
                'priority': self.module_validators.get(module, {}).get('priority', 'medium')
            }
            
            if 'validation_results' in module_result and 'validations' in module_result['validation_results']:
                validations = module_result['validation_results']['validations']
                module_issues = []
                module_recommendations = []
                
                for validation_name, validation_result in validations.items():
                    if 'issues' in validation_result:
                        module_issues.extend(validation_result['issues'])
                    if 'recommendations' in validation_result:
                        module_recommendations.extend(validation_result['recommendations'])
                
                module_detail['issues'] = module_issues
                module_detail['recommendations'] = module_recommendations
                
                # Add to global lists
                if module_result['status'] == 'FAIL':
                    report['critical_issues'].extend([f"{module}: {issue}" for issue in module_issues])
                elif module_result['status'] == 'WARN':
                    report['warnings'].extend([f"{module}: {issue}" for issue in module_issues])
                
                report['recommendations'].extend([f"{module}: {rec}" for rec in module_recommendations])
            
            if 'error' in module_result and module_result['error']:
                module_detail['error'] = module_result['error']
                if module_result['status'] in ['FAIL', 'ERROR']:
                    report['critical_issues'].append(f"{module}: {module_result['error']}")
            
            report['module_details'][module] = module_detail
        
        # Remove duplicates from recommendations
        report['recommendations'] = list(set(report['recommendations']))
        
        return report
    
    def get_available_modules(self) -> List[str]:
        """Get list of all available modules"""
        return list(self.module_validators.keys())
    
    def get_high_priority_modules(self) -> List[str]:
        """Get list of high priority modules"""
        return [module for module, config in self.module_validators.items() 
                if config['priority'] == 'high']

def main():
    parser = argparse.ArgumentParser(description='Module-Specific Validation Orchestrator')
    parser.add_argument('--api-key', help='New Relic API Key (overrides .env)')
    parser.add_argument('--account-id', help='New Relic Account ID (overrides .env)')
    parser.add_argument('--modules', nargs='+', help='Specific modules to validate')
    parser.add_argument('--all-modules', action='store_true', help='Validate all modules')
    parser.add_argument('--high-priority-only', action='store_true', help='Validate only high priority modules')
    parser.add_argument('--parallel', action='store_true', help='Run validations in parallel')
    parser.add_argument('--max-workers', type=int, default=3, help='Maximum parallel workers')
    parser.add_argument('--timeout', type=int, default=300, help='Timeout per module validation (seconds)')
    parser.add_argument('--output', help='Output file for results (JSON format)')
    parser.add_argument('--report', help='Output file for comprehensive report (JSON format)')
    parser.add_argument('--verbose', action='store_true', help='Verbose output')
    parser.add_argument('--list-modules', action='store_true', help='List available modules and exit')
    
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
    
    orchestrator = ModuleValidationOrchestrator(api_key, account_id)
    
    if args.list_modules:
        print("Available modules:")
        for module, config in orchestrator.module_validators.items():
            status = "✓" if orchestrator.check_validator_exists(module) else "✗"
            print(f"  {status} {module} (port {config['port']}) - {config['description']} [{config['priority']} priority]")
        sys.exit(0)
    
    # Determine modules to validate
    if args.modules:
        modules_to_validate = args.modules
    elif args.all_modules:
        modules_to_validate = orchestrator.get_available_modules()
    elif args.high_priority_only:
        modules_to_validate = orchestrator.get_high_priority_modules()
    else:
        print("Please specify --modules, --all-modules, or --high-priority-only")
        sys.exit(1)
    
    # Validate module names
    available_modules = orchestrator.get_available_modules()
    invalid_modules = [m for m in modules_to_validate if m not in available_modules]
    if invalid_modules:
        print(f"Invalid modules: {', '.join(invalid_modules)}")
        print(f"Available modules: {', '.join(available_modules)}")
        sys.exit(1)
    
    # Run validations
    logger.info(f"Starting validation for modules: {', '.join(modules_to_validate)}")
    
    if args.parallel:
        results = orchestrator.run_parallel_validations(
            modules_to_validate, 
            max_workers=args.max_workers,
            timeout=args.timeout
        )
    else:
        results = orchestrator.run_sequential_validations(
            modules_to_validate,
            timeout=args.timeout
        )
    
    # Generate comprehensive report
    report = orchestrator.generate_comprehensive_report(results)
    
    # Output results
    if args.output:
        with open(args.output, 'w') as f:
            json.dump(results, f, indent=2, default=str)
        print(f"Detailed results saved to {args.output}")
    
    if args.report:
        with open(args.report, 'w') as f:
            json.dump(report, f, indent=2, default=str)
        print(f"Comprehensive report saved to {args.report}")
    
    # Print summary
    print(f"\n{'='*60}")
    print(f"MODULE VALIDATION SUMMARY")
    print(f"{'='*60}")
    print(f"Overall Status: {report['validation_summary']['overall_status']}")
    print(f"Health Score: {report['validation_summary']['health_score']}%")
    print(f"Total Duration: {report['validation_summary']['total_duration_seconds']:.1f}s")
    print(f"Modules Tested: {report['validation_summary']['total_modules_tested']}")
    
    summary = report['module_status_summary']
    print(f"\nStatus Breakdown:")
    print(f"  ✓ Passed: {summary['passed']}")
    print(f"  ⚠ Warnings: {summary['warnings']}")
    print(f"  ✗ Failed: {summary['failures']}")
    print(f"  ⚡ Errors: {summary['errors']}")
    print(f"  ⏭ Skipped: {summary['skipped']}")
    print(f"  ⏱ Timeouts: {summary['timeouts']}")
    
    if report['critical_issues']:
        print(f"\nCritical Issues ({len(report['critical_issues'])}):")
        for issue in report['critical_issues'][:5]:  # Show first 5
            print(f"  - {issue}")
        if len(report['critical_issues']) > 5:
            print(f"  ... and {len(report['critical_issues']) - 5} more")
    
    if report['warnings']:
        print(f"\nWarnings ({len(report['warnings'])}):")
        for warning in report['warnings'][:3]:  # Show first 3
            print(f"  - {warning}")
        if len(report['warnings']) > 3:
            print(f"  ... and {len(report['warnings']) - 3} more")
    
    # Exit with appropriate code
    if summary['failures'] > 0 or summary['errors'] > 0:
        sys.exit(1)
    elif summary['warnings'] > 0:
        sys.exit(2)
    else:
        sys.exit(0)

if __name__ == '__main__':
    main()