#!/bin/bash

# Fix receiver imports for scraperhelper and scrapererror packages
# These packages have moved from receiver module to scraper module

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
cd "$PROJECT_ROOT"

echo -e "${BLUE}=== FIXING RECEIVER IMPORTS ===${NC}"

# Function to fix imports in a receiver
fix_receiver_imports() {
    local recv_name=$1
    local recv_path="receivers/$recv_name"
    
    if [ -d "$recv_path" ]; then
        echo -e "\n${YELLOW}Fixing imports in $recv_name receiver...${NC}"
        
        # Find all Go files that use scraperhelper or scrapererror
        find "$recv_path" -name "*.go" -type f | while read -r file; do
            if grep -q "receiver/scraperhelper\|receiver/scrapererror" "$file"; then
                echo "  Updating imports in $(basename "$file")"
                
                # Update scraperhelper import
                sed -i.bak 's|go.opentelemetry.io/collector/receiver/scraperhelper|go.opentelemetry.io/collector/scraper/scraperhelper|g' "$file"
                
                # Update scrapererror import
                sed -i.bak 's|go.opentelemetry.io/collector/receiver/scrapererror|go.opentelemetry.io/collector/scraper/scrapererror|g' "$file"
                
                # Clean up backup files
                rm -f "${file}.bak"
            fi
        done
        
        echo -e "${GREEN}[✓]${NC} $recv_name imports fixed"
    fi
}

# Fix imports in all receivers
for recv in ash enhancedsql kernelmetrics; do
    fix_receiver_imports "$recv"
done

echo -e "\n${GREEN}Import fixes complete!${NC}"
echo -e "\nNext step: Continue with version updates for receivers"