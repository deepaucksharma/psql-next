#!/bin/bash
# Unified MySQL Intelligence Monitoring Launcher
# Simplifies deployment with intelligent defaults

set -e

# Colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Banner
echo -e "${BLUE}"
echo "╔══════════════════════════════════════════════════════╗"
echo "║     MySQL Intelligence Monitoring Launcher           ║"
echo "║     Powered by OpenTelemetry & New Relic            ║"
echo "╚══════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Function to show usage
show_usage() {
    echo "Usage: ./start.sh [MODE] [OPTIONS]"
    echo
    echo "MODES:"
    echo "  quick       Quick start with Docker Compose (default)"
    echo "  deploy      Advanced deployment with custom settings"
    echo "  test        Run full test suite"
    echo "  validate    Validate configuration and metrics"
    echo "  stop        Stop all services"
    echo
    echo "OPTIONS:"
    echo "  --minimal   Use minimal resources"
    echo "  --standard  Use standard production settings"
    echo "  --advanced  Enable all features (default)"
    echo "  --debug     Enable debug mode"
    echo "  --workload  Generate sample workload"
    echo
    echo "EXAMPLES:"
    echo "  ./start.sh                    # Quick start with advanced mode"
    echo "  ./start.sh deploy --minimal   # Deploy with minimal resources"
    echo "  ./start.sh test               # Run test suite"
    echo "  ./start.sh stop               # Stop all services"
}

# Parse arguments
MODE=${1:-quick}
DEPLOYMENT_MODE="advanced"
WITH_WORKLOAD=false

for arg in "$@"; do
    case $arg in
        --minimal)
            DEPLOYMENT_MODE="minimal"
            ;;
        --standard)
            DEPLOYMENT_MODE="standard"
            ;;
        --advanced)
            DEPLOYMENT_MODE="advanced"
            ;;
        --debug)
            DEPLOYMENT_MODE="debug"
            ;;
        --workload)
            WITH_WORKLOAD=true
            ;;
        --help|-h)
            show_usage
            exit 0
            ;;
    esac
done

# Check prerequisites
check_prerequisites() {
    echo -e "${YELLOW}Checking prerequisites...${NC}"
    
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}❌ Docker not found. Please install Docker.${NC}"
        exit 1
    fi
    
    if ! docker compose version &> /dev/null; then
        echo -e "${RED}❌ Docker Compose V2 not found.${NC}"
        exit 1
    fi
    
    if [ ! -f .env ] && [ -f .env.example ]; then
        echo -e "${YELLOW}Creating .env from example...${NC}"
        cp .env.example .env
        echo -e "${RED}⚠️  Please edit .env with your New Relic credentials!${NC}"
        echo "Press Enter after updating .env..."
        read
    fi
    
    echo -e "${GREEN}✅ Prerequisites checked${NC}"
}

# Execute based on mode
case $MODE in
    quick)
        check_prerequisites
        echo -e "${BLUE}Starting with Docker Compose (mode: $DEPLOYMENT_MODE)...${NC}"
        export DEPLOYMENT_MODE
        docker compose up -d
        echo
        echo -e "${GREEN}✅ Services started!${NC}"
        echo "• Health check: http://localhost:13133/"
        echo "• Metrics: http://localhost:8888/metrics"
        echo "• Validate: ./operate/validate-metrics.sh"
        
        if [ "$WITH_WORKLOAD" = true ]; then
            echo -e "${YELLOW}Starting workload generator...${NC}"
            nohup ./operate/generate-workload.sh > /tmp/workload.log 2>&1 &
            echo -e "${GREEN}✅ Workload generator started${NC}"
        fi
        ;;
        
    deploy)
        check_prerequisites
        echo -e "${BLUE}Running advanced deployment (mode: $DEPLOYMENT_MODE)...${NC}"
        export DEPLOYMENT_MODE
        if [ "$WITH_WORKLOAD" = true ]; then
            ./deploy/deploy.sh --with-workload
        else
            ./deploy/deploy.sh
        fi
        ;;
        
    test)
        echo -e "${BLUE}Running test suite...${NC}"
        ./operate/full-test.sh
        ;;
        
    validate)
        echo -e "${BLUE}Validating configuration and metrics...${NC}"
        ./operate/diagnose.sh
        echo
        ./operate/test-connection.sh
        echo
        ./operate/validate-metrics.sh
        ;;
        
    stop)
        echo -e "${YELLOW}Stopping all services...${NC}"
        docker compose down
        pkill -f generate-workload.sh 2>/dev/null || true
        echo -e "${GREEN}✅ All services stopped${NC}"
        ;;
        
    *)
        echo -e "${RED}Unknown mode: $MODE${NC}"
        show_usage
        exit 1
        ;;
esac

# Show next steps
if [ "$MODE" != "stop" ]; then
    echo
    echo -e "${BLUE}Next steps:${NC}"
    echo "1. Check metrics in New Relic: https://one.newrelic.com"
    echo "2. Import dashboard: config/newrelic/dashboards.json"
    echo "3. View logs: docker compose logs -f"
    echo "4. Generate load: ./operate/generate-workload.sh"
fi