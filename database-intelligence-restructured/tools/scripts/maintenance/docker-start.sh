#!/bin/bash
# Start Database Intelligence Collector with Docker Compose

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}Starting Database Intelligence Collector...${NC}"

# Check if docker and docker-compose are installed
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker is not installed. Please install Docker first.${NC}"
    exit 1
fi

if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo -e "${RED}Docker Compose is not installed. Please install Docker Compose first.${NC}"
    exit 1
fi

# Create necessary directories
echo -e "${YELLOW}Creating data directories...${NC}"
mkdir -p data dashboards

# Stop any existing containers
echo -e "${YELLOW}Stopping existing containers...${NC}"
docker-compose down 2>/dev/null || true

# Build the collector image
echo -e "${YELLOW}Building collector image...${NC}"
docker-compose build otel-collector

# Start the services
echo -e "${YELLOW}Starting services...${NC}"
docker-compose up -d postgres mysql

# Wait for databases to be ready
echo -e "${YELLOW}Waiting for databases to be ready...${NC}"
sleep 10

# Start the collector
echo -e "${YELLOW}Starting collector...${NC}"
docker-compose up -d otel-collector

# Start monitoring services
echo -e "${YELLOW}Starting monitoring services...${NC}"
docker-compose up -d prometheus grafana

# Show status
echo -e "\n${GREEN}Services started successfully!${NC}"
echo -e "\nAccess points:"
echo -e "  - Grafana: ${GREEN}http://localhost:3000${NC} (admin/admin)"
echo -e "  - Prometheus: ${GREEN}http://localhost:9090${NC}"
echo -e "  - Collector Metrics: ${GREEN}http://localhost:8888/metrics${NC}"
echo -e "  - Collector Health: ${GREEN}http://localhost:13133${NC}"
echo -e "  - OTLP gRPC: ${GREEN}localhost:4317${NC}"
echo -e "  - OTLP HTTP: ${GREEN}http://localhost:4318${NC}"
echo -e "\nTo view logs:"
echo -e "  ${YELLOW}docker-compose logs -f otel-collector${NC}"
echo -e "\nTo stop all services:"
echo -e "  ${YELLOW}docker-compose down${NC}"
echo -e "\nTo run test data generator:"
echo -e "  ${YELLOW}docker-compose --profile test up test-generator${NC}"