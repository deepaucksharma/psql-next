#!/bin/bash
# Script to standardize all README files

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== README Standardization Tool ===${NC}"

# Function to create a standardized README
create_readme() {
    local dir=$1
    local title=$2
    local description=$3
    local readme_path="$dir/README.md"
    
    echo -e "${YELLOW}Creating README for $dir...${NC}"
    
    cat > "$readme_path" << EOF
# $title

$description

## Overview

This directory contains $(basename "$dir") for the Database Intelligence project.

## Contents

EOF

    # Add directory contents
    if [ -d "$dir" ]; then
        # List subdirectories
        local subdirs=$(find "$dir" -maxdepth 1 -type d -not -path "$dir" | sort)
        if [ -n "$subdirs" ]; then
            echo "### Directories" >> "$readme_path"
            echo "" >> "$readme_path"
            echo "$subdirs" | while read subdir; do
                if [ -n "$subdir" ]; then
                    echo "- \`$(basename "$subdir")/\` - $(get_dir_description "$subdir")" >> "$readme_path"
                fi
            done
            echo "" >> "$readme_path"
        fi
        
        # List important files
        local files=$(find "$dir" -maxdepth 1 -type f -name "*.yaml" -o -name "*.yml" -o -name "*.sh" | sort | head -10)
        if [ -n "$files" ]; then
            echo "### Key Files" >> "$readme_path"
            echo "" >> "$readme_path"
            echo "$files" | while read file; do
                if [ -n "$file" ]; then
                    echo "- \`$(basename "$file")\` - $(get_file_description "$file")" >> "$readme_path"
                fi
            done
            echo "" >> "$readme_path"
        fi
    fi
    
    # Add usage section
    cat >> "$readme_path" << EOF

## Usage

See the main [README](../README.md) for general usage instructions.

## Related Documentation

- [Architecture Guide](../docs/reference/ARCHITECTURE.md)
- [Configuration Guide](../docs/guides/CONFIGURATION.md)
- [Troubleshooting Guide](../docs/guides/TROUBLESHOOTING.md)
EOF

    echo -e "${GREEN}✓ Created $readme_path${NC}"
}

# Function to get directory description
get_dir_description() {
    local dir=$1
    case "$(basename "$dir")" in
        "validation") echo "Validation scripts and tools" ;;
        "testing") echo "Test execution scripts" ;;
        "building") echo "Build and compilation scripts" ;;
        "configs") echo "Configuration files" ;;
        "guides") echo "User and operator guides" ;;
        "reference") echo "API and architecture reference" ;;
        "development") echo "Developer documentation" ;;
        "docker") echo "Docker-related files" ;;
        "kubernetes") echo "Kubernetes manifests" ;;
        *) echo "$(basename "$dir") files" ;;
    esac
}

# Function to get file description
get_file_description() {
    local file=$1
    local basename=$(basename "$file")
    
    # Check for specific files
    case "$basename" in
        *"validate"*) echo "Validation script" ;;
        *"test"*) echo "Test script" ;;
        *"build"*) echo "Build script" ;;
        *"maximum-extraction"*) echo "Maximum metrics extraction configuration" ;;
        *"docker-compose"*) echo "Docker Compose configuration" ;;
        *.yaml|*.yml) echo "Configuration file" ;;
        *.sh) echo "Shell script" ;;
        *) echo "$(basename "$file" | sed 's/[-_]/ /g')" ;;
    esac
}

# Update main directories
echo -e "${YELLOW}Updating main directory READMEs...${NC}"

# Scripts README
create_readme "scripts" "Scripts" "Executable scripts for various operations including validation, testing, building, and maintenance."

# Configs README
create_readme "configs" "Configurations" "OpenTelemetry collector configurations for different databases and deployment scenarios."

# Tests README
create_readme "tests" "Tests" "Test suites including unit tests, integration tests, and end-to-end tests."

# Deployments README
create_readme "deployments" "Deployments" "Deployment configurations and examples for various platforms."

# Update subdirectory READMEs
echo -e "\n${YELLOW}Updating subdirectory READMEs...${NC}"

# Script subdirectories
for dir in scripts/validation scripts/testing scripts/building; do
    if [ -d "$dir" ]; then
        case "$(basename "$dir")" in
            "validation") 
                create_readme "$dir" "Validation Scripts" "Scripts for validating configurations, metrics, and system state."
                ;;
            "testing")
                create_readme "$dir" "Testing Scripts" "Scripts for running various test suites and integration tests."
                ;;
            "building")
                create_readme "$dir" "Build Scripts" "Scripts for building the collector and related components."
                ;;
        esac
    fi
done

# Create main project README update
echo -e "\n${YELLOW}Updating main project README...${NC}"

cat > README_UPDATED.md << 'EOF'
# Database Intelligence - Restructured

Comprehensive database monitoring solution using OpenTelemetry collectors to extract maximum metrics from PostgreSQL, MySQL, MongoDB, MSSQL, and Oracle databases.

## Quick Start

```bash
# 1. Set up environment
cp configs/env-templates/postgresql.env .env

# 2. Validate configuration
./scripts/validate-all.sh

# 3. Start collector
docker-compose -f docker-compose.databases.yml up -d
```

## Project Structure

```
database-intelligence-restructured/
├── configs/                 # OpenTelemetry configurations
│   ├── *-maximum-extraction.yaml  # Database-specific configs
│   └── env-templates/       # Environment variable templates
├── scripts/                 # Executable scripts
│   ├── validation/          # Configuration validators
│   ├── testing/             # Test runners
│   ├── building/            # Build tools
│   └── maintenance/         # Cleanup and fixes
├── docs/                    # Documentation
│   ├── guides/              # User guides
│   ├── reference/           # Technical reference
│   └── development/         # Developer docs
├── tests/                   # Test suites
│   ├── unit/                # Unit tests
│   ├── integration/         # Integration tests
│   └── e2e/                 # End-to-end tests
└── deployments/             # Deployment configs
    ├── docker/              # Docker files
    └── kubernetes/          # K8s manifests
```

## Supported Databases

- **PostgreSQL**: 100+ metrics including ASH simulation
- **MySQL**: 80+ metrics with Performance Schema
- **MongoDB**: 90+ metrics including Atlas support
- **MSSQL**: 100+ metrics with wait statistics
- **Oracle**: 120+ metrics via V$ views

## Key Features

- ✅ Config-only approach (no custom code required)
- ✅ Maximum metric extraction from each database
- ✅ Production-ready configurations
- ✅ Multi-pipeline architecture for performance
- ✅ Comprehensive documentation and guides
- ✅ Full test coverage

## Documentation

- [Quick Start Guide](docs/guides/QUICK_START.md)
- [Configuration Guide](docs/guides/CONFIGURATION.md)
- [Deployment Guide](docs/guides/UNIFIED_DEPLOYMENT_GUIDE.md)
- [Troubleshooting](docs/guides/TROUBLESHOOTING.md)
- [Architecture](docs/reference/ARCHITECTURE.md)

## Database-Specific Guides

- [PostgreSQL Maximum Extraction](docs/guides/CONFIG_ONLY_MAXIMUM_GUIDE.md)
- [MySQL Maximum Extraction](docs/guides/MYSQL_MAXIMUM_GUIDE.md)
- [MongoDB Maximum Extraction](docs/guides/MONGODB_MAXIMUM_GUIDE.md)
- [MSSQL Maximum Extraction](docs/guides/MSSQL_MAXIMUM_GUIDE.md)
- [Oracle Maximum Extraction](docs/guides/ORACLE_MAXIMUM_GUIDE.md)

## Testing

```bash
# Run all validations
./scripts/validate-all.sh

# Test specific database
./scripts/testing/test-database-config.sh postgresql

# Run integration tests
./scripts/testing/test-integration.sh all

# Performance benchmark
./scripts/testing/benchmark-performance.sh postgresql
```

## Contributing

1. Run validations before committing: `./scripts/validate-all.sh`
2. Follow the naming conventions in configs and scripts
3. Update documentation for any new features
4. Add tests for new functionality

## License

[License information]
EOF

echo -e "${GREEN}✓ Created README_UPDATED.md${NC}"

# Summary
echo -e "\n${BLUE}=== README Standardization Complete ===${NC}"
echo "Updated READMEs for:"
echo "  - Main directories (scripts, configs, tests, deployments)"
echo "  - Script subdirectories (validation, testing, building)"
echo "  - Created updated main README template"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Review README_UPDATED.md and replace main README.md"
echo "2. Remove redundant README files in subdirectories"
echo "3. Ensure all READMEs follow the same format"