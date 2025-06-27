use anyhow::Result;
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::collections::HashMap;

use postgres_collector_core::{
    UnifiedMetrics, MetricOutput, ProcessError, MetricType,
    SlowQueryMetric, WaitEventMetric, BlockingSessionMetric,
    IndividualQueryMetric, ExecutionPlanMetric,
};

/// Adapter pattern for New Relic Infrastructure output format
pub struct NRIAdapter {
    pub entity_key: String,
    pub integration_version: String,
}

impl NRIAdapter {
    pub fn new(entity_key: String) -> Self {
        Self {
            entity_key,
            integration_version: "2.0.0".to_string(),
        }
    }
    
    pub fn adapt(&self, metrics: &UnifiedMetrics) -> Result<NRIOutput, ProcessError> {
        let mut integration = Integration::new("com.newrelic.postgresql", &self.integration_version);
        let mut entity = integration.entity(&self.entity_key, "pg-instance")?;
        
        // Convert to NRI format exactly as OHI expects
        self.adapt_slow_queries(&mut entity, &metrics.slow_queries)?;
        self.adapt_wait_events(&mut entity, &metrics.wait_events)?;
        self.adapt_blocking_sessions(&mut entity, &metrics.blocking_sessions)?;
        self.adapt_individual_queries(&mut entity, &metrics.individual_queries)?;
        self.adapt_execution_plans(&mut entity, &metrics.execution_plans)?;
        
        Ok(integration.build())
    }
    
    fn adapt_slow_queries(&self, entity: &mut Entity, metrics: &[SlowQueryMetric]) -> Result<(), ProcessError> {
        for metric in metrics {
            let metric_set = entity.new_metric_set("PostgresSlowQueries");
            
            // Set metrics exactly as OHI does
            if let Some(v) = &metric.newrelic {
                metric_set.set_metric("newrelic", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.query_id {
                metric_set.set_metric("query_id", &v.to_string(), MetricType::Attribute)?;
            }
            if let Some(v) = &metric.query_text {
                metric_set.set_metric("query_text", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.database_name {
                metric_set.set_metric("database_name", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.schema_name {
                metric_set.set_metric("schema_name", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.execution_count {
                metric_set.set_metric("execution_count", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.avg_elapsed_time_ms {
                metric_set.set_metric("avg_elapsed_time_ms", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.avg_disk_reads {
                metric_set.set_metric("avg_disk_reads", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.avg_disk_writes {
                metric_set.set_metric("avg_disk_writes", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.statement_type {
                metric_set.set_metric("statement_type", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.collection_timestamp {
                metric_set.set_metric("collection_timestamp", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.individual_query {
                metric_set.set_metric("individual_query", v, MetricType::Attribute)?;
            }
        }
        Ok(())
    }
    
    fn adapt_wait_events(&self, entity: &mut Entity, metrics: &[WaitEventMetric]) -> Result<(), ProcessError> {
        for metric in metrics {
            let metric_set = entity.new_metric_set("PostgresWaitEvents");
            
            if let Some(v) = &metric.pid {
                metric_set.set_metric("pid", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.wait_event_type {
                metric_set.set_metric("wait_event_type", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.wait_event {
                metric_set.set_metric("wait_event", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.wait_time_ms {
                metric_set.set_metric("wait_time_ms", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.state {
                metric_set.set_metric("state", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.usename {
                metric_set.set_metric("usename", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.database_name {
                metric_set.set_metric("database_name", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.query_id {
                metric_set.set_metric("query_id", &v.to_string(), MetricType::Attribute)?;
            }
            if let Some(v) = &metric.query_text {
                metric_set.set_metric("query_text", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.collection_timestamp {
                metric_set.set_metric("collection_timestamp", v, MetricType::Attribute)?;
            }
        }
        Ok(())
    }
    
    fn adapt_blocking_sessions(&self, entity: &mut Entity, metrics: &[BlockingSessionMetric]) -> Result<(), ProcessError> {
        for metric in metrics {
            let metric_set = entity.new_metric_set("PostgresBlockingSessions");
            
            if let Some(v) = &metric.blocking_pid {
                metric_set.set_metric("blocking_pid", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.blocked_pid {
                metric_set.set_metric("blocked_pid", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.blocking_query {
                metric_set.set_metric("blocking_query", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.blocked_query {
                metric_set.set_metric("blocked_query", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.blocking_database {
                metric_set.set_metric("blocking_database", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.blocked_database {
                metric_set.set_metric("blocked_database", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.blocking_user {
                metric_set.set_metric("blocking_user", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.blocked_user {
                metric_set.set_metric("blocked_user", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.blocking_duration_ms {
                metric_set.set_metric("blocking_duration_ms", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.blocked_duration_ms {
                metric_set.set_metric("blocked_duration_ms", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.lock_type {
                metric_set.set_metric("lock_type", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.collection_timestamp {
                metric_set.set_metric("collection_timestamp", v, MetricType::Attribute)?;
            }
        }
        Ok(())
    }
    
    fn adapt_individual_queries(&self, entity: &mut Entity, metrics: &[IndividualQueryMetric]) -> Result<(), ProcessError> {
        for metric in metrics {
            let metric_set = entity.new_metric_set("PostgresIndividualQueries");
            
            if let Some(v) = &metric.pid {
                metric_set.set_metric("pid", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.query_id {
                metric_set.set_metric("query_id", &v.to_string(), MetricType::Attribute)?;
            }
            if let Some(v) = &metric.query_text {
                metric_set.set_metric("query_text", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.state {
                metric_set.set_metric("state", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.wait_event_type {
                metric_set.set_metric("wait_event_type", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.wait_event {
                metric_set.set_metric("wait_event", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.usename {
                metric_set.set_metric("usename", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.database_name {
                metric_set.set_metric("database_name", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.backend_start {
                metric_set.set_metric("backend_start", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.xact_start {
                metric_set.set_metric("xact_start", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.query_start {
                metric_set.set_metric("query_start", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.state_change {
                metric_set.set_metric("state_change", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.backend_type {
                metric_set.set_metric("backend_type", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.collection_timestamp {
                metric_set.set_metric("collection_timestamp", v, MetricType::Attribute)?;
            }
        }
        Ok(())
    }
    
    fn adapt_execution_plans(&self, entity: &mut Entity, metrics: &[ExecutionPlanMetric]) -> Result<(), ProcessError> {
        for metric in metrics {
            let metric_set = entity.new_metric_set("PostgresExecutionPlanMetrics");
            
            if let Some(v) = &metric.query_id {
                metric_set.set_metric("query_id", &v.to_string(), MetricType::Attribute)?;
            }
            if let Some(v) = &metric.query_text {
                metric_set.set_metric("query_text", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.database_name {
                metric_set.set_metric("database_name", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.plan {
                metric_set.set_metric("plan", &v.to_string(), MetricType::Attribute)?;
            }
            if let Some(v) = &metric.plan_text {
                metric_set.set_metric("plan_text", v, MetricType::Attribute)?;
            }
            if let Some(v) = &metric.total_cost {
                metric_set.set_metric("total_cost", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.execution_time_ms {
                metric_set.set_metric("execution_time_ms", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.planning_time_ms {
                metric_set.set_metric("planning_time_ms", v, MetricType::Gauge)?;
            }
            if let Some(v) = &metric.collection_timestamp {
                metric_set.set_metric("collection_timestamp", v, MetricType::Attribute)?;
            }
        }
        Ok(())
    }
}

/// NRI Integration Protocol v4 structures
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Integration {
    pub name: String,
    pub protocol_version: String,
    pub integration_version: String,
    pub data: Vec<EntityData>,
}

impl Integration {
    pub fn new(name: &str, version: &str) -> Self {
        Self {
            name: name.to_string(),
            protocol_version: "4".to_string(),
            integration_version: version.to_string(),
            data: Vec::new(),
        }
    }
    
    pub fn entity(&mut self, key: &str, entity_type: &str) -> Result<&mut Entity, ProcessError> {
        let entity_data = EntityData {
            entity: Entity {
                name: key.to_string(),
                entity_type: entity_type.to_string(),
                metrics: Vec::new(),
                inventory: HashMap::new(),
                events: Vec::new(),
            },
            common: HashMap::new(),
        };
        
        self.data.push(entity_data);
        Ok(&mut self.data.last_mut().unwrap().entity)
    }
    
    pub fn build(self) -> NRIOutput {
        NRIOutput(self)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EntityData {
    pub entity: Entity,
    pub common: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Entity {
    pub name: String,
    #[serde(rename = "type")]
    pub entity_type: String,
    pub metrics: Vec<MetricSet>,
    pub inventory: HashMap<String, serde_json::Value>,
    pub events: Vec<Event>,
}

impl Entity {
    pub fn new_metric_set(&mut self, event_type: &str) -> &mut MetricSet {
        let metric_set = MetricSet {
            event_type: event_type.to_string(),
            metrics: HashMap::new(),
        };
        
        self.metrics.push(metric_set);
        self.metrics.last_mut().unwrap()
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetricSet {
    pub event_type: String,
    #[serde(flatten)]
    pub metrics: HashMap<String, serde_json::Value>,
}

impl MetricSet {
    pub fn set_metric<T: Serialize>(
        &mut self,
        name: &str,
        value: T,
        _metric_type: MetricType,
    ) -> Result<(), ProcessError> {
        self.metrics.insert(name.to_string(), json!(value));
        Ok(())
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Event {
    pub event_type: String,
    pub timestamp: i64,
    pub attributes: HashMap<String, serde_json::Value>,
}

/// NRI Output wrapper
pub struct NRIOutput(Integration);

impl MetricOutput for NRIOutput {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError> {
        serde_json::to_vec(&self.0).map_err(ProcessError::from)
    }
    
    fn content_type(&self) -> &'static str {
        "application/json"
    }
}

/// Ingestion helper for OHI compatibility
pub struct IngestionHelper {
    publish_threshold: usize,
}

impl IngestionHelper {
    pub fn new() -> Self {
        Self {
            publish_threshold: 600, // OHI constant
        }
    }
    
    pub async fn ingest_metrics(
        &self,
        metrics: &UnifiedMetrics,
        entity_key: &str,
    ) -> Result<Vec<NRIOutput>, ProcessError> {
        let mut outputs = Vec::new();
        let adapter = NRIAdapter::new(entity_key.to_string());
        
        // Process metrics in batches if needed
        let output = adapter.adapt(metrics)?;
        outputs.push(output);
        
        Ok(outputs)
    }
}