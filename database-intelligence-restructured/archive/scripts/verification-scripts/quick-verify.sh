#!/bin/bash

# Quick verification script to check key aspects

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}=== QUICK PROJECT VERIFICATION ===${NC}"

cd /Users/deepaksharma/syc/db-otel/database-intelligence-restructured

# 1. Check critical directories
echo -e "\n${BLUE}1. Critical Directories:${NC}"
for dir in configs deployments docs processors receivers exporters extensions common distributions tests tools; do
    if [ -d "$dir" ]; then
        echo -e "${GREEN}[✓]${NC} $dir"
    else
        echo -e "${RED}[✗]${NC} $dir"
    fi
done

# 2. Check processors
echo -e "\n${BLUE}2. Processors:${NC}"
if [ -d "processors" ]; then
    for proc in processors/*/; do
        if [ -d "$proc" ]; then
            name=$(basename "$proc")
            if [ -f "$proc/go.mod" ]; then
                echo -e "${GREEN}[✓]${NC} $name (has go.mod)"
            else
                echo -e "${YELLOW}[!]${NC} $name (no go.mod)"
            fi
        fi
    done
fi

# 3. Check receivers
echo -e "\n${BLUE}3. Receivers:${NC}"
if [ -d "receivers" ]; then
    for recv in receivers/*/; do
        if [ -d "$recv" ]; then
            name=$(basename "$recv")
            if [ -f "$recv/factory.go" ]; then
                echo -e "${GREEN}[✓]${NC} $name"
            else
                echo -e "${YELLOW}[!]${NC} $name (no factory.go)"
            fi
        fi
    done
fi

# 4. Check critical files
echo -e "\n${BLUE}4. Critical Files:${NC}"
FILES=(
    "README.md"
    "go.work"
    "build.sh"
    "fix-dependencies.sh"
    "otelcol-builder-config.yaml"
)

for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
        echo -e "${GREEN}[✓]${NC} $file"
    else
        echo -e "${RED}[✗]${NC} $file"
    fi
done

# 5. Check Docker files
echo -e "\n${BLUE}5. Docker Files:${NC}"
docker_count=$(find deployments/docker/compose -name "docker-compose*.yaml" -type f 2>/dev/null | wc -l)
echo "Docker compose files in deployments: $docker_count"

# 6. Check for old imports
echo -e "\n${BLUE}6. Import Check:${NC}"
old_imports=$(grep -r "github.com/database-intelligence-mvp" . --include="*.go" --exclude-dir=backup* 2>/dev/null | wc -l || echo 0)
if [ "$old_imports" -eq 0 ]; then
    echo -e "${GREEN}[✓]${NC} No old imports found"
else
    echo -e "${RED}[✗]${NC} Found $old_imports old imports"
fi

# 7. Test Go compilation
echo -e "\n${BLUE}7. Go Compilation:${NC}"
echo 'package main; import "fmt"; func main() { fmt.Println("OK") }' > test.go
if go build test.go 2>/dev/null; then
    echo -e "${GREEN}[✓]${NC} Go compilation works"
    rm -f test test.go
else
    echo -e "${RED}[✗]${NC} Go compilation failed"
    rm -f test.go
fi

echo -e "\n${GREEN}Quick verification complete!${NC}"