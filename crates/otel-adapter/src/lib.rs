use anyhow::Result;
use opentelemetry::{global, metrics::Meter, KeyValue};
use postgres_collector_core::{
    UnifiedMetrics, MetricOutput, ProcessError,
    SlowQueryMetric, WaitEventMetric, BlockingSessionMetric,
};
use std::collections::HashMap;

/// New Relic focused OTLP Adapter using dimensional metrics
pub struct OTelAdapter {
    pub endpoint: String,
    meter: Meter,
}

impl OTelAdapter {
    pub fn new(endpoint: String) -> Self {
        // Get meter from global provider
        let meter = global::meter("postgresql-collector");
        
        Self { endpoint, meter }
    }
    
    pub fn adapt(&self, metrics: &UnifiedMetrics) -> Result<OTelOutput, ProcessError> {
        // For New Relic, we don't need to manually serialize metrics
        // The OpenTelemetry SDK handles this through the configured exporter
        // This adapter now serves as a pass-through that validates metrics
        
        // Validate and count metrics
        let metric_counts = HashMap::from([
            ("slow_queries", metrics.slow_queries.len()),
            ("wait_events", metrics.wait_events.len()),
            ("blocking_sessions", metrics.blocking_sessions.len()),
            ("individual_queries", metrics.individual_queries.len()),
            ("execution_plans", metrics.execution_plans.len()),
        ]);
        
        Ok(OTelOutput { 
            metric_counts,
            total_metrics: metric_counts.values().sum(),
        })
    }
    
    /// Convert slow query to dimensional attributes
    pub fn query_attributes(metric: &SlowQueryMetric) -> Vec<KeyValue> {
        let mut attrs = vec![
            KeyValue::new("db.name", metric.database_name.clone().unwrap_or_default()),
            KeyValue::new("db.schema", metric.schema_name.clone().unwrap_or_default()),
            KeyValue::new("db.operation", metric.statement_type.clone().unwrap_or_default()),
        ];
        
        if let Some(query_id) = metric.query_id {
            attrs.push(KeyValue::new("db.query.id", query_id.to_string()));
        }
        
        // Add normalized query fingerprint
        if let Some(query_text) = &metric.query_text {
            let fingerprint = create_query_fingerprint(query_text);
            attrs.push(KeyValue::new("db.query.fingerprint", fingerprint));
        }
        
        attrs
    }
    
    /// Convert wait event to dimensional attributes
    pub fn wait_event_attributes(metric: &WaitEventMetric) -> Vec<KeyValue> {
        let mut attrs = vec![
            KeyValue::new("postgresql.wait.type", metric.wait_event_type.clone().unwrap_or_default()),
            KeyValue::new("postgresql.wait.event", metric.wait_event.clone().unwrap_or_default()),
            KeyValue::new("db.name", metric.database_name.clone().unwrap_or_default()),
            KeyValue::new("postgresql.state", metric.state.clone().unwrap_or_default()),
        ];
        
        if let Some(query_id) = metric.query_id {
            attrs.push(KeyValue::new("db.query.id", query_id.to_string()));
        }
        
        if let Some(user) = &metric.usename {
            attrs.push(KeyValue::new("db.user", user.clone()));
        }
        
        attrs
    }
    
    /// Convert blocking session to dimensional attributes
    pub fn blocking_session_attributes(metric: &BlockingSessionMetric) -> Vec<KeyValue> {
        vec![
            KeyValue::new("postgresql.lock.type", metric.lock_type.clone().unwrap_or_default()),
            KeyValue::new("db.name", metric.blocking_database.clone().unwrap_or_default()),
            KeyValue::new("postgresql.blocking.pid", metric.blocking_pid.unwrap_or_default() as i64),
            KeyValue::new("postgresql.blocked.pid", metric.blocked_pid.unwrap_or_default() as i64),
            KeyValue::new("postgresql.blocking.user", metric.blocking_user.clone().unwrap_or_default()),
            KeyValue::new("postgresql.blocked.user", metric.blocked_user.clone().unwrap_or_default()),
        ]
    }
}

/// Create a query fingerprint for dimensional grouping
fn create_query_fingerprint(query: &str) -> String {
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};
    
    // Normalize the query first
    let normalized = postgres_query_engine::utils::anonymize_and_normalize(query);
    
    // Create a hash
    let mut hasher = DefaultHasher::new();
    normalized.hash(&mut hasher);
    format!("{:x}", hasher.finish())
}

/// OTLP Output for New Relic
pub struct OTelOutput {
    metric_counts: HashMap<&'static str, usize>,
    total_metrics: usize,
}

impl MetricOutput for OTelOutput {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError> {
        // For New Relic OTLP, metrics are sent by the OpenTelemetry SDK
        // This just returns metadata about what was processed
        let summary = format!(
            "Processed {} total metrics: {:?}",
            self.total_metrics,
            self.metric_counts
        );
        Ok(summary.as_bytes().to_vec())
    }
    
    fn content_type(&self) -> &'static str {
        "text/plain"
    }
}