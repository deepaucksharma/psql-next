#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - sqlquery Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/sqlqueryreceiver/* .
rm -rf temp-repo
echo "sqlquery receiver cloned successfully"
