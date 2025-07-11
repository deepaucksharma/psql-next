// Package main provides a simple production-ready Database Intelligence Collector
package main

import (
	"log"
	
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/confmap/provider/httpprovider"
	"go.opentelemetry.io/collector/confmap/provider/httpsprovider"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	info := component.BuildInfo{
		Command:     "otelcol-database-intelligence",
		Description: "Database Intelligence OpenTelemetry Collector",
		Version:     "2.0.0",
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: func() (otelcol.Factories, error) {
			return componentsComplete, nil
		},
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				ProviderFactories: []confmap.ProviderFactory{
					fileprovider.NewFactory(),
					envprovider.NewFactory(),
					yamlprovider.NewFactory(),
					httpprovider.NewFactory(),
					httpsprovider.NewFactory(),
				},
			},
		},
	}

	if err := run(set); err != nil {
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