#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - couchdb Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/couchdbreceiver/* .
rm -rf temp-repo
echo "couchdb receiver cloned successfully"
