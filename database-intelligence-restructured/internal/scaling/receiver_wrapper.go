package scaling

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// ReceiverWrapper wraps a receiver with horizontal scaling capabilities
type ReceiverWrapper struct {
	receiver     receiver.Metrics
	coordinator  *Coordinator
	logger       *zap.Logger
	resources    []string
	checkTicker  *time.Ticker
	shutdownCh   chan struct{}
	wg           sync.WaitGroup
	
	// Configuration
	config       *ScalingConfig
}

// ScalingConfig configures the scaling behavior
type ScalingConfig struct {
	Enabled           bool
	CheckInterval     time.Duration
	ResourcePrefix    string
	IgnoreAssignments bool // For single-node deployments
}

// NewReceiverWrapper creates a new scaling-aware receiver wrapper
func NewReceiverWrapper(
	receiver receiver.Metrics,
	coordinator *Coordinator,
	config *ScalingConfig,
	logger *zap.Logger,
) *ReceiverWrapper {
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	
	return &ReceiverWrapper{
		receiver:    receiver,
		coordinator: coordinator,
		config:      config,
		logger:      logger,
		shutdownCh:  make(chan struct{}),
		resources:   make([]string, 0),
	}
}

// Start starts the receiver with scaling support
func (rw *ReceiverWrapper) Start(ctx context.Context, host component.Host) error {
	// Start the underlying receiver
	if err := rw.receiver.Start(ctx, host); err != nil {
		return fmt.Errorf("failed to start receiver: %w", err)
	}
	
	// If scaling is disabled, we're done
	if !rw.config.Enabled {
		return nil
	}
	
	// Start assignment checker
	rw.checkTicker = time.NewTicker(rw.config.CheckInterval)
	rw.wg.Add(1)
	go rw.assignmentChecker()
	
	rw.logger.Info("Receiver wrapper started with scaling support",
		zap.Bool("scaling_enabled", rw.config.Enabled),
		zap.Duration("check_interval", rw.config.CheckInterval))
	
	return nil
}

// Shutdown shuts down the receiver
func (rw *ReceiverWrapper) Shutdown(ctx context.Context) error {
	// Stop assignment checker
	if rw.config.Enabled {
		close(rw.shutdownCh)
		if rw.checkTicker != nil {
			rw.checkTicker.Stop()
		}
		rw.wg.Wait()
	}
	
	// Shutdown the underlying receiver
	return rw.receiver.Shutdown(ctx)
}

// RegisterResource registers a resource for scaling
func (rw *ReceiverWrapper) RegisterResource(resource string) error {
	if !rw.config.Enabled {
		return nil
	}
	
	// Add prefix if configured
	if rw.config.ResourcePrefix != "" {
		resource = rw.config.ResourcePrefix + resource
	}
	
	// Register with coordinator
	if err := rw.coordinator.RegisterResource(resource); err != nil {
		return fmt.Errorf("failed to register resource: %w", err)
	}
	
	// Track locally
	rw.resources = append(rw.resources, resource)
	
	rw.logger.Debug("Registered resource for scaling",
		zap.String("resource", resource))
	
	return nil
}

// IsAssignedToMe checks if a resource is assigned to this node
func (rw *ReceiverWrapper) IsAssignedToMe(resource string) bool {
	if !rw.config.Enabled || rw.config.IgnoreAssignments {
		// In single-node mode, everything is assigned to us
		return true
	}
	
	// Add prefix if configured
	if rw.config.ResourcePrefix != "" {
		resource = rw.config.ResourcePrefix + resource
	}
	
	return rw.coordinator.IsAssignedToMe(resource)
}

// assignmentChecker periodically checks for assignment changes
func (rw *ReceiverWrapper) assignmentChecker() {
	defer rw.wg.Done()
	
	for {
		select {
		case <-rw.checkTicker.C:
			rw.checkAssignments()
			
		case <-rw.shutdownCh:
			return
		}
	}
}

// checkAssignments checks for assignment changes
func (rw *ReceiverWrapper) checkAssignments() {
	// Count assignments
	assigned := 0
	total := len(rw.resources)
	
	for _, resource := range rw.resources {
		if rw.coordinator.IsAssignedToMe(resource) {
			assigned++
		}
	}
	
	rw.logger.Debug("Assignment check completed",
		zap.Int("assigned", assigned),
		zap.Int("total", total))
	
	// Log if assignments have changed significantly
	if total > 0 {
		percentage := float64(assigned) / float64(total) * 100
		if percentage < 10 || percentage > 90 {
			rw.logger.Info("Unusual assignment distribution",
				zap.Float64("percentage", percentage),
				zap.Int("assigned", assigned),
				zap.Int("total", total))
		}
	}
}

// WrapMetricsReceiver wraps a metrics receiver factory with scaling support
func WrapMetricsReceiver(
	factory receiver.Factory,
	coordinator *Coordinator,
	scalingConfig *ScalingConfig,
) receiver.Factory {
	// Create a wrapper factory
	return &scalingReceiverFactory{
		baseFactory:   factory,
		coordinator:   coordinator,
		scalingConfig: scalingConfig,
	}
}

// scalingReceiverFactory wraps a receiver factory with scaling
type scalingReceiverFactory struct {
	baseFactory   receiver.Factory
	coordinator   *Coordinator
	scalingConfig *ScalingConfig
}

// Type returns the receiver type
func (f *scalingReceiverFactory) Type() component.Type {
	return f.baseFactory.Type()
}

// CreateDefaultConfig creates the default configuration
func (f *scalingReceiverFactory) CreateDefaultConfig() component.Config {
	return f.baseFactory.CreateDefaultConfig()
}

// CreateMetricsReceiver creates a metrics receiver with scaling
func (f *scalingReceiverFactory) CreateMetricsReceiver(
	ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	// Create the base receiver
	baseReceiver, err := f.baseFactory.CreateMetricsReceiver(ctx, set, cfg, consumer)
	if err != nil {
		return nil, err
	}
	
	// Wrap with scaling support if coordinator is available
	if f.coordinator != nil && f.scalingConfig != nil && f.scalingConfig.Enabled {
		wrapper := NewReceiverWrapper(
			baseReceiver,
			f.coordinator,
			f.scalingConfig,
			set.Logger,
		)
		return wrapper, nil
	}
	
	// Return unwrapped receiver if scaling is disabled
	return baseReceiver, nil
}

// CreateTracesReceiver creates a traces receiver (not supported)
func (f *scalingReceiverFactory) CreateTracesReceiver(
	ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Traces,
) (receiver.Traces, error) {
	return nil, fmt.Errorf("traces receiver not supported")
}

// CreateLogsReceiver creates a logs receiver (not supported)
func (f *scalingReceiverFactory) CreateLogsReceiver(
	ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	return nil, fmt.Errorf("logs receiver not supported")
}

// MetricsStability returns the stability level of the metrics receiver
func (f *scalingReceiverFactory) MetricsReceiverStability() component.StabilityLevel {
	return f.baseFactory.MetricsReceiverStability()
}

// TracesStability returns the stability level of the traces receiver
func (f *scalingReceiverFactory) TracesReceiverStability() component.StabilityLevel {
	return f.baseFactory.TracesReceiverStability()
}

// LogsStability returns the stability level of the logs receiver
func (f *scalingReceiverFactory) LogsReceiverStability() component.StabilityLevel {
	return f.baseFactory.LogsReceiverStability()
}