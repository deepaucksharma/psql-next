// Package registry provides a unified component registry for OpenTelemetry collectors
package registry

import (
	"fmt"
	
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	
	// Import contrib components
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	
	// Import custom components
	"github.com/database-intelligence/processors/adaptivesampler"
	"github.com/database-intelligence/processors/circuitbreaker"
	"github.com/database-intelligence/processors/costcontrol"
	"github.com/database-intelligence/processors/nrerrormonitor"
	"github.com/database-intelligence/processors/planattributeextractor"
	"github.com/database-intelligence/processors/querycorrelator"
	"github.com/database-intelligence/processors/verification"
	"github.com/database-intelligence/receivers/enhancedsql"
	"github.com/database-intelligence/receivers/ash"
	"github.com/database-intelligence/receivers/kernelmetrics"
	"github.com/database-intelligence/exporters/nri"
)

// ComponentSet defines a set of components
type ComponentSet struct {
	Receivers  []receiver.Factory
	Processors []processor.Factory
	Exporters  []exporter.Factory
	Extensions []extension.Factory
}

// Predefined component sets
var (
	// CoreComponents includes essential OTEL components
	CoreComponents = ComponentSet{
		Receivers: []receiver.Factory{
			otlpreceiver.NewFactory(),
		},
		Processors: []processor.Factory{
			batchprocessor.NewFactory(),
			memorylimiterprocessor.NewFactory(),
		},
		Exporters: []exporter.Factory{
			otlpexporter.NewFactory(),
			otlphttpexporter.NewFactory(),
			debugexporter.NewFactory(),
		},
		Extensions: []extension.Factory{
			zpagesextension.NewFactory(),
			healthcheckextension.NewFactory(),
		},
	}
	
	// DatabaseComponents includes database-specific receivers
	DatabaseComponents = ComponentSet{
		Receivers: []receiver.Factory{
			postgresqlreceiver.NewFactory(),
			mysqlreceiver.NewFactory(),
			sqlqueryreceiver.NewFactory(),
		},
		Processors: []processor.Factory{},
		Exporters:  []exporter.Factory{},
		Extensions: []extension.Factory{},
	}
	
	// CustomProcessors includes all custom processors
	CustomProcessors = ComponentSet{
		Receivers: []receiver.Factory{},
		Processors: []processor.Factory{
			adaptivesampler.NewFactory(),
			circuitbreaker.NewFactory(),
			costcontrol.NewFactory(),
			nrerrormonitor.NewFactory(),
			planattributeextractor.NewFactory(),
			querycorrelator.NewFactory(),
			verification.NewFactory(),
		},
		Exporters:  []exporter.Factory{},
		Extensions: []extension.Factory{},
	}
	
	// CustomReceivers includes custom receivers
	CustomReceivers = ComponentSet{
		Receivers: []receiver.Factory{
			enhancedsql.NewFactory(),
			ash.NewFactory(),
			kernelmetrics.NewFactory(),
		},
		Processors: []processor.Factory{},
		Exporters:  []exporter.Factory{},
		Extensions: []extension.Factory{},
	}
	
	// CustomExporters includes custom exporters
	CustomExporters = ComponentSet{
		Receivers:  []receiver.Factory{},
		Processors: []processor.Factory{},
		Exporters: []exporter.Factory{
			nri.NewFactory(),
		},
		Extensions: []extension.Factory{},
	}
	
	// ObservabilityComponents includes monitoring exporters
	ObservabilityComponents = ComponentSet{
		Receivers:  []receiver.Factory{},
		Processors: []processor.Factory{},
		Exporters: []exporter.Factory{
			prometheusexporter.NewFactory(),
		},
		Extensions: []extension.Factory{},
	}
)

// DistributionPresets defines common distribution configurations
var DistributionPresets = map[string][]ComponentSet{
	"minimal": {
		CoreComponents,
		DatabaseComponents,
	},
	"standard": {
		CoreComponents,
		DatabaseComponents,
		CustomProcessors,
		ObservabilityComponents,
	},
	"enterprise": {
		CoreComponents,
		DatabaseComponents,
		CustomProcessors,
		CustomReceivers,
		CustomExporters,
		ObservabilityComponents,
	},
	"development": {
		CoreComponents,
		DatabaseComponents,
		CustomProcessors,
		CustomReceivers,
		CustomExporters,
		ObservabilityComponents,
	},
}

// Builder helps construct component factories
type Builder struct {
	receivers  map[component.Type]receiver.Factory
	processors map[component.Type]processor.Factory
	exporters  map[component.Type]exporter.Factory
	extensions map[component.Type]extension.Factory
}

// NewBuilder creates a new component builder
func NewBuilder() *Builder {
	return &Builder{
		receivers:  make(map[component.Type]receiver.Factory),
		processors: make(map[component.Type]processor.Factory),
		exporters:  make(map[component.Type]exporter.Factory),
		extensions: make(map[component.Type]extension.Factory),
	}
}

// AddSet adds a component set to the builder
func (b *Builder) AddSet(set ComponentSet) *Builder {
	for _, factory := range set.Receivers {
		b.receivers[factory.Type()] = factory
	}
	for _, factory := range set.Processors {
		b.processors[factory.Type()] = factory
	}
	for _, factory := range set.Exporters {
		b.exporters[factory.Type()] = factory
	}
	for _, factory := range set.Extensions {
		b.extensions[factory.Type()] = factory
	}
	return b
}

// AddSets adds multiple component sets
func (b *Builder) AddSets(sets ...ComponentSet) *Builder {
	for _, set := range sets {
		b.AddSet(set)
	}
	return b
}

// AddReceiver adds a single receiver factory
func (b *Builder) AddReceiver(factory receiver.Factory) *Builder {
	b.receivers[factory.Type()] = factory
	return b
}

// AddProcessor adds a single processor factory
func (b *Builder) AddProcessor(factory processor.Factory) *Builder {
	b.processors[factory.Type()] = factory
	return b
}

// AddExporter adds a single exporter factory
func (b *Builder) AddExporter(factory exporter.Factory) *Builder {
	b.exporters[factory.Type()] = factory
	return b
}

// AddExtension adds a single extension factory
func (b *Builder) AddExtension(factory extension.Factory) *Builder {
	b.extensions[factory.Type()] = factory
	return b
}

// Build creates the final factories
func (b *Builder) Build() (otelcol.Factories, error) {
	// Ensure connectors map is initialized
	factories := otelcol.Factories{
		Receivers:  b.receivers,
		Processors: b.processors,
		Exporters:  b.exporters,
		Extensions: b.extensions,
		Connectors: make(map[component.Type]component.Factory),
	}
	
	// Validate that we have at least one component of each type
	if len(factories.Receivers) == 0 {
		return factories, fmt.Errorf("no receivers configured")
	}
	if len(factories.Processors) == 0 {
		return factories, fmt.Errorf("no processors configured")
	}
	if len(factories.Exporters) == 0 {
		return factories, fmt.Errorf("no exporters configured")
	}
	
	return factories, nil
}

// BuildFromPreset creates factories from a preset configuration
func BuildFromPreset(preset string) (otelcol.Factories, error) {
	sets, ok := DistributionPresets[preset]
	if !ok {
		return otelcol.Factories{}, fmt.Errorf("unknown preset: %s", preset)
	}
	
	builder := NewBuilder()
	return builder.AddSets(sets...).Build()
}

// ListAvailableComponents returns a list of all available components
func ListAvailableComponents() map[string][]string {
	components := make(map[string][]string)
	
	// Combine all component sets
	allSets := []ComponentSet{
		CoreComponents,
		DatabaseComponents,
		CustomProcessors,
		CustomReceivers,
		CustomExporters,
		ObservabilityComponents,
	}
	
	// Collect unique component names
	receiverNames := make(map[string]bool)
	processorNames := make(map[string]bool)
	exporterNames := make(map[string]bool)
	extensionNames := make(map[string]bool)
	
	for _, set := range allSets {
		for _, factory := range set.Receivers {
			receiverNames[string(factory.Type())] = true
		}
		for _, factory := range set.Processors {
			processorNames[string(factory.Type())] = true
		}
		for _, factory := range set.Exporters {
			exporterNames[string(factory.Type())] = true
		}
		for _, factory := range set.Extensions {
			extensionNames[string(factory.Type())] = true
		}
	}
	
	// Convert to slices
	components["receivers"] = mapKeysToSlice(receiverNames)
	components["processors"] = mapKeysToSlice(processorNames)
	components["exporters"] = mapKeysToSlice(exporterNames)
	components["extensions"] = mapKeysToSlice(extensionNames)
	
	return components
}

// Helper function to convert map keys to slice
func mapKeysToSlice(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}