#!/bin/bash
# Script to align all OpenTelemetry versions to v0.105.0

set -e

OTEL_VERSION="v0.105.0"
OTEL_PDATA_VERSION="v1.10.0"

echo "Fixing OpenTelemetry version compatibility issues..."

# Fix components/go.mod
echo "Updating components/go.mod..."
cd components
go get go.opentelemetry.io/collector/component@${OTEL_VERSION}
go get go.opentelemetry.io/collector/confmap@${OTEL_VERSION}
go get go.opentelemetry.io/collector/consumer@${OTEL_VERSION}
go get go.opentelemetry.io/collector/exporter@${OTEL_VERSION}
go get go.opentelemetry.io/collector/extension@${OTEL_VERSION}
go get go.opentelemetry.io/collector/pdata@${OTEL_PDATA_VERSION}
go get go.opentelemetry.io/collector/processor@${OTEL_VERSION}
go get go.opentelemetry.io/collector/receiver@${OTEL_VERSION}
go mod tidy
cd ..

# Fix common modules
echo "Updating common/featuredetector/go.mod..."
cd common/featuredetector
go get go.opentelemetry.io/collector/pdata@${OTEL_PDATA_VERSION}
go mod tidy
cd ../..

echo "Updating common/queryselector/go.mod..."
cd common/queryselector
go mod tidy
cd ../..

# Fix distributions
echo "Updating distributions/production/go.mod..."
cd distributions/production
go get go.opentelemetry.io/collector/component@${OTEL_VERSION}
go get go.opentelemetry.io/collector/confmap@${OTEL_VERSION}
go get go.opentelemetry.io/collector/otelcol@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver@${OTEL_VERSION}
go get github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver@${OTEL_VERSION}
go mod tidy
cd ../..

echo "Updating distributions/minimal/go.mod..."
cd distributions/minimal
go get go.opentelemetry.io/collector/component@${OTEL_VERSION}
go get go.opentelemetry.io/collector/otelcol@${OTEL_VERSION}
go mod tidy
cd ../..

# Update go.work
echo "Updating go.work..."
go work sync

echo "Version alignment complete!"
echo "Next step: Run the OpenTelemetry Collector Builder with otelcol-builder-config-complete.yaml"