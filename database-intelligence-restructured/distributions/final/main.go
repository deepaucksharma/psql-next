package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatal(err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence",
        Description: "Database Intelligence Collector",
        Version:     "2.0.0",
    }

    set := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }
    
    cmd := otelcol.NewCommand(set)
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    factories := otelcol.Factories{}
    
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type(): otlpreceiver.NewFactory(),
    }
    
    factories.Exporters = map[component.Type]exporter.Factory{
        debugexporter.NewFactory().Type(): debugexporter.NewFactory(),
    }
    
    return factories, nil
}
