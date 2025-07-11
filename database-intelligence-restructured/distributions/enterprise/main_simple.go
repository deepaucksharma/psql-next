package main

import (
    "fmt"
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/exporter/otlpexporter"
    "go.opentelemetry.io/collector/exporter/otlphttpexporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/extension/ballastextension"
    "go.opentelemetry.io/collector/extension/zpagesextension"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/processor/memorylimiterprocessor"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
    
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
        Command:     "database-intelligence-enterprise",
        Description: "Database Intelligence Collector - Enterprise Edition (New Relic Only)",
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

    // Receivers
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type(): otlpreceiver.NewFactory(),
    }

    // Processors - Core + Custom
    factories.Processors = map[component.Type]processor.Factory{
        batchprocessor.NewFactory().Type():         batchprocessor.NewFactory(),
        memorylimiterprocessor.NewFactory().Type(): memorylimiterprocessor.NewFactory(),
        // Custom processors
        adaptivesampler.NewFactory().Type():        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory().Type():         circuitbreaker.NewFactory(),
        costcontrol.NewFactory().Type():            costcontrol.NewFactory(),
        nrerrormonitor.NewFactory().Type():         nrerrormonitor.NewFactory(),
        planattributeextractor.NewFactory().Type(): planattributeextractor.NewFactory(),
        querycorrelator.NewFactory().Type():        querycorrelator.NewFactory(),
        verification.NewFactory().Type():           verification.NewFactory(),
    }

    // Exporters  
    factories.Exporters = map[component.Type]exporter.Factory{
        otlpexporter.NewFactory().Type():      otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory().Type():  otlphttpexporter.NewFactory(),
        debugexporter.NewFactory().Type():     debugexporter.NewFactory(),
    }

    // Extensions
    factories.Extensions = map[component.Type]extension.Factory{
        ballastextension.NewFactory().Type():      ballastextension.NewFactory(),
        zpagesextension.NewFactory().Type():       zpagesextension.NewFactory(),
    }

    // Initialize empty connectors map
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