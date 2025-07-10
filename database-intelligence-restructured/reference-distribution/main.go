package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	info := component.BuildInfo{
		Command:     "database-intelligence-collector",
		Description: "Database Intelligence Collector",
		Version:     "0.1.0",
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: otelcol.Factories{},
	}

	cmd := otelcol.NewCommand(set)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
