package exporters

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    
    "github.com/database-intelligence/db-intel/components/exporters/nri"
)

// All returns all exporter factories
func All() map[component.Type]exporter.Factory {
    return map[component.Type]exporter.Factory{
        nri.NewFactory().Type(): nri.NewFactory(),
    }
}