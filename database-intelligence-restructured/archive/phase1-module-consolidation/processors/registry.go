package processors

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/processor"
    
    "github.com/deepaksharma/db-otel/components/processors/adaptivesampler"
    "github.com/deepaksharma/db-otel/components/processors/circuitbreaker"
    "github.com/deepaksharma/db-otel/components/processors/costcontrol"
    "github.com/deepaksharma/db-otel/components/processors/nrerrormonitor"
    "github.com/deepaksharma/db-otel/components/processors/planattributeextractor"
    "github.com/deepaksharma/db-otel/components/processors/querycorrelator"
    "github.com/deepaksharma/db-otel/components/processors/verification"
)

// Factories returns all processor factories
func Factories() (map[component.Type]processor.Factory, error) {
    factories := make(map[component.Type]processor.Factory)
    
    // Register each processor with its factory
    // The key here doesn't matter as the factory itself contains the component type
    for _, factory := range []processor.Factory{
        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory(),
        costcontrol.NewFactory(),
        nrerrormonitor.NewFactory(),
        planattributeextractor.NewFactory(),
        querycorrelator.NewFactory(),
        verification.NewFactory(),
    } {
        factories[factory.Type()] = factory
    }
    
    return factories, nil
}
