use anyhow::Result;
use serde_json::json;
use std::time::SystemTime;

use postgres_collector_core::{
    UnifiedMetrics, MetricOutput, ProcessError,
};

/// OpenTelemetry Adapter
pub struct OTelAdapter {
    pub endpoint: String,
}

impl OTelAdapter {
    pub fn new(endpoint: String) -> Self {
        Self { endpoint }
    }
    
    pub fn adapt(&self, metrics: &UnifiedMetrics) -> Result<OTelOutput, ProcessError> {
        // Convert metrics to a simple JSON format for now
        let json_metrics = json!({
            "resource_metrics": [{
                "resource": {
                    "attributes": [
                        {"key": "service.name", "value": {"string_value": "postgresql"}},
                        {"key": "service.version", "value": {"string_value": "1.0.0"}}
                    ]
                },
                "scope_metrics": [{
                    "scope": {
                        "name": "postgres-unified-collector",
                        "version": env!("CARGO_PKG_VERSION")
                    },
                    "metrics": self.convert_metrics_to_otlp(metrics)
                }]
            }]
        });
        
        Ok(OTelOutput { 
            json_data: json_metrics.to_string() 
        })
    }
    
    fn convert_metrics_to_otlp(&self, metrics: &UnifiedMetrics) -> serde_json::Value {
        let mut otlp_metrics = Vec::new();
        
        // Convert slow queries
        for (i, metric) in metrics.slow_queries.iter().enumerate() {
            if let Some(duration) = metric.avg_elapsed_time_ms {
                otlp_metrics.push(json!({
                    "name": "postgresql.query.duration",
                    "description": "Average query execution time",
                    "unit": "ms",
                    "gauge": {
                        "data_points": [{
                            "attributes": [
                                {"key": "query_id", "value": {"string_value": metric.query_id.map(|q| q.to_string()).as_ref().unwrap_or(&format!("query_{}", i))}},
                                {"key": "database", "value": {"string_value": metric.database_name.as_ref().unwrap_or(&"unknown".to_string())}}
                            ],
                            "time_unix_nano": SystemTime::now().duration_since(SystemTime::UNIX_EPOCH).unwrap().as_nanos() as u64,
                            "as_double": duration
                        }]
                    }
                }));
            }
        }
        
        // Convert wait events
        for (i, metric) in metrics.wait_events.iter().enumerate() {
            if let Some(wait_time) = metric.wait_time_ms {
                otlp_metrics.push(json!({
                    "name": "postgresql.wait.time",
                    "description": "Wait event duration",
                    "unit": "ms",
                    "gauge": {
                        "data_points": [{
                            "attributes": [
                                {"key": "wait_event_type", "value": {"string_value": metric.wait_event_type.as_ref().unwrap_or(&format!("type_{}", i))}},
                                {"key": "wait_event", "value": {"string_value": metric.wait_event.as_ref().unwrap_or(&format!("event_{}", i))}}
                            ],
                            "time_unix_nano": SystemTime::now().duration_since(SystemTime::UNIX_EPOCH).unwrap().as_nanos() as u64,
                            "as_double": wait_time
                        }]
                    }
                }));
            }
        }
        
        // Convert blocking sessions
        for metric in metrics.blocking_sessions.iter() {
            if let Some(duration) = metric.blocking_duration_ms {
                otlp_metrics.push(json!({
                    "name": "postgresql.locks.blocking.duration",
                    "description": "Blocking lock duration",
                    "unit": "ms",
                    "gauge": {
                        "data_points": [{
                            "attributes": [
                                {"key": "blocking_pid", "value": {"string_value": metric.blocking_pid.unwrap_or_default().to_string()}},
                                {"key": "blocked_pid", "value": {"string_value": metric.blocked_pid.unwrap_or_default().to_string()}}
                            ],
                            "time_unix_nano": SystemTime::now().duration_since(SystemTime::UNIX_EPOCH).unwrap().as_nanos() as u64,
                            "as_double": duration
                        }]
                    }
                }));
            }
        }
        
        json!(otlp_metrics)
    }
}

/// OTLP Output wrapper
pub struct OTelOutput {
    json_data: String,
}

impl MetricOutput for OTelOutput {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError> {
        Ok(self.json_data.as_bytes().to_vec())
    }
    
    fn content_type(&self) -> &'static str {
        "application/json"
    }
}