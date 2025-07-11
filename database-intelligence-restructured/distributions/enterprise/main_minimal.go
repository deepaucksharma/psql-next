package main

import (
    "fmt"
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    // Import custom processors
    "github.com/deepaksharma/db-otel/components/processors/adaptivesampler"
    "github.com/deepaksharma/db-otel/components/processors/circuitbreaker"
    "github.com/deepaksharma/db-otel/components/processors/costcontrol"
    "github.com/deepaksharma/db-otel/components/processors/nrerrormonitor"
    "github.com/deepaksharma/db-otel/components/processors/planattributeextractor"
    "github.com/deepaksharma/db-otel/components/processors/querycorrelator"
    "github.com/deepaksharma/db-otel/components/processors/verification"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatalf("failed to build components: %v", err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence-minimal",
        Description: "Database Intelligence Collector - Minimal Edition",
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
    factories := otelcol.Factories{}

    // Custom processors only
    factories.Processors = map[component.Type]component.Factory{
        adaptivesampler.NewFactory().Type():        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory().Type():         circuitbreaker.NewFactory(),
        costcontrol.NewFactory().Type():            costcontrol.NewFactory(),
        nrerrormonitor.NewFactory().Type():         nrerrormonitor.NewFactory(),
        planattributeextractor.NewFactory().Type(): planattributeextractor.NewFactory(),
        querycorrelator.NewFactory().Type():        querycorrelator.NewFactory(),
        verification.NewFactory().Type():           verification.NewFactory(),
    }

    // Initialize empty maps for other components
    factories.Receivers = make(map[component.Type]component.Factory)
    factories.Exporters = make(map[component.Type]component.Factory)
    factories.Extensions = make(map[component.Type]component.Factory)
    factories.Connectors = make(map[component.Type]component.Factory)

    return factories, nil
}

func run(settings otelcol.CollectorSettings) error {
    cmd := otelcol.NewCommand(settings)
    if err := cmd.Execute(); err != nil {
        return fmt.Errorf("collector server run finished with error: %w", err)
    }
    
    return nil
}