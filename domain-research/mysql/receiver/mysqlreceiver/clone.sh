#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - mysql Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/mysqlreceiver/* .
rm -rf temp-repo
echo "mysql receiver cloned successfully"
