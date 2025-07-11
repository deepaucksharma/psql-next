package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"gopkg.in/yaml.v3"
)

type FullConfig struct {
	Receivers  map[string]interface{} `yaml:"receivers"`
	Processors map[string]interface{} `yaml:"processors"`
	Exporters  map[string]interface{} `yaml:"exporters"`
	Extensions map[string]interface{} `yaml:"extensions"`
	Service    ServiceConfig          `yaml:"service"`
}

type ServiceConfig struct {
	Extensions []string                `yaml:"extensions"`
	Pipelines  map[string]PipelineSpec `yaml:"pipelines"`
}

type PipelineSpec struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

func main() {
	fmt.Println("===== Configuration Validation Test Suite =====")
	fmt.Println()

	// Test standard config
	fmt.Println("1. Testing Standard Production Config")
	testStandardConfig()

	// Test complete config
	fmt.Println("\n2. Testing Complete Production Config")
	testCompleteConfig()

	// Test environment variable resolution
	fmt.Println("\n3. Testing Environment Variable Resolution")
	testEnvVarResolution()

	// Test processor ordering
	fmt.Println("\n4. Testing Processor Ordering")
	testProcessorOrdering()

	// Test New Relic integration
	fmt.Println("\n5. Testing New Relic Integration")
	testNewRelicIntegration()

	fmt.Println("\n===== All Tests Complete =====")
}

func testStandardConfig() {
	config := loadConfig("distributions/production/production-config.yaml")
	if config == nil {
		return
	}

	// Check basic receivers
	checkExists(config.Receivers, "otlp", "OTLP receiver")
	checkExists(config.Receivers, "postgresql", "PostgreSQL receiver")
	checkExists(config.Receivers, "mysql", "MySQL receiver")

	// Check basic processors
	checkExists(config.Processors, "batch", "Batch processor")
	checkExists(config.Processors, "memory_limiter", "Memory limiter processor")
	checkExists(config.Processors, "resource", "Resource processor")

	// Check exporters
	checkExists(config.Exporters, "otlphttp", "OTLP HTTP exporter")

	// Check pipelines
	if config.Service.Pipelines != nil {
		fmt.Printf("   ✓ Found %d pipelines\n", len(config.Service.Pipelines))
		
		// Check metrics pipeline has resource processor
		if pipeline, ok := config.Service.Pipelines["metrics"]; ok {
			hasResource := false
			for _, p := range pipeline.Processors {
				if p == "resource" {
					hasResource = true
					break
				}
			}
			if hasResource {
				fmt.Println("   ✓ Metrics pipeline includes resource processor")
			} else {
				fmt.Println("   ✗ Metrics pipeline missing resource processor")
			}
		}
	}
}

func testCompleteConfig() {
	config := loadConfig("distributions/production/production-config-complete.yaml")
	if config == nil {
		return
	}

	// Check all custom processors exist
	customProcessors := []string{
		"adaptivesampler",
		"circuit_breaker",
		"planattributeextractor",
		"verification",
		"costcontrol",
		"nrerrormonitor",
		"querycorrelator",
	}

	for _, proc := range customProcessors {
		checkExists(config.Processors, proc, fmt.Sprintf("Custom processor %s", proc))
	}

	// Check custom receivers
	checkExists(config.Receivers, "enhancedsql", "Enhanced SQL receiver")

	// Check complete pipeline
	if pipeline, ok := config.Service.Pipelines["metrics"]; ok {
		fmt.Printf("   ✓ Metrics pipeline has %d processors\n", len(pipeline.Processors))
		
		// Verify processor order
		expectedOrder := []string{"memory_limiter", "resource"}
		orderCorrect := true
		for i, exp := range expectedOrder {
			if i >= len(pipeline.Processors) || pipeline.Processors[i] != exp {
				orderCorrect = false
				break
			}
		}
		if orderCorrect {
			fmt.Println("   ✓ Critical processors are in correct order")
		} else {
			fmt.Println("   ✗ Processor order may be incorrect")
		}
	}
}

func testEnvVarResolution() {
	configContent, err := ioutil.ReadFile("distributions/production/production-config.yaml")
	if err != nil {
		fmt.Printf("   ✗ Could not read config: %v\n", err)
		return
	}

	// Check for proper env var syntax
	envVarPattern := regexp.MustCompile(`\$\{([A-Z_]+)(?::([^}]+))?\}`)
	matches := envVarPattern.FindAllStringSubmatch(string(configContent), -1)
	
	fmt.Printf("   ✓ Found %d environment variables\n", len(matches))
	
	// Check for standardized names
	standardVars := map[string]bool{
		"DB_POSTGRES_HOST": false,
		"DB_POSTGRES_PORT": false,
		"DB_MYSQL_HOST": false,
		"DB_MYSQL_PORT": false,
		"NEW_RELIC_LICENSE_KEY": false,
		"SERVICE_NAME": false,
	}
	
	for _, match := range matches {
		varName := match[1]
		if _, ok := standardVars[varName]; ok {
			standardVars[varName] = true
		}
	}
	
	for varName, found := range standardVars {
		if found {
			fmt.Printf("   ✓ Uses %s\n", varName)
		} else {
			fmt.Printf("   ✗ Missing %s\n", varName)
		}
	}
}

func testProcessorOrdering() {
	config := loadConfig("distributions/production/production-config-complete.yaml")
	if config == nil {
		return
	}

	if pipeline, ok := config.Service.Pipelines["metrics"]; ok {
		// Check that memory_limiter is first
		if len(pipeline.Processors) > 0 && pipeline.Processors[0] == "memory_limiter" {
			fmt.Println("   ✓ Memory limiter is first processor")
		} else {
			fmt.Println("   ✗ Memory limiter should be first processor")
		}
		
		// Check that batch is last
		if len(pipeline.Processors) > 0 && pipeline.Processors[len(pipeline.Processors)-1] == "batch" {
			fmt.Println("   ✓ Batch is last processor")
		} else {
			fmt.Println("   ✗ Batch should be last processor")
		}
		
		// Check resource processor is early in pipeline
		resourceIndex := -1
		for i, p := range pipeline.Processors {
			if p == "resource" {
				resourceIndex = i
				break
			}
		}
		if resourceIndex > 0 && resourceIndex < 3 {
			fmt.Println("   ✓ Resource processor is early in pipeline")
		} else {
			fmt.Println("   ✗ Resource processor should be early in pipeline")
		}
	}
}

func testNewRelicIntegration() {
	config := loadConfig("distributions/production/production-config-complete.yaml")
	if config == nil {
		return
	}

	// Check OTLP exporter configuration
	if exporterConfig, ok := config.Exporters["otlp/newrelic"].(map[string]interface{}); ok {
		// Check endpoint
		if endpoint, ok := exporterConfig["endpoint"].(string); ok {
			if strings.Contains(endpoint, "otlp.nr-data.net") || strings.Contains(endpoint, "OTLP_ENDPOINT") {
				fmt.Println("   ✓ New Relic OTLP endpoint configured")
			} else {
				fmt.Println("   ✗ Invalid New Relic endpoint")
			}
		}
		
		// Check headers
		if headers, ok := exporterConfig["headers"].(map[string]interface{}); ok {
			if apiKey, ok := headers["api-key"].(string); ok {
				if strings.Contains(apiKey, "NEW_RELIC_LICENSE_KEY") {
					fmt.Println("   ✓ Uses NEW_RELIC_LICENSE_KEY")
				} else {
					fmt.Println("   ✗ Should use NEW_RELIC_LICENSE_KEY")
				}
			}
		}
		
		// Check compression
		if compression, ok := exporterConfig["compression"].(string); ok && compression == "gzip" {
			fmt.Println("   ✓ Gzip compression enabled")
		} else {
			fmt.Println("   ✗ Gzip compression not enabled")
		}
		
		// Check retry
		if retry, ok := exporterConfig["retry_on_failure"].(map[string]interface{}); ok {
			if enabled, ok := retry["enabled"].(bool); ok && enabled {
				fmt.Println("   ✓ Retry on failure enabled")
			}
		}
	} else {
		fmt.Println("   ✗ New Relic OTLP exporter not found")
	}
}

func loadConfig(path string) *FullConfig {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("   ✗ Could not read %s: %v\n", path, err)
		return nil
	}

	var config FullConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		fmt.Printf("   ✗ Could not parse %s: %v\n", path, err)
		return nil
	}

	fmt.Printf("   ✓ Successfully loaded %s\n", path)
	return &config
}

func checkExists(m map[string]interface{}, key, desc string) {
	if _, ok := m[key]; ok {
		fmt.Printf("   ✓ %s configured\n", desc)
	} else {
		fmt.Printf("   ✗ %s missing\n", desc)
	}
}