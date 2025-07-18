package main

import (
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"

	// Core exporters
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	// Core processors
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"

	// Core receivers
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	// Core extensions
	"go.opentelemetry.io/collector/extension/zpagesextension"

	// Contrib components
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

	// Custom components - conditionally included based on profile
	"github.com/database-intelligence/db-intel/components/exporters/nri"
	"github.com/database-intelligence/db-intel/components/processors/adaptivesampler"
	"github.com/database-intelligence/db-intel/components/processors/circuitbreaker"
	"github.com/database-intelligence/db-intel/components/processors/costcontrol"
	"github.com/database-intelligence/db-intel/components/processors/planattributeextractor"
	"github.com/database-intelligence/db-intel/components/processors/querycorrelator"
	"github.com/database-intelligence/db-intel/components/receivers/ash"
	"github.com/database-intelligence/db-intel/components/receivers/enhancedsql"
	"github.com/database-intelligence/db-intel/components/receivers/kernelmetrics"
)

// MinimalComponents returns factories for minimal distribution
func MinimalComponents() (otelcol.Factories, error) {
	var err error
	factories := otelcol.Factories{}

	factories.Extensions, err = extension.MakeFactoryMap(
		healthcheckextension.NewFactory(),
		zpagesextension.NewFactory(),
	)
	if err != nil {
		return factories, err
	}

	factories.Receivers, err = receiver.MakeFactoryMap(
		otlpreceiver.NewFactory(),
		postgresqlreceiver.NewFactory(),
		mysqlreceiver.NewFactory(),
		sqlqueryreceiver.NewFactory(),
	)
	if err != nil {
		return factories, err
	}

	factories.Exporters, err = exporter.MakeFactoryMap(
		debugexporter.NewFactory(),
		otlpexporter.NewFactory(),
		otlphttpexporter.NewFactory(),
		fileexporter.NewFactory(),
	)
	if err != nil {
		return factories, err
	}

	factories.Processors, err = processor.MakeFactoryMap(
		batchprocessor.NewFactory(),
		memorylimiterprocessor.NewFactory(),
		attributesprocessor.NewFactory(),
		filterprocessor.NewFactory(),
		resourceprocessor.NewFactory(),
	)
	if err != nil {
		return factories, err
	}

	factories.Connectors, err = connector.MakeFactoryMap()
	if err != nil {
		return factories, err
	}

	return factories, nil
}

// StandardComponents returns factories for standard distribution
func StandardComponents() (otelcol.Factories, error) {
	factories, err := MinimalComponents()
	if err != nil {
		return factories, err
	}

	// Add standard profile components
	standardExtensions := []extension.Factory{
		pprofextension.NewFactory(),
	}

	standardReceivers := []receiver.Factory{
		prometheusreceiver.NewFactory(),
		ash.NewFactory(),
		enhancedsql.NewFactory(),
		kernelmetrics.NewFactory(),
	}

	standardProcessors := []processor.Factory{
		transformprocessor.NewFactory(),
		adaptivesampler.NewFactory(),
		circuitbreaker.NewFactory(),
		planattributeextractor.NewFactory(),
		querycorrelator.NewFactory(),
		costcontrol.NewFactory(),
	}

	standardExporters := []exporter.Factory{
		prometheusexporter.NewFactory(),
		nri.NewFactory(),
	}

	// Merge additional components
	for _, ext := range standardExtensions {
		factories.Extensions[ext.Type()] = ext
	}
	for _, rcv := range standardReceivers {
		factories.Receivers[rcv.Type()] = rcv
	}
	for _, proc := range standardProcessors {
		factories.Processors[proc.Type()] = proc
	}
	for _, exp := range standardExporters {
		factories.Exporters[exp.Type()] = exp
	}

	return factories, nil
}

// EnterpriseComponents returns factories for enterprise distribution
func EnterpriseComponents() (otelcol.Factories, error) {
	// Enterprise includes everything from standard
	return StandardComponents()
}