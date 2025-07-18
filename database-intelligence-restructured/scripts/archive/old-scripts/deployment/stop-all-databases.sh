#!/bin/bash
# Script to stop all database containers and their OpenTelemetry collectors

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Stopping all database services...${NC}"

# Stop all services
docker-compose -f docker-compose.databases.yml down

echo -e "${GREEN}All services stopped!${NC}"

# Optional: Remove volumes (uncomment if needed)
# read -p "Do you want to remove data volumes? (y/N) " -n 1 -r
# echo
# if [[ $REPLY =~ ^[Yy]$ ]]; then
#     echo -e "${RED}Removing data volumes...${NC}"
#     docker-compose -f docker-compose.databases.yml down -v
# fi