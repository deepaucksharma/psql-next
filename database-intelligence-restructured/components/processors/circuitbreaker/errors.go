package circuitbreaker

import (
	"fmt"
)

// FeatureError indicates an error due to missing database features
type FeatureError struct {
	Query   string
	Feature string
	Message string
}

func (e *FeatureError) Error() string {
	return fmt.Sprintf("feature error for query %s: missing %s - %s", e.Query, e.Feature, e.Message)
}

// FallbackError indicates a fallback query should be used
type FallbackError struct {
	Query         string
	FallbackQuery string
	Feature       string
	Message       string
}

func (e *FallbackError) Error() string {
	return fmt.Sprintf("fallback required for query %s: missing %s - %s (use %s)", 
		e.Query, e.Feature, e.Message, e.FallbackQuery)
}

// IsFeatureError checks if error is due to missing features
func IsFeatureError(err error) bool {
	_, ok := err.(*FeatureError)
	return ok
}

// IsFallbackError checks if error requires fallback
func IsFallbackError(err error) bool {
	_, ok := err.(*FallbackError)
	return ok
}