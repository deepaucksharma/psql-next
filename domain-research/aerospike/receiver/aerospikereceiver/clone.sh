#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - aerospike Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/aerospikereceiver/* .
rm -rf temp-repo
echo "aerospike receiver cloned successfully"
