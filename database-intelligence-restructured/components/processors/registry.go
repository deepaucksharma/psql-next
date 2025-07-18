package processors

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/processor"
    
    "github.com/database-intelligence/db-intel/components/processors/adaptivesampler"
    "github.com/database-intelligence/db-intel/components/processors/circuitbreaker"
    "github.com/database-intelligence/db-intel/components/processors/costcontrol"
    "github.com/database-intelligence/db-intel/components/processors/nrerrormonitor"
    "github.com/database-intelligence/db-intel/components/processors/planattributeextractor"
    "github.com/database-intelligence/db-intel/components/processors/querycorrelator"
    "github.com/database-intelligence/db-intel/components/processors/verification"
    "github.com/database-intelligence/db-intel/components/processors/ohitransform"
)

// All returns all processor factories
func All() map[component.Type]processor.Factory {
    return map[component.Type]processor.Factory{
        adaptivesampler.NewFactory().Type():        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory().Type():         circuitbreaker.NewFactory(),
        costcontrol.NewFactory().Type():            costcontrol.NewFactory(),
        nrerrormonitor.NewFactory().Type():         nrerrormonitor.NewFactory(),
        planattributeextractor.NewFactory().Type(): planattributeextractor.NewFactory(),
        querycorrelator.NewFactory().Type():        querycorrelator.NewFactory(),
        verification.NewFactory().Type():           verification.NewFactory(),
        ohitransform.NewFactory().Type():           ohitransform.NewFactory(),
    }
}