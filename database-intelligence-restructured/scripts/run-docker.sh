#!/bin/bash
# Run Database Intelligence with Docker

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
MODE="standard"
ACTION="up"
COMPOSE_FILE="deployments/docker/compose/docker-compose.yaml"

# Help function
show_help() {
    echo "Database Intelligence Docker Runner"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -m, --mode MODE        Collector mode: standard (default) or enterprise"
    echo "  -a, --action ACTION    Docker action: up (default), down, logs, restart"
    echo "  -e, --env-file FILE    Path to .env file (default: .env)"
    echo "  -h, --help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                    # Run standard mode"
    echo "  $0 -m enterprise      # Run enterprise mode"
    echo "  $0 -a logs           # View logs"
    echo "  $0 -a down           # Stop services"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--mode)
            MODE="$2"
            shift 2
            ;;
        -a|--action)
            ACTION="$2"
            shift 2
            ;;
        -e|--env-file)
            ENV_FILE="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# Validate mode
if [[ "$MODE" != "standard" && "$MODE" != "enterprise" ]]; then
    echo -e "${RED}Invalid mode: $MODE${NC}"
    echo "Mode must be 'standard' or 'enterprise'"
    exit 1
fi

# Check for .env file
if [[ ! -f "${ENV_FILE:-.env}" ]]; then
    echo -e "${YELLOW}Warning: .env file not found${NC}"
    echo "Creating from template..."
    cp configs/examples/.env.template .env
    echo -e "${GREEN}Created .env file. Please edit it with your settings.${NC}"
    exit 1
fi

# Load environment variables
set -a
source "${ENV_FILE:-.env}"
set +a

# Validate required variables
if [[ -z "$NEW_RELIC_LICENSE_KEY" || "$NEW_RELIC_LICENSE_KEY" == "your_license_key_here" ]]; then
    echo -e "${RED}Error: NEW_RELIC_LICENSE_KEY not set in .env file${NC}"
    exit 1
fi

# Set Docker Compose profile
export COMPOSE_PROFILES="$MODE"

# Execute action
case $ACTION in
    up)
        echo -e "${GREEN}Starting Database Intelligence in $MODE mode...${NC}"
        
        # Pull/build images first
        if [[ "$MODE" == "enterprise" ]]; then
            echo "Building enterprise image..."
            make docker-build-enterprise
        fi
        
        # Start services
        docker-compose -f "$COMPOSE_FILE" --profile "$MODE" up -d
        
        echo -e "${GREEN}Services started!${NC}"
        echo ""
        echo "Access points:"
        echo "  - Health check: http://localhost:13133/health"
        echo "  - Metrics: http://localhost:8888/metrics"
        echo "  - Prometheus: http://localhost:9090"
        if [[ "$MODE" == "enterprise" ]]; then
            echo "  - Enhanced metrics: http://localhost:8889/metrics"
            echo "  - pprof: http://localhost:1777/debug/pprof"
        fi
        echo ""
        echo "View logs: $0 -a logs"
        ;;
        
    down)
        echo -e "${YELLOW}Stopping Database Intelligence...${NC}"
        docker-compose -f "$COMPOSE_FILE" down
        echo -e "${GREEN}Services stopped${NC}"
        ;;
        
    logs)
        echo "Showing logs for $MODE mode..."
        if [[ "$MODE" == "standard" ]]; then
            docker-compose -f "$COMPOSE_FILE" logs -f collector-standard
        else
            docker-compose -f "$COMPOSE_FILE" logs -f collector-enterprise
        fi
        ;;
        
    restart)
        echo -e "${YELLOW}Restarting Database Intelligence...${NC}"
        $0 -m "$MODE" -a down
        sleep 2
        $0 -m "$MODE" -a up
        ;;
        
    *)
        echo -e "${RED}Invalid action: $ACTION${NC}"
        show_help
        exit 1
        ;;
esac