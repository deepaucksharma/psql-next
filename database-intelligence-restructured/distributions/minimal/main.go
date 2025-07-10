package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/exporter"
    
    // Import only essential components
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/exporter/debugexporter"
)

func main() {
    factories := otelcol.Factories{}
    
    // Minimal receivers
    factories.Receivers = map[component.Type]receiver.Factory{
        "postgresql": postgresqlreceiver.NewFactory(),
    }
    
    // Minimal processors
    factories.Processors = map[component.Type]processor.Factory{
        "batch": batchprocessor.NewFactory(),
    }
    
    // Minimal exporters
    factories.Exporters = map[component.Type]exporter.Factory{
        "prometheus": prometheusexporter.NewFactory(),
        "debug": debugexporter.NewFactory(),
    }
    
    info := component.BuildInfo{
        Command:     "database-intelligence-minimal",
        Description: "Database Intelligence Collector - Minimal Edition",
        Version:     "2.0.0",
    }

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
