# OHI Metric Mapping - Detailed Implementation

## Complete Metric Coverage Mapping

### 1. SlowRunningQueryMetrics → UnifiedMetrics

```go
// Original OHI structure
type SlowRunningQueryMetrics struct {
    Newrelic            *string  `db:"newrelic"              metric_name:"newrelic"                   source_type:"attribute"`
    QueryID             *string  `db:"query_id"              metric_name:"query_id"                   source_type:"attribute"`
    QueryText           *string  `db:"query_text"            metric_name:"query_text"                 source_type:"attribute"`
    DatabaseName        *string  `db:"database_name"         metric_name:"database_name"              source_type:"attribute"`
    SchemaName          *string  `db:"schema_name"           metric_name:"schema_name"                source_type:"attribute"`
    ExecutionCount      *int64   `db:"execution_count"       metric_name:"execution_count"            source_type:"gauge"`
    AvgElapsedTimeMs    *float64 `db:"avg_elapsed_time_ms"   metric_name:"avg_elapsed_time_ms"        source_type:"gauge"`
    AvgDiskReads        *float64 `db:"avg_disk_reads"        metric_name:"avg_disk_reads"             source_type:"gauge"`
    AvgDiskWrites       *float64 `db:"avg_disk_writes"       metric_name:"avg_disk_writes"            source_type:"gauge"`
    StatementType       *string  `db:"statement_type"        metric_name:"statement_type"             source_type:"attribute"`
    CollectionTimestamp *string  `db:"collection_timestamp"  metric_name:"collection_timestamp"       source_type:"attribute"`
    IndividualQuery     *string  `db:"individual_query"      metric_name:"individual_query"           source_type:"attribute"`
}
```

```rust
// Unified implementation with 100% compatibility
impl From<SlowRunningQueryMetrics> for UnifiedSlowQueryMetric {
    fn from(ohi: SlowRunningQueryMetrics) -> Self {
        UnifiedSlowQueryMetric {
            // Direct mappings - MUST maintain exact field names for NRI
            newrelic: ohi.newrelic,
            query_id: ohi.query_id,
            query_text: ohi.query_text,
            database_name: ohi.database_name,
            schema_name: ohi.schema_name,
            execution_count: ohi.execution_count,
            avg_elapsed_time_ms: ohi.avg_elapsed_time_ms,
            avg_disk_reads: ohi.avg_disk_reads,
            avg_disk_writes: ohi.avg_disk_writes,
            statement_type: ohi.statement_type,
            collection_timestamp: ohi.collection_timestamp,
            individual_query: ohi.individual_query,
            
            // Extended fields (only populated if enabled)
            extended: if ENABLE_EXTENDED_METRICS {
                Some(ExtendedSlowQueryMetrics {
                    percentiles: self.calculate_percentiles(&ohi),
                    cpu_breakdown: self.get_cpu_breakdown(&ohi),
                    memory_stats: self.get_memory_stats(&ohi),
                    ebpf_data: self.get_ebpf_data(&ohi),
                })
            } else {
                None
            },
        }
    }
}
```

### 2. Query Implementation Compatibility

```rust
// Maintain exact OHI query strings
pub mod ohi_queries {
    pub const SLOW_QUERIES_V13_ABOVE: &str = r#"
        SELECT 'newrelic' as newrelic,
            pss.queryid AS query_id,
            LEFT(pss.query, 4095) AS query_text,
            pd.datname AS database_name,
            current_schema() AS schema_name,
            pss.calls AS execution_count,
            ROUND((pss.total_exec_time / pss.calls)::numeric, 3) AS avg_elapsed_time_ms,
            pss.shared_blks_read / pss.calls AS avg_disk_reads,
            pss.shared_blks_written / pss.calls AS avg_disk_writes,
            CASE
                WHEN pss.query ILIKE 'SELECT%%' THEN 'SELECT'
                WHEN pss.query ILIKE 'INSERT%%' THEN 'INSERT'
                WHEN pss.query ILIKE 'UPDATE%%' THEN 'UPDATE'
                WHEN pss.query ILIKE 'DELETE%%' THEN 'DELETE'
                ELSE 'OTHER'
            END AS statement_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_statements pss
        JOIN pg_database pd ON pss.dbid = pd.oid
        WHERE pd.datname in (%s)
            AND pss.query NOT ILIKE 'EXPLAIN (FORMAT JSON)%%'
            AND pss.query NOT ILIKE 'SELECT $1 as newrelic%%'
            AND pss.query NOT ILIKE 'WITH wait_history AS%%'
            AND pss.query NOT ILIKE 'select -- BLOATQUERY%%'
            AND pss.query NOT ILIKE 'select -- INDEXQUERY%%'
            AND pss.query NOT ILIKE 'SELECT -- TABLEQUERY%%'
            AND pss.query NOT ILIKE 'SELECT table_schema%%'
        ORDER BY avg_elapsed_time_ms DESC
        LIMIT %d;
    "#;
    
    // Include all other OHI queries verbatim...
}

// Query executor that maintains OHI behavior
impl OHICompatibleQueryExecutor {
    pub async fn execute_slow_queries(
        &self,
        conn: &PgConnection,
        params: &CommonParameters,
    ) -> Result<Vec<SlowQueryMetric>, Error> {
        let query = match params.version {
            12 => ohi_queries::SLOW_QUERIES_V12,
            v if v >= 13 => ohi_queries::SLOW_QUERIES_V13_ABOVE,
            _ => return Err(Error::UnsupportedVersion(params.version)),
        };
        
        let formatted = format!(
            query,
            params.databases,
            params.query_monitoring_count_threshold
        );
        
        let rows = sqlx::query_as::<_, SlowQueryMetric>(&formatted)
            .fetch_all(conn)
            .await?;
        
        // Apply OHI post-processing
        Ok(self.post_process_slow_queries(rows, params))
    }
    
    fn post_process_slow_queries(
        &self,
        mut queries: Vec<SlowQueryMetric>,
        params: &CommonParameters,
    ) -> Vec<SlowQueryMetric> {
        for query in &mut queries {
            // OHI anonymizes ALTER statements
            if let Some(text) = &query.query_text {
                if text.to_lowercase().contains("alter") {
                    query.query_text = Some(self.anonymize_query_text(text));
                }
            }
        }
        queries
    }
}
```

### 3. Parameter Validation (100% OHI Compatible)

```rust
// Match OHI parameter validation exactly
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
    pub fn from_ohi_args(args: &ArgumentList) -> Self {
        Self {
            version: args.version,
            databases: args.databases,
            query_monitoring_count_threshold: Self::validate_count_threshold(
                args.query_monitoring_count_threshold
            ),
            query_monitoring_response_time_threshold: Self::validate_response_threshold(
                args.query_monitoring_response_time_threshold
            ),
            host: args.hostname.clone(),
            port: args.port.clone(),
            is_rds: args.is_rds,
        }
    }
    
    // Match OHI validation logic exactly
    fn validate_count_threshold(input: i32) -> i32 {
        const DEFAULT: i32 = 20;
        const MAX: i32 = 30;
        
        match input {
            i if i < 0 => {
                log::warn!(
                    "QueryCountThreshold should be greater than 0 but the input is {}, \
                     setting value to default which is {}",
                    i, DEFAULT
                );
                DEFAULT
            }
            i if i > MAX => {
                log::warn!(
                    "QueryCountThreshold should be less than or equal to max limit but \
                     the input is {}, setting value to max limit which is {}",
                    i, MAX
                );
                MAX
            }
            i => i,
        }
    }
    
    fn validate_response_threshold(input: i32) -> i32 {
        const DEFAULT: i32 = 500;
        
        if input < 0 {
            log::warn!(
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
```

### 4. Extension Detection (OHI Compatible)

```rust
// Match OHI extension detection exactly
pub async fn fetch_all_extensions(
    conn: &PgConnection,
) -> Result<HashMap<String, bool>, Error> {
    let rows = sqlx::query!("SELECT extname FROM pg_extension")
        .fetch_all(conn)
        .await?;
    
    let mut enabled_extensions = HashMap::new();
    for row in rows {
        enabled_extensions.insert(row.extname, true);
    }
    
    Ok(enabled_extensions)
}

// OHI eligibility checks
pub struct OHIValidations;

impl OHIValidations {
    pub fn check_slow_query_metrics_fetch_eligibility(
        enabled_extensions: &HashMap<String, bool>,
    ) -> bool {
        enabled_extensions.get("pg_stat_statements").copied().unwrap_or(false)
    }
    
    pub fn check_wait_event_metrics_fetch_eligibility(
        enabled_extensions: &HashMap<String, bool>,
    ) -> bool {
        enabled_extensions.get("pg_stat_statements").copied().unwrap_or(false)
            && enabled_extensions.get("pg_wait_sampling").copied().unwrap_or(false)
    }
    
    pub fn check_blocking_session_metrics_fetch_eligibility(
        enabled_extensions: &HashMap<String, bool>,
        version: u64,
    ) -> bool {
        // Version 12 and 13 don't require pg_stat_statements
        if version == 12 || version == 13 {
            return true;
        }
        enabled_extensions.get("pg_stat_statements").copied().unwrap_or(false)
    }
    
    pub fn check_individual_query_metrics_fetch_eligibility(
        enabled_extensions: &HashMap<String, bool>,
    ) -> bool {
        enabled_extensions.get("pg_stat_monitor").copied().unwrap_or(false)
    }
    
    pub fn check_postgres_version_support_for_query_monitoring(version: u64) -> bool {
        version >= 12
    }
}
```

### 5. NRI Adapter - Exact OHI Output Format

```rust
pub struct NRIAdapter {
    entity_key: String,
    integration_version: String,
}

impl NRIAdapter {
    pub fn adapt_to_nri(&self, metrics: &UnifiedMetrics) -> Result<String, Error> {
        let mut output = NRIOutput::new("com.newrelic.postgresql", "2.0.0");
        let entity = output.entity(&self.entity_key, "pg-instance");
        
        // PostgresSlowQueries event type (exact OHI format)
        for metric in &metrics.slow_queries {
            let mut metric_set = entity.new_metric_set("PostgresSlowQueries");
            
            // Set metrics exactly as OHI does
            if let Some(v) = &metric.query_id {
                metric_set.set_metric("query_id", v, MetricType::ATTRIBUTE)?;
            }
            if let Some(v) = &metric.query_text {
                metric_set.set_metric("query_text", v, MetricType::ATTRIBUTE)?;
            }
            if let Some(v) = &metric.database_name {
                metric_set.set_metric("database_name", v, MetricType::ATTRIBUTE)?;
            }
            if let Some(v) = &metric.schema_name {
                metric_set.set_metric("schema_name", v, MetricType::ATTRIBUTE)?;
            }
            if let Some(v) = &metric.execution_count {
                metric_set.set_metric("execution_count", *v, MetricType::GAUGE)?;
            }
            if let Some(v) = &metric.avg_elapsed_time_ms {
                metric_set.set_metric("avg_elapsed_time_ms", *v, MetricType::GAUGE)?;
            }
            if let Some(v) = &metric.avg_disk_reads {
                metric_set.set_metric("avg_disk_reads", *v, MetricType::GAUGE)?;
            }
            if let Some(v) = &metric.avg_disk_writes {
                metric_set.set_metric("avg_disk_writes", *v, MetricType::GAUGE)?;
            }
            if let Some(v) = &metric.statement_type {
                metric_set.set_metric("statement_type", v, MetricType::ATTRIBUTE)?;
            }
            if let Some(v) = &metric.collection_timestamp {
                metric_set.set_metric("collection_timestamp", v, MetricType::ATTRIBUTE)?;
            }
        }
        
        // Repeat for all other event types...
        self.adapt_wait_events(&mut entity, &metrics.wait_events)?;
        self.adapt_blocking_sessions(&mut entity, &metrics.blocking_sessions)?;
        self.adapt_individual_queries(&mut entity, &metrics.individual_queries)?;
        self.adapt_execution_plans(&mut entity, &metrics.execution_plans)?;
        
        Ok(output.to_json()?)
    }
}
```

### 6. Ingestion Helper Compatibility

```rust
// Match OHI ingestion behavior
pub struct IngestionHelper {
    publish_threshold: usize,
}

impl IngestionHelper {
    pub fn new() -> Self {
        Self {
            publish_threshold: 600, // OHI constant
        }
    }
    
    pub async fn ingest_metric<T: Metric>(
        &self,
        metric_list: Vec<T>,
        event_name: &str,
        integration: &mut Integration,
        params: &CommonParameters,
    ) -> Result<(), Error> {
        let entity = self.create_entity(integration, params)?;
        let mut metric_count = 0;
        
        for model in metric_list {
            metric_count += 1;
            let metric_set = entity.new_metric_set(event_name);
            
            self.process_model(&model, &mut metric_set)?;
            
            // Match OHI batch publishing
            if metric_count == self.publish_threshold {
                metric_count = 0;
                self.publish_metrics(integration, params).await?;
            }
        }
        
        if metric_count > 0 {
            self.publish_metrics(integration, params).await?;
        }
        
        Ok(())
    }
    
    fn create_entity(
        &self,
        integration: &mut Integration,
        params: &CommonParameters,
    ) -> Result<Entity, Error> {
        let entity_key = format!("{}:{}", params.host, params.port);
        integration.entity(&entity_key, "pg-instance")
    }
}
```

### 7. Query Text Processing (OHI Compatible)

```rust
// Match OHI anonymization exactly
pub fn anonymize_query_text(query: &str) -> String {
    // OHI regex: `'[^']*'|\d+|".*?"`
    lazy_static! {
        static ref RE: Regex = Regex::new(r#"'[^']*'|\d+|".*?""#).unwrap();
    }
    RE.replace_all(query, "?").to_string()
}

pub fn anonymize_and_normalize(query: &str) -> String {
    // Match OHI normalization steps exactly
    let mut result = query.to_string();
    
    // Replace numbers
    let re_numbers = Regex::new(r"\d+").unwrap();
    result = re_numbers.replace_all(&result, "?").to_string();
    
    // Replace single quotes
    let re_single = Regex::new(r"'[^']*'").unwrap();
    result = re_single.replace_all(&result, "?").to_string();
    
    // Replace double quotes
    let re_double = Regex::new(r#""[^"]*""#).unwrap();
    result = re_double.replace_all(&result, "?").to_string();
    
    // Remove dollar signs
    result = result.replace('$', "");
    
    // Convert to lowercase
    result = result.to_lowercase();
    
    // Remove semicolons
    result = result.replace(';', "");
    
    // Trim and normalize spaces
    result = result.trim().to_string();
    result = result.split_whitespace().collect::<Vec<_>>().join(" ");
    
    result
}
```

### 8. RDS Mode Implementation

```rust
// RDS-specific implementations matching OHI
pub struct RDSModeCollector {
    pub async fn collect_slow_queries_rds(
        &self,
        conn: &PgConnection,
        params: &CommonParameters,
    ) -> Result<Vec<SlowQueryMetric>, Error> {
        // Use same query as non-RDS but with individual query correlation
        let mut metrics = self.collect_slow_queries(conn, params).await?;
        
        // Get individual queries from pg_stat_activity (RDS mode)
        let individual_queries = self.get_individual_queries_from_pg_stat(conn).await?;
        
        // Correlate by normalized text (OHI approach)
        let filtered = self.get_filtered_slow_metrics(individual_queries, metrics);
        
        Ok(filtered)
    }
    
    async fn get_individual_queries_from_pg_stat(
        &self,
        conn: &PgConnection,
    ) -> Result<Vec<String>, Error> {
        let query = "SELECT query FROM pg_stat_activity WHERE query IS NOT NULL AND query != '';";
        
        let rows = sqlx::query!(query)
            .fetch_all(conn)
            .await?;
        
        Ok(rows.into_iter().map(|r| r.query).collect())
    }
    
    fn get_filtered_slow_metrics(
        &self,
        individual_queries: Vec<String>,
        slow_metrics: Vec<SlowQueryMetric>,
    ) -> Vec<SlowQueryMetric> {
        let mut individual_query_map = HashMap::new();
        
        for query in individual_queries {
            let normalized = anonymize_and_normalize(&query);
            individual_query_map.insert(normalized, query);
        }
        
        slow_metrics
            .into_iter()
            .filter_map(|mut metric| {
                if let Some(query_text) = &metric.query_text {
                    let normalized = anonymize_and_normalize(query_text);
                    if let Some(individual) = individual_query_map.get(&normalized) {
                        metric.individual_query = Some(individual.clone());
                        return Some(metric);
                    }
                }
                None
            })
            .collect()
    }
}
```

## Testing Strategy for 100% Compatibility

```rust
#[cfg(test)]
mod ohi_compatibility_tests {
    use super::*;
    
    #[test]
    fn test_slow_query_metric_compatibility() {
        // Test data from actual OHI
        let ohi_metric = SlowRunningQueryMetrics {
            newrelic: Some("newrelic".to_string()),
            query_id: Some("-1234567890".to_string()),
            query_text: Some("SELECT * FROM users WHERE id = ?".to_string()),
            database_name: Some("testdb".to_string()),
            schema_name: Some("public".to_string()),
            execution_count: Some(100),
            avg_elapsed_time_ms: Some(45.678),
            avg_disk_reads: Some(1.23),
            avg_disk_writes: Some(0.45),
            statement_type: Some("SELECT".to_string()),
            collection_timestamp: Some("2025-06-26T21:00:00Z".to_string()),
            individual_query: None,
        };
        
        // Convert to unified format
        let unified: UnifiedSlowQueryMetric = ohi_metric.into();
        
        // Convert back to NRI format
        let nri_output = NRIAdapter::new().adapt_slow_query(&unified).unwrap();
        
        // Verify exact field matching
        assert_eq!(nri_output.get("query_id"), Some(&json!("-1234567890")));
        assert_eq!(nri_output.get("query_text"), Some(&json!("SELECT * FROM users WHERE id = ?")));
        assert_eq!(nri_output.get("database_name"), Some(&json!("testdb")));
        assert_eq!(nri_output.get("execution_count"), Some(&json!(100)));
        assert_eq!(nri_output.get("avg_elapsed_time_ms"), Some(&json!(45.678)));
    }
    
    #[test]
    fn test_parameter_validation_compatibility() {
        // Test OHI parameter validation edge cases
        assert_eq!(CommonParameters::validate_count_threshold(-1), 20);
        assert_eq!(CommonParameters::validate_count_threshold(0), 0);
        assert_eq!(CommonParameters::validate_count_threshold(25), 25);
        assert_eq!(CommonParameters::validate_count_threshold(31), 30);
        
        assert_eq!(CommonParameters::validate_response_threshold(-1), 500);
        assert_eq!(CommonParameters::validate_response_threshold(0), 0);
        assert_eq!(CommonParameters::validate_response_threshold(1000), 1000);
    }
    
    #[test]
    fn test_query_anonymization_compatibility() {
        // Test cases from OHI test suite
        let test_cases = vec![
            (
                "SELECT * FROM users WHERE id = 1 AND name = 'John'",
                "SELECT * FROM users WHERE id = ? AND name = ?",
            ),
            (
                "SELECT * FROM employees WHERE id = 10 OR name <> 'John Doe'",
                "SELECT * FROM employees WHERE id = ? OR name <> ?",
            ),
        ];
        
        for (input, expected) in test_cases {
            assert_eq!(anonymize_query_text(input), expected);
        }
    }
}
```

## Migration Guide from OHI

```yaml
# Step 1: Deploy in parallel mode
migration:
  phase: "validation"
  old_integration:
    enabled: true
    config: "/etc/newrelic-infra/integrations.d/postgresql-config.yml"
  
  new_integration:
    enabled: true
    mode: "shadow"  # Collect but don't send
    validation:
      compare_metrics: true
      tolerance: 0.01  # 1% tolerance for float values
      log_differences: true

# Step 2: Gradual rollout
migration:
  phase: "rollout"
  percentage: 25  # Start with 25% of instances
  
# Step 3: Full migration
migration:
  phase: "complete"
  old_integration:
    enabled: false
  new_integration:
    enabled: true
    enable_extended_metrics: true  # Now safe to enable
```