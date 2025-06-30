#\!/bin/bash
# Quick fix for key configuration files

echo "=== Quick Configuration Fix ==="

# Function to fix a file
fix_file() {
    local file=$1
    echo "Fixing: $file"
    
    # Fix environment variables
    sed -i.bak 's/${POSTGRES_HOST:/${env:POSTGRES_HOST:-/g' "$file"
    sed -i.bak 's/${POSTGRES_PORT:/${env:POSTGRES_PORT:-/g' "$file"
    sed -i.bak 's/${POSTGRES_USER:/${env:POSTGRES_USER:-/g' "$file"
    sed -i.bak 's/${POSTGRES_PASSWORD:/${env:POSTGRES_PASSWORD:-/g' "$file"
    sed -i.bak 's/${POSTGRES_DB:/${env:POSTGRES_DB:-/g' "$file"
    
    sed -i.bak 's/${MYSQL_HOST:/${env:MYSQL_HOST:-/g' "$file"
    sed -i.bak 's/${MYSQL_PORT:/${env:MYSQL_PORT:-/g' "$file"
    sed -i.bak 's/${MYSQL_USER:/${env:MYSQL_USER:-/g' "$file"
    sed -i.bak 's/${MYSQL_PASSWORD:/${env:MYSQL_PASSWORD:-/g' "$file"
    sed -i.bak 's/${MYSQL_DB:/${env:MYSQL_DB:-/g' "$file"
    
    sed -i.bak 's/${NEW_RELIC_LICENSE_KEY}/${env:NEW_RELIC_LICENSE_KEY}/g' "$file"
    sed -i.bak 's/${HOSTNAME}/${env:HOSTNAME}/g' "$file"
    
    # Fix memory limiter
    sed -i.bak 's/limit_percentage: *[0-9]*/limit_mib: 1024/g' "$file"
    sed -i.bak 's/spike_limit_percentage: *[0-9]*/spike_limit_mib: 256/g' "$file"
    
    # Clean up
    rm -f "${file}.bak"
    
    echo "✓ Fixed: $file"
}

# Fix key files
for file in config/collector.yaml config/collector-simplified.yaml config/collector-resilient.yaml deployments/kubernetes/configmap.yaml; do
    if [ -f "$file" ]; then
        fix_file "$file"
    fi
done

echo "✓ Quick fixes complete\!"
