#!/bin/bash
# Clone OpenTelemetry Collector Contrib - Elasticsearch Receiver
git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git temp-repo
cp -r temp-repo/receiver/elasticsearchreceiver/* .
rm -rf temp-repo
echo "Elasticsearch receiver cloned successfully"