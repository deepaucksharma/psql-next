package circuitbreaker

import (
	"context"
	"regexp"
	"sync"
	"time"
	
	"github.com/database-intelligence/common/featuredetector"
	"go.uber.org/zap"
)

// FeatureAwareCircuitBreaker extends the circuit breaker with feature detection awareness
type FeatureAwareCircuitBreaker struct {
	*CircuitBreaker
	
	// Feature-specific breakers
	featureBreakers map[string]*CircuitBreaker
	featureMutex    sync.RWMutex
	
	// Error patterns that indicate missing features
	errorPatterns []ErrorPattern
	
	// Feature detector for checking capabilities
	detector featuredetector.Detector
	
	// Query fallback map
	queryFallbacks map[string]string
	
	// Disabled queries due to missing features
	disabledQueries map[string]DisabledQuery
	disabledMutex   sync.RWMutex
}

// ErrorPattern defines patterns that trigger feature-specific actions
type ErrorPattern struct {
	Name        string         `mapstructure:"name"`
	Pattern     string         `mapstructure:"pattern"`
	Regex       *regexp.Regexp `mapstructure:"-"`
	Action      string         `mapstructure:"action"` // "disable_query", "use_fallback", "circuit_break"
	Feature     string         `mapstructure:"feature"` // Feature that caused the error
	Backoff     time.Duration  `mapstructure:"backoff"`
	Description string         `mapstructure:"description"`
}

// DisabledQuery tracks queries disabled due to missing features
type DisabledQuery struct {
	QueryName    string
	Feature      string
	DisabledAt   time.Time
	ReenableAt   time.Time
	ErrorMessage string
	Attempts     int
}

// NewFeatureAwareCircuitBreaker creates a feature-aware circuit breaker
func NewFeatureAwareCircuitBreaker(config *Config, logger *zap.Logger) *FeatureAwareCircuitBreaker {
	facb := &FeatureAwareCircuitBreaker{
		CircuitBreaker:  NewCircuitBreaker(config, logger),
		featureBreakers: make(map[string]*CircuitBreaker),
		errorPatterns:   make([]ErrorPattern, 0),
		queryFallbacks:  make(map[string]string),
		disabledQueries: make(map[string]DisabledQuery),
	}
	
	// Initialize error patterns
	facb.initializeErrorPatterns()
	
	return facb
}

// initializeErrorPatterns sets up common database error patterns
func (facb *FeatureAwareCircuitBreaker) initializeErrorPatterns() {
	patterns := []ErrorPattern{
		// PostgreSQL extension errors
		{
			Name:        "pg_stat_statements_missing",
			Pattern:     `relation "pg_stat_statements" does not exist`,
			Action:      "disable_query",
			Feature:     featuredetector.ExtPgStatStatements,
			Backoff:     30 * time.Minute,
			Description: "pg_stat_statements extension not installed",
		},
		{
			Name:        "pg_stat_monitor_missing",
			Pattern:     `relation "pg_stat_monitor" does not exist`,
			Action:      "use_fallback",
			Feature:     featuredetector.ExtPgStatMonitor,
			Backoff:     5 * time.Minute,
			Description: "pg_stat_monitor extension not installed",
		},
		{
			Name:        "pg_wait_sampling_missing",
			Pattern:     `relation "pg_wait_sampling.*" does not exist`,
			Action:      "disable_query",
			Feature:     featuredetector.ExtPgWaitSampling,
			Backoff:     30 * time.Minute,
			Description: "pg_wait_sampling extension not installed",
		},
		{
			Name:        "extension_not_loaded",
			Pattern:     `function .* does not exist|extension .* not installed`,
			Action:      "disable_query",
			Feature:     "unknown_extension",
			Backoff:     1 * time.Hour,
			Description: "Required extension not loaded",
		},
		
		// MySQL Performance Schema errors
		{
			Name:        "performance_schema_disabled",
			Pattern:     `Table 'performance_schema\..*' doesn't exist|performance_schema not enabled`,
			Action:      "use_fallback",
			Feature:     featuredetector.CapPerfSchemaEnabled,
			Backoff:     30 * time.Minute,
			Description: "Performance Schema not enabled",
		},
		{
			Name:        "events_statements_missing",
			Pattern:     `Table 'performance_schema\.events_statements.*' doesn't exist`,
			Action:      "use_fallback",
			Feature:     featuredetector.CapPerfSchemaStatementsDigest,
			Backoff:     30 * time.Minute,
			Description: "Statement events not available",
		},
		
		// Permission errors
		{
			Name:        "permission_denied",
			Pattern:     `permission denied|access denied|insufficient privileges`,
			Action:      "disable_query",
			Feature:     "permissions",
			Backoff:     1 * time.Hour,
			Description: "Insufficient permissions",
		},
		
		// Connection errors
		{
			Name:        "connection_failed",
			Pattern:     `connection refused|connection reset|no route to host`,
			Action:      "circuit_break",
			Feature:     "connection",
			Backoff:     30 * time.Second,
			Description: "Database connection failed",
		},
		{
			Name:        "too_many_connections",
			Pattern:     `too many connections|connection limit exceeded`,
			Action:      "circuit_break",
			Feature:     "connection_limit",
			Backoff:     1 * time.Minute,
			Description: "Connection limit reached",
		},
		
		// Query timeout
		{
			Name:        "query_timeout",
			Pattern:     `statement timeout|query timeout|canceling statement due to statement timeout`,
			Action:      "circuit_break",
			Feature:     "query_performance",
			Backoff:     30 * time.Second,
			Description: "Query execution timeout",
		},
		
		// Cloud-specific errors
		{
			Name:        "rds_feature_unavailable",
			Pattern:     `feature not supported on.*RDS|RDS does not support`,
			Action:      "disable_query",
			Feature:     "cloud_limitation",
			Backoff:     24 * time.Hour,
			Description: "Feature not available on RDS",
		},
	}
	
	// Compile regex patterns
	for i := range patterns {
		regex, err := regexp.Compile(patterns[i].Pattern)
		if err != nil {
			facb.CircuitBreaker.logger.Warn("Failed to compile error pattern",
				zap.String("pattern", patterns[i].Pattern),
				zap.Error(err))
			continue
		}
		patterns[i].Regex = regex
		facb.errorPatterns = append(facb.errorPatterns, patterns[i])
	}
}

// ProcessError processes an error with feature awareness
func (facb *FeatureAwareCircuitBreaker) ProcessError(ctx context.Context, err error, queryName string) error {
	if err == nil {
		return nil
	}
	
	// Check if query is disabled
	if facb.isQueryDisabled(queryName) {
		return &FeatureError{
			Query:   queryName,
			Feature: facb.getDisabledFeature(queryName),
			Message: "query disabled due to missing feature",
		}
	}
	
	// Check error patterns
	errorStr := err.Error()
	for _, pattern := range facb.errorPatterns {
		if pattern.Regex != nil && pattern.Regex.MatchString(errorStr) {
			facb.CircuitBreaker.logger.Info("Error pattern matched",
				zap.String("pattern", pattern.Name),
				zap.String("action", pattern.Action),
				zap.String("feature", pattern.Feature),
				zap.String("query", queryName))
			
			switch pattern.Action {
			case "disable_query":
				facb.disableQuery(queryName, pattern.Feature, pattern.Backoff, errorStr)
				return &FeatureError{
					Query:   queryName,
					Feature: pattern.Feature,
					Message: pattern.Description,
				}
				
			case "use_fallback":
				if fallback := facb.getFallbackQuery(queryName); fallback != "" {
					return &FallbackError{
						Query:         queryName,
						FallbackQuery: fallback,
						Feature:       pattern.Feature,
						Message:       pattern.Description,
					}
				}
				// No fallback available, disable query
				facb.disableQuery(queryName, pattern.Feature, pattern.Backoff, errorStr)
				return &FeatureError{
					Query:   queryName,
					Feature: pattern.Feature,
					Message: "no fallback available: " + pattern.Description,
				}
				
			case "circuit_break":
				// Use feature-specific circuit breaker
				breaker := facb.getFeatureBreaker(pattern.Feature)
				breaker.RecordError(err)
				return nil
			}
		}
	}
	
	// Default circuit breaker behavior
	facb.CircuitBreaker.RecordError(err)
	return nil
}

// ProcessSuccess processes a successful operation
func (facb *FeatureAwareCircuitBreaker) ProcessSuccess(queryName string) {
	// Re-enable query if it was disabled
	facb.reenableQuery(queryName)
	
	// Record success in main breaker
	facb.CircuitBreaker.RecordSuccess()
}

// disableQuery disables a query due to missing features
func (facb *FeatureAwareCircuitBreaker) disableQuery(queryName, feature string, backoff time.Duration, errorMsg string) {
	facb.disabledMutex.Lock()
	defer facb.disabledMutex.Unlock()
	
	disabled, exists := facb.disabledQueries[queryName]
	if exists {
		disabled.Attempts++
		disabled.ReenableAt = time.Now().Add(backoff * time.Duration(disabled.Attempts))
	} else {
		disabled = DisabledQuery{
			QueryName:    queryName,
			Feature:      feature,
			DisabledAt:   time.Now(),
			ReenableAt:   time.Now().Add(backoff),
			ErrorMessage: errorMsg,
			Attempts:     1,
		}
	}
	
	facb.disabledQueries[queryName] = disabled
	
	facb.CircuitBreaker.logger.Warn("Query disabled due to missing feature",
		zap.String("query", queryName),
		zap.String("feature", feature),
		zap.Time("reenable_at", disabled.ReenableAt),
		zap.Int("attempts", disabled.Attempts))
}

// isQueryDisabled checks if a query is currently disabled
func (facb *FeatureAwareCircuitBreaker) isQueryDisabled(queryName string) bool {
	facb.disabledMutex.RLock()
	defer facb.disabledMutex.RUnlock()
	
	disabled, exists := facb.disabledQueries[queryName]
	if !exists {
		return false
	}
	
	// Check if it's time to re-enable
	if time.Now().After(disabled.ReenableAt) {
		return false
	}
	
	return true
}

// getDisabledFeature returns the feature that caused query to be disabled
func (facb *FeatureAwareCircuitBreaker) getDisabledFeature(queryName string) string {
	facb.disabledMutex.RLock()
	defer facb.disabledMutex.RUnlock()
	
	if disabled, exists := facb.disabledQueries[queryName]; exists {
		return disabled.Feature
	}
	
	return "unknown"
}

// reenableQuery re-enables a previously disabled query
func (facb *FeatureAwareCircuitBreaker) reenableQuery(queryName string) {
	facb.disabledMutex.Lock()
	defer facb.disabledMutex.Unlock()
	
	if _, exists := facb.disabledQueries[queryName]; exists {
		delete(facb.disabledQueries, queryName)
		facb.CircuitBreaker.logger.Info("Query re-enabled after successful execution",
			zap.String("query", queryName))
	}
}

// getFeatureBreaker gets or creates a circuit breaker for a specific feature
func (facb *FeatureAwareCircuitBreaker) getFeatureBreaker(feature string) *CircuitBreaker {
	facb.featureMutex.RLock()
	breaker, exists := facb.featureBreakers[feature]
	facb.featureMutex.RUnlock()
	
	if exists {
		return breaker
	}
	
	// Create new breaker for this feature
	facb.featureMutex.Lock()
	defer facb.featureMutex.Unlock()
	
	// Double-check after acquiring write lock
	if breaker, exists := facb.featureBreakers[feature]; exists {
		return breaker
	}
	
	// Create feature-specific config
	featureConfig := *facb.CircuitBreaker.config
	featureConfig.Timeout = 30 * time.Second // Shorter timeout for features
	
	breaker = NewCircuitBreaker(&featureConfig, facb.CircuitBreaker.logger.With(zap.String("feature", feature)))
	facb.featureBreakers[feature] = breaker
	
	return breaker
}

// SetFallbackQuery sets a fallback query for a primary query
func (facb *FeatureAwareCircuitBreaker) SetFallbackQuery(primaryQuery, fallbackQuery string) {
	facb.queryFallbacks[primaryQuery] = fallbackQuery
}

// getFallbackQuery returns the fallback query if available
func (facb *FeatureAwareCircuitBreaker) getFallbackQuery(queryName string) string {
	return facb.queryFallbacks[queryName]
}

// GetDisabledQueries returns currently disabled queries
func (facb *FeatureAwareCircuitBreaker) GetDisabledQueries() []DisabledQuery {
	facb.disabledMutex.RLock()
	defer facb.disabledMutex.RUnlock()
	
	disabled := make([]DisabledQuery, 0, len(facb.disabledQueries))
	for _, d := range facb.disabledQueries {
		disabled = append(disabled, d)
	}
	
	return disabled
}

// SetDetector sets the feature detector
func (facb *FeatureAwareCircuitBreaker) SetDetector(detector featuredetector.Detector) {
	facb.detector = detector
}

// ValidateQueryRequirements validates if a query can run with current features
func (facb *FeatureAwareCircuitBreaker) ValidateQueryRequirements(query *featuredetector.QueryDefinition) error {
	if facb.detector == nil {
		return nil // No detector, assume query can run
	}
	
	return facb.detector.ValidateQuery(query)
}