#!/usr/bin/env python3
"""
Simple webhook receiver for testing alert manager notifications.
"""

import json
import logging
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class AlertWebhookHandler(BaseHTTPRequestHandler):
    """Handle incoming alert webhooks."""
    
    def do_POST(self):
        """Process POST requests containing alerts."""
        if self.path == '/alerts':
            content_length = int(self.headers['Content-Length'])
            post_data = self.rfile.read(content_length)
            
            try:
                alert_data = json.loads(post_data.decode('utf-8'))
                self._process_alert(alert_data)
                
                # Send success response
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.end_headers()
                response = {'status': 'received', 'timestamp': datetime.now().isoformat()}
                self.wfile.write(json.dumps(response).encode())
                
            except json.JSONDecodeError as e:
                logger.error(f"Failed to parse JSON: {e}")
                self.send_error(400, "Invalid JSON")
            except Exception as e:
                logger.error(f"Error processing alert: {e}")
                self.send_error(500, "Internal server error")
        else:
            self.send_error(404, "Not found")
    
    def _process_alert(self, alert_data):
        """Process and log the alert data."""
        logger.info("=" * 60)
        logger.info("ALERT RECEIVED")
        logger.info("=" * 60)
        
        if 'resourceMetrics' in alert_data:
            for resource_metric in alert_data['resourceMetrics']:
                # Extract resource attributes
                resource_attrs = {}
                if 'resource' in resource_metric and 'attributes' in resource_metric['resource']:
                    for attr in resource_metric['resource']['attributes']:
                        resource_attrs[attr['key']] = attr['value']
                
                logger.info(f"Resource: {resource_attrs.get('service.name', {}).get('stringValue', 'unknown')}")
                
                # Process metrics
                if 'scopeMetrics' in resource_metric:
                    for scope_metric in resource_metric['scopeMetrics']:
                        if 'metrics' in scope_metric:
                            for metric in scope_metric['metrics']:
                                self._log_metric(metric)
        
        # Save to file for analysis
        with open('alerts_received.jsonl', 'a') as f:
            f.write(json.dumps(alert_data) + '\n')
    
    def _log_metric(self, metric):
        """Log individual metric details."""
        metric_name = metric.get('name', 'unknown')
        logger.info(f"\nMetric: {metric_name}")
        
        # Handle gauge metrics
        if 'gauge' in metric and 'dataPoints' in metric['gauge']:
            for data_point in metric['gauge']['dataPoints']:
                value = data_point.get('asDouble', data_point.get('asInt', 'N/A'))
                logger.info(f"  Value: {value}")
                
                # Log attributes
                if 'attributes' in data_point:
                    for attr in data_point['attributes']:
                        key = attr['key']
                        value = attr['value']
                        # Extract the actual value from the value object
                        if isinstance(value, dict):
                            actual_value = value.get('stringValue', value.get('intValue', value.get('doubleValue', str(value))))
                        else:
                            actual_value = str(value)
                        logger.info(f"  {key}: {actual_value}")
    
    def log_message(self, format, *args):
        """Suppress default HTTP logging."""
        return


def run_webhook_server(port=9999):
    """Run the webhook receiver server."""
    server_address = ('', port)
    httpd = HTTPServer(server_address, AlertWebhookHandler)
    logger.info(f"Starting webhook receiver on http://localhost:{port}/alerts")
    logger.info("Press Ctrl+C to stop")
    
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        logger.info("\nShutting down webhook receiver...")
        httpd.shutdown()


if __name__ == '__main__':
    import sys
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9999
    run_webhook_server(port)