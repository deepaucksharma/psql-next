#!/bin/bash
# Set New Relic OTLP endpoint based on region

source .env

# Set the correct endpoint based on region
case "${NEW_RELIC_REGION:-US}" in
    "US"|"us")
        export NEW_RELIC_OTLP_ENDPOINT="https://otlp.nr-data.net:4318"
        export NEW_RELIC_OTLP_ENDPOINT_GRPC="https://otlp.nr-data.net:4317"
        export NEW_RELIC_API_ENDPOINT="https://api.newrelic.com"
        export NEW_RELIC_METRIC_ENDPOINT="https://metric-api.newrelic.com"
        export NEW_RELIC_INFRA_ENDPOINT="https://infra-api.newrelic.com"
        echo "Using US region endpoints"
        ;;
    "EU"|"eu")
        export NEW_RELIC_OTLP_ENDPOINT="https://otlp.eu01.nr-data.net:4318"
        export NEW_RELIC_OTLP_ENDPOINT_GRPC="https://otlp.eu01.nr-data.net:4317"
        export NEW_RELIC_API_ENDPOINT="https://api.eu.newrelic.com"
        export NEW_RELIC_METRIC_ENDPOINT="https://metric-api.eu.newrelic.com"
        export NEW_RELIC_INFRA_ENDPOINT="https://infra-api.eu.newrelic.com"
        echo "Using EU region endpoints"
        ;;
    *)
        echo "Unknown region: ${NEW_RELIC_REGION}. Defaulting to US."
        export NEW_RELIC_OTLP_ENDPOINT="https://otlp.nr-data.net:4318"
        export NEW_RELIC_OTLP_ENDPOINT_GRPC="https://otlp.nr-data.net:4317"
        export NEW_RELIC_API_ENDPOINT="https://api.newrelic.com"
        export NEW_RELIC_METRIC_ENDPOINT="https://metric-api.newrelic.com"
        export NEW_RELIC_INFRA_ENDPOINT="https://infra-api.newrelic.com"
        ;;
esac

echo "OTLP Endpoint: $NEW_RELIC_OTLP_ENDPOINT"
echo "License Key: ${NEW_RELIC_LICENSE_KEY:0:10}..."