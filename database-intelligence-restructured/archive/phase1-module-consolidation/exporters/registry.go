package exporters

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    
    "github.com/deepaksharma/db-otel/components/exporters/nri"
)

// Factories returns all exporter factories
func Factories() (map[component.Type]exporter.Factory, error) {
    factories := make(map[component.Type]exporter.Factory)
    
    // Register each exporter with its factory
    for _, factory := range []exporter.Factory{
        nri.NewFactory(),
    } {
        factories[factory.Type()] = factory
    }
    
    return factories, nil
}
