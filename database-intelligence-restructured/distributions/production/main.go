package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/confmap"
    "go.opentelemetry.io/collector/confmap/provider/envprovider"
    "go.opentelemetry.io/collector/confmap/provider/fileprovider"
    "go.opentelemetry.io/collector/confmap/provider/yamlprovider"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/exporter/otlpexporter"
    "go.opentelemetry.io/collector/exporter/otlphttpexporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/processor/memorylimiterprocessor"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
    
    "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
    "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatal(err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence",
        Description: "Database Intelligence Collector - Production Build",
        Version:     "2.0.0",
    }

    configProviderSettings := otelcol.ConfigProviderSettings{
        ResolverSettings: confmap.ResolverSettings{
            ProviderFactories: []confmap.ProviderFactory{
                fileprovider.NewFactory(),
                envprovider.NewFactory(),
                yamlprovider.NewFactory(),
            },
        },
    }
    
    set := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: func() (otelcol.Factories, error) {
            return factories, nil
        },
        ConfigProviderSettings: configProviderSettings,
    }
    
    cmd := otelcol.NewCommand(set)
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    factories := otelcol.Factories{}
    
    // Extensions
    factories.Extensions = map[component.Type]extension.Factory{
        healthcheckextension.NewFactory().Type(): healthcheckextension.NewFactory(),
    }
    
    // Receivers
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type():        otlpreceiver.NewFactory(),
        mysqlreceiver.NewFactory().Type():       mysqlreceiver.NewFactory(),
        postgresqlreceiver.NewFactory().Type():  postgresqlreceiver.NewFactory(),
    }
    
    // Processors
    factories.Processors = map[component.Type]processor.Factory{
        batchprocessor.NewFactory().Type():           batchprocessor.NewFactory(),
        memorylimiterprocessor.NewFactory().Type():   memorylimiterprocessor.NewFactory(),
    }
    
    // Exporters
    factories.Exporters = map[component.Type]exporter.Factory{
        debugexporter.NewFactory().Type():      debugexporter.NewFactory(),
        otlpexporter.NewFactory().Type():       otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory().Type():   otlphttpexporter.NewFactory(),
        fileexporter.NewFactory().Type():      fileexporter.NewFactory(),
    }
    
    return factories, nil
}
