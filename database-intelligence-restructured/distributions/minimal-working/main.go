package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    // Import only custom processors
    "github.com/database-intelligence/processors/adaptivesampler"
    "github.com/database-intelligence/processors/circuitbreaker"
    "github.com/database-intelligence/processors/costcontrol"
    "github.com/database-intelligence/processors/nrerrormonitor"
    "github.com/database-intelligence/processors/planattributeextractor"
    "github.com/database-intelligence/processors/querycorrelator"
    "github.com/database-intelligence/processors/verification"
)

func main() {
    info := component.BuildInfo{
        Command:     "database-intelligence-minimal",
        Description: "Database Intelligence Minimal Collector",
        Version:     "1.0.0",
    }

    factories := otelcol.Factories{
        Processors: map[component.Type]component.Factory{
            adaptivesampler.NewFactory().Type():        adaptivesampler.NewFactory(),
            circuitbreaker.NewFactory().Type():         circuitbreaker.NewFactory(),
            costcontrol.NewFactory().Type():            costcontrol.NewFactory(),
            nrerrormonitor.NewFactory().Type():         nrerrormonitor.NewFactory(),
            planattributeextractor.NewFactory().Type(): planattributeextractor.NewFactory(),
            querycorrelator.NewFactory().Type():        querycorrelator.NewFactory(),
            verification.NewFactory().Type():           verification.NewFactory(),
        },
        Receivers:  map[component.Type]component.Factory{},
        Exporters:  map[component.Type]component.Factory{},
        Extensions: map[component.Type]component.Factory{},
        Connectors: map[component.Type]component.Factory{},
    }

    cmd := otelcol.NewCommand(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    })
    
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}
