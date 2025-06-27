#\!/usr/bin/env python3
import json
import requests
import os
from datetime import datetime

# Load environment
license_key = os.getenv('NEW_RELIC_LICENSE_KEY')
account_id = os.getenv('NEW_RELIC_ACCOUNT_ID')

# Read latest metrics from collector
with open('latest-metrics.json', 'r') as f:
    line = f.readline().strip()
    data = json.loads(line)

# Extract metrics
metrics = data['data'][0]['entity']['metrics']

# Convert to New Relic custom events format
events = []
for metric in metrics:
    event = {
        "eventType": metric['event_type'],
        "timestamp": int(datetime.utcnow().timestamp()),
        **metric
    }
    events.append(event)

# Send to New Relic
url = f"https://insights-collector.newrelic.com/v1/accounts/{account_id}/events"
headers = {
    "Content-Type": "application/json",
    "X-Insert-Key": license_key
}

print(f"Sending {len(events)} events to New Relic...")
print(json.dumps(events, indent=2))

response = requests.post(url, json=events, headers=headers)
print(f"Response: {response.status_code}")
print(response.text)
