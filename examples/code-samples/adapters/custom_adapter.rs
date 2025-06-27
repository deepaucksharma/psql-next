// Example: Creating a Custom Output Adapter
// This demonstrates how to create a new output format adapter

use async_trait::async_trait;
use postgres_collector_core::{MetricOutput, ProcessError, UnifiedMetrics};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

/// Custom adapter that outputs metrics in a simplified JSON format
/// optimized for custom monitoring systems
pub struct CustomJsonAdapter {
    config: CustomAdapterConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CustomAdapterConfig {
    pub include_metadata: bool,
    pub max_slow_queries: usize,
    pub custom_fields: HashMap<String, String>,
}

impl Default for CustomAdapterConfig {
    fn default() -> Self {
        Self {
            include_metadata: true,
            max_slow_queries: 100,
            custom_fields: HashMap::new(),
        }
    }
}

impl CustomJsonAdapter {
    pub fn new(config: CustomAdapterConfig) -> Self {
        Self { config }
    }

    pub fn with_custom_field(mut self, key: String, value: String) -> Self {
        self.config.custom_fields.insert(key, value);
        self
    }
}

/// Output format for the custom adapter
#[derive(Debug, Serialize)]
pub struct CustomOutput {
    /// Metric format version
    version: String,
    /// Collection timestamp
    timestamp: String,
    /// Metadata about the collection
    #[serde(skip_serializing_if = "Option::is_none")]
    metadata: Option<CustomMetadata>,
    /// Simplified metrics
    metrics: CustomMetrics,
}

#[derive(Debug, Serialize)]
pub struct CustomMetadata {
    collector_version: String,
    database_version: String,
    collection_duration_ms: u64,
    custom_fields: HashMap<String, String>,
}

#[derive(Debug, Serialize)]
pub struct CustomMetrics {
    slow_queries: Vec<SimplifiedSlowQuery>,
    summary: MetricsSummary,
}

#[derive(Debug, Serialize)]
pub struct SimplifiedSlowQuery {
    query_hash: String,
    database: String,
    avg_duration_ms: f64,
    execution_count: i64,
    query_preview: String, // First 100 chars of sanitized query
}

#[derive(Debug, Serialize)]
pub struct MetricsSummary {
    total_slow_queries: usize,
    avg_query_duration_ms: f64,
    total_execution_count: i64,
    databases_monitored: Vec<String>,
}

// Implement the trait for dynamic dispatch
#[async_trait]
impl crate::MetricAdapterDyn for CustomJsonAdapter {
    async fn adapt_dyn(
        &self,
        metrics: &UnifiedMetrics,
    ) -> Result<Box<dyn crate::MetricOutputDyn>, ProcessError> {
        let output = self.adapt(metrics).await?;
        Ok(Box::new(output))
    }

    fn name(&self) -> &str {
        "CustomJSON"
    }
}

// Implement the core adapter trait
#[async_trait]
impl crate::MetricAdapter for CustomJsonAdapter {
    type Output = CustomOutput;

    async fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, ProcessError> {
        let start_time = std::time::Instant::now();

        // Transform slow queries
        let slow_queries = self.transform_slow_queries(&metrics.slow_queries)?;

        // Generate summary
        let summary = self.generate_summary(&metrics.slow_queries);

        // Optional metadata
        let metadata = if self.config.include_metadata {
            Some(CustomMetadata {
                collector_version: env!("CARGO_PKG_VERSION").to_string(),
                database_version: "Unknown".to_string(), // Could be extracted from capabilities
                collection_duration_ms: start_time.elapsed().as_millis() as u64,
                custom_fields: self.config.custom_fields.clone(),
            })
        } else {
            None
        };

        Ok(CustomOutput {
            version: "1.0".to_string(),
            timestamp: chrono::Utc::now().to_rfc3339(),
            metadata,
            metrics: CustomMetrics {
                slow_queries,
                summary,
            },
        })
    }
}

impl CustomJsonAdapter {
    fn transform_slow_queries(
        &self,
        slow_queries: &[postgres_collector_core::SlowQueryMetric],
    ) -> Result<Vec<SimplifiedSlowQuery>, ProcessError> {
        slow_queries
            .iter()
            .take(self.config.max_slow_queries)
            .map(|sq| {
                Ok(SimplifiedSlowQuery {
                    query_hash: sq.query_id.clone().unwrap_or_else(|| "unknown".to_string()),
                    database: sq.database_name.clone().unwrap_or_else(|| "unknown".to_string()),
                    avg_duration_ms: sq.mean_elapsed_time_ms.unwrap_or(0.0),
                    execution_count: sq.execution_count.unwrap_or(0),
                    query_preview: sq
                        .query_text
                        .as_ref()
                        .map(|q| self.create_query_preview(q))
                        .unwrap_or_else(|| "N/A".to_string()),
                })
            })
            .collect()
    }

    fn create_query_preview(&self, query: &str) -> String {
        // Take first 100 characters and clean up whitespace
        query
            .chars()
            .take(100)
            .collect::<String>()
            .split_whitespace()
            .collect::<Vec<_>>()
            .join(" ")
    }

    fn generate_summary(
        &self,
        slow_queries: &[postgres_collector_core::SlowQueryMetric],
    ) -> MetricsSummary {
        let total_slow_queries = slow_queries.len();
        
        let avg_query_duration_ms = if total_slow_queries > 0 {
            slow_queries
                .iter()
                .filter_map(|sq| sq.mean_elapsed_time_ms)
                .sum::<f64>() / total_slow_queries as f64
        } else {
            0.0
        };

        let total_execution_count = slow_queries
            .iter()
            .filter_map(|sq| sq.execution_count)
            .sum();

        let databases_monitored: Vec<String> = slow_queries
            .iter()
            .filter_map(|sq| sq.database_name.clone())
            .collect::<std::collections::HashSet<_>>()
            .into_iter()
            .collect();

        MetricsSummary {
            total_slow_queries,
            avg_query_duration_ms,
            total_execution_count,
            databases_monitored,
        }
    }
}

impl MetricOutput for CustomOutput {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError> {
        serde_json::to_vec_pretty(self)
            .map_err(|e| ProcessError::SerializationError(e.to_string()))
    }

    fn content_type(&self) -> &'static str {
        "application/json"
    }
}

// Example usage and integration
#[cfg(test)]
mod tests {
    use super::*;
    use postgres_collector_core::SlowQueryMetric;

    fn create_test_metrics() -> UnifiedMetrics {
        let mut metrics = UnifiedMetrics::default();
        
        metrics.slow_queries = vec![
            SlowQueryMetric {
                query_id: Some("query_1".to_string()),
                database_name: Some("test_db".to_string()),
                query_text: Some("SELECT * FROM users WHERE created_at > NOW() - INTERVAL '1 day'".to_string()),
                mean_elapsed_time_ms: Some(250.5),
                execution_count: Some(150),
                ..Default::default()
            },
            SlowQueryMetric {
                query_id: Some("query_2".to_string()),
                database_name: Some("test_db".to_string()),
                query_text: Some("UPDATE orders SET status = 'processed' WHERE id = $1".to_string()),
                mean_elapsed_time_ms: Some(89.2),
                execution_count: Some(75),
                ..Default::default()
            },
        ];

        metrics
    }

    #[tokio::test]
    async fn test_custom_adapter_basic() {
        let config = CustomAdapterConfig::default();
        let adapter = CustomJsonAdapter::new(config);
        let test_metrics = create_test_metrics();

        let output = adapter.adapt(&test_metrics).await.unwrap();
        
        assert_eq!(output.version, "1.0");
        assert_eq!(output.metrics.slow_queries.len(), 2);
        assert_eq!(output.metrics.summary.total_slow_queries, 2);
        assert!(output.metadata.is_some());
    }

    #[tokio::test]
    async fn test_custom_adapter_with_limit() {
        let config = CustomAdapterConfig {
            max_slow_queries: 1,
            ..Default::default()
        };
        let adapter = CustomJsonAdapter::new(config);
        let test_metrics = create_test_metrics();

        let output = adapter.adapt(&test_metrics).await.unwrap();
        
        assert_eq!(output.metrics.slow_queries.len(), 1);
    }

    #[tokio::test]
    async fn test_custom_adapter_serialization() {
        let config = CustomAdapterConfig::default();
        let adapter = CustomJsonAdapter::new(config);
        let test_metrics = create_test_metrics();

        let output = adapter.adapt(&test_metrics).await.unwrap();
        let serialized = output.serialize().unwrap();
        
        // Verify it's valid JSON
        let _: Value = serde_json::from_slice(&serialized).unwrap();
        
        // Verify content type
        assert_eq!(output.content_type(), "application/json");
    }

    #[tokio::test]
    async fn test_custom_fields() {
        let adapter = CustomJsonAdapter::new(CustomAdapterConfig::default())
            .with_custom_field("environment".to_string(), "production".to_string())
            .with_custom_field("region".to_string(), "us-east-1".to_string());
        
        let test_metrics = create_test_metrics();
        let output = adapter.adapt(&test_metrics).await.unwrap();
        
        let metadata = output.metadata.unwrap();
        assert_eq!(metadata.custom_fields.get("environment"), Some(&"production".to_string()));
        assert_eq!(metadata.custom_fields.get("region"), Some(&"us-east-1".to_string()));
    }
}

// Example integration with the collection engine
pub fn example_integration() -> Result<(), Box<dyn std::error::Error>> {
    use postgres_collector::{CollectorConfig, UnifiedCollectionEngine};

    tokio::runtime::Runtime::new()?.block_on(async {
        // Load configuration
        let config = CollectorConfig::from_file("config.toml")?;
        
        // Create collection engine
        let mut engine = UnifiedCollectionEngine::new(config).await?;
        
        // Add the custom adapter
        let custom_config = CustomAdapterConfig {
            include_metadata: true,
            max_slow_queries: 50,
            custom_fields: [
                ("environment".to_string(), "production".to_string()),
                ("team".to_string(), "database-team".to_string()),
            ].iter().cloned().collect(),
        };
        
        engine.add_adapter(Box::new(CustomJsonAdapter::new(custom_config)));
        
        // Collect and export metrics
        let metrics = engine.collect_all_metrics().await?;
        engine.send_metrics(&metrics).await?;
        
        Ok(())
    })
}

/*
Example output format:

{
  "version": "1.0",
  "timestamp": "2025-06-27T10:30:00Z",
  "metadata": {
    "collector_version": "1.0.0",
    "database_version": "PostgreSQL 15.3",
    "collection_duration_ms": 45,
    "custom_fields": {
      "environment": "production",
      "team": "database-team"
    }
  },
  "metrics": {
    "slow_queries": [
      {
        "query_hash": "query_1",
        "database": "test_db",
        "avg_duration_ms": 250.5,
        "execution_count": 150,
        "query_preview": "SELECT * FROM users WHERE created_at > NOW() - INTERVAL"
      }
    ],
    "summary": {
      "total_slow_queries": 1,
      "avg_query_duration_ms": 250.5,
      "total_execution_count": 150,
      "databases_monitored": ["test_db"]
    }
  }
}
*/