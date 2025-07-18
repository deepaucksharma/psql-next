package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

const (
	ProfileMinimal    = "minimal"
	ProfileStandard   = "standard"
	ProfileEnterprise = "enterprise"
)

var (
	profile     = flag.String("profile", ProfileStandard, "Distribution profile: minimal, standard, or enterprise")
	showVersion = flag.Bool("version", false, "Show version information")
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("Database Intelligence Collector\n")
		fmt.Printf("Profile: %s\n", *profile)
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Date: %s\n", BuildDate)
		os.Exit(0)
	}

	info := component.BuildInfo{
		Command:     "database-intelligence-collector",
		Description: fmt.Sprintf("Database Intelligence Collector - %s profile", *profile),
		Version:     Version,
	}

	var factories otelcol.Factories
	var err error

	switch *profile {
	case ProfileMinimal:
		factories, err = MinimalComponents()
	case ProfileStandard:
		factories, err = StandardComponents()
	case ProfileEnterprise:
		factories, err = EnterpriseComponents()
	default:
		log.Fatalf("Unknown profile: %s. Valid profiles are: minimal, standard, enterprise", *profile)
	}

	if err != nil {
		log.Fatalf("Failed to build components for %s profile: %v", *profile, err)
	}

	params := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: func() (otelcol.Factories, error) {
			return factories, nil
		},
	}

	if err := runInteractive(params); err != nil {
		log.Fatal(err)
	}
}

var (
	Version   = "dev"
	BuildDate = "unknown"
)

func runInteractive(params otelcol.CollectorSettings) error {
	cmd := otelcol.NewCommand(params)
	if err := cmd.Execute(); err != nil {
		log.Fatalf("collector server run finished with error: %v", err)
	}
	return nil
}