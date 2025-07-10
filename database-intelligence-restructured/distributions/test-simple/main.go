package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/confmap"
    "go.opentelemetry.io/collector/confmap/converter/expandconverter"
    "go.opentelemetry.io/collector/confmap/provider/envprovider"
    "go.opentelemetry.io/collector/confmap/provider/fileprovider"
    "go.opentelemetry.io/collector/confmap/provider/httpprovider"
    "go.opentelemetry.io/collector/confmap/provider/httpsprovider"
    "go.opentelemetry.io/collector/confmap/provider/yamlprovider"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/receiver"
    
    "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
)

func main() {
    info := component.BuildInfo{
        Command:     "otelcol-test",
        Description: "Test OpenTelemetry Collector for database receivers",
        Version:     "1.0.0",
    }

    set := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: components,
        ConfigProviderSettings: otelcol.ConfigProviderSettings{
            ResolverSettings: confmap.ResolverSettings{
                URIs: []string{confmap.SchemeFile + ":config.yaml"},
                ProviderFactories: []confmap.ProviderFactory{
                    fileprovider.NewFactory(),
                    envprovider.NewFactory(),
                    yamlprovider.NewFactory(),
                    httpprovider.NewFactory(),
                    httpsprovider.NewFactory(),
                },
                ConverterFactories: []confmap.ConverterFactory{
                    expandconverter.NewFactory(),
                },
            },
        },
    }

    cmd := otelcol.NewCommand(set)
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    var err error
    factories := otelcol.Factories{}

    factories.Extensions, err = extension.MakeFactoryMap(
        healthcheckextension.NewFactory(),
    )
    if err != nil {
        return otelcol.Factories{}, err
    }

    factories.Receivers, err = receiver.MakeFactoryMap(
        mysqlreceiver.NewFactory(),
        postgresqlreceiver.NewFactory(),
    )
    if err != nil {
        return otelcol.Factories{}, err
    }

    factories.Processors, err = processor.MakeFactoryMap(
        batchprocessor.NewFactory(),
    )
    if err != nil {
        return otelcol.Factories{}, err
    }

    factories.Exporters, err = exporter.MakeFactoryMap(
        debugexporter.NewFactory(),
    )
    if err != nil {
        return otelcol.Factories{}, err
    }

    return factories, nil
}