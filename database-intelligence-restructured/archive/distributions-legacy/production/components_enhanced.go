// Enhanced components file that includes both standard and custom components
// This is the correct way to register all components for the production build

package main

import (
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	
	// Standard components
	forwardconnector "go.opentelemetry.io/collector/connector/forwardconnector"
	debugexporter "go.opentelemetry.io/collector/exporter/debugexporter"
	otlpexporter "go.opentelemetry.io/collector/exporter/otlpexporter"
	otlphttpexporter "go.opentelemetry.io/collector/exporter/otlphttpexporter"
	prometheusexporter "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	fileexporter "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	zpagesextension "go.opentelemetry.io/collector/extension/zpagesextension"
	healthcheckextension "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	pprofextension "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	batchprocessor "go.opentelemetry.io/collector/processor/batchprocessor"
	memorylimiterprocessor "go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	attributesprocessor "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	resourceprocessor "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	filterprocessor "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	transformprocessor "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	otlpreceiver "go.opentelemetry.io/collector/receiver/otlpreceiver"
	postgresqlreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	mysqlreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
	sqlqueryreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"
	prometheusreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	
	// Custom component registries
	customexporters "github.com/database-intelligence/db-intel/components/exporters"
	customprocessors "github.com/database-intelligence/db-intel/components/processors"
	customreceivers "github.com/database-intelligence/db-intel/components/receivers"
)

func components() (otelcol.Factories, error) {
	var err error
	factories := otelcol.Factories{}

	// Extensions
	factories.Extensions, err = extension.MakeFactoryMap(
		zpagesextension.NewFactory(),
		healthcheckextension.NewFactory(),
		pprofextension.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	// Standard receivers
	standardReceivers := map[component.Type]receiver.Factory{
		otlpreceiver.NewFactory().Type():        otlpreceiver.NewFactory(),
		postgresqlreceiver.NewFactory().Type():  postgresqlreceiver.NewFactory(),
		mysqlreceiver.NewFactory().Type():       mysqlreceiver.NewFactory(),
		sqlqueryreceiver.NewFactory().Type():    sqlqueryreceiver.NewFactory(),
		prometheusreceiver.NewFactory().Type():  prometheusreceiver.NewFactory(),
	}
	
	// Merge custom receivers
	customReceiverFactories := customreceivers.All()
	allReceivers := make(map[component.Type]receiver.Factory)
	for k, v := range standardReceivers {
		allReceivers[k] = v
	}
	for k, v := range customReceiverFactories {
		allReceivers[k] = v
	}
	
	factories.Receivers, err = receiver.MakeFactoryMap(allReceivers)
	if err != nil {
		return otelcol.Factories{}, err
	}

	// Standard exporters
	standardExporters := map[component.Type]exporter.Factory{
		debugexporter.NewFactory().Type():      debugexporter.NewFactory(),
		otlpexporter.NewFactory().Type():       otlpexporter.NewFactory(),
		otlphttpexporter.NewFactory().Type():   otlphttpexporter.NewFactory(),
		prometheusexporter.NewFactory().Type(): prometheusexporter.NewFactory(),
		fileexporter.NewFactory().Type():       fileexporter.NewFactory(),
	}
	
	// Merge custom exporters
	customExporterFactories := customexporters.All()
	allExporters := make(map[component.Type]exporter.Factory)
	for k, v := range standardExporters {
		allExporters[k] = v
	}
	for k, v := range customExporterFactories {
		allExporters[k] = v
	}
	
	factories.Exporters, err = exporter.MakeFactoryMap(allExporters)
	if err != nil {
		return otelcol.Factories{}, err
	}

	// Standard processors
	standardProcessors := map[component.Type]processor.Factory{
		batchprocessor.NewFactory().Type():         batchprocessor.NewFactory(),
		memorylimiterprocessor.NewFactory().Type(): memorylimiterprocessor.NewFactory(),
		attributesprocessor.NewFactory().Type():    attributesprocessor.NewFactory(),
		resourceprocessor.NewFactory().Type():      resourceprocessor.NewFactory(),
		filterprocessor.NewFactory().Type():        filterprocessor.NewFactory(),
		transformprocessor.NewFactory().Type():     transformprocessor.NewFactory(),
	}
	
	// Merge custom processors
	customProcessorFactories := customprocessors.All()
	allProcessors := make(map[component.Type]processor.Factory)
	for k, v := range standardProcessors {
		allProcessors[k] = v
	}
	for k, v := range customProcessorFactories {
		allProcessors[k] = v
	}
	
	factories.Processors, err = processor.MakeFactoryMap(allProcessors)
	if err != nil {
		return otelcol.Factories{}, err
	}

	// Connectors
	factories.Connectors, err = connector.MakeFactoryMap(
		forwardconnector.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	return factories, nil
}