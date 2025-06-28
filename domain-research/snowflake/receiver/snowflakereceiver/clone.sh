#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - snowflake Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/snowflakereceiver/* .
rm -rf temp-repo
echo "snowflake receiver cloned successfully"
