package featuredetector

import (
	"errors"
	"fmt"
)

// Common errors
var (
	// ErrNoFeatureData indicates no feature data is available
	ErrNoFeatureData = errors.New("no feature data available")
	
	// ErrDetectionFailed indicates feature detection failed
	ErrDetectionFailed = errors.New("feature detection failed")
	
	// ErrNoQueries indicates no queries provided
	ErrNoQueries = errors.New("no queries provided")
	
	// ErrDatabaseNotSupported indicates database type is not supported
	ErrDatabaseNotSupported = errors.New("database type not supported")
	
	// ErrConnectionFailed indicates database connection failed
	ErrConnectionFailed = errors.New("database connection failed")
)

// MissingFeatureError indicates a required feature is missing
type MissingFeatureError struct {
	FeatureType string // "extension" or "capability"
	FeatureName string
}

func (e *MissingFeatureError) Error() string {
	return fmt.Sprintf("missing required %s: %s", e.FeatureType, e.FeatureName)
}

// VersionMismatchError indicates version requirements not met
type VersionMismatchError struct {
	RequiredVersion string
	ActualVersion   string
}

func (e *VersionMismatchError) Error() string {
	return fmt.Sprintf("version mismatch: required %s, got %s", e.RequiredVersion, e.ActualVersion)
}

// DetectionError wraps errors during detection
type DetectionError struct {
	Phase   string // "extensions", "capabilities", "cloud", etc.
	Message string
	Err     error
}

func (e *DetectionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("detection error in %s: %s: %v", e.Phase, e.Message, e.Err)
	}
	return fmt.Sprintf("detection error in %s: %s", e.Phase, e.Message)
}

func (e *DetectionError) Unwrap() error {
	return e.Err
}

// IsMissingFeature checks if error is due to missing feature
func IsMissingFeature(err error) bool {
	var mfe *MissingFeatureError
	return errors.As(err, &mfe)
}

// IsVersionMismatch checks if error is due to version mismatch
func IsVersionMismatch(err error) bool {
	var vme *VersionMismatchError
	return errors.As(err, &vme)
}

// IsDetectionError checks if error occurred during detection
func IsDetectionError(err error) bool {
	var de *DetectionError
	return errors.As(err, &de)
}