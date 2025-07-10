package main

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/exporter/otlpexporter"
    "go.opentelemetry.io/collector/exporter/otlphttpexporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/extension/ballastextension"
    "go.opentelemetry.io/collector/extension/zpagesextension"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/processor/memorylimiterprocessor"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
    
    // Import contrib components
    "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
    "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
    "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
    "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"
)

// standardReceivers returns standard OTEL receivers needed for database monitoring
func standardReceivers() (map[component.Type]receiver.Factory, error) {
    receivers := map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type():        otlpreceiver.NewFactory(),
        postgresqlreceiver.NewFactory().Type():  postgresqlreceiver.NewFactory(),
        mysqlreceiver.NewFactory().Type():       mysqlreceiver.NewFactory(),
        sqlqueryreceiver.NewFactory().Type():    sqlqueryreceiver.NewFactory(),
        filelogreceiver.NewFactory().Type():     filelogreceiver.NewFactory(),
        prometheusreceiver.NewFactory().Type():  prometheusreceiver.NewFactory(),
    }
    return receivers, nil
}

// standardProcessors returns standard OTEL processors
func standardProcessors() (map[component.Type]processor.Factory, error) {
    processors := map[component.Type]processor.Factory{
        batchprocessor.NewFactory().Type():         batchprocessor.NewFactory(),
        memorylimiterprocessor.NewFactory().Type(): memorylimiterprocessor.NewFactory(),
        attributesprocessor.NewFactory().Type():    attributesprocessor.NewFactory(),
        filterprocessor.NewFactory().Type():        filterprocessor.NewFactory(),
        resourceprocessor.NewFactory().Type():      resourceprocessor.NewFactory(),
        transformprocessor.NewFactory().Type():     transformprocessor.NewFactory(),
    }
    return processors, nil
}

// standardExporters returns standard OTEL exporters
func standardExporters() (map[component.Type]exporter.Factory, error) {
    exporters := map[component.Type]exporter.Factory{
        otlpexporter.NewFactory().Type():      otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory().Type():  otlphttpexporter.NewFactory(),
        debugexporter.NewFactory().Type():     debugexporter.NewFactory(),
        fileexporter.NewFactory().Type():      fileexporter.NewFactory(),
        prometheusexporter.NewFactory().Type(): prometheusexporter.NewFactory(),
    }
    return exporters, nil
}

// standardExtensions returns standard OTEL extensions
func standardExtensions() (map[component.Type]extension.Factory, error) {
    extensions := map[component.Type]extension.Factory{
        ballastextension.NewFactory().Type():    ballastextension.NewFactory(),
        zpagesextension.NewFactory().Type():     zpagesextension.NewFactory(),
        healthcheckextension.NewFactory().Type(): healthcheckextension.NewFactory(),
        pprofextension.NewFactory().Type():      pprofextension.NewFactory(),
    }
    return extensions, nil
}