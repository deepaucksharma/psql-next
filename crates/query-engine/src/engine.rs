use std::collections::HashMap;
use anyhow::Result;
use async_trait::async_trait;
use sqlx::{PgConnection, FromRow, Row};
use postgres_collector_core::{CollectorError, CommonParameters};

use crate::queries;

pub struct QueryRegistry {
    queries: HashMap<String, &'static str>,
}

impl QueryRegistry {
    pub fn new() -> Self {
        let mut queries = HashMap::new();
        
        // Register OHI-compatible queries
        queries.insert("slow_queries_v12".to_string(), queries::ohi_queries::SLOW_QUERIES_V12);
        queries.insert("slow_queries_v13+".to_string(), queries::ohi_queries::SLOW_QUERIES_V13_ABOVE);
        queries.insert("wait_events".to_string(), queries::ohi_queries::WAIT_EVENTS);
        queries.insert("wait_events_rds".to_string(), queries::ohi_queries::WAIT_EVENTS_RDS);
        queries.insert("blocking_v12_13".to_string(), queries::ohi_queries::BLOCKING_V12_13);
        queries.insert("blocking_v14+".to_string(), queries::ohi_queries::BLOCKING_V14_ABOVE);
        queries.insert("blocking_rds".to_string(), queries::ohi_queries::BLOCKING_RDS);
        queries.insert("individual_v12".to_string(), queries::ohi_queries::INDIVIDUAL_V12);
        queries.insert("individual_v13+".to_string(), queries::ohi_queries::INDIVIDUAL_V13_ABOVE);
        
        // Register extended queries
        queries.insert("ash_sample".to_string(), queries::extended_queries::ASH_SAMPLE);
        queries.insert("plan_history".to_string(), queries::extended_queries::PLAN_HISTORY);
        queries.insert("buffer_stats_detail".to_string(), queries::extended_queries::BUFFER_STATS_DETAIL);
        
        Self { queries }
    }
    
    pub fn get(&self, key: &str) -> Option<&'static str> {
        self.queries.get(key).copied()
    }
}

pub struct QueryEngine {
    registry: QueryRegistry,
}

impl QueryEngine {
    pub fn new() -> Self {
        Self {
            registry: QueryRegistry::new(),
        }
    }
    
    pub fn select_query_version(&self, query_key: &str, version: u64) -> Result<&'static str, CollectorError> {
        let versioned_key = match query_key {
            "slow_queries" => {
                if version >= 13 {
                    "slow_queries_v13+"
                } else if version == 12 {
                    "slow_queries_v12"
                } else {
                    return Err(CollectorError::UnsupportedVersion(version));
                }
            }
            "blocking" => {
                if version >= 14 {
                    "blocking_v14+"
                } else if version == 12 || version == 13 {
                    "blocking_v12_13"
                } else {
                    return Err(CollectorError::UnsupportedVersion(version));
                }
            }
            "individual" => {
                if version >= 13 {
                    "individual_v13+"
                } else if version == 12 {
                    "individual_v12"
                } else {
                    return Err(CollectorError::UnsupportedVersion(version));
                }
            }
            _ => query_key,
        };
        
        self.registry
            .get(versioned_key)
            .ok_or_else(|| CollectorError::QueryError(format!("Query not found: {}", versioned_key)))
    }
    
    pub fn format_query(&self, query: &str, params: &QueryParams) -> Result<String, CollectorError> {
        let formatted = match params {
            QueryParams::SlowQueries { databases, limit } => {
                format!(query, databases, limit)
            }
            QueryParams::WaitEvents { databases, limit } => {
                format!(query, databases, limit)
            }
            QueryParams::Blocking { databases, limit } => {
                format!(query, databases, limit)
            }
            QueryParams::Individual { databases, limit } => {
                format!(query, databases, limit)
            }
            QueryParams::None => query.to_string(),
        };
        
        Ok(formatted)
    }
    
    pub async fn execute_versioned<T: for<'r> FromRow<'r, sqlx::postgres::PgRow> + Unpin + Send>(
        &self,
        conn: &mut PgConnection,
        query_key: &str,
        version: u64,
        params: QueryParams,
    ) -> Result<Vec<T>, CollectorError> {
        let query = self.select_query_version(query_key, version)?;
        let formatted = self.format_query(query, &params)?;
        
        sqlx::query_as::<_, T>(&formatted)
            .fetch_all(conn)
            .await
            .map_err(CollectorError::from)
    }
}

#[derive(Debug, Clone)]
pub enum QueryParams {
    SlowQueries { databases: String, limit: i32 },
    WaitEvents { databases: String, limit: i32 },
    Blocking { databases: String, limit: i32 },
    Individual { databases: String, limit: i32 },
    None,
}

impl QueryParams {
    pub fn from_common_params(params: &CommonParameters, query_type: &str) -> Self {
        let databases = format!("'{}'", params.databases.replace(',', "','"));
        
        match query_type {
            "slow_queries" => QueryParams::SlowQueries {
                databases,
                limit: params.query_monitoring_count_threshold,
            },
            "wait_events" => QueryParams::WaitEvents {
                databases,
                limit: 100, // Default limit for wait events
            },
            "blocking" => QueryParams::Blocking {
                databases,
                limit: 100, // Default limit for blocking sessions
            },
            "individual" => QueryParams::Individual {
                databases,
                limit: 100, // Default limit for individual queries
            },
            _ => QueryParams::None,
        }
    }
}

/// OHI-compatible query executor
pub struct OHICompatibleQueryExecutor {
    engine: QueryEngine,
}

impl OHICompatibleQueryExecutor {
    pub fn new() -> Self {
        Self {
            engine: QueryEngine::new(),
        }
    }
    
    pub async fn execute_slow_queries(
        &self,
        conn: &mut PgConnection,
        params: &CommonParameters,
    ) -> Result<Vec<postgres_collector_core::SlowQueryMetric>, CollectorError> {
        let query_params = QueryParams::from_common_params(params, "slow_queries");
        let mut rows = self.engine.execute_versioned(
            conn,
            "slow_queries",
            params.version,
            query_params,
        ).await?;
        
        // Apply OHI post-processing
        self.post_process_slow_queries(&mut rows, params);
        
        Ok(rows)
    }
    
    fn post_process_slow_queries(
        &self,
        queries: &mut Vec<postgres_collector_core::SlowQueryMetric>,
        params: &CommonParameters,
    ) {
        for query in queries {
            // OHI anonymizes ALTER statements
            if let Some(text) = &query.query_text {
                if text.to_lowercase().contains("alter") {
                    query.query_text = Some(crate::utils::anonymize_query_text(text));
                }
            }
        }
    }
    
    pub async fn execute_wait_events(
        &self,
        conn: &mut PgConnection,
        params: &CommonParameters,
    ) -> Result<Vec<postgres_collector_core::WaitEventMetric>, CollectorError> {
        let query_key = if params.is_rds { "wait_events_rds" } else { "wait_events" };
        let query_params = QueryParams::from_common_params(params, "wait_events");
        
        self.engine.execute_versioned(
            conn,
            query_key,
            params.version,
            query_params,
        ).await
    }
    
    pub async fn execute_blocking_sessions(
        &self,
        conn: &mut PgConnection,
        params: &CommonParameters,
    ) -> Result<Vec<postgres_collector_core::BlockingSessionMetric>, CollectorError> {
        let query_key = if params.is_rds { "blocking_rds" } else { "blocking" };
        let query_params = QueryParams::from_common_params(params, "blocking");
        
        self.engine.execute_versioned(
            conn,
            query_key,
            params.version,
            query_params,
        ).await
    }
    
    pub async fn execute_individual_queries(
        &self,
        conn: &mut PgConnection,
        params: &CommonParameters,
    ) -> Result<Vec<postgres_collector_core::IndividualQueryMetric>, CollectorError> {
        let query_params = QueryParams::from_common_params(params, "individual");
        
        self.engine.execute_versioned(
            conn,
            "individual",
            params.version,
            query_params,
        ).await
    }
}