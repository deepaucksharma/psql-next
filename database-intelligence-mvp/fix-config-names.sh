#!/bin/bash

# Fix configuration naming issues across all config files

echo "Fixing configuration naming issues..."

# Find all YAML config files
for file in $(find config -name "*.yaml"); do
    echo "Processing $file..."
    
    # Fix circuit_breaker -> circuitbreaker
    sed -i '' 's/circuit_breaker:/circuitbreaker:/g' "$file"
    sed -i '' 's/- circuit_breaker/- circuitbreaker/g' "$file"
    
    # Fix health_check -> healthcheck
    sed -i '' 's/health_check:/healthcheck:/g' "$file"
    sed -i '' 's/\[health_check\]/[healthcheck]/g' "$file"
    
    # Fix adaptive_sampler -> adaptivesampler
    sed -i '' 's/adaptive_sampler:/adaptivesampler:/g' "$file"
    sed -i '' 's/- adaptive_sampler/- adaptivesampler/g' "$file"
    
    # Fix plan_attribute_extractor -> planattributeextractor
    sed -i '' 's/plan_attribute_extractor:/planattributeextractor:/g' "$file"
    sed -i '' 's/- plan_attribute_extractor/- planattributeextractor/g' "$file"
    
    # Fix nr_error_monitor -> nrerrormonitor
    sed -i '' 's/nr_error_monitor:/nrerrormonitor:/g' "$file"
    sed -i '' 's/- nr_error_monitor/- nrerrormonitor/g' "$file"
    
    # Fix cost_control -> costcontrol
    sed -i '' 's/cost_control:/costcontrol:/g' "$file"
    sed -i '' 's/- cost_control/- costcontrol/g' "$file"
    
    # Fix query_correlator -> querycorrelator
    sed -i '' 's/query_correlator:/querycorrelator:/g' "$file"
    sed -i '' 's/- query_correlator/- querycorrelator/g' "$file"
done

echo "Configuration naming issues fixed!"