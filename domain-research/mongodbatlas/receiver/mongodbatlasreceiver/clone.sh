#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - mongodbatlas Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/mongodbatlasreceiver/* .
rm -rf temp-repo
echo "mongodbatlas receiver cloned successfully"
