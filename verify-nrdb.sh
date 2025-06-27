#!/bin/bash

# PostgreSQL Unified Collector - NRDB Verification Script
# This script verifies that PostgreSQL metrics are reaching NRDB

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
else
    echo -e "${RED}Error: .env file not found!${NC}"
    exit 1
fi

# Validate required environment variables
required_vars=("NEW_RELIC_API_KEY" "NEW_RELIC_ACCOUNT_ID")
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo -e "${RED}Error: $var is not set in .env file!${NC}"
        exit 1
    fi
done

# Set API endpoint based on region
if [ "$NEW_RELIC_REGION" = "EU" ]; then
    GRAPHQL_ENDPOINT="https://api.eu.newrelic.com/graphql"
else
    GRAPHQL_ENDPOINT="https://api.newrelic.com/graphql"
fi

# Functions
run_nrql_query() {
    local query="$1"
    local description="$2"
    
    echo -e "\n${BLUE}${description}${NC}"
    echo -e "${YELLOW}NRQL: ${query}${NC}\n"
    
    # Prepare GraphQL query
    local graphql_query=$(cat <<EOF
{
  "query": "{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \"$query\") { results } } } }"
}
EOF
)
    
    # Execute query
    local response=$(curl -s -X POST "$GRAPHQL_ENDPOINT" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "$graphql_query")
    
    # Parse and display results
    echo "$response" | jq -r '.data.actor.account.nrql.results' 2>/dev/null || {
        echo -e "${RED}Query failed. Response:${NC}"
        echo "$response" | jq '.'
    }
}

verify_slow_queries() {
    echo -e "${GREEN}=== Verifying PostgreSQL Slow Queries in NRDB ===${NC}"
    
    # Check if any slow queries exist
    run_nrql_query \
        "FROM PostgresSlowQueries SELECT count(*) WHERE newrelic = 'newrelic' SINCE 1 hour ago" \
        "Checking for slow query events"
    
    # Show recent slow queries
    run_nrql_query \
        "FROM PostgresSlowQueries SELECT query_text, avg_elapsed_time_ms, execution_count WHERE newrelic = 'newrelic' SINCE 1 hour ago LIMIT 10" \
        "Recent slow queries"
    
    # Top slowest queries
    run_nrql_query \
        "FROM PostgresSlowQueries SELECT query_text, avg_elapsed_time_ms, execution_count WHERE newrelic = 'newrelic' ORDER BY avg_elapsed_time_ms DESC LIMIT 5 SINCE 1 hour ago" \
        "Top 5 slowest queries"
    
    # Query performance over time
    run_nrql_query \
        "FROM PostgresSlowQueries SELECT average(avg_elapsed_time_ms) WHERE newrelic = 'newrelic' TIMESERIES 5 minutes SINCE 1 hour ago" \
        "Query performance trend"
}

verify_wait_events() {
    echo -e "\n${GREEN}=== Verifying PostgreSQL Wait Events ===${NC}"
    
    run_nrql_query \
        "FROM PostgresWaitSampling SELECT count(*) WHERE newrelic = 'newrelic' SINCE 1 hour ago" \
        "Checking for wait events"
    
    run_nrql_query \
        "FROM PostgresWaitSampling SELECT wait_event, wait_event_type, count(*) WHERE newrelic = 'newrelic' FACET wait_event SINCE 1 hour ago LIMIT 10" \
        "Wait event distribution"
}

verify_blocking_sessions() {
    echo -e "\n${GREEN}=== Verifying PostgreSQL Blocking Sessions ===${NC}"
    
    run_nrql_query \
        "FROM PostgresBlockingSessions SELECT count(*) WHERE newrelic = 'newrelic' SINCE 1 hour ago" \
        "Checking for blocking sessions"
}

verify_infrastructure_metrics() {
    echo -e "\n${GREEN}=== Verifying Infrastructure Integration ===${NC}"
    
    run_nrql_query \
        "FROM SystemSample SELECT * WHERE entityName LIKE '%postgres%' SINCE 5 minutes ago LIMIT 1" \
        "Checking Infrastructure Agent integration"
}

show_dashboard_queries() {
    echo -e "\n${GREEN}=== Dashboard NRQL Queries ===${NC}"
    
    cat << EOF

You can create a New Relic dashboard with these queries:

1. ${BLUE}Slow Query Count${NC}
   FROM PostgresSlowQueries SELECT count(*) 
   WHERE newrelic = 'newrelic' 
   TIMESERIES AUTO SINCE 1 hour ago

2. ${BLUE}Average Query Time${NC}
   FROM PostgresSlowQueries SELECT average(avg_elapsed_time_ms) 
   WHERE newrelic = 'newrelic' 
   TIMESERIES AUTO SINCE 1 hour ago

3. ${BLUE}Top Slow Queries Table${NC}
   FROM PostgresSlowQueries SELECT query_text, avg_elapsed_time_ms, execution_count 
   WHERE newrelic = 'newrelic' 
   ORDER BY avg_elapsed_time_ms DESC 
   LIMIT 20 SINCE 1 hour ago

4. ${BLUE}Query Types Distribution${NC}
   FROM PostgresSlowQueries SELECT count(*) 
   WHERE newrelic = 'newrelic' 
   FACET statement_type SINCE 1 hour ago

5. ${BLUE}Database Activity${NC}
   FROM PostgresSlowQueries SELECT count(*) 
   WHERE newrelic = 'newrelic' 
   FACET database_name TIMESERIES AUTO SINCE 1 hour ago

EOF
}

test_events_api() {
    echo -e "\n${GREEN}=== Testing Events API ===${NC}"
    
    # Create a test event
    local test_event=$(cat <<EOF
[{
    "eventType": "PostgresSlowQueries",
    "query_text": "TEST QUERY FROM VERIFICATION SCRIPT",
    "database_name": "testdb",
    "execution_count": 1,
    "query_id": "test-$(date +%s)",
    "avg_elapsed_time_ms": 1234.56,
    "newrelic": "newrelic"
}]
EOF
)
    
    echo -e "${YELLOW}Sending test event to Events API...${NC}"
    
    local response=$(curl -s -X POST \
        "https://insights-collector.newrelic.com/v1/accounts/$NEW_RELIC_ACCOUNT_ID/events" \
        -H "Content-Type: application/json" \
        -H "X-Insert-Key: $NEW_RELIC_LICENSE_KEY" \
        -d "$test_event")
    
    if [ "$response" = '{"success":true}' ]; then
        echo -e "${GREEN}Test event sent successfully!${NC}"
        echo "Wait 30 seconds for it to appear in NRDB..."
        sleep 30
        
        run_nrql_query \
            "FROM PostgresSlowQueries SELECT * WHERE query_text = 'TEST QUERY FROM VERIFICATION SCRIPT' SINCE 5 minutes ago" \
            "Verifying test event in NRDB"
    else
        echo -e "${RED}Failed to send test event. Response: $response${NC}"
    fi
}

# Main menu
show_menu() {
    cat << EOF

${GREEN}PostgreSQL Unified Collector - NRDB Verification${NC}

Select verification option:
1) Verify all metrics
2) Verify slow queries only
3) Verify wait events only
4) Verify blocking sessions only
5) Test Events API
6) Show dashboard queries
7) Exit

EOF
}

# Main script logic
if [ "$1" = "--all" ] || [ "$1" = "-a" ]; then
    verify_slow_queries
    verify_wait_events
    verify_blocking_sessions
    verify_infrastructure_metrics
    show_dashboard_queries
else
    while true; do
        show_menu
        read -p "Enter option (1-7): " option
        
        case $option in
            1)
                verify_slow_queries
                verify_wait_events
                verify_blocking_sessions
                verify_infrastructure_metrics
                ;;
            2)
                verify_slow_queries
                ;;
            3)
                verify_wait_events
                ;;
            4)
                verify_blocking_sessions
                ;;
            5)
                test_events_api
                ;;
            6)
                show_dashboard_queries
                ;;
            7)
                echo -e "${GREEN}Goodbye!${NC}"
                exit 0
                ;;
            *)
                echo -e "${RED}Invalid option!${NC}"
                ;;
        esac
        
        echo -e "\nPress Enter to continue..."
        read
    done
fi