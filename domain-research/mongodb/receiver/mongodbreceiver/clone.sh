#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - mongodb Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/mongodbreceiver/* .
rm -rf temp-repo
echo "mongodb receiver cloned successfully"
