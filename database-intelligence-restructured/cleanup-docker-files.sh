#!/bin/bash

# Cleanup redundant Docker Compose files
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
DOCKER_DIR="$PROJECT_ROOT/deployments/docker/compose"
BACKUP_DIR="/Users/deepaksharma/syc/db-otel/backup-docker-cleanup-$(date +%Y%m%d-%H%M%S)"

echo -e "${BLUE}=== Cleaning Up Docker Compose Files ===${NC}"

mkdir -p "$BACKUP_DIR"
cd "$DOCKER_DIR"

# Define which files to keep
KEEP_FILES=(
    "docker-compose.yaml"          # Default development
    "docker-compose.prod.yaml"     # Production
    "docker-compose.test.yaml"     # Testing
    "docker-compose-databases.yaml" # Database-only setup
    ".env.example"                 # Environment template
)

# List current files
echo -e "\n${BLUE}Current Docker Compose files:${NC}"
ls -la *.y*ml 2>/dev/null | awk '{print "  - " $9}'

# Backup and remove redundant files
echo -e "\n${BLUE}Removing redundant files...${NC}"
for file in *.yml *.yaml; do
    if [[ ! " ${KEEP_FILES[@]} " =~ " ${file} " ]]; then
        if [ -f "$file" ]; then
            mv "$file" "$BACKUP_DIR/"
            echo -e "${YELLOW}[!]${NC} Moved $file to backup"
        fi
    fi
done

# Rename any .yml files to .yaml for consistency
for file in *.yml; do
    if [ -f "$file" ]; then
        newname="${file%.yml}.yaml"
        if [ ! -f "$newname" ]; then
            mv "$file" "$newname"
            echo -e "${GREEN}[✓]${NC} Renamed $file to $newname"
        else
            mv "$file" "$BACKUP_DIR/"
            echo -e "${YELLOW}[!]${NC} Backed up duplicate $file"
        fi
    fi
done

# Create a consolidated high-availability configuration
if [ ! -f "docker-compose-ha.yaml" ]; then
    cat > docker-compose-ha.yaml << 'EOF'
# Database Intelligence - High Availability Configuration
# This configuration includes replicas and load balancing

version: '3.8'

services:
  # PostgreSQL Primary
  postgres-primary:
    image: postgres:16
    environment:
      POSTGRES_DB: testdb
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
      POSTGRES_REPLICATION_MODE: master
      POSTGRES_REPLICATION_USER: replicator
      POSTGRES_REPLICATION_PASSWORD: replicator_password
    volumes:
      - postgres_primary_data:/var/lib/postgresql/data
    networks:
      - database-intelligence

  # PostgreSQL Replica
  postgres-replica:
    image: postgres:16
    environment:
      POSTGRES_REPLICATION_MODE: slave
      POSTGRES_MASTER_HOST: postgres-primary
      POSTGRES_MASTER_PORT_NUMBER: 5432
      POSTGRES_REPLICATION_USER: replicator
      POSTGRES_REPLICATION_PASSWORD: replicator_password
    depends_on:
      - postgres-primary
    networks:
      - database-intelligence

  # Multiple collector instances
  collector-1:
    extends:
      file: docker-compose.yaml
      service: collector
    container_name: db-intel-collector-1
    networks:
      - database-intelligence

  collector-2:
    extends:
      file: docker-compose.yaml
      service: collector
    container_name: db-intel-collector-2
    networks:
      - database-intelligence

  # Load balancer
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx-ha.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - collector-1
      - collector-2
    networks:
      - database-intelligence

volumes:
  postgres_primary_data:
  postgres_replica_data:

networks:
  database-intelligence:
    driver: bridge
EOF
    echo -e "${GREEN}[✓]${NC} Created docker-compose-ha.yaml"
fi

# Update main docker-compose.yaml to be cleaner
echo -e "\n${BLUE}Optimizing main docker-compose.yaml...${NC}"
if [ -f "docker-compose.yaml" ]; then
    # Check if it needs optimization
    if grep -q "Database Intelligence - Unified Docker Compose" docker-compose.yaml; then
        echo -e "${GREEN}[✓]${NC} Main docker-compose.yaml already optimized"
    fi
fi

# Final cleanup
echo -e "\n${BLUE}Final Docker Compose files:${NC}"
ls -la *.yaml 2>/dev/null | awk '{print "  - " $9}'

echo -e "\n${GREEN}[✓]${NC} Docker Compose cleanup complete"
echo -e "Backup location: $BACKUP_DIR"