#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - saphana Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/saphanareceiver/* .
rm -rf temp-repo
echo "saphana receiver cloned successfully"
