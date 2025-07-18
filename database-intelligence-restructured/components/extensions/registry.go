package extensions

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/extension"
    
    "github.com/database-intelligence/db-intel/components/extensions/postgresqlquery"
)

// All returns all extension factories
func All() map[component.Type]extension.Factory {
    return map[component.Type]extension.Factory{
        postgresqlquery.NewFactory().Type(): postgresqlquery.NewFactory(),
    }
}