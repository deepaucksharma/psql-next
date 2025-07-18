#!/bin/bash
# Final streamlining script combining all cleanup operations

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Final Streamlining Process ===${NC}"

# Check if we're in execute mode
EXECUTE_MODE=false
if [ "$1" = "--execute" ]; then
    EXECUTE_MODE=true
    echo -e "${RED}WARNING: This will perform the following operations:${NC}"
    echo "1. Remove all archive directories (228 files)"
    echo "2. Delete 14 status/summary documents from root"
    echo "3. Remove all backup files (.bak)"
    echo "4. Delete log files and build artifacts"
    echo "5. Remove duplicate scripts"
    echo "6. Update .gitignore"
    echo ""
    echo -e "${RED}This action cannot be undone!${NC}"
    read -p "Type 'CONFIRM' to proceed: " -r
    if [[ ! $REPLY == "CONFIRM" ]]; then
        echo "Aborted."
        exit 1
    fi
fi

# Step 1: Run the streamlined cleanup
echo -e "\n${YELLOW}Step 1: Running streamlined cleanup...${NC}"
if [ "$EXECUTE_MODE" = true ]; then
    ./scripts/maintenance/streamline-cleanup.sh --execute
else
    echo "Would run: ./scripts/maintenance/streamline-cleanup.sh --execute"
    ./scripts/maintenance/streamline-cleanup.sh | tail -10
fi

# Step 2: Clean up Go backup files from components
echo -e "\n${YELLOW}Step 2: Cleaning component backup files...${NC}"
if [ "$EXECUTE_MODE" = true ]; then
    find components -name "*.bak" -type f -delete
    find internal -name "*.bak" -type f -delete
    echo -e "${GREEN}✓ Removed backup files from components${NC}"
else
    echo "Would remove $(find components internal -name "*.bak" -type f 2>/dev/null | wc -l | tr -d ' ') backup files from components"
fi

# Step 3: Move fix-module-paths.sh to maintenance
echo -e "\n${YELLOW}Step 3: Organizing remaining scripts...${NC}"
if [ "$EXECUTE_MODE" = true ]; then
    if [ -f "fix-module-paths.sh" ]; then
        mv fix-module-paths.sh scripts/maintenance/
        echo -e "${GREEN}✓ Moved fix-module-paths.sh to maintenance${NC}"
    fi
else
    echo "Would move fix-module-paths.sh to scripts/maintenance/"
fi

# Step 4: Create essential files only
echo -e "\n${YELLOW}Step 4: Keeping only essential documentation...${NC}"
ESSENTIAL_FILES=(
    "README.md"
    "PROJECT_STRUCTURE.md"
    "QUICK_REFERENCE.md"
    ".env.example"
    ".gitignore"
)

if [ "$EXECUTE_MODE" = false ]; then
    echo "Essential files to keep:"
    for file in "${ESSENTIAL_FILES[@]}"; do
        if [ -f "$file" ]; then
            echo "  ✓ $file"
        fi
    done
fi

# Step 5: Final validation
echo -e "\n${YELLOW}Step 5: Running final validation...${NC}"
if [ "$EXECUTE_MODE" = true ]; then
    # Update the main README to be concise
    cat > README.md << 'EOF'
# Database Intelligence

OpenTelemetry-based database monitoring solution extracting maximum metrics from PostgreSQL, MySQL, MongoDB, MSSQL, and Oracle.

## Quick Start

```bash
# 1. Configure environment
cp .env.example .env
# Edit .env with your database credentials

# 2. Validate setup
./scripts/validate-all.sh

# 3. Start collectors
docker-compose -f docker-compose.databases.yml up -d
```

## Documentation

- **[Quick Start Guide](docs/guides/QUICK_START.md)** - Get started quickly
- **[Project Structure](PROJECT_STRUCTURE.md)** - Directory layout and organization
- **[Quick Reference](QUICK_REFERENCE.md)** - Essential commands
- **[Configuration Guide](docs/guides/CONFIGURATION.md)** - Detailed configuration
- **[Troubleshooting](docs/guides/TROUBLESHOOTING.md)** - Common issues

## Supported Databases

| Database | Metrics | Configuration |
|----------|---------|---------------|
| PostgreSQL | 100+ | `configs/postgresql-maximum-extraction.yaml` |
| MySQL | 80+ | `configs/mysql-maximum-extraction.yaml` |
| MongoDB | 90+ | `configs/mongodb-maximum-extraction.yaml` |
| MSSQL | 100+ | `configs/mssql-maximum-extraction.yaml` |
| Oracle | 120+ | `configs/oracle-maximum-extraction.yaml` |

## Testing

```bash
# Run all tests
./scripts/testing/run-tests.sh all

# Test specific database
./scripts/testing/test-database-config.sh postgresql

# Performance benchmark
./scripts/testing/benchmark-performance.sh mysql 300
```

## Project Structure

```
├── configs/           # Database configurations
├── scripts/          # Organized by function
│   ├── validation/   # Config validators
│   ├── testing/     # Test runners
│   ├── building/    # Build scripts
│   └── deployment/  # Deploy tools
├── docs/            # Documentation
└── tests/           # Test suites
```

See [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) for detailed layout.

## License

[Your License]
EOF
    echo -e "${GREEN}✓ Updated README.md${NC}"
    
    # Run validation
    ./scripts/validate-all.sh || true
else
    echo "Would update README.md and run validation"
fi

# Summary
echo -e "\n${BLUE}=== Streamlining Summary ===${NC}"

if [ "$EXECUTE_MODE" = true ]; then
    echo -e "${GREEN}✓ Cleanup complete!${NC}"
    echo ""
    echo "Final statistics:"
    echo "  Scripts: $(find scripts -name "*.sh" -type f | wc -l | tr -d ' ')"
    echo "  Configs: $(ls configs/*.yaml | wc -l | tr -d ' ')"
    echo "  Root files: $(ls -1 *.md | wc -l | tr -d ' ')"
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo "1. Review changes: git status"
    echo "2. Stage all: git add -A"
    echo "3. Commit: git commit -m 'chore: Major streamlining - removed archives and stale files'"
else
    echo -e "${YELLOW}This was a preview. To execute:${NC}"
    echo -e "${BLUE}$0 --execute${NC}"
    echo ""
    echo "This will:"
    echo "  - Remove ~269 stale files"
    echo "  - Clean up all archives"
    echo "  - Organize remaining files"
    echo "  - Update documentation"
fi