"""
E2E Validation Framework for Database Intelligence Monorepo

This package provides comprehensive end-to-end testing capabilities
for validating metrics collection, dashboard rendering, and alert
configurations across all modules in the monorepo.
"""

from .validation_framework import (
    ValidationFramework,
    ValidationResult,
    ModuleConfig,
    BaseValidator
)

from .metric_validator import MetricValidator
from .dashboard_validator import DashboardValidator
from .alert_validator import AlertValidator

__version__ = "1.0.0"

__all__ = [
    "ValidationFramework",
    "ValidationResult",
    "ModuleConfig",
    "BaseValidator",
    "MetricValidator",
    "DashboardValidator",
    "AlertValidator"
]