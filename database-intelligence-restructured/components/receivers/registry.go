package receivers

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/receiver"
    
    "github.com/deepaksharma/db-otel/components/receivers/ash"
    "github.com/deepaksharma/db-otel/components/receivers/enhancedsql"
    "github.com/deepaksharma/db-otel/components/receivers/kernelmetrics"
)

// All returns all receiver factories
func All() map[component.Type]receiver.Factory {
    return map[component.Type]receiver.Factory{
        ash.NewFactory().Type():           ash.NewFactory(),
        enhancedsql.NewFactory().Type():   enhancedsql.NewFactory(),
        kernelmetrics.NewFactory().Type(): kernelmetrics.NewFactory(),
    }
}