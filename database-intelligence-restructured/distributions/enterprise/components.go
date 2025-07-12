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
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"
    
    // Import custom components
    ashreceiver "github.com/deepaksharma/db-otel/components/receivers/ash"
    enhancedsqlreceiver "github.com/deepaksharma/db-otel/components/receivers/enhancedsql"
    kernelmetricsreceiver "github.com/deepaksharma/db-otel/components/receivers/kernelmetrics"
    "github.com/deepaksharma/db-otel/components/processors/adaptivesampler"
    "github.com/deepaksharma/db-otel/components/processors/circuitbreaker"
    "github.com/deepaksharma/db-otel/components/processors/costcontrol"
    "github.com/deepaksharma/db-otel/components/processors/nrerrormonitor"
    "github.com/deepaksharma/db-otel/components/processors/planattributeextractor"
    "github.com/deepaksharma/db-otel/components/processors/querycorrelator"
    "github.com/deepaksharma/db-otel/components/processors/verification"
    "github.com/deepaksharma/db-otel/components/processors/ohitransform"
    "github.com/deepaksharma/db-otel/components/exporters/nri"
    "github.com/deepaksharma/db-otel/components/extensions/postgresqlquery"
)

func components() (component.Factories, error) {
    var err error
    factories := component.Factories{}

    factories.Receivers, err = receiver.MakeFactoryMap(
        otlpreceiver.NewFactory(),
        postgresqlreceiver.NewFactory(),
        mysqlreceiver.NewFactory(),
        sqlqueryreceiver.NewFactory(),
        filelogreceiver.NewFactory(),
        prometheusreceiver.NewFactory(),
        hostmetricsreceiver.NewFactory(),
        // Custom receivers
        ashreceiver.NewFactory(),
        enhancedsqlreceiver.NewFactory(),
        kernelmetricsreceiver.NewFactory(),
    )
    if err != nil {
        return component.Factories{}, err
    }

    factories.Processors, err = processor.MakeFactoryMap(
        batchprocessor.NewFactory(),
        memorylimiterprocessor.NewFactory(),
        attributesprocessor.NewFactory(),
        filterprocessor.NewFactory(),
        resourceprocessor.NewFactory(),
        transformprocessor.NewFactory(),
        cumulativetodeltaprocessor.NewFactory(),
        // Custom processors
        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory(),
        costcontrol.NewFactory(),
        nrerrormonitor.NewFactory(),
        planattributeextractor.NewFactory(),
        querycorrelator.NewFactory(),
        verification.NewFactory(),
        ohitransform.NewFactory(),
    )
    if err != nil {
        return component.Factories{}, err
    }

    factories.Exporters, err = exporter.MakeFactoryMap(
        otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory(),
        debugexporter.NewFactory(),
        fileexporter.NewFactory(),
        prometheusexporter.NewFactory(),
        // Custom exporters
        nri.NewFactory(),
    )
    if err != nil {
        return component.Factories{}, err
    }

    factories.Extensions, err = extension.MakeFactoryMap(
        ballastextension.NewFactory(),
        zpagesextension.NewFactory(),
        healthcheckextension.NewFactory(),
        pprofextension.NewFactory(),
        // Custom extensions
        postgresqlquery.NewFactory(),
    )
    if err != nil {
        return component.Factories{}, err
    }

    return factories, nil
}