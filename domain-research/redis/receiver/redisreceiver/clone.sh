#\!/bin/bash
# Clone OpenTelemetry Collector Contrib - redis Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/redisreceiver/* .
rm -rf temp-repo
echo "redis receiver cloned successfully"
