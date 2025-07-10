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
    "go.opentelemetry.io/collector/otelcol"
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
    
    // Use the unified component registry for minimal distribution
    factories, err := registry.BuildFromPreset("minimal")
    if err != nil {
        logger.Fatal("Failed to build components", zap.Error(err))
    }
    
    info := component.BuildInfo{
        Command:     "database-intelligence-minimal",
        Description: "Database Intelligence Collector - Minimal Edition",
        Version:     "2.0.0",
    }
    
    // Create health checker
    healthChecker := health.NewHealthChecker(logger, info.Version)
    
    // Start minimal health monitoring server (port 8081 to avoid conflicts)
    startHealthServer(healthChecker, logger, ":8081")
    
    // Setup signal handling
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        logger.Info("Shutting down minimal collector...")
        cancel()
    }()
    
    // Start background health checks
    go healthChecker.StartBackgroundCheck(ctx)

    settings := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: func() (otelcol.Factories, error) {
            return factories, nil
        },
    }

    if err := otelcol.NewCommand(settings).Execute(); err != nil {
        log.Fatal(err)
    }
}

// startHealthServer starts the health monitoring HTTP server
func startHealthServer(healthChecker *health.HealthChecker, logger *zap.Logger, addr string) {
    mux := http.NewServeMux()
    
    // Register health endpoints
    mux.HandleFunc("/health", healthChecker.ReadinessHandler())
    mux.HandleFunc("/health/live", healthChecker.LivenessHandler())
    mux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
    
    server := &http.Server{
        Addr:         addr,
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
