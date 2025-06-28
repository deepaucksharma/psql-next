#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - postgresql Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/postgresqlreceiver/* .
rm -rf temp-repo
echo "postgresql receiver cloned successfully"
