package extensions

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/extension"
    
    "github.com/database-intelligence/extensions/healthcheck"
)

// Factories returns all extension factories
func Factories() (map[component.Type]extension.Factory, error) {
    factories := make(map[component.Type]extension.Factory)
    
    // Register each extension with its factory
    for _, factory := range []extension.Factory{
        healthcheck.NewFactory(),
    } {
        factories[factory.Type()] = factory
    }
    
    return factories, nil
}
