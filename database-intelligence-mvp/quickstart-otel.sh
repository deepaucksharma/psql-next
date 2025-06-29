#!/bin/bash
# Database Intelligence Collector - OTEL-First Quick Start Script

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Banner
echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}Database Intelligence Collector - OTEL-First${NC}"
echo -e "${GREEN}Quick Start Script${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker is not installed${NC}"
    echo "Please install Docker from https://docs.docker.com/get-docker/"
    exit 1
fi

# Check Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}Error: Docker Compose is not installed${NC}"
    echo "Please install Docker Compose from https://docs.docker.com/compose/install/"
    exit 1
fi

# Check Go (optional for local build)
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo -e "${GREEN}✓${NC} Go ${GO_VERSION} installed"
else
    echo -e "${YELLOW}⚠${NC} Go not installed (optional for Docker deployment)"
fi

# Check for .env file
if [ ! -f .env ]; then
    echo -e "${YELLOW}Creating .env file...${NC}"
    cat > .env << 'EOF'
# Database Configuration
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=testdb

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your-license-key-here
OTLP_ENDPOINT=otlp.nr-data.net:4317

# Environment
ENVIRONMENT=development
LOG_LEVEL=info
EOF
    echo -e "${GREEN}✓${NC} Created .env file"
    echo -e "${YELLOW}⚠${NC} Please edit .env and add your New Relic license key"
fi

# Check for New Relic license key
if grep -q "your-license-key-here" .env; then
    echo -e "${RED}Error: Please update NEW_RELIC_LICENSE_KEY in .env file${NC}"
    exit 1
fi

# Build collector
echo ""
echo -e "${YELLOW}Building Database Intelligence Collector...${NC}"
if [ -f "Makefile" ]; then
    make build || {
        echo -e "${YELLOW}⚠${NC} Local build failed, will use Docker build"
    }
fi

# Start services
echo ""
echo -e "${YELLOW}Starting services with Docker Compose...${NC}"
docker-compose -f deploy/docker-compose.yaml up -d

# Wait for services
echo ""
echo -e "${YELLOW}Waiting for services to start...${NC}"
sleep 10

# Check service health
echo ""
echo -e "${YELLOW}Checking service health...${NC}"

# Check PostgreSQL
if docker exec postgres-db pg_isready -U postgres > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} PostgreSQL is ready"
else
    echo -e "${RED}✗${NC} PostgreSQL is not ready"
fi

# Check Collector health
if curl -s http://localhost:13133/health > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Collector is healthy"
else
    echo -e "${RED}✗${NC} Collector is not healthy"
fi

# Check Prometheus
if curl -s http://localhost:9090/-/healthy > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Prometheus is ready"
else
    echo -e "${YELLOW}⚠${NC} Prometheus is not ready (optional)"
fi

# Display access information
echo ""
echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}Services are running!${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""
echo "Access points:"
echo "  • Collector Health: http://localhost:13133/health"
echo "  • Prometheus Metrics: http://localhost:8889/metrics"
echo "  • Prometheus UI: http://localhost:9090"
echo "  • Grafana: http://localhost:3000 (admin/admin)"
echo ""
echo "Useful commands:"
echo "  • View logs: docker logs -f db-intelligence-collector"
echo "  • Stop services: make docker-down"
echo "  • Restart collector: docker restart db-intelligence-collector"
echo ""
echo -e "${YELLOW}To verify New Relic integration:${NC}"
echo "1. Go to New Relic One"
echo "2. Navigate to Infrastructure > Third-party services"
echo "3. Look for 'database-monitoring' service"
echo ""
echo -e "${GREEN}Happy monitoring! 🚀${NC}"