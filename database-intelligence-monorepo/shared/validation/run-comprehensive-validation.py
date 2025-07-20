#!/usr/bin/env python3
"""
Comprehensive Validation Runner

This script orchestrates all validation tools in the database intelligence
validation framework, providing a single entry point for complete system validation.

Validation Phases:
1. Connection Testing - Verify New Relic API connectivity
2. End-to-End Pipeline - Validate complete data flow pipeline
3. Module-Specific Validation - Deep validation for each module
4. Integration Testing - Cross-module consistency and data integrity
5. Performance Validation - Query performance and latency checks

Usage:
    python3 run-comprehensive-validation.py
    python3 run-comprehensive-validation.py --quick
    python3 run-comprehensive-validation.py --phase pipeline --phase integration
    python3 run-comprehensive-validation.py --modules core-metrics sql-intelligence
"""

import os
import sys
import json
import subprocess
import argparse
import time
from datetime import datetime
from typing import Dict, List, Optional, Any
import logging
from dataclasses import dataclass
from enum import Enum
from concurrent.futures import ThreadPoolExecutor, as_completed
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class ValidationPhase(Enum):
    CONNECTION = "connection"
    PIPELINE = "pipeline"
    MODULES = "modules"
    INTEGRATION = "integration"
    PERFORMANCE = "performance"

class PhaseResult(Enum):
    SUCCESS = "SUCCESS"
    WARNING = "WARNING"
    FAILURE = "FAILURE"
    SKIPPED = "SKIPPED"

@dataclass
class ValidationPhaseResult:
    phase: ValidationPhase
    result: PhaseResult
    duration_seconds: float
    details: Dict[str, Any]
    issues: List[str]
    recommendations: List[str]
    timestamp: datetime

class ComprehensiveValidationRunner:
    def __init__(self):
        self.validation_dir = os.path.dirname(os.path.abspath(__file__))
        self.project_root = os.path.dirname(os.path.dirname(self.validation_dir))
        
        # Verify environment
        self.nr_api_key = os.getenv('NEW_RELIC_API_KEY')
        self.nr_account_id = os.getenv('NEW_RELIC_ACCOUNT_ID')
        
        if not all([self.nr_api_key, self.nr_account_id]):
            raise ValueError("NEW_RELIC_API_KEY and NEW_RELIC_ACCOUNT_ID must be set in .env file")
        
        self.modules = [
            'core-metrics', 'sql-intelligence', 'wait-profiler', 'anomaly-detector',
            'business-impact', 'replication-monitor', 'performance-advisor', 
            'resource-monitor', 'alert-manager', 'canary-tester', 'cross-signal-correlator'
        ]
        
        self.phase_results = []

    def run_script(self, script_path: str, args: List[str] = None, timeout: int = 300) -> Dict:
        """Run a validation script and return results"""
        if args is None:
            args = []
        
        cmd = ['python3', script_path] + args
        start_time = time.time()
        
        try:
            # Change to project root for consistent paths
            result = subprocess.run(
                cmd,
                cwd=self.project_root,
                capture_output=True,
                text=True,
                timeout=timeout
            )
            
            duration = time.time() - start_time
            
            # Try to parse JSON output
            output_data = {}
            if result.stdout.strip():
                try:
                    output_data = json.loads(result.stdout)
                except json.JSONDecodeError:
                    output_data = {'raw_output': result.stdout}
            
            return {
                'success': result.returncode == 0,
                'exit_code': result.returncode,
                'duration_seconds': duration,
                'output': output_data,
                'stderr': result.stderr,
                'command': ' '.join(cmd)
            }
            
        except subprocess.TimeoutExpired:
            return {
                'success': False,
                'exit_code': -1,
                'duration_seconds': timeout,
                'error': f'Script timed out after {timeout} seconds',
                'command': ' '.join(cmd)
            }
        except Exception as e:
            return {
                'success': False,
                'exit_code': -1,
                'duration_seconds': time.time() - start_time,
                'error': str(e),
                'command': ' '.join(cmd)
            }

    def validate_connection(self) -> ValidationPhaseResult:
        """Phase 1: Test New Relic API connectivity"""
        logger.info("Phase 1: Testing New Relic API connectivity...")
        start_time = time.time()
        
        issues = []
        recommendations = []
        details = {}
        
        # Run connection test
        script_path = os.path.join(self.validation_dir, 'test-nrdb-connection.py')
        result = self.run_script(script_path, timeout=60)
        
        details['connection_test'] = result
        
        if result['success']:
            phase_result = PhaseResult.SUCCESS
            logger.info("✅ New Relic API connectivity verified")
        else:
            phase_result = PhaseResult.FAILURE
            issues.append("New Relic API connectivity failed")
            recommendations.append("Check API credentials and network connectivity")
            if result.get('stderr'):
                issues.append(f"Error: {result['stderr']}")
        
        duration = time.time() - start_time
        
        return ValidationPhaseResult(
            phase=ValidationPhase.CONNECTION,
            result=phase_result,
            duration_seconds=duration,
            details=details,
            issues=issues,
            recommendations=recommendations,
            timestamp=datetime.now()
        )

    def validate_pipeline(self, modules: List[str] = None) -> ValidationPhaseResult:
        """Phase 2: End-to-end pipeline validation"""
        logger.info("Phase 2: Running end-to-end pipeline validation...")
        start_time = time.time()
        
        issues = []
        recommendations = []
        details = {}
        
        # Run pipeline validator
        script_path = os.path.join(self.validation_dir, 'end-to-end-pipeline-validator.py')
        args = []
        if modules:
            args.extend(['--modules'] + modules)
        
        result = self.run_script(script_path, args, timeout=600)
        details['pipeline_validation'] = result
        
        # Parse pipeline results
        if result['success'] and 'output' in result:
            pipeline_data = result['output']
            
            if isinstance(pipeline_data, dict) and 'pipeline_validation' in pipeline_data:
                pipeline_status = pipeline_data['pipeline_validation'].get('overall_status', 'UNKNOWN')
                health_score = pipeline_data['pipeline_validation'].get('pipeline_health_score', 0)
                
                details['health_score'] = health_score
                details['pipeline_status'] = pipeline_status
                
                if pipeline_status == 'HEALTHY':
                    phase_result = PhaseResult.SUCCESS
                elif pipeline_status == 'DEGRADED':
                    phase_result = PhaseResult.WARNING
                    issues.extend(pipeline_data.get('critical_issues', []))
                    recommendations.extend(pipeline_data.get('recommendations', []))
                else:
                    phase_result = PhaseResult.FAILURE
                    issues.extend(pipeline_data.get('critical_issues', []))
                    recommendations.extend(pipeline_data.get('recommendations', []))
            else:
                phase_result = PhaseResult.WARNING
                issues.append("Pipeline validation completed but results format unexpected")
        else:
            phase_result = PhaseResult.FAILURE
            issues.append("Pipeline validation failed to execute")
            if result.get('error'):
                issues.append(result['error'])
            if result.get('stderr'):
                issues.append(f"Error output: {result['stderr']}")
        
        duration = time.time() - start_time
        
        return ValidationPhaseResult(
            phase=ValidationPhase.PIPELINE,
            result=phase_result,
            duration_seconds=duration,
            details=details,
            issues=issues,
            recommendations=recommendations,
            timestamp=datetime.now()
        )

    def validate_modules(self, modules: List[str] = None) -> ValidationPhaseResult:
        """Phase 3: Module-specific validations"""
        logger.info("Phase 3: Running module-specific validations...")
        start_time = time.time()
        
        issues = []
        recommendations = []
        details = {}
        
        # Run module-specific validation orchestrator
        script_path = os.path.join(self.validation_dir, 'module-specific', 'validate-all-modules.py')
        args = ['--parallel', '--max-workers', '3']
        
        if modules:
            args.extend(['--modules'] + modules)
        else:
            args.append('--high-priority-only')  # Focus on critical modules
        
        result = self.run_script(script_path, args, timeout=900)
        details['module_validation'] = result
        
        # Parse module results
        if result['success'] and 'output' in result:
            module_data = result['output']
            
            if isinstance(module_data, dict) and 'validation_summary' in module_data:
                overall_status = module_data['validation_summary'].get('overall_status', 'UNKNOWN')
                health_score = module_data['validation_summary'].get('health_score', 0)
                
                details['health_score'] = health_score
                details['module_status'] = overall_status
                details['module_summaries'] = module_data.get('module_summaries', {})
                
                if overall_status == 'PASS':
                    phase_result = PhaseResult.SUCCESS
                elif overall_status == 'WARN':
                    phase_result = PhaseResult.WARNING
                    # Extract module-specific issues
                    for module, summary in details['module_summaries'].items():
                        issues.extend(summary.get('issues', []))
                else:
                    phase_result = PhaseResult.FAILURE
                    for module, summary in details['module_summaries'].items():
                        issues.extend(summary.get('issues', []))
                
                recommendations.extend(module_data.get('recommendations', []))
            else:
                phase_result = PhaseResult.WARNING
                issues.append("Module validation completed but results format unexpected")
        else:
            phase_result = PhaseResult.FAILURE
            issues.append("Module validation failed to execute")
            if result.get('error'):
                issues.append(result['error'])
        
        duration = time.time() - start_time
        
        return ValidationPhaseResult(
            phase=ValidationPhase.MODULES,
            result=phase_result,
            duration_seconds=duration,
            details=details,
            issues=issues,
            recommendations=recommendations,
            timestamp=datetime.now()
        )

    def validate_integration(self, modules: List[str] = None) -> ValidationPhaseResult:
        """Phase 4: Integration and consistency testing"""
        logger.info("Phase 4: Running integration and consistency tests...")
        start_time = time.time()
        
        issues = []
        recommendations = []
        details = {}
        
        # Run integration test suite
        script_path = os.path.join(self.validation_dir, 'integration-test-suite.py')
        args = []
        if modules:
            args.extend(['--modules'] + modules)
        
        result = self.run_script(script_path, args, timeout=600)
        details['integration_testing'] = result
        
        # Parse integration results
        if result['success'] and 'output' in result:
            integration_data = result['output']
            
            if isinstance(integration_data, dict) and 'integration_test_results' in integration_data:
                overall_result = integration_data['integration_test_results'].get('overall_result', 'UNKNOWN')
                success_rate = integration_data['integration_test_results'].get('success_rate', 0)
                
                details['success_rate'] = success_rate
                details['integration_result'] = overall_result
                details['test_summary'] = integration_data.get('test_summary', {})
                
                if overall_result == 'PASS':
                    phase_result = PhaseResult.SUCCESS
                elif overall_result == 'PARTIAL':
                    phase_result = PhaseResult.WARNING
                else:
                    phase_result = PhaseResult.FAILURE
                
                # Extract failed test details
                failed_tests = [tc for tc in integration_data.get('test_cases', []) 
                              if tc.get('result') in ['FAIL', 'ERROR']]
                for test in failed_tests:
                    issues.append(f"Test {test.get('test_id')}: {test.get('name')} failed")
                    if test.get('error_message'):
                        issues.append(f"  Error: {test.get('error_message')}")
            else:
                phase_result = PhaseResult.WARNING
                issues.append("Integration testing completed but results format unexpected")
        else:
            phase_result = PhaseResult.FAILURE
            issues.append("Integration testing failed to execute")
            if result.get('error'):
                issues.append(result['error'])
        
        duration = time.time() - start_time
        
        return ValidationPhaseResult(
            phase=ValidationPhase.INTEGRATION,
            result=phase_result,
            duration_seconds=duration,
            details=details,
            issues=issues,
            recommendations=recommendations,
            timestamp=datetime.now()
        )

    def validate_performance(self) -> ValidationPhaseResult:
        """Phase 5: Performance validation"""
        logger.info("Phase 5: Running performance validation...")
        start_time = time.time()
        
        issues = []
        recommendations = []
        details = {}
        
        # Run main NRDB validator with performance focus
        script_path = os.path.join(self.validation_dir, 'automated-nrdb-validator.py')
        args = ['--validate-all']
        
        result = self.run_script(script_path, args, timeout=300)
        details['nrdb_validation'] = result
        
        # Analyze performance metrics from the results
        if result['success'] and 'output' in result:
            nrdb_data = result['output']
            
            if isinstance(nrdb_data, dict) and 'overall_health_score' in nrdb_data:
                health_score = nrdb_data.get('overall_health_score', 0)
                
                details['health_score'] = health_score
                details['performance_metrics'] = {
                    'validation_duration': result['duration_seconds'],
                    'modules_validated': len(nrdb_data.get('module_summaries', {}))
                }
                
                if health_score >= 90:
                    phase_result = PhaseResult.SUCCESS
                elif health_score >= 70:
                    phase_result = PhaseResult.WARNING
                    issues.append(f"Performance concerns detected (health score: {health_score}%)")
                else:
                    phase_result = PhaseResult.FAILURE
                    issues.append(f"Performance issues detected (health score: {health_score}%)")
                
                # Extract performance-related issues
                for issue in nrdb_data.get('critical_issues', []):
                    if any(keyword in issue.lower() for keyword in ['slow', 'latency', 'timeout', 'performance']):
                        issues.append(issue)
                
                recommendations.extend(nrdb_data.get('recommendations', []))
            else:
                phase_result = PhaseResult.WARNING
                issues.append("Performance validation completed but results format unexpected")
        else:
            phase_result = PhaseResult.FAILURE
            issues.append("Performance validation failed to execute")
            if result.get('error'):
                issues.append(result['error'])
        
        duration = time.time() - start_time
        
        return ValidationPhaseResult(
            phase=ValidationPhase.PERFORMANCE,
            result=phase_result,
            duration_seconds=duration,
            details=details,
            issues=issues,
            recommendations=recommendations,
            timestamp=datetime.now()
        )

    def run_comprehensive_validation(self, 
                                   phases: List[ValidationPhase] = None,
                                   modules: List[str] = None,
                                   quick_mode: bool = False) -> Dict:
        """Run comprehensive validation across all phases"""
        logger.info("Starting comprehensive database intelligence validation...")
        
        if phases is None:
            if quick_mode:
                phases = [ValidationPhase.CONNECTION, ValidationPhase.PIPELINE]
            else:
                phases = list(ValidationPhase)
        
        validation_start = time.time()
        
        # Phase execution mapping
        phase_executors = {
            ValidationPhase.CONNECTION: lambda: self.validate_connection(),
            ValidationPhase.PIPELINE: lambda: self.validate_pipeline(modules),
            ValidationPhase.MODULES: lambda: self.validate_modules(modules),
            ValidationPhase.INTEGRATION: lambda: self.validate_integration(modules),
            ValidationPhase.PERFORMANCE: lambda: self.validate_performance()
        }
        
        # Execute phases in order
        for phase in phases:
            if phase in phase_executors:
                try:
                    phase_result = phase_executors[phase]()
                    self.phase_results.append(phase_result)
                    
                    # Log phase completion
                    status_icon = {
                        PhaseResult.SUCCESS: "✅",
                        PhaseResult.WARNING: "⚠️", 
                        PhaseResult.FAILURE: "❌",
                        PhaseResult.SKIPPED: "⏭"
                    }.get(phase_result.result, "❓")
                    
                    logger.info(f"{status_icon} Phase {phase.value}: {phase_result.result.value} "
                              f"({phase_result.duration_seconds:.1f}s)")
                    
                    # Stop on critical failures unless in quick mode
                    if (phase_result.result == PhaseResult.FAILURE and 
                        phase == ValidationPhase.CONNECTION and 
                        not quick_mode):
                        logger.warning("Connection phase failed - skipping remaining phases")
                        break
                        
                except Exception as e:
                    logger.error(f"Phase {phase.value} failed with exception: {e}")
                    error_result = ValidationPhaseResult(
                        phase=phase,
                        result=PhaseResult.FAILURE,
                        duration_seconds=0,
                        details={'exception': str(e)},
                        issues=[f"Phase execution failed: {e}"],
                        recommendations=[],
                        timestamp=datetime.now()
                    )
                    self.phase_results.append(error_result)
        
        # Calculate overall results
        total_phases = len(self.phase_results)
        successful_phases = len([r for r in self.phase_results if r.result == PhaseResult.SUCCESS])
        warning_phases = len([r for r in self.phase_results if r.result == PhaseResult.WARNING])
        failed_phases = len([r for r in self.phase_results if r.result == PhaseResult.FAILURE])
        
        # Determine overall validation result
        if failed_phases > 0:
            overall_result = "FAILURE"
        elif warning_phases > 0:
            overall_result = "WARNING"
        else:
            overall_result = "SUCCESS"
        
        # Calculate health score
        health_score = ((successful_phases + 0.5 * warning_phases) / total_phases * 100) if total_phases > 0 else 0
        
        # Aggregate all issues and recommendations
        all_issues = []
        all_recommendations = []
        for result in self.phase_results:
            all_issues.extend(result.issues)
            all_recommendations.extend(result.recommendations)
        
        # Remove duplicates while preserving order
        all_issues = list(dict.fromkeys(all_issues))
        all_recommendations = list(dict.fromkeys(all_recommendations))
        
        total_duration = time.time() - validation_start
        
        return {
            'comprehensive_validation': {
                'timestamp': datetime.now().isoformat(),
                'overall_result': overall_result,
                'health_score': round(health_score, 1),
                'total_duration_seconds': round(total_duration, 2),
                'phases_executed': total_phases,
                'quick_mode': quick_mode
            },
            'phase_summary': {
                'successful': successful_phases,
                'warnings': warning_phases,
                'failures': failed_phases,
                'total': total_phases
            },
            'phase_results': [
                {
                    'phase': result.phase.value,
                    'result': result.result.value,
                    'duration_seconds': round(result.duration_seconds, 2),
                    'timestamp': result.timestamp.isoformat(),
                    'details': result.details,
                    'issues_count': len(result.issues),
                    'recommendations_count': len(result.recommendations)
                }
                for result in self.phase_results
            ],
            'critical_issues': all_issues[:10],  # Top 10 issues
            'top_recommendations': all_recommendations[:10],  # Top 10 recommendations
            'validation_config': {
                'phases_requested': [p.value for p in phases],
                'modules_tested': modules or self.modules,
                'quick_mode': quick_mode
            }
        }

def main():
    parser = argparse.ArgumentParser(description='Comprehensive Database Intelligence Validation')
    parser.add_argument('--phase', action='append', 
                       choices=[p.value for p in ValidationPhase],
                       help='Specific validation phases to run (can specify multiple)')
    parser.add_argument('--modules', nargs='+', help='Specific modules to validate')
    parser.add_argument('--quick', action='store_true', 
                       help='Quick validation (connection + pipeline only)')
    parser.add_argument('--output', help='Output file for results (JSON format)')
    parser.add_argument('--verbose', action='store_true', help='Verbose output')
    
    args = parser.parse_args()
    
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    
    try:
        runner = ComprehensiveValidationRunner()
    except ValueError as e:
        print(f"Configuration error: {e}")
        print("Please ensure NEW_RELIC_API_KEY and NEW_RELIC_ACCOUNT_ID are set in .env file")
        sys.exit(1)
    
    # Parse phases
    phases = None
    if args.phase:
        phases = [ValidationPhase(p) for p in args.phase]
    
    # Run validation
    try:
        results = runner.run_comprehensive_validation(
            phases=phases,
            modules=args.modules,
            quick_mode=args.quick
        )
    except KeyboardInterrupt:
        print("\nValidation interrupted by user")
        sys.exit(130)
    except Exception as e:
        print(f"Validation failed with unexpected error: {e}")
        sys.exit(1)
    
    # Output results
    if args.output:
        with open(args.output, 'w') as f:
            json.dump(results, f, indent=2, default=str)
        print(f"Detailed results saved to {args.output}")
    
    # Print summary
    validation_summary = results['comprehensive_validation']
    phase_summary = results['phase_summary']
    
    print(f"\n{'='*80}")
    print(f"COMPREHENSIVE DATABASE INTELLIGENCE VALIDATION RESULTS")
    print(f"{'='*80}")
    print(f"Overall Result: {validation_summary['overall_result']}")
    print(f"Health Score: {validation_summary['health_score']}%")
    print(f"Total Duration: {validation_summary['total_duration_seconds']}s")
    print(f"Phases Executed: {validation_summary['phases_executed']}")
    
    print(f"\nPhase Summary:")
    print(f"  ✅ Successful: {phase_summary['successful']}")
    print(f"  ⚠️  Warnings: {phase_summary['warnings']}")
    print(f"  ❌ Failures: {phase_summary['failures']}")
    
    # Show phase details
    print(f"\nPhase Details:")
    for phase_result in results['phase_results']:
        status_icon = {
            'SUCCESS': "✅",
            'WARNING': "⚠️",
            'FAILURE': "❌",
            'SKIPPED': "⏭"
        }.get(phase_result['result'], "❓")
        
        print(f"  {status_icon} {phase_result['phase'].capitalize()}: "
              f"{phase_result['result']} ({phase_result['duration_seconds']}s)")
        
        if phase_result['issues_count'] > 0:
            print(f"    Issues: {phase_result['issues_count']}")
        if phase_result['recommendations_count'] > 0:
            print(f"    Recommendations: {phase_result['recommendations_count']}")
    
    # Show critical issues
    if results['critical_issues']:
        print(f"\nCritical Issues:")
        for i, issue in enumerate(results['critical_issues'], 1):
            print(f"  {i}. {issue}")
    
    # Show top recommendations
    if results['top_recommendations']:
        print(f"\nTop Recommendations:")
        for i, rec in enumerate(results['top_recommendations'], 1):
            print(f"  {i}. {rec}")
    
    print(f"\nNext Steps:")
    if validation_summary['overall_result'] == 'SUCCESS':
        print("  🎉 All validations passed! System is healthy.")
        print("  💡 Consider scheduling regular validation runs")
    elif validation_summary['overall_result'] == 'WARNING':
        print("  ⚠️  Address warnings to improve system health")
        print("  🔍 Review individual phase results for details")
    else:
        print("  🚨 Critical issues detected - immediate attention required")
        print("  🛠️  Use troubleshooting script: ./shared/validation/troubleshoot-missing-data.sh")
        print("  📞 Consider escalating to infrastructure team")
    
    # Exit with appropriate code
    exit_codes = {
        'SUCCESS': 0,
        'WARNING': 1, 
        'FAILURE': 2
    }
    sys.exit(exit_codes.get(validation_summary['overall_result'], 3))

if __name__ == '__main__':
    main()