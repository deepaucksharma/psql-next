use anyhow::Result;
use async_trait::async_trait;
use chrono::Utc;
use opentelemetry::{KeyValue};
use opentelemetry_otlp::WithExportConfig;
use opentelemetry_sdk::metrics::{
    data::{Metric, ResourceMetrics, ScopeMetrics, Temporality},
    reader::{DefaultTemporalitySelector, TemporalitySelector},
    Aggregation, InstrumentKind, SdkMeterProvider,
};
use opentelemetry_sdk::{
    export::metrics::aggregation,
    runtime,
    Resource,
};
use opentelemetry_semantic_conventions::{
    resource::{SERVICE_NAME, SERVICE_VERSION},
    trace::DB_SYSTEM,
};
use std::time::{Duration, SystemTime};

use postgres_collector_core::{
    UnifiedMetrics, MetricOutput, ProcessError,
};

/// OpenTelemetry Adapter
pub struct OTelAdapter {
    pub resource: Resource,
    pub instrumentation_scope: InstrumentationScope,
    pub endpoint: String,
}

#[derive(Clone)]
pub struct InstrumentationScope {
    pub name: String,
    pub version: String,
}

impl OTelAdapter {
    pub fn new(endpoint: String) -> Self {
        let resource = Resource::new(vec![
            KeyValue::new(SERVICE_NAME, "postgresql"),
            KeyValue::new(SERVICE_VERSION, "1.0.0"),
            KeyValue::new(DB_SYSTEM, "postgresql"),
        ]);
        
        let instrumentation_scope = InstrumentationScope {
            name: "postgres-unified-collector".to_string(),
            version: env!("CARGO_PKG_VERSION").to_string(),
        };
        
        Self {
            resource,
            instrumentation_scope,
            endpoint,
        }
    }
    
    pub fn adapt(&self, metrics: &UnifiedMetrics) -> Result<OTelOutput, ProcessError> {
        let mut metric_data = Vec::new();
        let timestamp = SystemTime::now();
        
        // Convert slow queries to OTLP metrics
        for metric in &metrics.slow_queries {
            // Query duration gauge
            if let Some(duration) = metric.avg_elapsed_time_ms {
                metric_data.push(self.create_gauge_metric(
                    "postgresql.query.duration",
                    "Average query execution time",
                    "ms",
                    duration,
                    timestamp,
                    vec![
                        KeyValue::new("query_id", metric.query_id.clone().unwrap_or_default()),
                        KeyValue::new("database", metric.database_name.clone().unwrap_or_default()),
                        KeyValue::new("statement_type", metric.statement_type.clone().unwrap_or_default()),
                    ],
                ));
            }
            
            // Query execution count
            if let Some(count) = metric.execution_count {
                metric_data.push(self.create_sum_metric(
                    "postgresql.query.count",
                    "Number of query executions",
                    "",
                    count as f64,
                    timestamp,
                    vec![
                        KeyValue::new("query_id", metric.query_id.clone().unwrap_or_default()),
                        KeyValue::new("database", metric.database_name.clone().unwrap_or_default()),
                    ],
                ));
            }
            
            // Disk reads
            if let Some(reads) = metric.avg_disk_reads {
                metric_data.push(self.create_gauge_metric(
                    "postgresql.query.disk.reads",
                    "Average disk reads per query",
                    "blocks",
                    reads,
                    timestamp,
                    vec![
                        KeyValue::new("query_id", metric.query_id.clone().unwrap_or_default()),
                        KeyValue::new("database", metric.database_name.clone().unwrap_or_default()),
                    ],
                ));
            }
        }
        
        // Convert wait events
        for metric in &metrics.wait_events {
            if let Some(wait_time) = metric.wait_time_ms {
                metric_data.push(self.create_gauge_metric(
                    "postgresql.wait.time",
                    "Wait event duration",
                    "ms",
                    wait_time,
                    timestamp,
                    vec![
                        KeyValue::new("wait_event_type", metric.wait_event_type.clone().unwrap_or_default()),
                        KeyValue::new("wait_event", metric.wait_event.clone().unwrap_or_default()),
                        KeyValue::new("database", metric.database_name.clone().unwrap_or_default()),
                        KeyValue::new("state", metric.state.clone().unwrap_or_default()),
                    ],
                ));
            }
        }
        
        // Convert blocking sessions
        for metric in &metrics.blocking_sessions {
            if let Some(duration) = metric.blocking_duration_ms {
                metric_data.push(self.create_gauge_metric(
                    "postgresql.locks.blocking.duration",
                    "Blocking lock duration",
                    "ms",
                    duration,
                    timestamp,
                    vec![
                        KeyValue::new("blocking_pid", metric.blocking_pid.unwrap_or_default().to_string()),
                        KeyValue::new("blocked_pid", metric.blocked_pid.unwrap_or_default().to_string()),
                        KeyValue::new("lock_type", metric.lock_type.clone().unwrap_or_default()),
                        KeyValue::new("database", metric.blocking_database.clone().unwrap_or_default()),
                    ],
                ));
            }
        }
        
        // Build the export request
        let resource_metrics = self.build_resource_metrics(metric_data);
        Ok(OTelOutput { resource_metrics })
    }
    
    fn create_gauge_metric(
        &self,
        name: &str,
        description: &str,
        unit: &str,
        value: f64,
        timestamp: SystemTime,
        attributes: Vec<KeyValue>,
    ) -> MetricData {
        MetricData {
            name: name.to_string(),
            description: description.to_string(),
            unit: unit.to_string(),
            data: MetricKind::Gauge(GaugeData {
                data_points: vec![DataPoint {
                    attributes,
                    timestamp,
                    value,
                }],
            }),
        }
    }
    
    fn create_sum_metric(
        &self,
        name: &str,
        description: &str,
        unit: &str,
        value: f64,
        timestamp: SystemTime,
        attributes: Vec<KeyValue>,
    ) -> MetricData {
        MetricData {
            name: name.to_string(),
            description: description.to_string(),
            unit: unit.to_string(),
            data: MetricKind::Sum(SumData {
                data_points: vec![DataPoint {
                    attributes,
                    timestamp,
                    value,
                }],
                temporality: Temporality::Cumulative,
                is_monotonic: true,
            }),
        }
    }
    
    fn build_resource_metrics(&self, metrics: Vec<MetricData>) -> ResourceMetrics {
        ResourceMetrics {
            resource: self.resource.clone(),
            scope_metrics: vec![ScopeMetrics {
                scope: opentelemetry_sdk::InstrumentationScope {
                    name: self.instrumentation_scope.name.clone(),
                    version: Some(self.instrumentation_scope.version.clone()),
                    schema_url: None,
                    attributes: Vec::new(),
                },
                metrics: metrics
                    .into_iter()
                    .map(|m| m.into_otel_metric())
                    .collect(),
            }],
        }
    }
}

// Internal metric representation
struct MetricData {
    name: String,
    description: String,
    unit: String,
    data: MetricKind,
}

enum MetricKind {
    Gauge(GaugeData),
    Sum(SumData),
}

struct GaugeData {
    data_points: Vec<DataPoint>,
}

struct SumData {
    data_points: Vec<DataPoint>,
    temporality: Temporality,
    is_monotonic: bool,
}

struct DataPoint {
    attributes: Vec<KeyValue>,
    timestamp: SystemTime,
    value: f64,
}

impl MetricData {
    fn into_otel_metric(self) -> Metric {
        match self.data {
            MetricKind::Gauge(gauge) => Metric {
                name: self.name.into(),
                description: self.description.into(),
                unit: self.unit.into(),
                data: Box::new(opentelemetry_sdk::metrics::data::Gauge {
                    data_points: gauge
                        .data_points
                        .into_iter()
                        .map(|dp| opentelemetry_sdk::metrics::data::DataPoint {
                            attributes: dp.attributes.into_iter().collect(),
                            start_time: None,
                            time: Some(dp.timestamp),
                            value: dp.value,
                            exemplars: vec![],
                        })
                        .collect(),
                }),
            },
            MetricKind::Sum(sum) => Metric {
                name: self.name.into(),
                description: self.description.into(),
                unit: self.unit.into(),
                data: Box::new(opentelemetry_sdk::metrics::data::Sum {
                    data_points: sum
                        .data_points
                        .into_iter()
                        .map(|dp| opentelemetry_sdk::metrics::data::DataPoint {
                            attributes: dp.attributes.into_iter().collect(),
                            start_time: None,
                            time: Some(dp.timestamp),
                            value: dp.value,
                            exemplars: vec![],
                        })
                        .collect(),
                    temporality: sum.temporality,
                    is_monotonic: sum.is_monotonic,
                }),
            },
        }
    }
}

/// OTLP Output wrapper
pub struct OTelOutput {
    resource_metrics: ResourceMetrics,
}

impl MetricOutput for OTelOutput {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError> {
        // In a real implementation, this would use prost to encode to protobuf
        // For now, we'll use JSON representation
        let json = serde_json::to_vec(&self.resource_metrics)
            .map_err(|e| ProcessError::SerializationError(e))?;
        Ok(json)
    }
    
    fn content_type(&self) -> &'static str {
        "application/x-protobuf"
    }
}

/// Builder for creating an OTel meter provider
pub fn create_meter_provider(endpoint: &str) -> Result<SdkMeterProvider> {
    let exporter = opentelemetry_otlp::new_exporter()
        .tonic()
        .with_endpoint(endpoint)
        .build_metrics_exporter(
            Box::new(DefaultTemporalitySelector::new()),
            Box::new(aggregation::default_aggregation_selector()),
        )?;
    
    let reader = opentelemetry_sdk::metrics::PeriodicReader::builder(
        exporter,
        runtime::Tokio,
    )
    .with_interval(Duration::from_secs(60))
    .build();
    
    let provider = SdkMeterProvider::builder()
        .with_reader(reader)
        .with_resource(Resource::new(vec![
            KeyValue::new(SERVICE_NAME, "postgresql"),
            KeyValue::new(SERVICE_VERSION, "1.0.0"),
        ]))
        .build();
    
    Ok(provider)
}