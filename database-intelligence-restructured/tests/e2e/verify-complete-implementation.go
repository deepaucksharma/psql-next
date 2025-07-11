package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Receivers  map[string]interface{} `yaml:"receivers"`
	Processors map[string]interface{} `yaml:"processors"`
	Exporters  map[string]interface{} `yaml:"exporters"`
	Service    Service                `yaml:"service"`
}

type Service struct {
	Pipelines map[string]Pipeline `yaml:"pipelines"`
}

type Pipeline struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

func main() {
	fmt.Println("===== Database Intelligence Implementation Verification =====")
	fmt.Println()

	// Test 1: Verify configuration files
	fmt.Println("1. Verifying configuration files...")
	verifyConfigFiles()

	// Test 2: Verify environment variables
	fmt.Println("\n2. Verifying environment variable standardization...")
	verifyEnvironmentVariables()

	// Test 3: Verify custom components
	fmt.Println("\n3. Verifying custom components...")
	verifyCustomComponents()

	// Test 4: Verify Go module versions
	fmt.Println("\n4. Verifying Go module versions...")
	verifyGoVersions()

	// Test 5: Verify Docker configuration
	fmt.Println("\n5. Verifying Docker configuration...")
	verifyDockerConfig()

	// Test 6: Verify complete pipeline
	fmt.Println("\n6. Verifying complete pipeline configuration...")
	verifyCompletePipeline()

	fmt.Println("\n===== Verification Complete =====")
}

func verifyConfigFiles() {
	configs := []string{
		"distributions/production/production-config.yaml",
		"distributions/production/production-config-complete.yaml",
	}

	for _, config := range configs {
		if _, err := os.Stat(config); err == nil {
			fmt.Printf("   ✓ Found %s\n", config)
			
			// Check for standardized variables
			content, _ := ioutil.ReadFile(config)
			if strings.Contains(string(content), "DB_POSTGRES_HOST") &&
			   strings.Contains(string(content), "NEW_RELIC_LICENSE_KEY") {
				fmt.Printf("     ✓ Uses standardized environment variables\n")
			} else {
				fmt.Printf("     ✗ May not use standardized variables\n")
			}
		} else {
			fmt.Printf("   ✗ Missing %s\n", config)
		}
	}
}

func verifyEnvironmentVariables() {
	envFile := ".env"
	expectedVars := []string{
		"DB_POSTGRES_HOST",
		"DB_POSTGRES_PORT",
		"DB_MYSQL_HOST",
		"DB_MYSQL_PORT",
		"NEW_RELIC_LICENSE_KEY",
		"SERVICE_NAME",
	}

	if content, err := ioutil.ReadFile(envFile); err == nil {
		fmt.Printf("   ✓ Found .env file\n")
		for _, v := range expectedVars {
			if strings.Contains(string(content), v) {
				fmt.Printf("   ✓ Contains %s\n", v)
			} else {
				fmt.Printf("   ✗ Missing %s\n", v)
			}
		}
	} else {
		fmt.Printf("   ✗ .env file not found\n")
	}
}

func verifyCustomComponents() {
	processors := []string{
		"adaptivesampler",
		"circuitbreaker",
		"costcontrol",
		"nrerrormonitor",
		"planattributeextractor",
		"querycorrelator",
		"verification",
	}

	receivers := []string{
		"ash",
		"enhancedsql",
		"kernelmetrics",
	}

	fmt.Println("   Processors:")
	for _, p := range processors {
		path := filepath.Join("components/processors", p)
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("     ✓ %s\n", p)
		} else {
			fmt.Printf("     ✗ %s\n", p)
		}
	}

	fmt.Println("   Receivers:")
	for _, r := range receivers {
		path := filepath.Join("components/receivers", r)
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("     ✓ %s\n", r)
		} else {
			fmt.Printf("     ✗ %s\n", r)
		}
	}
}

func verifyGoVersions() {
	goModFiles := []string{
		"components/go.mod",
		"common/featuredetector/go.mod",
		"common/queryselector/go.mod",
		"distributions/production/go.mod",
	}

	invalidVersionPattern := regexp.MustCompile(`go 1\.23|go 1\.24|toolchain`)

	for _, modFile := range goModFiles {
		if content, err := ioutil.ReadFile(modFile); err == nil {
			if invalidVersionPattern.Match(content) {
				fmt.Printf("   ✗ %s has invalid Go version\n", modFile)
			} else {
				fmt.Printf("   ✓ %s has valid Go version\n", modFile)
			}
		}
	}
}

func verifyDockerConfig() {
	dockerCompose := "deployments/docker/compose/docker-compose.yaml"
	
	if content, err := ioutil.ReadFile(dockerCompose); err == nil {
		fmt.Printf("   ✓ Found docker-compose.yaml\n")
		
		// Check for standardized env vars
		if strings.Contains(string(content), "DB_POSTGRES_HOST=postgres") &&
		   strings.Contains(string(content), "NEW_RELIC_LICENSE_KEY") {
			fmt.Printf("   ✓ Docker compose uses standardized variables\n")
		} else {
			fmt.Printf("   ✗ Docker compose may not use standardized variables\n")
		}
		
		// Check config path
		if strings.Contains(string(content), "production-config-complete.yaml") {
			fmt.Printf("   ✓ Uses complete production config\n")
		} else {
			fmt.Printf("   ✗ May not use complete config\n")
		}
	} else {
		fmt.Printf("   ✗ docker-compose.yaml not found\n")
	}
}

func verifyCompletePipeline() {
	configFile := "distributions/production/production-config-complete.yaml"
	
	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Printf("   ✗ Could not read complete config: %v\n", err)
		return
	}

	var config Config
	if err := yaml.Unmarshal(content, &config); err != nil {
		fmt.Printf("   ✗ Could not parse config: %v\n", err)
		return
	}

	// Check receivers
	fmt.Println("   Receivers:")
	expectedReceivers := []string{"postgresql", "mysql", "otlp", "sqlquery", "enhancedsql"}
	for _, r := range expectedReceivers {
		if _, ok := config.Receivers[r]; ok {
			fmt.Printf("     ✓ %s\n", r)
		} else {
			fmt.Printf("     ✗ %s missing\n", r)
		}
	}

	// Check processors in pipeline
	if pipeline, ok := config.Service.Pipelines["metrics"]; ok {
		fmt.Println("   Pipeline processors:")
		expectedProcessors := []string{
			"memory_limiter",
			"resource",
			"adaptivesampler",
			"circuit_breaker",
			"planattributeextractor",
			"verification",
			"costcontrol",
			"nrerrormonitor",
			"querycorrelator",
			"batch",
		}
		
		for _, p := range expectedProcessors {
			found := false
			for _, proc := range pipeline.Processors {
				if proc == p {
					found = true
					break
				}
			}
			if found {
				fmt.Printf("     ✓ %s\n", p)
			} else {
				fmt.Printf("     ✗ %s missing from pipeline\n", p)
			}
		}
	}

	// Check exporters
	fmt.Println("   Exporters:")
	if _, ok := config.Exporters["otlp/newrelic"]; ok {
		fmt.Printf("     ✓ New Relic OTLP exporter configured\n")
	} else {
		fmt.Printf("     ✗ New Relic OTLP exporter missing\n")
	}
}