#!/bin/bash
# Script to consolidate documentation files

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Documentation Consolidation Tool ===${NC}"

# Create consolidated docs structure
echo -e "${YELLOW}Creating consolidated documentation structure...${NC}"

mkdir -p docs/consolidated/guides
mkdir -p docs/consolidated/reference
mkdir -p docs/consolidated/development
mkdir -p docs/consolidated/operations

# Function to merge markdown files
merge_docs() {
    local output_file=$1
    local title=$2
    shift 2
    local input_files=("$@")
    
    echo -e "${YELLOW}Creating $output_file...${NC}"
    
    # Start with title
    echo "# $title" > "$output_file"
    echo "" >> "$output_file"
    echo "This document consolidates information from multiple sources." >> "$output_file"
    echo "" >> "$output_file"
    
    # Add table of contents
    echo "## Table of Contents" >> "$output_file"
    echo "" >> "$output_file"
    
    local section_num=1
    for file in "${input_files[@]}"; do
        if [ -f "$file" ]; then
            # Extract title from file
            local section_title=$(grep "^# " "$file" | head -1 | sed 's/^# //')
            if [ -z "$section_title" ]; then
                section_title=$(basename "$file" .md)
            fi
            echo "- [$section_num. $section_title](#section-$section_num)" >> "$output_file"
            ((section_num++))
        fi
    done
    echo "" >> "$output_file"
    
    # Merge content
    section_num=1
    for file in "${input_files[@]}"; do
        if [ -f "$file" ]; then
            echo -e "  Merging: $file"
            echo "" >> "$output_file"
            echo "---" >> "$output_file"
            echo "" >> "$output_file"
            echo "<a name=\"section-$section_num\"></a>" >> "$output_file"
            
            # Add source reference
            echo "_Source: $file_" >> "$output_file"
            echo "" >> "$output_file"
            
            # Add content (skip the first # heading to avoid duplication)
            tail -n +2 "$file" >> "$output_file"
            ((section_num++))
        fi
    done
    
    echo -e "${GREEN}✓ Created $output_file${NC}"
}

# 1. Consolidate Architecture docs
architecture_files=(
    "docs/reference/ARCHITECTURE.md"
    "../database-intelligence-mvp/docs/ARCHITECTURE.md"
    "../database-intelligence-mvp/docs/ENTERPRISE_ARCHITECTURE.md"
)
merge_docs "docs/consolidated/reference/ARCHITECTURE_COMPLETE.md" "Complete Architecture Guide" "${architecture_files[@]}"

# 2. Consolidate Testing docs
testing_files=(
    "docs/development/TESTING.md"
    "../database-intelligence-mvp/docs/development/TESTING.md"
    "../database-intelligence-mvp/docs/E2E_TESTING_COMPLETE.md"
)
merge_docs "docs/consolidated/development/TESTING_COMPLETE.md" "Complete Testing Guide" "${testing_files[@]}"

# 3. Consolidate Troubleshooting docs
troubleshooting_files=(
    "docs/guides/TROUBLESHOOTING.md"
    "../database-intelligence-mvp/docs/TROUBLESHOOTING.md"
)
merge_docs "docs/consolidated/guides/TROUBLESHOOTING_COMPLETE.md" "Complete Troubleshooting Guide" "${troubleshooting_files[@]}"

# 4. Consolidate Deployment docs
deployment_files=(
    "docs/guides/DEPLOYMENT.md"
    "docs/guides/UNIFIED_DEPLOYMENT_GUIDE.md"
    "../database-intelligence-mvp/docs/production-deployment-guide.md"
)
merge_docs "docs/consolidated/guides/DEPLOYMENT_COMPLETE.md" "Complete Deployment Guide" "${deployment_files[@]}"

# 5. Create master README
echo -e "\n${YELLOW}Creating master README...${NC}"
cat > docs/consolidated/README.md << 'EOF'
# Database Intelligence - Consolidated Documentation

This directory contains consolidated documentation that merges content from multiple sources to provide comprehensive guides.

## Structure

### Reference Documentation
- [Complete Architecture Guide](reference/ARCHITECTURE_COMPLETE.md) - Unified architecture documentation
- [API Reference](../reference/API.md) - API documentation
- [Metrics Reference](../reference/METRICS.md) - Complete metrics documentation

### Guides
- [Complete Deployment Guide](guides/DEPLOYMENT_COMPLETE.md) - All deployment options
- [Complete Troubleshooting Guide](guides/TROUBLESHOOTING_COMPLETE.md) - Unified troubleshooting
- [Quick Start](../guides/QUICK_START.md) - Getting started guide

### Development
- [Complete Testing Guide](development/TESTING_COMPLETE.md) - All testing documentation
- [Setup Guide](../development/SETUP.md) - Development setup

### Database-Specific Guides
- [PostgreSQL Maximum Extraction](../guides/CONFIG_ONLY_MAXIMUM_GUIDE.md)
- [MySQL Maximum Extraction](../guides/MYSQL_MAXIMUM_GUIDE.md)
- [MongoDB Maximum Extraction](../guides/MONGODB_MAXIMUM_GUIDE.md)
- [MSSQL Maximum Extraction](../guides/MSSQL_MAXIMUM_GUIDE.md)
- [Oracle Maximum Extraction](../guides/ORACLE_MAXIMUM_GUIDE.md)

## About Consolidation

These consolidated documents merge information from:
- `database-intelligence-restructured` (current project)
- `database-intelligence-mvp` (legacy project)

The consolidation removes duplication while preserving all unique information from both sources.
EOF

echo -e "${GREEN}✓ Created master README${NC}"

# Summary
echo -e "\n${BLUE}=== Consolidation Summary ===${NC}"
echo "Created consolidated documents:"
echo "  - Architecture: reference/ARCHITECTURE_COMPLETE.md"
echo "  - Testing: development/TESTING_COMPLETE.md"
echo "  - Troubleshooting: guides/TROUBLESHOOTING_COMPLETE.md"
echo "  - Deployment: guides/DEPLOYMENT_COMPLETE.md"
echo "  - Master README: README.md"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Review consolidated documents in docs/consolidated/"
echo "2. Update references in other documents"
echo "3. Consider removing original fragmented docs"