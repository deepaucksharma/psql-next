use serde::{Deserialize, Serialize};

/// Common parameters matching OHI exactly
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommonParameters {
    pub version: u64,
    pub databases: String,
    pub query_monitoring_count_threshold: i32,
    pub query_monitoring_response_time_threshold: i32,
    pub host: String,
    pub port: String,
    pub is_rds: bool,
}

impl CommonParameters {
    pub fn validate_count_threshold(input: i32) -> i32 {
        const DEFAULT: i32 = 20;
        const MAX: i32 = 30;
        
        match input {
            i if i < 0 => {
                tracing::warn!(
                    "QueryCountThreshold should be greater than 0 but the input is {}, \
                     setting value to default which is {}",
                    i, DEFAULT
                );
                DEFAULT
            }
            i if i > MAX => {
                tracing::warn!(
                    "QueryCountThreshold should be less than or equal to max limit but \
                     the input is {}, setting value to max limit which is {}",
                    i, MAX
                );
                MAX
            }
            i => i,
        }
    }
    
    pub fn validate_response_threshold(input: i32) -> i32 {
        const DEFAULT: i32 = 500;
        
        if input < 0 {
            tracing::warn!(
                "QueryResponseTimeThreshold should be greater than or equal to 0 but \
                 the input is {}, setting value to default which is {}",
                input, DEFAULT
            );
            DEFAULT
        } else {
            input
        }
    }
}

/// Metric type for NRI compatibility
#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
pub enum MetricType {
    #[serde(rename = "gauge")]
    Gauge,
    #[serde(rename = "attribute")]
    Attribute,
    #[serde(rename = "count")]
    Count,
    #[serde(rename = "summary")]
    Summary,
}

/// Event types matching OHI
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum EventType {
    PostgresSlowQueries,
    PostgresWaitEvents,
    PostgresBlockingSessions,
    PostgresIndividualQueries,
    PostgresExecutionPlanMetrics,
    PostgresExtendedMetrics,
}