package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    // Import standard components
    "github.com/database-intelligence/processors"
    "github.com/database-intelligence/receivers"
    "github.com/database-intelligence/exporters"
    "github.com/database-intelligence/extensions"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatalf("failed to build components: %v", err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence-standard",
        Description: "Database Intelligence Collector - Standard Edition",
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
    var err error
    factories := otelcol.Factories{}

    // Add custom processors (subset)
    factories.Processors = map[string]processor.Factory{
        "adaptivesampler": processors.AdaptiveSamplerFactory(),
        "circuitbreaker": processors.CircuitBreakerFactory(),
    }

    // Standard receivers
    factories.Receivers, err = receivers.Factories()
    if err != nil {
        return factories, err
    }

    // Standard exporters
    factories.Exporters, err = exporters.Factories()
    if err != nil {
        return factories, err
    }

    return factories, nil
}
