#!/bin/bash
# NerdGraph-based verification for Database Intelligence MVP
# Uses GraphQL API for advanced entity and relationship validation

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Source common functions
source "${SCRIPT_DIR}/lib/common.sh"

# Load environment
load_env_file "${PROJECT_ROOT}/.env"

# Validate required vars
check_env_var "NEW_RELIC_LICENSE_KEY"
check_env_var "NEW_RELIC_ACCOUNT_ID"

# NerdGraph endpoint
NERDGRAPH_URL="https://api.newrelic.com/graphql"

# ====================
# NerdGraph Queries
# ====================

execute_nerdgraph() {
    local query="$1"
    local description="$2"
    
    echo -e "${BLUE}Executing: ${description}${NC}"
    
    curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_LICENSE_KEY" \
        -d "{\"query\": \"$query\"}"
}

verify_database_entities() {
    echo -e "\n${YELLOW}=== Verifying Database Entities via NerdGraph ===${NC}\n"
    
    local query=$(cat <<'GRAPHQL'
{
  actor {
    entitySearch(query: "type = 'DATABASE' AND reporting = 'true'") {
      count
      results {
        entities {
          guid
          name
          type
          reporting
          tags {
            key
            values
          }
          ... on DatabaseEntityOutline {
            alertSeverity
            accountId
          }
        }
      }
    }
  }
}
GRAPHQL
    )
    
    local result=$(execute_nerdgraph "$query" "Searching for database entities")
    local count=$(echo "$result" | jq -r '.data.actor.entitySearch.count' 2>/dev/null || echo "0")
    
    if [[ "$count" -gt 0 ]]; then
        success "✓ Found $count database entities"
        echo "$result" | jq -r '.data.actor.entitySearch.results.entities[] | "    - \(.name) (\(.guid))"' 2>/dev/null
    else
        warning "⚠ No database entities found in NerdGraph"
    fi
}

verify_entity_relationships() {
    echo -e "\n${YELLOW}=== Verifying Entity Relationships ===${NC}\n"
    
    # First, get a database entity GUID
    local entity_query=$(cat <<'GRAPHQL'
{
  actor {
    entitySearch(query: "type = 'DATABASE' AND reporting = 'true'", options: {limit: 1}) {
      results {
        entities {
          guid
          name
        }
      }
    }
  }
}
GRAPHQL
    )
    
    local entity_result=$(execute_nerdgraph "$entity_query" "Getting sample database entity")
    local entity_guid=$(echo "$entity_result" | jq -r '.data.actor.entitySearch.results.entities[0].guid' 2>/dev/null)
    
    if [[ -n "$entity_guid" && "$entity_guid" != "null" ]]; then
        # Check relationships for this entity
        local relationship_query=$(cat <<GRAPHQL
{
  actor {
    entity(guid: "$entity_guid") {
      name
      relatedEntities {
        results {
          source {
            entity {
              name
              type
            }
          }
          target {
            entity {
              name
              type
            }
          }
          type
        }
      }
    }
  }
}
GRAPHQL
        )
        
        local rel_result=$(execute_nerdgraph "$relationship_query" "Checking entity relationships")
        local rel_count=$(echo "$rel_result" | jq -r '.data.actor.entity.relatedEntities.results | length' 2>/dev/null || echo "0")
        
        if [[ "$rel_count" -gt 0 ]]; then
            success "✓ Found $rel_count relationships"
            echo "$rel_result" | jq -r '.data.actor.entity.relatedEntities.results[] | "    - \(.type): \(.source.entity.name) → \(.target.entity.name)"' 2>/dev/null
        else
            warning "⚠ No relationships found for database entities"
        fi
    fi
}

verify_golden_metrics() {
    echo -e "\n${YELLOW}=== Verifying Golden Metrics ===${NC}\n"
    
    local query=$(cat <<'GRAPHQL'
{
  actor {
    account(id: NEW_RELIC_ACCOUNT_ID) {
      nrql(query: "SELECT average(duration_ms) as 'Latency', count(*) as 'Throughput', percentage(count(*), WHERE error = true) as 'Error Rate' FROM Log WHERE collector.name = 'database-intelligence' SINCE 1 hour ago") {
        results
      }
    }
  }
}
GRAPHQL
    )
    
    # Replace account ID in query
    query=${query//NEW_RELIC_ACCOUNT_ID/$NEW_RELIC_ACCOUNT_ID}
    
    local result=$(execute_nerdgraph "$query" "Checking golden metrics")
    
    if [[ -n "$result" ]]; then
        echo "$result" | jq -r '.data.actor.account.nrql.results[0] | "    Latency: \(.Latency)ms\n    Throughput: \(.Throughput) queries\n    Error Rate: \(."Error Rate")%"' 2>/dev/null || warning "⚠ No golden metrics data"
    fi
}

verify_alert_conditions() {
    echo -e "\n${YELLOW}=== Verifying Alert Conditions ===${NC}\n"
    
    local query=$(cat <<'GRAPHQL'
{
  actor {
    account(id: NEW_RELIC_ACCOUNT_ID) {
      alerts {
        nrqlConditionsSearch {
          nrqlConditions {
            name
            enabled
            nrql {
              query
            }
            terms {
              threshold
              thresholdDuration
              operator
              priority
            }
          }
        }
      }
    }
  }
}
GRAPHQL
    )
    
    query=${query//NEW_RELIC_ACCOUNT_ID/$NEW_RELIC_ACCOUNT_ID}
    
    local result=$(execute_nerdgraph "$query" "Checking alert conditions")
    local conditions=$(echo "$result" | jq -r '.data.actor.account.alerts.nrqlConditionsSearch.nrqlConditions[] | select(.nrql.query | contains("database") or contains("NrIntegrationError"))' 2>/dev/null)
    
    if [[ -n "$conditions" ]]; then
        success "✓ Found database-related alert conditions"
        echo "$conditions" | jq -r '. | "    - \(.name) (Enabled: \(.enabled))"' 2>/dev/null
    else
        warning "⚠ No database-related alert conditions found"
        echo "  Consider creating alerts for:"
        echo "    - NrIntegrationError events"
        echo "    - Data freshness"
        echo "    - Circuit breaker opens"
    fi
}

create_recommended_alerts() {
    echo -e "\n${YELLOW}=== Recommended Alert Configuration ===${NC}\n"
    
    cat > "${PROJECT_ROOT}/monitoring/recommended-alerts.json" <<'EOF'
{
  "alerts": [
    {
      "name": "Database Intelligence - Integration Errors",
      "description": "Alert when NrIntegrationError events are detected",
      "nrql": "SELECT count(*) FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' AND message LIKE '%database%'",
      "threshold": 10,
      "duration": 5,
      "priority": "CRITICAL"
    },
    {
      "name": "Database Intelligence - Data Freshness",
      "description": "Alert when no data received for 10 minutes",
      "nrql": "SELECT count(*) FROM Log WHERE collector.name = 'database-intelligence'",
      "threshold": 1,
      "duration": 10,
      "operator": "BELOW",
      "priority": "WARNING"
    },
    {
      "name": "Database Intelligence - Circuit Breaker Opens",
      "description": "Alert when circuit breaker opens for any database",
      "nrql": "SELECT sum(cb.opened_count) FROM Log WHERE cb.state = 'open'",
      "threshold": 1,
      "duration": 5,
      "priority": "WARNING"
    },
    {
      "name": "Database Intelligence - High Cardinality",
      "description": "Alert on cardinality warnings",
      "nrql": "SELECT count(*) FROM NrIntegrationError WHERE message LIKE '%cardinality%'",
      "threshold": 5,
      "duration": 30,
      "priority": "WARNING"
    },
    {
      "name": "Database Intelligence - Query Latency",
      "description": "Alert on high query latency",
      "nrql": "SELECT percentile(duration_ms, 95) FROM Log WHERE collector.name = 'database-intelligence'",
      "threshold": 5000,
      "duration": 5,
      "priority": "WARNING"
    }
  ]
}
EOF
    
    success "Recommended alerts saved to: monitoring/recommended-alerts.json"
}

# ====================
# Continuous Monitoring
# ====================

setup_continuous_monitoring() {
    echo -e "\n${YELLOW}=== Setting Up Continuous Monitoring ===${NC}\n"
    
    # Create monitoring script
    cat > "${PROJECT_ROOT}/scripts/continuous-monitor.sh" <<'SCRIPT'
#!/bin/bash
# Continuous monitoring script for Database Intelligence MVP

while true; do
    echo "=== $(date) ==="
    
    # Check for integration errors
    errors=$(curl -s -X POST "https://api.newrelic.com/graphql" \
        -H "API-Key: $NEW_RELIC_LICENSE_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"SELECT count(*) FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' SINCE 5 minutes ago\\\") { results } } } }\"}" | \
        jq -r '.data.actor.account.nrql.results[0].count')
    
    if [[ "$errors" -gt 0 ]]; then
        echo "⚠️  ALERT: $errors integration errors detected!"
    else
        echo "✅ No integration errors"
    fi
    
    # Check data freshness
    latest=$(curl -s -X POST "https://api.newrelic.com/graphql" \
        -H "API-Key: $NEW_RELIC_LICENSE_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"SELECT latest(timestamp) FROM Log WHERE collector.name = 'database-intelligence' SINCE 10 minutes ago\\\") { results } } } }\"}" | \
        jq -r '.data.actor.account.nrql.results[0].latest')
    
    if [[ -n "$latest" && "$latest" != "null" ]]; then
        echo "✅ Data is fresh (last: $latest)"
    else
        echo "⚠️  ALERT: No recent data!"
    fi
    
    echo ""
    sleep 300  # Check every 5 minutes
done
SCRIPT
    
    chmod +x "${PROJECT_ROOT}/scripts/continuous-monitor.sh"
    success "Continuous monitoring script created"
}

# ====================
# Main Execution
# ====================

main() {
    echo -e "${GREEN}=== NerdGraph-Based Verification Platform ===${NC}"
    echo -e "Account ID: $NEW_RELIC_ACCOUNT_ID\n"
    
    # Run verifications
    verify_database_entities
    verify_entity_relationships
    verify_golden_metrics
    verify_alert_conditions
    
    # Create configurations
    create_recommended_alerts
    setup_continuous_monitoring
    
    echo -e "\n${GREEN}=== Verification Complete ===${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Import recommended alerts from: monitoring/recommended-alerts.json"
    echo "2. Run continuous monitoring: ./scripts/continuous-monitor.sh"
    echo "3. Check the dashboard at: https://one.newrelic.com"
}

# Run main function
main "$@"