#!/bin/bash
# Quick start deployment for immediate MySQL monitoring with comprehensive features
# Version: 2.0.0

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}      MySQL Master Configuration Deployment             ${NC}"
echo -e "${BLUE}         Mode: ${DEPLOYMENT_MODE^^}                      ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"

# Check for required environment variables
if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
    echo -e "${RED}ERROR: NEW_RELIC_LICENSE_KEY not set${NC}"
    echo "Please set: export NEW_RELIC_LICENSE_KEY='your-key-here'"
    exit 1
fi

# Set default values
export MYSQL_PRIMARY_ENDPOINT=${MYSQL_PRIMARY_ENDPOINT:-"localhost:3306"}
export MYSQL_REPLICA_ENDPOINT=${MYSQL_REPLICA_ENDPOINT:-"localhost:3307"}
export MYSQL_USER=${MYSQL_USER:-"otel_monitor"}
export MYSQL_PASSWORD=${MYSQL_PASSWORD:-"otel_password"}
export MYSQL_DATABASE=${MYSQL_DATABASE:-"test"}
export MYSQL_VERSION=${MYSQL_VERSION:-"8.0"}
export NAMESPACE=${NAMESPACE:-"mysql-monitoring"}
export ENVIRONMENT=${ENVIRONMENT:-"development"}
export CLOUD_PROVIDER=${CLOUD_PROVIDER:-"local"}
export CLOUD_REGION=${CLOUD_REGION:-"us-east-1"}
export TEAM_NAME=${TEAM_NAME:-"database-team"}
export COST_CENTER=${COST_CENTER:-"engineering"}
export CLUSTER_NAME=${CLUSTER_NAME:-"mysql-cluster"}
export MYSQL_EXPORTER_ENDPOINT=${MYSQL_EXPORTER_ENDPOINT:-"localhost:9104"}
export MYSQL_ROLE=${MYSQL_ROLE:-"primary"}
export NEW_RELIC_OTLP_ENDPOINT=${NEW_RELIC_OTLP_ENDPOINT:-"https://otlp.nr-data.net:4318"}
export NEW_RELIC_API_KEY=${NEW_RELIC_API_KEY:-$NEW_RELIC_LICENSE_KEY}
export NEW_RELIC_ACCOUNT_ID=${NEW_RELIC_ACCOUNT_ID:-""}
export OTEL_FILE_STORAGE_DIR=${OTEL_FILE_STORAGE_DIR:-"/tmp/otel-storage"}

# Master configuration deployment settings
export DEPLOYMENT_MODE=${DEPLOYMENT_MODE:-"advanced"}
export ENABLE_SQL_INTELLIGENCE=${ENABLE_SQL_INTELLIGENCE:-"true"}
export MYSQL_COLLECTION_INTERVAL=${MYSQL_COLLECTION_INTERVAL:-"5s"}
export SQL_INTELLIGENCE_INTERVAL=${SQL_INTELLIGENCE_INTERVAL:-"5s"}
export BATCH_SIZE=${BATCH_SIZE:-"1000"}
export MEMORY_LIMIT_PERCENT=${MEMORY_LIMIT_PERCENT:-"80"}
export SCRAPE_INTERVAL=${SCRAPE_INTERVAL:-"10s"}
export PERFORMANCE_SCHEMA_EVENTS_LIMIT=${PERFORMANCE_SCHEMA_EVENTS_LIMIT:-"100"}
export WAIT_PROFILE_ENABLED=${WAIT_PROFILE_ENABLED:-"true"}
export ML_FEATURES_ENABLED=${ML_FEATURES_ENABLED:-"true"}
export BUSINESS_CONTEXT_ENABLED=${BUSINESS_CONTEXT_ENABLED:-"true"}
export ANOMALY_DETECTION_ENABLED=${ANOMALY_DETECTION_ENABLED:-"true"}
export ADVISOR_ENGINE_ENABLED=${ADVISOR_ENGINE_ENABLED:-"true"}

echo -e "${GREEN}✅ Prerequisites checked${NC}"

# Check if MySQL is running (Docker or local)
echo -e "${YELLOW}1. Checking MySQL status...${NC}"
if docker ps | grep -q mysql-primary; then
    echo -e "${GREEN}✅ MySQL is running in Docker${NC}"
    export MYSQL_PRIMARY_ENDPOINT="mysql-primary:3306"
elif mysql -h localhost -P 3306 -u$MYSQL_USER -p$MYSQL_PASSWORD -e "SELECT 1" >/dev/null 2>&1; then
    echo -e "${GREEN}✅ MySQL is running locally${NC}"
else
    echo -e "${RED}❌ MySQL not found. Starting MySQL in Docker...${NC}"
    docker run -d \
        --name mysql-primary \
        -e MYSQL_ROOT_PASSWORD=rootpassword \
        -e MYSQL_DATABASE=test \
        -e MYSQL_USER=$MYSQL_USER \
        -e MYSQL_PASSWORD=$MYSQL_PASSWORD \
        -p 3306:3306 \
        mysql:8.0 \
        --performance-schema=ON \
        --performance-schema-instrument='%=ON'
    echo "Waiting for MySQL to start..."
    sleep 30
    export MYSQL_PRIMARY_ENDPOINT="mysql-primary:3306"
fi

# Initialize MySQL with data
echo -e "${YELLOW}2. Initializing MySQL database...${NC}"
cd "$(dirname "$0")"
if [ -f "initialize-mysql-comprehensive.sh" ]; then
    chmod +x initialize-mysql-comprehensive.sh
    ./initialize-mysql-comprehensive.sh
    echo -e "${GREEN}✅ MySQL initialized${NC}"
else
    echo -e "${YELLOW}⚠️  No initialization script found, skipping...${NC}"
fi

# Stop any existing collectors
echo -e "${YELLOW}3. Stopping existing collectors...${NC}"
docker stop mysql-comprehensive-collector mysql-quick-collector otel-collector 2>/dev/null || true
docker rm mysql-comprehensive-collector mysql-quick-collector otel-collector 2>/dev/null || true
echo -e "${GREEN}✅ Cleaned up old collectors${NC}"

# Prepare environment
echo -e "${YELLOW}4. Preparing environment...${NC}"
cd ..

# Create storage directory
mkdir -p $OTEL_FILE_STORAGE_DIR
echo -e "${GREEN}✅ Storage directory created${NC}"

# Deploy master collector
echo -e "${YELLOW}5. Deploying MySQL master collector (mode: $DEPLOYMENT_MODE)...${NC}"

# Determine network mode based on MySQL location
if docker ps | grep -q mysql-primary; then
    NETWORK_MODE="--network bridge --link mysql-primary:mysql-primary"
else
    NETWORK_MODE="--network host"
fi

docker run -d \
    --name mysql-comprehensive-collector \
    --restart unless-stopped \
    $NETWORK_MODE \
    --memory=2g \
    --cpus=2.0 \
    -e NEW_RELIC_API_KEY \
    -e NEW_RELIC_OTLP_ENDPOINT \
    -e NEW_RELIC_ACCOUNT_ID \
    -e MYSQL_PRIMARY_ENDPOINT \
    -e MYSQL_REPLICA_ENDPOINT \
    -e MYSQL_USER \
    -e MYSQL_PASSWORD \
    -e MYSQL_DATABASE \
    -e MYSQL_VERSION \
    -e NAMESPACE \
    -e ENVIRONMENT \
    -e CLOUD_PROVIDER \
    -e CLOUD_REGION \
    -e TEAM_NAME \
    -e COST_CENTER \
    -e CLUSTER_NAME \
    -e MYSQL_EXPORTER_ENDPOINT \
    -e MYSQL_ROLE \
    -e OTEL_FILE_STORAGE_DIR \
    -e DEPLOYMENT_MODE \
    -e ENABLE_SQL_INTELLIGENCE \
    -e MYSQL_COLLECTION_INTERVAL \
    -e SQL_INTELLIGENCE_INTERVAL \
    -e BATCH_SIZE \
    -e MEMORY_LIMIT_PERCENT \
    -e SCRAPE_INTERVAL \
    -e PERFORMANCE_SCHEMA_EVENTS_LIMIT \
    -e WAIT_PROFILE_ENABLED \
    -e ML_FEATURES_ENABLED \
    -e BUSINESS_CONTEXT_ENABLED \
    -e ANOMALY_DETECTION_ENABLED \
    -e ADVISOR_ENGINE_ENABLED \
    -v $(pwd)/config/collector/master.yaml:/etc/otelcol-contrib/config.yaml:ro \
    -v $OTEL_FILE_STORAGE_DIR:$OTEL_FILE_STORAGE_DIR \
    -p 8888:8888 \
    -p 8889:8889 \
    -p 13133:13133 \
    -p 55679:55679 \
    -p 1777:1777 \
    otel/opentelemetry-collector-contrib:latest

echo -e "${GREEN}✅ Master collector deployed in $DEPLOYMENT_MODE mode${NC}"

# Start workload generator if requested
if [ "$1" = "--with-workload" ]; then
    echo -e "${YELLOW}6. Starting workload generator...${NC}"
    if [ -f "operate/generate-workload.sh" ]; then
        chmod +x operate/generate-workload.sh
        nohup operate/generate-workload.sh > /tmp/mysql-workload.log 2>&1 &
        WORKLOAD_PID=$!
        echo "Workload generator PID: $WORKLOAD_PID"
        echo -e "${GREEN}✅ Workload generator started${NC}"
    fi
fi

# Wait and check
echo -e "${YELLOW}7. Waiting for metrics to flow...${NC}"
sleep 15

# Check collector health
echo -e "${YELLOW}8. Checking collector health...${NC}"
if curl -s http://localhost:13133/health | grep -q "Server available"; then
    echo -e "${GREEN}✅ Collector is healthy${NC}"
else
    echo -e "${RED}❌ Collector health check failed${NC}"
    echo "Checking logs..."
    docker logs mysql-comprehensive-collector --tail 50
fi

# Validate pipelines
echo -e "${YELLOW}9. Validating pipelines...${NC}"
ZPAGES=$(curl -s http://localhost:55679/debug/pipelinez 2>/dev/null || echo "")
if [ -n "$ZPAGES" ]; then
    if echo "$ZPAGES" | grep -q "metrics/critical_realtime"; then
        echo -e "${GREEN}✅ Critical realtime pipeline active${NC}"
    fi
    if echo "$ZPAGES" | grep -q "metrics/standard"; then
        echo -e "${GREEN}✅ Standard metrics pipeline active${NC}"
    fi
fi

# Check metrics endpoint
echo -e "${YELLOW}10. Checking metrics...${NC}"
METRICS=$(curl -s http://localhost:8888/metrics | grep -E "(mysql_|otelcol_)" | wc -l)
echo "Found $METRICS metric lines"

if [ $METRICS -gt 0 ]; then
    echo -e "${GREEN}✅ Metrics are being collected${NC}"
    
    # Show sample metrics
    echo -e "${YELLOW}Sample metrics:${NC}"
    curl -s http://localhost:8888/metrics | grep -E "mysql_intelligence_comprehensive|mysql_query_wait_profile|mysql_health_score" | head -5
else
    echo -e "${RED}❌ No metrics found${NC}"
fi

# Test Prometheus endpoint
echo -e "${YELLOW}11. Testing Prometheus endpoint...${NC}"
PROM_METRICS=$(curl -s http://localhost:8889/metrics | grep mysql_ | wc -l)
echo "Found $PROM_METRICS MySQL metrics in Prometheus format"

echo
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}    Master Configuration Deployed Successfully!         ${NC}"
echo -e "${GREEN}    Mode: $DEPLOYMENT_MODE | SQL Intelligence: $ENABLE_SQL_INTELLIGENCE${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo
echo "🔍 Check metrics locally:"
echo "   - Internal metrics: http://localhost:8888/metrics"
echo "   - Prometheus format: http://localhost:8889/metrics"
echo "   - Health check: http://localhost:13133/health"
echo "   - Pipeline status: http://localhost:55679/debug/pipelinez"
echo "   - Performance profiling: http://localhost:1777/debug/pprof/"
echo
echo "📊 Check New Relic - Key queries:"
echo "   - Basic: FROM Metric SELECT * WHERE instrumentation.provider = 'opentelemetry' SINCE 5 minutes ago"
echo "   - Intelligence: FROM Metric SELECT * WHERE metricName = 'mysql.intelligence.comprehensive' SINCE 5 minutes ago"
echo "   - Wait Analysis: FROM Metric SELECT * WHERE metricName = 'mysql.query.wait_profile' SINCE 5 minutes ago"
echo "   - Health Score: FROM Metric SELECT * WHERE metricName = 'mysql.health.score' SINCE 5 minutes ago"
echo "   - Advisories: FROM Metric SELECT * WHERE attributes.advisor.type IS NOT NULL SINCE 5 minutes ago"
echo "   - Anomalies: FROM Metric SELECT * WHERE attributes.ml.is_anomaly = true SINCE 5 minutes ago"
echo
if [ -n "$WORKLOAD_PID" ]; then
    echo "🛑 To stop workload: kill $WORKLOAD_PID"
fi
echo "📋 View collector logs: docker logs -f mysql-comprehensive-collector"
echo "🛑 Stop collector: docker stop mysql-comprehensive-collector"
echo
echo "⚠️  If no data in New Relic after 2 minutes:"
echo "   1. Check API key: echo \$NEW_RELIC_LICENSE_KEY"
echo "   2. Check errors: docker logs mysql-comprehensive-collector 2>&1 | grep -i error"
echo "   3. Test connection: docker exec mysql-comprehensive-collector nc -zv otlp.nr-data.net 4318"
echo "   4. Verify account ID is set if using NerdGraph features"
echo
echo "💡 Pro tip: Run with --with-workload to generate sample traffic"
echo
echo "🎛️  Deployment modes available:"
echo "   DEPLOYMENT_MODE=minimal ./deploy-quick-start.sh    # Basic metrics only"
echo "   DEPLOYMENT_MODE=standard ./deploy-quick-start.sh   # Production recommended"
echo "   DEPLOYMENT_MODE=advanced ./deploy-quick-start.sh   # Full intelligence (default)"
echo "   DEPLOYMENT_MODE=debug ./deploy-quick-start.sh      # Troubleshooting mode"