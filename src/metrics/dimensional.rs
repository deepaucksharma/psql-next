use opentelemetry::{
    metrics::{Counter, Histogram, Meter, ObservableGauge, Unit},
    KeyValue,
};
use opentelemetry_sdk::Resource;
use postgres_collector_core::{
    SlowQueryMetric, WaitEventMetric, BlockingSessionMetric,
    IndividualQueryMetric, UnifiedMetrics,
};
use std::time::Duration;

/// Dimensional metrics for New Relic
pub struct DimensionalMetrics {
    // Query metrics
    query_duration: Histogram<f64>,
    query_count: Counter<u64>,
    query_rows: Histogram<i64>,
    
    // Wait event metrics
    wait_duration: Histogram<f64>,
    wait_count: Counter<u64>,
    
    // Connection metrics
    connection_count: ObservableGauge<i64>,
    connection_utilization: ObservableGauge<f64>,
    
    // Lock metrics
    lock_wait_duration: Histogram<f64>,
    deadlock_count: Counter<u64>,
    
    // Table/Index metrics
    table_size: ObservableGauge<u64>,
    table_rows: ObservableGauge<i64>,
    index_scans: Counter<u64>,
    sequential_scans: Counter<u64>,
    
    // Replication metrics
    replication_lag: ObservableGauge<i64>,
    replication_delay: ObservableGauge<f64>,
}

impl DimensionalMetrics {
    pub fn new(meter: &Meter) -> Self {
        Self {
            // Query performance metrics
            query_duration: meter
                .f64_histogram("postgresql.query.duration")
                .with_description("Query execution duration")
                .with_unit(Unit::new("milliseconds"))
                .init(),
                
            query_count: meter
                .u64_counter("postgresql.query.count")
                .with_description("Number of query executions")
                .init(),
                
            query_rows: meter
                .i64_histogram("postgresql.query.rows")
                .with_description("Rows returned/affected by queries")
                .init(),
            
            // Wait event metrics
            wait_duration: meter
                .f64_histogram("postgresql.wait.duration")
                .with_description("Wait event duration")
                .with_unit(Unit::new("milliseconds"))
                .init(),
                
            wait_count: meter
                .u64_counter("postgresql.wait.count")
                .with_description("Wait event occurrences")
                .init(),
            
            // Connection metrics
            connection_count: meter
                .i64_observable_gauge("postgresql.connection.count")
                .with_description("Current connection count")
                .init(),
                
            connection_utilization: meter
                .f64_observable_gauge("postgresql.connection.utilization")
                .with_description("Connection pool utilization")
                .with_unit(Unit::new("percent"))
                .init(),
            
            // Lock metrics
            lock_wait_duration: meter
                .f64_histogram("postgresql.lock.wait_duration")
                .with_description("Lock wait duration")
                .with_unit(Unit::new("milliseconds"))
                .init(),
                
            deadlock_count: meter
                .u64_counter("postgresql.lock.deadlock.count")
                .with_description("Deadlock occurrences")
                .init(),
            
            // Table metrics
            table_size: meter
                .u64_observable_gauge("postgresql.table.size")
                .with_description("Table size on disk")
                .with_unit(Unit::new("bytes"))
                .init(),
                
            table_rows: meter
                .i64_observable_gauge("postgresql.table.rows")
                .with_description("Estimated row count")
                .init(),
                
            index_scans: meter
                .u64_counter("postgresql.index.scans")
                .with_description("Index scan count")
                .init(),
                
            sequential_scans: meter
                .u64_counter("postgresql.table.sequential_scans")
                .with_description("Sequential scan count")
                .init(),
            
            // Replication metrics
            replication_lag: meter
                .i64_observable_gauge("postgresql.replication.lag")
                .with_description("Replication lag in bytes")
                .with_unit(Unit::new("bytes"))
                .init(),
                
            replication_delay: meter
                .f64_observable_gauge("postgresql.replication.delay")
                .with_description("Replication delay")
                .with_unit(Unit::new("milliseconds"))
                .init(),
        }
    }
    
    /// Record query metrics with dimensional attributes
    pub fn record_query_metric(&self, metric: &SlowQueryMetric) {
        let attributes = vec![
            KeyValue::new("database.name", metric.database_name.clone().unwrap_or_default()),
            KeyValue::new("schema.name", metric.schema_name.clone().unwrap_or_default()),
            KeyValue::new("query.operation", metric.statement_type.clone().unwrap_or_default()),
            KeyValue::new("query.id", metric.query_id.unwrap_or(0).to_string()),
            KeyValue::new("query.fingerprint", hash_query(&metric.query_text)),
        ];
        
        // Record duration
        if let Some(duration) = metric.avg_elapsed_time_ms {
            self.query_duration.record(duration, &attributes);
        }
        
        // Record execution count
        if let Some(count) = metric.execution_count {
            self.query_count.add(count as u64, &attributes);
        }
        
        // Record disk I/O as dimensions
        if let Some(reads) = metric.avg_disk_reads {
            let mut attrs = attributes.clone();
            attrs.push(KeyValue::new("io.type", "read"));
            self.query_rows.record(reads as i64, &attrs);
        }
        
        if let Some(writes) = metric.avg_disk_writes {
            let mut attrs = attributes.clone();
            attrs.push(KeyValue::new("io.type", "write"));
            self.query_rows.record(writes as i64, &attrs);
        }
    }
    
    /// Record wait event metrics
    pub fn record_wait_event(&self, metric: &WaitEventMetric) {
        let attributes = vec![
            KeyValue::new("wait.event_type", metric.wait_event_type.clone().unwrap_or_default()),
            KeyValue::new("wait.event_name", metric.wait_event.clone().unwrap_or_default()),
            KeyValue::new("database.name", metric.database_name.clone().unwrap_or_default()),
            KeyValue::new("query.fingerprint", hash_query(&metric.query_text)),
            KeyValue::new("backend.state", metric.state.clone().unwrap_or_default()),
        ];
        
        if let Some(duration) = metric.wait_time_ms {
            self.wait_duration.record(duration, &attributes);
            self.wait_count.add(1, &attributes);
        }
    }
    
    /// Record blocking session metrics as lock metrics
    pub fn record_blocking_session(&self, metric: &BlockingSessionMetric) {
        let attributes = vec![
            KeyValue::new("lock.type", metric.lock_type.clone().unwrap_or_default()),
            KeyValue::new("database.name", metric.blocking_database.clone().unwrap_or_default()),
            KeyValue::new("blocking.query_fingerprint", hash_query(&metric.blocking_query)),
            KeyValue::new("blocked.query_fingerprint", hash_query(&metric.blocked_query)),
            KeyValue::new("blocking.user", metric.blocking_user.clone().unwrap_or_default()),
            KeyValue::new("blocked.user", metric.blocked_user.clone().unwrap_or_default()),
        ];
        
        if let Some(duration) = metric.blocked_duration_ms {
            self.lock_wait_duration.record(duration, &attributes);
        }
    }
    
    /// Record all metrics from UnifiedMetrics
    pub fn record_metrics_batch(&self, metrics: &UnifiedMetrics) {
        // Record slow queries
        for query in &metrics.slow_queries {
            self.record_query_metric(query);
        }
        
        // Record wait events
        for wait_event in &metrics.wait_events {
            self.record_wait_event(wait_event);
        }
        
        // Record blocking sessions
        for blocking in &metrics.blocking_sessions {
            self.record_blocking_session(blocking);
        }
    }
}

/// Create resource attributes for New Relic entity synthesis
pub fn create_resource(
    service_name: &str,
    instance_id: &str,
    host: &str,
    port: u16,
    environment: &str,
) -> Resource {
    Resource::new(vec![
        // Service identity
        KeyValue::new("service.name", service_name.to_string()),
        KeyValue::new("service.namespace", environment.to_string()),
        KeyValue::new("service.instance.id", instance_id.to_string()),
        
        // Host information
        KeyValue::new("host.name", host.to_string()),
        KeyValue::new("host.port", port as i64),
        
        // Database specific
        KeyValue::new("db.system", "postgresql"),
        KeyValue::new("db.connection_string", format!("postgresql://{}:{}", host, port)),
        
        // Environment
        KeyValue::new("deployment.environment", environment.to_string()),
        
        // New Relic specific
        KeyValue::new("newrelic.entity.type", "POSTGRESQL_INSTANCE"),
    ])
}

/// Hash query text for fingerprinting
fn hash_query(query_text: &Option<String>) -> String {
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};
    
    match query_text {
        Some(text) => {
            // Normalize query first
            let normalized = postgres_query_engine::utils::anonymize_and_normalize(text);
            
            // Create hash
            let mut hasher = DefaultHasher::new();
            normalized.hash(&mut hasher);
            format!("{:x}", hasher.finish())
        }
        None => "unknown".to_string(),
    }
}

/// Convert duration metrics to appropriate units
pub fn duration_to_millis(duration: Duration) -> f64 {
    duration.as_secs_f64() * 1000.0
}