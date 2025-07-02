// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	// TypeStr is the type identifier for this extension
	TypeStr = "healthcheck"
)

// NewFactory creates a new factory for the health check extension
func NewFactory() extension.Factory {
	return extension.NewFactory(
		component.MustNewType(TypeStr),
		createDefaultConfig,
		createExtension,
		component.StabilityLevelAlpha,
	)
}

// createExtension creates a new health check extension
func createExtension(
	ctx context.Context,
	set extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	config := cfg.(*Config)

	ext := &HealthCheckExtension{
		config:       config,
		logger:       set.Logger,
		shutdownChan: make(chan struct{}),
	}

	return ext, nil
}