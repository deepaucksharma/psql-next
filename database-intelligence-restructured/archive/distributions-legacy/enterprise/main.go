package main

import (
    "fmt"
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    "go.uber.org/zap"
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

    if err := run(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    receivers, err := standardReceivers()
    if err != nil {
        return otelcol.Factories{}, err
    }

    processors, err := standardProcessors()
    if err != nil {
        return otelcol.Factories{}, err
    }

    exporters, err := standardExporters()
    if err != nil {
        return otelcol.Factories{}, err
    }

    extensions, err := standardExtensions()
    if err != nil {
        return otelcol.Factories{}, err
    }

    return otelcol.Factories{
        Receivers:  receivers,
        Processors: processors,
        Exporters:  exporters,
        Extensions: extensions,
    }, nil
}

func run(settings otelcol.CollectorSettings) error {
    cmd := otelcol.NewCommand(settings)
    if err := cmd.Execute(); err != nil {
        return fmt.Errorf("collector server run finished with error: %w", err)
    }
    
    return nil
}

