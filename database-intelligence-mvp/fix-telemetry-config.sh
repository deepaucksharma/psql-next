#!/bin/bash

# Script to fix telemetry metrics configuration across all YAML files
# Standardizes to the simpler format: address: 0.0.0.0:8888

echo "Fixing telemetry metrics configuration..."

# Find all YAML files with the new readers format
files_with_readers=$(grep -l "readers:" config/*.yaml config/**/*.yaml 2>/dev/null || true)

if [ -z "$files_with_readers" ]; then
    echo "No files with 'readers:' format found."
else
    echo "Files with 'readers:' format:"
    echo "$files_with_readers"
    
    # Create a temporary Python script to fix the YAML structure
    cat > fix_telemetry.py << 'EOF'
import sys
import yaml
import re

def fix_telemetry_config(file_path):
    try:
        with open(file_path, 'r') as f:
            content = f.read()
        
        # Check if file has the readers format
        if 'readers:' not in content:
            return False
            
        # Load YAML
        data = yaml.safe_load(content)
        
        # Check if telemetry.metrics.readers exists
        if ('service' in data and 
            'telemetry' in data['service'] and 
            'metrics' in data['service']['telemetry'] and
            'readers' in data['service']['telemetry']['metrics']):
            
            # Extract the port from the readers configuration
            port = 8888  # default
            readers = data['service']['telemetry']['metrics']['readers']
            if isinstance(readers, list) and len(readers) > 0:
                reader = readers[0]
                if 'pull' in reader and 'exporter' in reader['pull']:
                    if 'prometheus' in reader['pull']['exporter']:
                        prom_config = reader['pull']['exporter']['prometheus']
                        if 'port' in prom_config:
                            port = prom_config['port']
            
            # Replace readers with simple address format
            data['service']['telemetry']['metrics'] = {
                'level': data['service']['telemetry']['metrics'].get('level', 'detailed'),
                'address': f'0.0.0.0:{port}'
            }
            
            # Write back to file
            with open(file_path, 'w') as f:
                yaml.dump(data, f, default_flow_style=False, sort_keys=False)
            
            return True
    except Exception as e:
        print(f"Error processing {file_path}: {e}")
        return False
    
    return False

if __name__ == "__main__":
    for file_path in sys.argv[1:]:
        if fix_telemetry_config(file_path):
            print(f"Fixed: {file_path}")
        else:
            print(f"Skipped: {file_path}")
EOF

    # Run the Python script on files with readers format
    python3 fix_telemetry.py $files_with_readers
    
    # Clean up
    rm -f fix_telemetry.py
fi

# Also check for any health_check references that should be healthcheck
echo -e "\nChecking for health_check references that should be healthcheck..."
grep -l "health_check" config/*.yaml config/**/*.yaml 2>/dev/null || echo "No health_check references found"

echo -e "\nTelemetry configuration fix complete!"