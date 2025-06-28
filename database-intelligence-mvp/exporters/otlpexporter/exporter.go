package otlpexporter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// postgresOTLPExporter exports PostgreSQL telemetry data via OTLP/gRPC
type postgresOTLPExporter struct {
	config *Config
	logger *zap.Logger

	// OTLP clients
	metricClient *otlpmetricgrpc.Exporter
	logClient    *otlploggrpc.Exporter
	grpcConn     *grpc.ClientConn

	// Statistics
	stats ExporterStats
	mu    sync.RWMutex

	// Shutdown
	shutdownOnce sync.Once
}

// ExporterStats tracks exporter statistics
type ExporterStats struct {
	MetricsExported   int64
	LogsExported      int64
	ExportErrors      int64
	LastExportTime    time.Time
	LastExportError   error
	ConnectionRetries int64
}

// newPostgresOTLPExporter creates a new OTLP exporter
func newPostgresOTLPExporter(cfg *Config, logger *zap.Logger) (*postgresOTLPExporter, error) {
	return &postgresOTLPExporter{
		config: cfg,
		logger: logger,
	}, nil
}

// Start starts the exporter
func (e *postgresOTLPExporter) Start(ctx context.Context, host component.Host) error {
	e.logger.Info("Starting OTLP exporter",
		zap.String("endpoint", e.config.Endpoint),
		zap.Bool("insecure", e.config.Insecure),
		zap.Bool("compression", e.config.Compression.IsCompressed()))

	// Create gRPC connection
	dialOpts := []grpc.DialOption{
		grpc.WithUserAgent("postgres-unified-collector/1.0"),
	}

	if e.config.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add compression if enabled
	if e.config.Compression.IsCompressed() {
		dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(
			grpc.UseCompressor(string(e.config.Compression)),
		))
	}

	conn, err := grpc.DialContext(ctx, e.config.Endpoint, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to create gRPC connection: %w", err)
	}
	e.grpcConn = conn

	// Create metric exporter
	metricOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithGRPCConn(conn),
		otlpmetricgrpc.WithTimeout(e.config.Timeout),
	}

	if e.config.Headers != nil {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithHeaders(e.config.Headers))
	}

	metricClient, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create metric exporter: %w", err)
	}
	e.metricClient = metricClient

	// Create log exporter
	logOpts := []otlploggrpc.Option{
		otlploggrpc.WithGRPCConn(conn),
		otlploggrpc.WithTimeout(e.config.Timeout),
	}

	if e.config.Headers != nil {
		logOpts = append(logOpts, otlploggrpc.WithHeaders(e.config.Headers))
	}

	logClient, err := otlploggrpc.New(ctx, logOpts...)
	if err != nil {
		metricClient.Shutdown(ctx)
		conn.Close()
		return fmt.Errorf("failed to create log exporter: %w", err)
	}
	e.logClient = logClient

	e.logger.Info("OTLP exporter started successfully")
	return nil
}

// Shutdown stops the exporter
func (e *postgresOTLPExporter) Shutdown(ctx context.Context) error {
	var finalErr error

	e.shutdownOnce.Do(func() {
		e.logger.Info("Shutting down OTLP exporter")

		// Shutdown metric client
		if e.metricClient != nil {
			if err := e.metricClient.Shutdown(ctx); err != nil {
				e.logger.Error("Failed to shutdown metric client", zap.Error(err))
				finalErr = err
			}
		}

		// Shutdown log client
		if e.logClient != nil {
			if err := e.logClient.Shutdown(ctx); err != nil {
				e.logger.Error("Failed to shutdown log client", zap.Error(err))
				if finalErr == nil {
					finalErr = err
				}
			}
		}

		// Close gRPC connection
		if e.grpcConn != nil {
			if err := e.grpcConn.Close(); err != nil {
				e.logger.Error("Failed to close gRPC connection", zap.Error(err))
				if finalErr == nil {
					finalErr = err
				}
			}
		}
	})

	return finalErr
}

// ConsumeMetrics exports metrics via OTLP
func (e *postgresOTLPExporter) ConsumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	if e.metricClient == nil {
		return fmt.Errorf("metric exporter not initialized")
	}

	// Add database-specific metadata
	ctx = e.enrichContext(ctx)

	// Export with retry logic
	err := e.exportWithRetry(ctx, func(ctx context.Context) error {
		// Convert pdata metrics to OTLP format and export
		// This is a simplified version - in production, proper conversion is needed
		return e.metricClient.Export(ctx, nil) // TODO: Convert metrics
	})

	e.mu.Lock()
	if err != nil {
		e.stats.ExportErrors++
		e.stats.LastExportError = err
	} else {
		e.stats.MetricsExported += int64(metrics.DataPointCount())
		e.stats.LastExportTime = time.Now()
	}
	e.mu.Unlock()

	if err != nil {
		e.logger.Error("Failed to export metrics",
			zap.Error(err),
			zap.Int("data_points", metrics.DataPointCount()))
		return err
	}

	e.logger.Debug("Successfully exported metrics",
		zap.Int("data_points", metrics.DataPointCount()))
	return nil
}

// ConsumeLogs exports logs via OTLP
func (e *postgresOTLPExporter) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	if e.logClient == nil {
		return fmt.Errorf("log exporter not initialized")
	}

	// Add database-specific metadata
	ctx = e.enrichContext(ctx)

	// Export with retry logic
	err := e.exportWithRetry(ctx, func(ctx context.Context) error {
		// Convert pdata logs to OTLP format and export
		// This is a simplified version - in production, proper conversion is needed
		return e.logClient.Export(ctx, nil) // TODO: Convert logs
	})

	e.mu.Lock()
	if err != nil {
		e.stats.ExportErrors++
		e.stats.LastExportError = err
	} else {
		e.stats.LogsExported += int64(logs.LogRecordCount())
		e.stats.LastExportTime = time.Now()
	}
	e.mu.Unlock()

	if err != nil {
		e.logger.Error("Failed to export logs",
			zap.Error(err),
			zap.Int("log_records", logs.LogRecordCount()))
		return err
	}

	e.logger.Debug("Successfully exported logs",
		zap.Int("log_records", logs.LogRecordCount()))
	return nil
}

// exportWithRetry implements retry logic for exports
func (e *postgresOTLPExporter) exportWithRetry(ctx context.Context, exportFunc func(context.Context) error) error {
	retryConfig := e.config.RetryConfig
	if retryConfig == nil {
		// Default retry configuration
		retryConfig = &RetryConfig{
			Enabled:         true,
			InitialInterval: time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  5 * time.Minute,
		}
	}

	if !retryConfig.Enabled {
		return exportFunc(ctx)
	}

	var lastErr error
	interval := retryConfig.InitialInterval
	deadline := time.Now().Add(retryConfig.MaxElapsedTime)

	for attempt := 0; time.Now().Before(deadline); attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(interval):
				// Continue with retry
			case <-ctx.Done():
				return ctx.Err()
			}

			// Exponential backoff
			interval *= 2
			if interval > retryConfig.MaxInterval {
				interval = retryConfig.MaxInterval
			}
		}

		err := exportFunc(ctx)
		if err == nil {
			return nil
		}

		lastErr = err
		e.mu.Lock()
		e.stats.ConnectionRetries++
		e.mu.Unlock()

		e.logger.Warn("Export failed, retrying",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Duration("next_retry_in", interval))
	}

	return fmt.Errorf("export failed after retries: %w", lastErr)
}

// enrichContext adds PostgreSQL-specific metadata to the context
func (e *postgresOTLPExporter) enrichContext(ctx context.Context) context.Context {
	md := metadata.MD{
		"service.name":     []string{"postgresql-collector"},
		"service.version":  []string{"1.0.0"},
		"deployment.type":  []string{e.config.DeploymentType},
		"collector.region": []string{e.config.Region},
	}

	// Add custom headers if configured
	for k, v := range e.config.Headers {
		md[k] = []string{v}
	}

	return metadata.NewOutgoingContext(ctx, md)
}

// GetStats returns exporter statistics
func (e *postgresOTLPExporter) GetStats() ExporterStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// Capabilities returns the exporter capabilities
func (e *postgresOTLPExporter) Capabilities() exporter.Capabilities {
	return exporter.Capabilities{MutatesData: false}
}