package conventions

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// CriticalAttributes defines the OpenTelemetry semantic conventions that are critical
// for New Relic entity synthesis and correlation
var CriticalAttributes = struct {
	// Service attributes
	ServiceName       string
	ServiceVersion    string
	ServiceInstanceID string
	
	// Infrastructure attributes
	HostID   string
	HostName string
	
	// Kubernetes attributes
	K8sPodUID        string
	K8sPodName       string
	K8sNamespaceName string
	K8sDeploymentName string
	K8sClusterName   string
	
	// Database attributes
	DBSystem         string
	DBName           string
	DBConnectionString string
	
	// Telemetry SDK
	TelemetrySDKLanguage string
	TelemetrySDKName     string
	TelemetrySDKVersion  string
}{
	// Service attributes - Critical for APM
	ServiceName:       "service.name",
	ServiceVersion:    "service.version",
	ServiceInstanceID: "service.instance.id",
	
	// Infrastructure - Critical for APM/Infra correlation
	HostID:   "host.id",
	HostName: "host.name",
	
	// Kubernetes - Critical for K8s correlation
	K8sPodUID:         "k8s.pod.uid",
	K8sPodName:        "k8s.pod.name",
	K8sNamespaceName:  "k8s.namespace.name",
	K8sDeploymentName: "k8s.deployment.name",
	K8sClusterName:    "k8s.cluster.name",
	
	// Database - Critical for database monitoring
	DBSystem:           "db.system",
	DBName:             "db.name",
	DBConnectionString: "db.connection_string",
	
	// Telemetry SDK - Critical for runtime-specific UI
	TelemetrySDKLanguage: "telemetry.sdk.language",
	TelemetrySDKName:     "telemetry.sdk.name",
	TelemetrySDKVersion:  "telemetry.sdk.version",
}

// Validator ensures OpenTelemetry semantic conventions are properly applied
type Validator struct {
	logger *zap.Logger
	config Config
}

// Config configures the semantic convention validator
type Config struct {
	// EnforceServiceName requires service.name to be present
	EnforceServiceName bool `mapstructure:"enforce_service_name"`
	
	// EnforceHostID requires host.id for infrastructure correlation
	EnforceHostID bool `mapstructure:"enforce_host_id"`
	
	// EnforceK8sAttributes requires Kubernetes attributes when detected
	EnforceK8sAttributes bool `mapstructure:"enforce_k8s_attributes"`
	
	// WarnOnMissing logs warnings for missing recommended attributes
	WarnOnMissing bool `mapstructure:"warn_on_missing"`
	
	// AutoCorrect attempts to derive missing attributes
	AutoCorrect bool `mapstructure:"auto_correct"`
}

// NewValidator creates a new semantic convention validator
func NewValidator(logger *zap.Logger, config Config) *Validator {
	return &Validator{
		logger: logger,
		config: config,
	}
}

// ValidateAndEnrichResource checks and enriches resource attributes
func (v *Validator) ValidateAndEnrichResource(resource pcommon.Resource) error {
	attrs := resource.Attributes()
	
	// Check critical service attributes
	if v.config.EnforceServiceName {
		if _, ok := attrs.Get(CriticalAttributes.ServiceName); !ok {
			if v.config.AutoCorrect {
				// Attempt to derive from other attributes
				if err := v.deriveServiceName(attrs); err != nil {
					return fmt.Errorf("missing required attribute %s", CriticalAttributes.ServiceName)
				}
			} else {
				return fmt.Errorf("missing required attribute %s", CriticalAttributes.ServiceName)
			}
		}
	}
	
	// Check infrastructure correlation attributes
	if v.config.EnforceHostID {
		if _, ok := attrs.Get(CriticalAttributes.HostID); !ok {
			if v.config.WarnOnMissing {
				v.logger.Warn("Missing host.id attribute - APM/Infrastructure correlation will not work",
					zap.String("service.name", v.getServiceName(attrs)))
			}
		}
	}
	
	// Check Kubernetes attributes if in K8s environment
	if v.isKubernetesEnvironment(attrs) && v.config.EnforceK8sAttributes {
		missing := v.getMissingK8sAttributes(attrs)
		if len(missing) > 0 {
			if v.config.WarnOnMissing {
				v.logger.Warn("Missing Kubernetes attributes - correlation will be limited",
					zap.Strings("missing_attributes", missing),
					zap.String("service.name", v.getServiceName(attrs)))
			}
		}
	}
	
	// Validate database attributes for database services
	if v.isDatabaseService(attrs) {
		if _, ok := attrs.Get(CriticalAttributes.DBSystem); !ok {
			if v.config.WarnOnMissing {
				v.logger.Warn("Database service missing db.system attribute",
					zap.String("service.name", v.getServiceName(attrs)))
			}
		}
	}
	
	// Ensure telemetry SDK attributes for proper UI display
	v.validateTelemetrySDKAttributes(attrs)
	
	return nil
}

// deriveServiceName attempts to derive service.name from other attributes
func (v *Validator) deriveServiceName(attrs pcommon.Map) error {
	// Try common patterns
	// 1. From container.name
	if containerName, ok := attrs.Get("container.name"); ok {
		attrs.PutStr(CriticalAttributes.ServiceName, containerName.Str())
		v.logger.Info("Derived service.name from container.name",
			zap.String("service.name", containerName.Str()))
		return nil
	}
	
	// 2. From k8s.deployment.name
	if deploymentName, ok := attrs.Get(CriticalAttributes.K8sDeploymentName); ok {
		attrs.PutStr(CriticalAttributes.ServiceName, deploymentName.Str())
		v.logger.Info("Derived service.name from k8s.deployment.name",
			zap.String("service.name", deploymentName.Str()))
		return nil
	}
	
	// 3. From process.executable.name
	if execName, ok := attrs.Get("process.executable.name"); ok {
		attrs.PutStr(CriticalAttributes.ServiceName, execName.Str())
		v.logger.Info("Derived service.name from process.executable.name",
			zap.String("service.name", execName.Str()))
		return nil
	}
	
	return fmt.Errorf("unable to derive service.name from available attributes")
}

// isKubernetesEnvironment checks if running in Kubernetes
func (v *Validator) isKubernetesEnvironment(attrs pcommon.Map) bool {
	k8sKeys := []string{
		CriticalAttributes.K8sPodName,
		CriticalAttributes.K8sNamespaceName,
		"k8s.node.name",
	}
	
	for _, key := range k8sKeys {
		if _, ok := attrs.Get(key); ok {
			return true
		}
	}
	return false
}

// isDatabaseService checks if this is a database-related service
func (v *Validator) isDatabaseService(attrs pcommon.Map) bool {
	serviceName := v.getServiceName(attrs)
	dbKeywords := []string{"postgres", "mysql", "mongodb", "redis", "cassandra", "elasticsearch"}
	
	serviceNameLower := strings.ToLower(serviceName)
	for _, keyword := range dbKeywords {
		if strings.Contains(serviceNameLower, keyword) {
			return true
		}
	}
	
	// Check if db.system is already set
	if _, ok := attrs.Get(CriticalAttributes.DBSystem); ok {
		return true
	}
	
	return false
}

// getMissingK8sAttributes returns list of missing Kubernetes attributes
func (v *Validator) getMissingK8sAttributes(attrs pcommon.Map) []string {
	required := []string{
		CriticalAttributes.K8sPodUID,
		CriticalAttributes.K8sPodName,
		CriticalAttributes.K8sNamespaceName,
	}
	
	var missing []string
	for _, attr := range required {
		if _, ok := attrs.Get(attr); !ok {
			missing = append(missing, attr)
		}
	}
	return missing
}

// validateTelemetrySDKAttributes ensures SDK attributes are present
func (v *Validator) validateTelemetrySDKAttributes(attrs pcommon.Map) {
	// These are often set by the SDK, but we should warn if missing
	if _, ok := attrs.Get(CriticalAttributes.TelemetrySDKLanguage); !ok {
		if v.config.WarnOnMissing {
			v.logger.Debug("Missing telemetry.sdk.language - runtime-specific UI features may be limited")
		}
	}
}

// getServiceName safely retrieves service.name or returns "unknown"
func (v *Validator) getServiceName(attrs pcommon.Map) string {
	if serviceName, ok := attrs.Get(CriticalAttributes.ServiceName); ok {
		return serviceName.Str()
	}
	return "unknown"
}

// ValidateMetricDataPoint validates attributes on a metric data point
func (v *Validator) ValidateMetricDataPoint(attrs pcommon.Map, metricName string) {
	// Check for high-cardinality attributes that could cause cost issues
	highCardinalityKeys := []string{"user.id", "session.id", "request.id", "trace.id", "span.id"}
	
	for _, key := range highCardinalityKeys {
		if _, ok := attrs.Get(key); ok {
			v.logger.Warn("High-cardinality attribute detected on metric - may cause cost issues",
				zap.String("metric", metricName),
				zap.String("attribute", key))
		}
	}
}