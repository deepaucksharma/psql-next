#\!/bin/bash
# Display complete project summary

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Database Intelligence Project Summary ===${NC}"
echo ""

# Statistics
echo -e "${YELLOW}Project Statistics:${NC}"
echo "Scripts: $(find scripts -name "*.sh" -type f | wc -l | tr -d ' ') organized scripts"
echo "Configs: $(ls configs/*.yaml 2>/dev/null | wc -l | tr -d ' ') configuration files"
echo "Documentation: $(find docs -name "*.md" -type f | grep -v archive | wc -l | tr -d ' ') documentation files"
echo "Tests: $(find tests -name "*.sh" -type f 2>/dev/null | wc -l | tr -d ' ') test scripts"

echo -e "\n${YELLOW}Directory Structure:${NC}"
echo "scripts/"
for dir in scripts/*/; do
    if [ -d "$dir" ]; then
        echo "  └── $(basename $dir)/ ($(find "$dir" -name "*.sh" | wc -l | tr -d ' ') scripts)"
    fi
done

echo -e "\n${YELLOW}Supported Databases:${NC}"
for db in postgresql mysql mongodb mssql oracle; do
    if [ -f "configs/${db}-maximum-extraction.yaml" ]; then
        echo "  ✓ $db"
    fi
done

echo -e "\n${YELLOW}Key Commands:${NC}"
echo "  Validate: ./scripts/validate-all.sh"
echo "  Test: ./scripts/testing/run-tests.sh all"
echo "  Deploy: ./scripts/deployment/start-all-databases.sh"
echo "  Benchmark: ./scripts/testing/benchmark-performance.sh [database]"

echo -e "\n${YELLOW}Documentation:${NC}"
echo "  Quick Start: docs/guides/QUICK_START.md"
echo "  Deployment: docs/guides/UNIFIED_DEPLOYMENT_GUIDE.md"
echo "  Troubleshooting: docs/guides/TROUBLESHOOTING.md"
echo "  Project Structure: PROJECT_STRUCTURE.md"
echo "  Quick Reference: QUICK_REFERENCE.md"

echo -e "\n${GREEN}✓ Project successfully consolidated and organized\!${NC}"
