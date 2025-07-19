#!/usr/bin/env python3
"""
Main script to run E2E validation with all validators
"""

import argparse
import sys
from pathlib import Path

from validation_framework import ValidationFramework
from metric_validator import MetricValidator
from dashboard_validator import DashboardValidator
from alert_validator import AlertValidator


def main():
    """Run E2E validation with all configured validators"""
    parser = argparse.ArgumentParser(
        description="Database Intelligence E2E Validation Framework",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Run all validations for all modules
  python run_validation.py

  # Run validations for specific modules
  python run_validation.py --modules anomaly-detector query-insights

  # Run with debug logging
  python run_validation.py --log-level DEBUG

  # Run only specific validators
  python run_validation.py --validators metrics dashboards
        """
    )
    
    parser.add_argument(
        "--modules", 
        nargs="+", 
        help="Specific modules to test (default: all modules)"
    )
    parser.add_argument(
        "--validators",
        nargs="+",
        choices=["metrics", "dashboards", "alerts"],
        default=["metrics", "dashboards", "alerts"],
        help="Validators to run (default: all)"
    )
    parser.add_argument(
        "--log-level", 
        default="INFO", 
        choices=["DEBUG", "INFO", "WARNING", "ERROR"],
        help="Logging verbosity level (default: INFO)"
    )
    parser.add_argument(
        "--base-path", 
        type=Path, 
        default=Path(__file__).parent.parent.parent,
        help="Base path to the monorepo (default: auto-detected)"
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        help="Directory to save results (default: shared/e2e/results)"
    )
    
    args = parser.parse_args()
    
    # Initialize framework
    print(f"Initializing E2E Validation Framework...")
    print(f"Base path: {args.base_path}")
    print(f"Validators: {', '.join(args.validators)}")
    
    framework = ValidationFramework(args.base_path, args.log_level)
    
    # Discover modules
    if args.modules:
        print(f"Testing modules: {', '.join(args.modules)}")
    else:
        print("Testing all modules")
    
    framework.discover_modules(args.modules)
    
    if not framework.modules:
        print("ERROR: No modules found to test!")
        sys.exit(1)
    
    print(f"Found {len(framework.modules)} modules to test")
    
    # Register selected validators
    validator_map = {
        "metrics": MetricValidator,
        "dashboards": DashboardValidator,
        "alerts": AlertValidator
    }
    
    for validator_name in args.validators:
        if validator_name in validator_map:
            framework.register_validator(validator_map[validator_name])
            print(f"Registered {validator_name} validator")
    
    # Run validations
    print("\nStarting validation tests...")
    print("-" * 60)
    
    results, summary = framework.run_validations()
    
    # Save results
    framework.save_results(results, summary, args.output_dir)
    
    # Print summary
    print("-" * 60)
    print("\nValidation Summary:")
    print(f"  Total Tests: {summary['total_tests']}")
    print(f"  Passed:      {summary['passed']}")
    print(f"  Failed:      {summary['failed']}")
    print(f"  Skipped:     {summary['skipped']}")
    print(f"  Duration:    {summary['duration']:.2f} seconds")
    
    # Print per-module summary
    if summary['by_module']:
        print("\nResults by Module:")
        for module, stats in summary['by_module'].items():
            status = "PASS" if stats['failed'] == 0 else "FAIL"
            print(f"  {module:20} [{status}] - "
                  f"Passed: {stats['passed']}, "
                  f"Failed: {stats['failed']}, "
                  f"Skipped: {stats['skipped']}")
    
    # Print failed tests for easy debugging
    failed_tests = [r for r in results if r.status == "failed"]
    if failed_tests:
        print("\nFailed Tests:")
        for test in failed_tests:
            print(f"  [{test.module}] {test.test_name}: {test.message}")
    
    # Exit with appropriate code
    exit_code = 1 if summary['failed'] > 0 else 0
    print(f"\nExiting with code: {exit_code}")
    sys.exit(exit_code)


if __name__ == "__main__":
    main()