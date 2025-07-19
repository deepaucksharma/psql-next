#!/usr/bin/env python3
"""
Base E2E Validation Framework for Database Intelligence Monorepo
Provides abstract base classes and utilities for E2E testing
"""

import abc
import json
import logging
import os
import sys
import time
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import requests
import yaml
from prometheus_client.parser import text_string_to_metric_families


@dataclass
class ValidationResult:
    """Result of a validation test"""
    test_name: str
    module: str
    status: str  # 'passed', 'failed', 'skipped'
    message: str
    duration: float
    timestamp: datetime
    details: Optional[Dict[str, Any]] = None


@dataclass
class ModuleConfig:
    """Configuration for a module under test"""
    name: str
    path: Path
    config_file: Optional[Path] = None
    prometheus_url: Optional[str] = None
    grafana_url: Optional[str] = None
    alertmanager_url: Optional[str] = None
    custom_endpoints: Optional[Dict[str, str]] = None


class BaseValidator(abc.ABC):
    """Abstract base class for all validators"""
    
    def __init__(self, module_config: ModuleConfig, logger: Optional[logging.Logger] = None):
        self.module_config = module_config
        self.logger = logger or logging.getLogger(self.__class__.__name__)
        self.results: List[ValidationResult] = []
        
    @abc.abstractmethod
    def validate(self) -> List[ValidationResult]:
        """Run validation tests and return results"""
        pass
    
    def _record_result(self, test_name: str, status: str, message: str, 
                      duration: float, details: Optional[Dict[str, Any]] = None) -> ValidationResult:
        """Record a test result"""
        result = ValidationResult(
            test_name=test_name,
            module=self.module_config.name,
            status=status,
            message=message,
            duration=duration,
            timestamp=datetime.now(),
            details=details
        )
        self.results.append(result)
        self.logger.info(f"[{status.upper()}] {test_name}: {message}")
        return result
    
    def _make_request(self, url: str, timeout: int = 30) -> Optional[requests.Response]:
        """Make HTTP request with error handling"""
        try:
            response = requests.get(url, timeout=timeout)
            response.raise_for_status()
            return response
        except requests.exceptions.RequestException as e:
            self.logger.error(f"Request failed for {url}: {e}")
            return None
    
    def _load_yaml_config(self, file_path: Path) -> Optional[Dict[str, Any]]:
        """Load YAML configuration file"""
        try:
            with open(file_path, 'r') as f:
                return yaml.safe_load(f)
        except Exception as e:
            self.logger.error(f"Failed to load YAML config from {file_path}: {e}")
            return None


class ValidationFramework:
    """Main framework for running E2E validations"""
    
    def __init__(self, base_path: Path, log_level: str = "INFO"):
        self.base_path = base_path
        self.logger = self._setup_logging(log_level)
        self.modules: List[ModuleConfig] = []
        self.validators: List[BaseValidator] = []
        
    def _setup_logging(self, log_level: str) -> logging.Logger:
        """Configure logging for the framework"""
        logger = logging.getLogger("ValidationFramework")
        logger.setLevel(getattr(logging, log_level))
        
        # Console handler
        console_handler = logging.StreamHandler(sys.stdout)
        console_handler.setLevel(getattr(logging, log_level))
        formatter = logging.Formatter(
            '%(asctime)s - %(name)s - %(levelname)s - %(message)s'
        )
        console_handler.setFormatter(formatter)
        logger.addHandler(console_handler)
        
        # File handler
        log_dir = self.base_path / "shared" / "e2e" / "logs"
        log_dir.mkdir(parents=True, exist_ok=True)
        file_handler = logging.FileHandler(
            log_dir / f"validation_{datetime.now().strftime('%Y%m%d_%H%M%S')}.log"
        )
        file_handler.setLevel(getattr(logging, log_level))
        file_handler.setFormatter(formatter)
        logger.addHandler(file_handler)
        
        return logger
    
    def discover_modules(self, module_names: Optional[List[str]] = None) -> None:
        """Discover modules to test"""
        modules_dir = self.base_path / "modules"
        
        if module_names:
            # Test specific modules
            for name in module_names:
                module_path = modules_dir / name
                if module_path.exists():
                    self._load_module_config(name, module_path)
                else:
                    self.logger.warning(f"Module '{name}' not found at {module_path}")
        else:
            # Test all modules
            for module_path in modules_dir.iterdir():
                if module_path.is_dir() and not module_path.name.startswith('.'):
                    self._load_module_config(module_path.name, module_path)
    
    def _load_module_config(self, name: str, path: Path) -> None:
        """Load configuration for a module"""
        config_file = path / "e2e-config.yaml"
        module_config = ModuleConfig(name=name, path=path)
        
        if config_file.exists():
            config_data = self._load_yaml_config(config_file)
            if config_data:
                module_config.config_file = config_file
                module_config.prometheus_url = config_data.get('prometheus_url')
                module_config.grafana_url = config_data.get('grafana_url')
                module_config.alertmanager_url = config_data.get('alertmanager_url')
                module_config.custom_endpoints = config_data.get('custom_endpoints', {})
        else:
            # Use default endpoints if no config file
            module_config.prometheus_url = "http://localhost:9090"
            module_config.grafana_url = "http://localhost:3000"
            module_config.alertmanager_url = "http://localhost:9093"
        
        self.modules.append(module_config)
        self.logger.info(f"Loaded module configuration for '{name}'")
    
    def _load_yaml_config(self, file_path: Path) -> Optional[Dict[str, Any]]:
        """Load YAML configuration file"""
        try:
            with open(file_path, 'r') as f:
                return yaml.safe_load(f)
        except Exception as e:
            self.logger.error(f"Failed to load YAML config from {file_path}: {e}")
            return None
    
    def register_validator(self, validator_class: type, enabled: bool = True) -> None:
        """Register a validator class to run for all modules"""
        if not enabled:
            self.logger.info(f"Validator {validator_class.__name__} is disabled")
            return
            
        for module in self.modules:
            validator = validator_class(module, self.logger)
            self.validators.append(validator)
    
    def run_validations(self) -> Tuple[List[ValidationResult], Dict[str, Any]]:
        """Run all registered validators and return results"""
        all_results = []
        start_time = time.time()
        
        self.logger.info(f"Starting E2E validation for {len(self.modules)} modules")
        
        for validator in self.validators:
            self.logger.info(f"Running {validator.__class__.__name__} for module '{validator.module_config.name}'")
            try:
                results = validator.validate()
                all_results.extend(results)
            except Exception as e:
                self.logger.error(f"Validator {validator.__class__.__name__} failed: {e}")
                all_results.append(ValidationResult(
                    test_name=validator.__class__.__name__,
                    module=validator.module_config.name,
                    status="failed",
                    message=f"Validator crashed: {str(e)}",
                    duration=0,
                    timestamp=datetime.now()
                ))
        
        duration = time.time() - start_time
        summary = self._generate_summary(all_results, duration)
        
        return all_results, summary
    
    def _generate_summary(self, results: List[ValidationResult], duration: float) -> Dict[str, Any]:
        """Generate summary of test results"""
        summary = {
            "total_tests": len(results),
            "passed": sum(1 for r in results if r.status == "passed"),
            "failed": sum(1 for r in results if r.status == "failed"),
            "skipped": sum(1 for r in results if r.status == "skipped"),
            "duration": duration,
            "modules_tested": len(set(r.module for r in results)),
            "timestamp": datetime.now().isoformat()
        }
        
        # Group results by module
        by_module = {}
        for result in results:
            if result.module not in by_module:
                by_module[result.module] = {"passed": 0, "failed": 0, "skipped": 0}
            by_module[result.module][result.status] += 1
        
        summary["by_module"] = by_module
        
        return summary
    
    def save_results(self, results: List[ValidationResult], summary: Dict[str, Any], 
                    output_dir: Optional[Path] = None) -> None:
        """Save validation results to files"""
        if not output_dir:
            output_dir = self.base_path / "shared" / "e2e" / "results"
        
        output_dir.mkdir(parents=True, exist_ok=True)
        timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
        
        # Save detailed results as JSON
        results_file = output_dir / f"results_{timestamp}.json"
        results_data = [
            {
                "test_name": r.test_name,
                "module": r.module,
                "status": r.status,
                "message": r.message,
                "duration": r.duration,
                "timestamp": r.timestamp.isoformat(),
                "details": r.details
            }
            for r in results
        ]
        
        with open(results_file, 'w') as f:
            json.dump({
                "results": results_data,
                "summary": summary
            }, f, indent=2)
        
        # Save summary as markdown
        summary_file = output_dir / f"summary_{timestamp}.md"
        with open(summary_file, 'w') as f:
            f.write(f"# E2E Validation Summary\n\n")
            f.write(f"**Date**: {summary['timestamp']}\n")
            f.write(f"**Duration**: {summary['duration']:.2f} seconds\n\n")
            f.write(f"## Overall Results\n\n")
            f.write(f"- Total Tests: {summary['total_tests']}\n")
            f.write(f"- Passed: {summary['passed']}\n")
            f.write(f"- Failed: {summary['failed']}\n")
            f.write(f"- Skipped: {summary['skipped']}\n\n")
            f.write(f"## Results by Module\n\n")
            
            for module, stats in summary['by_module'].items():
                f.write(f"### {module}\n")
                f.write(f"- Passed: {stats['passed']}\n")
                f.write(f"- Failed: {stats['failed']}\n")
                f.write(f"- Skipped: {stats['skipped']}\n\n")
        
        self.logger.info(f"Results saved to {output_dir}")


def main():
    """Example usage of the validation framework"""
    import argparse
    
    parser = argparse.ArgumentParser(description="E2E Validation Framework")
    parser.add_argument("--modules", nargs="+", help="Specific modules to test")
    parser.add_argument("--log-level", default="INFO", choices=["DEBUG", "INFO", "WARNING", "ERROR"])
    parser.add_argument("--base-path", type=Path, default=Path(__file__).parent.parent.parent)
    
    args = parser.parse_args()
    
    # Initialize framework
    framework = ValidationFramework(args.base_path, args.log_level)
    
    # Discover modules
    framework.discover_modules(args.modules)
    
    # Register validators (import them here to avoid circular imports)
    # from metric_validator import MetricValidator
    # from dashboard_validator import DashboardValidator
    # from alert_validator import AlertValidator
    
    # framework.register_validator(MetricValidator)
    # framework.register_validator(DashboardValidator)
    # framework.register_validator(AlertValidator)
    
    # Run validations
    results, summary = framework.run_validations()
    
    # Save results
    framework.save_results(results, summary)
    
    # Print summary
    print(f"\nValidation Complete!")
    print(f"Total: {summary['total_tests']} | Passed: {summary['passed']} | Failed: {summary['failed']}")
    
    # Exit with error code if any tests failed
    sys.exit(1 if summary['failed'] > 0 else 0)


if __name__ == "__main__":
    main()