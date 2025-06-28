#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - jmx Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/jmxreceiver/* .
rm -rf temp-repo
echo "jmx receiver cloned successfully"
