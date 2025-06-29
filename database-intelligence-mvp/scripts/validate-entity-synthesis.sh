#!/bin/bash
# Validate New Relic Entity Synthesis for Database Intelligence MVP

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

# Validate environment
check_env_var "NEW_RELIC_LICENSE_KEY"
check_env_var "NEW_RELIC_ACCOUNT_ID"

# NRQL queries for entity validation
validate_entity_creation() {
    echo -e "${YELLOW}Validating entity creation...${NC}"
    
    # Query to check if database entities are being created
    local query="SELECT uniques(entity.guid) as 'Database Entities', \
                        uniques(database_name) as 'Unique Databases', \
                        latest(entity.type), \
                        latest(entity.name) \
                 FROM Log \
                 WHERE entity.type = 'DATABASE' \
                   AND instrumentation.provider = 'opentelemetry' \
                 FACET database_name \
                 SINCE 1 hour ago"
    
    echo "Running NRQL query to validate entity creation..."
    # Note: This would require New Relic CLI or API integration
    echo "Query: $query"
}

validate_entity_correlation() {
    echo -e "${YELLOW}Validating entity correlation...${NC}"
    
    local query="SELECT count(*) as 'Correlated Records', \
                        percentage(count(*), WHERE service.name IS NOT NULL) as 'Service Correlation %', \
                        percentage(count(*), WHERE host.id IS NOT NULL) as 'Host Correlation %' \
                 FROM Log \
                 WHERE collector.name = 'database-intelligence' \
                 FACET database_name \
                 SINCE 1 hour ago"
    
    echo "Query: $query"
}

check_missing_correlations() {
    echo -e "${YELLOW}Checking for missing correlations...${NC}"
    
    local query="SELECT count(*) as 'Missing Correlations', \
                        sample(query_text) as 'Sample Query' \
                 FROM Log \
                 WHERE service.name IS NULL \
                   AND entity.guid IS NULL \
                   AND database_name IS NOT NULL \
                 FACET database_name \
                 SINCE 1 hour ago"
    
    echo "Query: $query"
}

validate_required_attributes() {
    echo -e "${YELLOW}Validating required attributes...${NC}"
    
    # Check collector configuration for required attributes
    local config_file="${CONFIG_DIR:-./config}/collector-newrelic-optimized.yaml"
    
    if [[ -f "$config_file" ]]; then
        echo "Checking configuration for entity synthesis attributes..."
        
        # Required attributes for entity synthesis
        local required_attrs=(
            "entity.guid"
            "entity.type"
            "entity.name"
            "service.name"
            "host.id"
            "instrumentation.provider"
            "telemetry.sdk.name"
        )
        
        for attr in "${required_attrs[@]}"; do
            if grep -q "$attr" "$config_file"; then
                echo -e "${GREEN}✓${NC} Found attribute: $attr"
            else
                echo -e "${RED}✗${NC} Missing attribute: $attr"
            fi
        done
    else
        echo -e "${RED}Configuration file not found: $config_file${NC}"
        exit 1
    fi
}

check_entity_relationships() {
    echo -e "${YELLOW}Checking entity relationships...${NC}"
    
    local query="FROM Relationship \
                 SELECT sourceEntityGuid, \
                        targetEntityGuid, \
                        relationshipType \
                 WHERE sourceEntityGuid IN ( \
                   SELECT entity.guid \
                   FROM Log \
                   WHERE entity.type = 'DATABASE' \
                   LIMIT 100 \
                 ) \
                 SINCE 1 hour ago"
    
    echo "Query: $query"
}

generate_test_data() {
    echo -e "${YELLOW}Generating test data for entity synthesis...${NC}"
    
    # Create a test log with all required attributes
    cat > /tmp/entity_test.json <<EOF
{
  "resourceLogs": [{
    "resource": {
      "attributes": [{
        "key": "service.name",
        "value": {"stringValue": "database-intelligence-test"}
      }, {
        "key": "host.id",
        "value": {"stringValue": "test-host-001"}
      }, {
        "key": "instrumentation.provider",
        "value": {"stringValue": "opentelemetry"}
      }, {
        "key": "telemetry.sdk.name",
        "value": {"stringValue": "opentelemetry"}
      }, {
        "key": "telemetry.sdk.version",
        "value": {"stringValue": "0.89.0"}
      }]
    },
    "scopeLogs": [{
      "scope": {
        "name": "database-intelligence"
      },
      "logRecords": [{
        "timeUnixNano": "$(date +%s)000000000",
        "severityNumber": 9,
        "severityText": "INFO",
        "attributes": [{
          "key": "database_name",
          "value": {"stringValue": "test_db"}
        }, {
          "key": "entity.guid",
          "value": {"stringValue": "DATABASE|test|test_db"}
        }, {
          "key": "entity.type",
          "value": {"stringValue": "DATABASE"}
        }, {
          "key": "entity.name",
          "value": {"stringValue": "test_db"}
        }, {
          "key": "query_text",
          "value": {"stringValue": "SELECT 1"}
        }]
      }]
    }]
  }]
}
EOF

    echo "Test data generated at /tmp/entity_test.json"
    echo "Use this to test entity synthesis:"
    echo "curl -X POST https://otlp.nr-data.net:4318/v1/logs \\"
    echo "  -H 'Content-Type: application/json' \\"
    echo "  -H 'Api-Key: \$NEW_RELIC_LICENSE_KEY' \\"
    echo "  -d @/tmp/entity_test.json"
}

create_validation_dashboard() {
    echo -e "${YELLOW}Creating entity validation dashboard...${NC}"
    
    cat > /tmp/entity_dashboard.json <<EOF
{
  "name": "Database Entity Synthesis Validation",
  "pages": [{
    "name": "Entity Validation",
    "widgets": [
      {
        "title": "Database Entities Created",
        "nrql": "SELECT uniques(entity.guid) FROM Log WHERE entity.type = 'DATABASE' SINCE 1 hour ago"
      },
      {
        "title": "Entity Correlation Success Rate",
        "nrql": "SELECT percentage(count(*), WHERE entity.guid IS NOT NULL) FROM Log WHERE database_name IS NOT NULL SINCE 1 hour ago"
      },
      {
        "title": "Missing Correlations by Database",
        "nrql": "SELECT count(*) FROM Log WHERE entity.guid IS NULL AND database_name IS NOT NULL FACET database_name SINCE 1 hour ago"
      }
    ]
  }]
}
EOF
    
    echo "Dashboard configuration saved to /tmp/entity_dashboard.json"
}

# Main execution
main() {
    echo -e "${GREEN}=== New Relic Entity Synthesis Validation ===${NC}"
    echo
    
    validate_required_attributes
    echo
    
    echo -e "${YELLOW}NRQL Queries for Manual Validation:${NC}"
    echo
    
    validate_entity_creation
    echo
    
    validate_entity_correlation
    echo
    
    check_missing_correlations
    echo
    
    check_entity_relationships
    echo
    
    generate_test_data
    echo
    
    create_validation_dashboard
    echo
    
    echo -e "${GREEN}=== Validation Complete ===${NC}"
    echo
    echo "Next steps:"
    echo "1. Run the NRQL queries in New Relic Query Builder"
    echo "2. Send test data using the generated curl command"
    echo "3. Import the dashboard configuration"
    echo "4. Monitor entity creation in real-time"
}

# Run main function
main "$@"