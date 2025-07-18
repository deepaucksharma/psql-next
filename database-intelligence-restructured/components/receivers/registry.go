package receivers

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/receiver"
    
    "github.com/database-intelligence/db-intel/components/receivers/ash"
    "github.com/database-intelligence/db-intel/components/receivers/enhancedsql"
    "github.com/database-intelligence/db-intel/components/receivers/kernelmetrics"
    "github.com/database-intelligence/db-intel/components/receivers/mongodb"
    "github.com/database-intelligence/db-intel/components/receivers/redis"
)

// All returns all receiver factories
func All() map[component.Type]receiver.Factory {
    return map[component.Type]receiver.Factory{
        ash.NewFactory().Type():           ash.NewFactory(),
        enhancedsql.NewFactory().Type():   enhancedsql.NewFactory(),
        kernelmetrics.NewFactory().Type(): kernelmetrics.NewFactory(),
        mongodb.NewFactory().Type():       mongodb.NewFactory(),
        redis.NewFactory().Type():         redis.NewFactory(),
    }
}