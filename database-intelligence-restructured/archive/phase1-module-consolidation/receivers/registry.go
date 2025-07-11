package receivers

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/receiver"
    
    "github.com/deepaksharma/db-otel/components/receivers/ash"
    "github.com/deepaksharma/db-otel/components/receivers/enhancedsql"
    "github.com/deepaksharma/db-otel/components/receivers/kernelmetrics"
)

// Factories returns all receiver factories
func Factories() (map[component.Type]receiver.Factory, error) {
    factories := make(map[component.Type]receiver.Factory)
    
    // Register each receiver with its factory
    for _, factory := range []receiver.Factory{
        ash.NewFactory(),
        enhancedsql.NewFactory(),
        kernelmetrics.NewFactory(),
    } {
        factories[factory.Type()] = factory
    }
    
    return factories, nil
}
