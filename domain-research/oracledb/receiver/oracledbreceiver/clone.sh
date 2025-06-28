#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - oracledb Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/oracledbreceiver/* .
rm -rf temp-repo
echo "oracledb receiver cloned successfully"
