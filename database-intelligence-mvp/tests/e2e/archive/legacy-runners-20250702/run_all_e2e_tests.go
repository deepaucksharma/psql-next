package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("Running All E2E Tests")
	fmt.Println("========================================")

	// Get the current directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Define test files to run
	testFiles := []struct {
		name string
		file string
		run  string
	}{
		{"Basic E2E Test", "real_e2e_test.go", "TestRealE2EPipeline"},
		{"Processor Validation", "processor_validation_test.go", "TestCustomProcessorValidation"},
		{"Security and PII", "security_pii_test.go", "TestSecurityAndPII"},
		{"Error Scenarios", "error_scenarios_test.go", "TestErrorScenarios"},
		{"Performance Scale", "performance_scale_test.go", "TestPerformanceAndScale"},
	}

	passed := 0
	failed := 0
	skipped := 0

	for _, test := range testFiles {
		fmt.Printf("\n--- Running %s ---\n", test.name)
		
		// Check if file exists
		testPath := filepath.Join(cwd, test.file)
		if _, err := os.Stat(testPath); os.IsNotExist(err) {
			fmt.Printf("SKIP: Test file %s not found\n", test.file)
			skipped++
			continue
		}

		// Run the test with timeout
		cmd := exec.Command("go", "test", "-v", "-run", test.run, test.file, "-tags=e2e", "-timeout", "2m")
		cmd.Dir = cwd
		
		// Capture output
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			fmt.Printf("FAIL: %s\n", test.name)
			fmt.Printf("Error: %v\n", err)
			// Print first 1000 chars of output for debugging
			outputStr := string(output)
			if len(outputStr) > 1000 {
				outputStr = outputStr[:1000] + "..."
			}
			fmt.Printf("Output:\n%s\n", outputStr)
			failed++
		} else {
			// Check if test actually passed
			outputStr := string(output)
			if strings.Contains(outputStr, "PASS") && !strings.Contains(outputStr, "FAIL") {
				fmt.Printf("PASS: %s\n", test.name)
				passed++
			} else if strings.Contains(outputStr, "no test files") {
				fmt.Printf("SKIP: %s (no tests found)\n", test.name)
				skipped++
			} else {
				fmt.Printf("FAIL: %s\n", test.name)
				failed++
			}
		}
		
		// Small delay between tests
		time.Sleep(2 * time.Second)
	}

	// Summary
	fmt.Println("\n========================================")
	fmt.Println("Test Summary")
	fmt.Println("========================================")
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed) 
	fmt.Printf("Skipped: %d\n", skipped)
	fmt.Printf("Total: %d\n", len(testFiles))

	if failed > 0 {
		os.Exit(1)
	}
}