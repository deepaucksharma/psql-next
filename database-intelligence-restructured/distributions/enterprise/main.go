package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/exporter/otlpexporter"
    "go.opentelemetry.io/collector/exporter/otlphttpexporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
    "go.uber.org/zap"
    
    // Import health checker
    "github.com/database-intelligence/core/internal/health"
    
    // Import unified component registry
    "github.com/database-intelligence/core/registry"
)

func main() {
    // Create logger
    logger, err := zap.NewProduction()
    if err != nil {
        log.Fatalf("failed to create logger: %v", err)
    }
    defer logger.Sync()
    
    factories, err := components()
    if err != nil {
        log.Fatalf("failed to build components: %v", err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence-collector",
        Description: "Database Intelligence Collector with Custom Processors",
        Version:     "2.0.0",
    }
    
    // Create health checker
    healthChecker := health.NewHealthChecker(logger, info.Version)
    
    // Start health monitoring server
    startHealthServer(healthChecker, logger)
    
    // Setup signal handling
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        logger.Info("Shutting down collector...")
        cancel()
    }()
    
    // Start background health checks
    go healthChecker.StartBackgroundCheck(ctx)

    if err := run(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    // Use the unified component registry for enterprise distribution
    return registry.BuildFromPreset("enterprise")
}

func run(settings otelcol.CollectorSettings) error {
    cmd := otelcol.NewCommand(settings)
    if err := cmd.Execute(); err != nil {
        return fmt.Errorf("collector server run finished with error: %w", err)
    }
    
    return nil
}

// startHealthServer starts the health monitoring HTTP server
func startHealthServer(healthChecker *health.HealthChecker, logger *zap.Logger) {
    mux := http.NewServeMux()
    
    // Register health endpoints
    mux.HandleFunc("/health", healthChecker.ReadinessHandler())
    mux.HandleFunc("/health/live", healthChecker.LivenessHandler())
    mux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
    mux.HandleFunc("/health/detail", healthChecker.DetailedHealthHandler())
    
    // Add metrics endpoint
    mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        // In production, this would expose Prometheus metrics
        w.Header().Set("Content-Type", "text/plain")
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "# HELP database_intelligence_health Health status\n")
        fmt.Fprintf(w, "# TYPE database_intelligence_health gauge\n")
        
        status := healthChecker.CheckHealth(r.Context())
        if status.Healthy {
            fmt.Fprintf(w, "database_intelligence_health 1\n")
        } else {
            fmt.Fprintf(w, "database_intelligence_health 0\n")
        }
    })
    
    server := &http.Server{
        Addr:         ":8080",
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    go func() {
        logger.Info("Starting health monitoring server", zap.String("addr", server.Addr))
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Error("Health server error", zap.Error(err))
        }
    }()
}
