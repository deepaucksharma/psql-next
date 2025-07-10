// Package main provides a simple production-ready Database Intelligence Collector
package main

import (
	"log"
	
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	info := component.BuildInfo{
		Command:     "otelcol-database-intelligence",
		Description: "Database Intelligence OpenTelemetry Collector",
		Version:     "2.0.0",
	}

	if err := run(otelcol.CollectorSettings{BuildInfo: info, Factories: func() (otelcol.Factories, error) {
		return components, nil
	}}); err != nil {
		log.Fatal(err)
	}
}

func run(settings otelcol.CollectorSettings) error {
	cmd := otelcol.NewCommand(settings)
	if err := cmd.Execute(); err != nil {
		return err
	}
	return nil
}