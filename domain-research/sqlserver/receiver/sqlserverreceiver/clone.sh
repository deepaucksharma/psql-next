#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - sqlserver Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/sqlserverreceiver/* .
rm -rf temp-repo
echo "sqlserver receiver cloned successfully"
