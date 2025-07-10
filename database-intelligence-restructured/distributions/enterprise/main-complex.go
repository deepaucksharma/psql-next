package main

import (
    "fmt"
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/connector"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/receiver"
    
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
        Command:     "database-intelligence-enterprise",
        Description: "Database Intelligence Collector - Enterprise Edition",
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

    // Initialize maps
    factories.Receivers = make(map[component.Type]receiver.Factory)
    factories.Processors = make(map[component.Type]processor.Factory)
    factories.Exporters = make(map[component.Type]exporter.Factory)
    factories.Extensions = make(map[component.Type]extension.Factory)
    factories.Connectors = make(map[component.Type]connector.Factory)

    // Add standard receivers
    stdReceivers, err := standardReceivers()
    if err != nil {
        return factories, err
    }
    for k, v := range stdReceivers {
        factories.Receivers[k] = v
    }

    // Add standard processors
    stdProcessors, err := standardProcessors()
    if err != nil {
        return factories, err
    }
    for k, v := range stdProcessors {
        factories.Processors[k] = v
    }

    // Add standard exporters
    stdExporters, err := standardExporters()
    if err != nil {
        return factories, err
    }
    for k, v := range stdExporters {
        factories.Exporters[k] = v
    }

    // Add standard extensions
    stdExtensions, err := standardExtensions()
    if err != nil {
        return factories, err
    }
    for k, v := range stdExtensions {
        factories.Extensions[k] = v
    }

    // Add custom processors
    customProcessors, err := processors.Factories()
    if err != nil {
        return factories, err
    }
    for k, v := range customProcessors {
        factories.Processors[k] = v
    }

    // Add custom receivers
    customReceivers, err := receivers.Factories()
    if err != nil {
        return factories, err
    }
    for k, v := range customReceivers {
        factories.Receivers[k] = v
    }

    // Add custom exporters
    customExporters, err := exporters.Factories()
    if err != nil {
        return factories, err
    }
    for k, v := range customExporters {
        factories.Exporters[k] = v
    }

    // Add custom extensions
    customExtensions, err := extensions.Factories()
    if err != nil {
        return factories, err
    }
    for k, v := range customExtensions {
        factories.Extensions[k] = v
    }

    return factories, nil
}

func run(settings otelcol.CollectorSettings) error {
    cmd := otelcol.NewCommand(settings)
    if err := cmd.Execute(); err != nil {
        return fmt.Errorf("collector server run finished with error: %w", err)
    }
    
    return nil
}
