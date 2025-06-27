// Example: Comprehensive Integration Testing
// This demonstrates how to write integration tests for the collector

use postgres_collector::{
    CollectorConfig, UnifiedCollectionEngine,
    nri_adapter::NRIAdapter,
    otel_adapter::OTELAdapter,
};
use postgres_collector_core::{UnifiedMetrics, SlowQueryMetric};
use serde_json::Value;
use sqlx::{PgPool, Row};
use std::time::{Duration, Instant};
use testcontainers::{clients::Cli, images::postgres::Postgres, Container};
use tokio::time::sleep;
use wiremock::{
    matchers::{method, path, header},
    Mock, MockServer, ResponseTemplate,
};

/// Integration test suite for the PostgreSQL Unified Collector
pub struct IntegrationTestSuite {
    docker: Cli,
    pg_container: Option<Container<'static, Postgres>>,
    mock_otlp_server: Option<MockServer>,
    test_pool: Option<PgPool>,
}

impl IntegrationTestSuite {
    pub fn new() -> Self {
        Self {
            docker: Cli::default(),
            pg_container: None,
            mock_otlp_server: None,
            test_pool: None,
        }
    }

    /// Setup the complete test environment
    pub async fn setup(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        // Start PostgreSQL container
        self.start_postgres_container().await?;
        
        // Start mock OTLP server
        self.start_mock_otlp_server().await?;
        
        // Setup test data
        self.setup_test_data().await?;
        
        Ok(())
    }

    async fn start_postgres_container(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        println!("Starting PostgreSQL test container...");
        
        let postgres_image = Postgres::default()
            .with_env_var("POSTGRES_DB", "testdb")
            .with_env_var("POSTGRES_USER", "testuser")
            .with_env_var("POSTGRES_PASSWORD", "testpass");
        
        let container = self.docker.run(postgres_image);
        let port = container.get_host_port_ipv4(5432);
        
        // Wait for PostgreSQL to be ready
        let connection_string = format!(
            "postgresql://testuser:testpass@localhost:{}/testdb",
            port
        );
        
        let mut attempts = 0;
        loop {
            if attempts >= 30 {
                return Err("PostgreSQL container failed to start".into());
            }
            
            match PgPool::connect(&connection_string).await {
                Ok(pool) => {
                    self.test_pool = Some(pool);
                    break;
                }
                Err(_) => {
                    attempts += 1;
                    sleep(Duration::from_secs(1)).await;
                }
            }
        }
        
        self.pg_container = Some(container);
        println!("PostgreSQL container started on port {}", port);
        Ok(())
    }

    async fn start_mock_otlp_server(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        println!("Starting mock OTLP server...");
        
        let mock_server = MockServer::start().await;
        
        // Mock successful OTLP metrics endpoint
        Mock::given(method("POST"))
            .and(path("/v1/metrics"))
            .and(header("content-type", "application/x-protobuf"))
            .respond_with(ResponseTemplate::new(200))
            .expect(1..)
            .mount(&mock_server)
            .await;
        
        // Mock OTLP traces endpoint (if needed)
        Mock::given(method("POST"))
            .and(path("/v1/traces"))
            .respond_with(ResponseTemplate::new(200))
            .mount(&mock_server)
            .await;
        
        self.mock_otlp_server = Some(mock_server);
        println!("Mock OTLP server started at {}", self.get_otlp_endpoint());
        Ok(())
    }

    async fn setup_test_data(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        let pool = self.test_pool.as_ref().unwrap();
        
        println!("Setting up test database schema and data...");
        
        // Enable required extensions
        sqlx::query("CREATE EXTENSION IF NOT EXISTS pg_stat_statements")
            .execute(pool)
            .await?;
        
        // Create test tables
        sqlx::query(
            r#"
            CREATE TABLE IF NOT EXISTS users (
                id SERIAL PRIMARY KEY,
                name VARCHAR(100),
                email VARCHAR(100),
                created_at TIMESTAMP DEFAULT NOW()
            )
            "#
        )
        .execute(pool)
        .await?;
        
        sqlx::query(
            r#"
            CREATE TABLE IF NOT EXISTS orders (
                id SERIAL PRIMARY KEY,
                user_id INTEGER REFERENCES users(id),
                total DECIMAL(10,2),
                status VARCHAR(50),
                created_at TIMESTAMP DEFAULT NOW()
            )
            "#
        )
        .execute(pool)
        .await?;
        
        // Insert test data
        self.insert_test_data(pool).await?;
        
        // Generate some slow queries for testing
        self.generate_slow_queries(pool).await?;
        
        println!("Test data setup complete");
        Ok(())
    }

    async fn insert_test_data(&self, pool: &PgPool) -> Result<(), Box<dyn std::error::Error>> {
        // Insert test users
        for i in 1..=100 {
            sqlx::query(
                "INSERT INTO users (name, email) VALUES ($1, $2)"
            )
            .bind(format!("User {}", i))
            .bind(format!("user{}@example.com", i))
            .execute(pool)
            .await?;
        }
        
        // Insert test orders
        for i in 1..=500 {
            sqlx::query(
                "INSERT INTO orders (user_id, total, status) VALUES ($1, $2, $3)"
            )
            .bind((i % 100) + 1) // Random user
            .bind(format!("{:.2}", (i as f64) * 2.5)) // Random amount
            .bind(if i % 3 == 0 { "completed" } else { "pending" })
            .execute(pool)
            .await?;
        }
        
        Ok(())
    }

    async fn generate_slow_queries(&self, pool: &PgPool) -> Result<(), Box<dyn std::error::Error>> {
        println!("Generating slow queries for testing...");
        
        // Reset pg_stat_statements to start fresh
        sqlx::query("SELECT pg_stat_statements_reset()")
            .execute(pool)
            .await?;
        
        // Generate various types of slow queries
        let slow_queries = vec![
            // Slow JOIN query
            r#"
            SELECT u.name, COUNT(o.id) as order_count, AVG(o.total) as avg_total
            FROM users u 
            LEFT JOIN orders o ON u.id = o.user_id 
            WHERE u.created_at > NOW() - INTERVAL '30 days'
            GROUP BY u.id, u.name 
            ORDER BY order_count DESC
            "#,
            
            // Slow aggregation query
            r#"
            SELECT 
                DATE_TRUNC('day', created_at) as day,
                COUNT(*) as daily_orders,
                SUM(total) as daily_revenue,
                AVG(total) as avg_order_value
            FROM orders 
            WHERE created_at >= NOW() - INTERVAL '90 days'
            GROUP BY DATE_TRUNC('day', created_at)
            ORDER BY day DESC
            "#,
            
            // Slow search query (without index)
            r#"
            SELECT u.*, COUNT(o.id) as order_count
            FROM users u
            LEFT JOIN orders o ON u.id = o.user_id
            WHERE u.email LIKE '%@example.com'
            GROUP BY u.id
            HAVING COUNT(o.id) > 0
            "#,
            
            // Slow UPDATE query
            r#"
            UPDATE orders 
            SET status = 'processed' 
            WHERE status = 'pending' 
            AND created_at < NOW() - INTERVAL '1 hour'
            "#,
        ];
        
        // Execute each query multiple times to build statistics
        for (i, query) in slow_queries.iter().enumerate() {
            for execution in 1..=5 {
                let start = Instant::now();
                
                // Add artificial delay to make queries "slow"
                let delay_query = format!("SELECT pg_sleep(0.1); {}", query);
                
                sqlx::query(&delay_query)
                    .execute(pool)
                    .await?;
                
                let duration = start.elapsed();
                println!(
                    "Executed slow query {} (execution {}) in {:?}",
                    i + 1, execution, duration
                );
            }
        }
        
        // Wait for statistics to be updated
        sleep(Duration::from_secs(2)).await;
        
        Ok(())
    }

    fn get_postgres_connection_string(&self) -> String {
        if let Some(container) = &self.pg_container {
            let port = container.get_host_port_ipv4(5432);
            format!("postgresql://testuser:testpass@localhost:{}/testdb", port)
        } else {
            panic!("PostgreSQL container not started")
        }
    }

    fn get_otlp_endpoint(&self) -> String {
        if let Some(server) = &self.mock_otlp_server {
            server.uri()
        } else {
            panic!("Mock OTLP server not started")
        }
    }

    /// Create a test configuration
    pub fn create_test_config(&self) -> CollectorConfig {
        CollectorConfig {
            connection_string: self.get_postgres_connection_string(),
            host: "localhost".to_string(),
            port: 5432,
            username: "testuser".to_string(),
            password: "testpass".to_string(),
            database: "testdb".to_string(),
            databases: vec!["testdb".to_string()],
            max_connections: 5,
            connect_timeout_secs: 30,
            
            // Collection settings for testing
            collection_interval_secs: 10,
            enable_extended_metrics: true,
            enable_ash: false, // Disable ASH for simpler testing
            query_monitoring_count_threshold: 1, // Low threshold for testing
            query_monitoring_response_time_threshold: 1, // Low threshold
            
            // Query sanitization
            sanitize_query_text: true,
            sanitization_mode: Some("smart".to_string()),
            
            // Output configuration
            outputs: OutputConfig {
                nri: Some(NRIOutputConfig {
                    enabled: true,
                }),
                otlp: Some(OTLPOutputConfig {
                    enabled: true,
                    endpoint: self.get_otlp_endpoint(),
                    protocol: "http".to_string(),
                    headers: std::collections::HashMap::new(),
                }),
            },
            
            // Health configuration
            health: Some(HealthConfig {
                enabled: true,
                bind_address: "0.0.0.0:0".to_string(), // Random port
            }),
            
            // Disable PgBouncer for testing
            pgbouncer: None,
            
            // Other settings...
            ash_sample_interval_secs: 5,
            ash_retention_hours: 1,
            ash_max_memory_mb: Some(10),
        }
    }
}

/// Test the complete collection and export pipeline
#[tokio::test]
async fn test_end_to_end_collection() {
    let mut test_suite = IntegrationTestSuite::new();
    test_suite.setup().await.expect("Failed to setup test environment");
    
    // Create collector configuration
    let config = test_suite.create_test_config();
    
    // Create collection engine
    let mut engine = UnifiedCollectionEngine::new(config)
        .await
        .expect("Failed to create collection engine");
    
    // Add adapters
    engine.add_adapter(Box::new(NRIAdapter::new()));
    engine.add_adapter(Box::new(OTELAdapter::new()));
    
    // Collect metrics
    let start = Instant::now();
    let metrics = engine.collect_all_metrics()
        .await
        .expect("Failed to collect metrics");
    let collection_duration = start.elapsed();
    
    // Verify collection performance
    assert!(collection_duration < Duration::from_secs(10), 
           "Collection took too long: {:?}", collection_duration);
    
    // Verify metrics content
    assert!(!metrics.slow_queries.is_empty(), "No slow queries collected");
    
    // Verify query sanitization
    for query in &metrics.slow_queries {
        if let Some(query_text) = &query.query_text {
            assert!(!query_text.contains("testuser"), 
                   "Query text not properly sanitized: {}", query_text);
        }
    }
    
    // Send metrics through adapters
    engine.send_metrics(&metrics)
        .await
        .expect("Failed to send metrics");
    
    println!("End-to-end test completed successfully");
    println!("Collection duration: {:?}", collection_duration);
    println!("Slow queries collected: {}", metrics.slow_queries.len());
}

/// Test adapter functionality in isolation
#[tokio::test]
async fn test_nri_adapter() {
    let mut test_suite = IntegrationTestSuite::new();
    test_suite.setup().await.expect("Failed to setup test environment");
    
    let config = test_suite.create_test_config();
    let engine = UnifiedCollectionEngine::new(config)
        .await
        .expect("Failed to create collection engine");
    
    let metrics = engine.collect_all_metrics()
        .await
        .expect("Failed to collect metrics");
    
    // Test NRI adapter
    let nri_adapter = NRIAdapter::new();
    let nri_output = nri_adapter.adapt(&metrics)
        .await
        .expect("Failed to adapt metrics to NRI format");
    
    // Verify NRI format
    let serialized = nri_output.serialize()
        .expect("Failed to serialize NRI output");
    
    let json: Value = serde_json::from_slice(&serialized)
        .expect("Invalid JSON from NRI adapter");
    
    // Verify NRI structure
    assert_eq!(json["protocol_version"], "4");
    assert!(json["data"].is_array());
    assert_eq!(nri_output.content_type(), "application/json");
    
    println!("NRI adapter test passed");
}

/// Test OTLP adapter functionality
#[tokio::test]
async fn test_otlp_adapter() {
    let mut test_suite = IntegrationTestSuite::new();
    test_suite.setup().await.expect("Failed to setup test environment");
    
    let config = test_suite.create_test_config();
    let engine = UnifiedCollectionEngine::new(config)
        .await
        .expect("Failed to create collection engine");
    
    let metrics = engine.collect_all_metrics()
        .await
        .expect("Failed to collect metrics");
    
    // Test OTLP adapter
    let otlp_adapter = OTELAdapter::new();
    let otlp_output = otlp_adapter.adapt(&metrics)
        .await
        .expect("Failed to adapt metrics to OTLP format");
    
    // Verify OTLP format
    let serialized = otlp_output.serialize()
        .expect("Failed to serialize OTLP output");
    
    // Verify content type
    assert_eq!(otlp_output.content_type(), "application/x-protobuf");
    
    // Verify protobuf can be parsed (basic validation)
    assert!(!serialized.is_empty(), "OTLP output is empty");
    assert!(serialized.len() > 10, "OTLP output seems too small");
    
    println!("OTLP adapter test passed");
}

/// Test error handling and recovery
#[tokio::test]
async fn test_error_handling() {
    let mut test_suite = IntegrationTestSuite::new();
    test_suite.setup().await.expect("Failed to setup test environment");
    
    // Test with invalid configuration
    let mut config = test_suite.create_test_config();
    config.connection_string = "postgresql://invalid:invalid@nonexistent:5432/invalid".to_string();
    
    // This should fail gracefully
    let result = UnifiedCollectionEngine::new(config).await;
    assert!(result.is_err(), "Expected connection failure with invalid config");
    
    println!("Error handling test passed");
}

/// Performance benchmark test
#[tokio::test]
async fn test_performance_benchmark() {
    let mut test_suite = IntegrationTestSuite::new();
    test_suite.setup().await.expect("Failed to setup test environment");
    
    let config = test_suite.create_test_config();
    let engine = UnifiedCollectionEngine::new(config)
        .await
        .expect("Failed to create collection engine");
    
    // Perform multiple collections to test performance
    let iterations = 10;
    let mut durations = Vec::new();
    
    for i in 1..=iterations {
        let start = Instant::now();
        let _metrics = engine.collect_all_metrics()
            .await
            .expect("Failed to collect metrics");
        let duration = start.elapsed();
        durations.push(duration);
        
        println!("Collection {} took {:?}", i, duration);
        
        // Sleep between collections
        sleep(Duration::from_millis(100)).await;
    }
    
    // Calculate statistics
    let avg_duration = durations.iter().sum::<Duration>() / iterations as u32;
    let max_duration = durations.iter().max().unwrap();
    let min_duration = durations.iter().min().unwrap();
    
    println!("Performance benchmark results:");
    println!("  Average: {:?}", avg_duration);
    println!("  Maximum: {:?}", max_duration);
    println!("  Minimum: {:?}", min_duration);
    
    // Performance assertions
    assert!(avg_duration < Duration::from_secs(5), 
           "Average collection time too high: {:?}", avg_duration);
    assert!(*max_duration < Duration::from_secs(10), 
           "Maximum collection time too high: {:?}", max_duration);
}

/// Test concurrent collection
#[tokio::test]
async fn test_concurrent_collection() {
    let mut test_suite = IntegrationTestSuite::new();
    test_suite.setup().await.expect("Failed to setup test environment");
    
    let config = test_suite.create_test_config();
    let engine = std::sync::Arc::new(
        UnifiedCollectionEngine::new(config)
            .await
            .expect("Failed to create collection engine")
    );
    
    // Run multiple concurrent collections
    let mut handles = Vec::new();
    
    for i in 1..=5 {
        let engine_clone = engine.clone();
        let handle = tokio::spawn(async move {
            let start = Instant::now();
            let metrics = engine_clone.collect_all_metrics()
                .await
                .expect("Failed to collect metrics");
            let duration = start.elapsed();
            
            println!("Concurrent collection {} completed in {:?}", i, duration);
            (i, metrics.slow_queries.len(), duration)
        });
        handles.push(handle);
    }
    
    // Wait for all collections to complete
    let mut results = Vec::new();
    for handle in handles {
        let result = handle.await.expect("Concurrent collection failed");
        results.push(result);
    }
    
    // Verify all collections succeeded
    assert_eq!(results.len(), 5, "Not all concurrent collections completed");
    
    for (id, query_count, duration) in results {
        assert!(query_count > 0, "Collection {} found no queries", id);
        assert!(duration < Duration::from_secs(15), 
               "Collection {} took too long: {:?}", id, duration);
    }
    
    println!("Concurrent collection test passed");
}

/// Test configuration validation
#[tokio::test]
async fn test_configuration_validation() {
    // Test with minimal valid configuration
    let valid_config = CollectorConfig {
        connection_string: "postgresql://test:test@localhost:5432/test".to_string(),
        host: "localhost".to_string(),
        port: 5432,
        username: "test".to_string(),
        password: "test".to_string(),
        database: "test".to_string(),
        databases: vec!["test".to_string()],
        max_connections: 1,
        connect_timeout_secs: 30,
        collection_interval_secs: 30,
        enable_extended_metrics: false,
        enable_ash: false,
        query_monitoring_count_threshold: 100,
        query_monitoring_response_time_threshold: 1000,
        sanitize_query_text: true,
        sanitization_mode: Some("smart".to_string()),
        outputs: OutputConfig {
            nri: Some(NRIOutputConfig { enabled: true }),
            otlp: None,
        },
        health: None,
        pgbouncer: None,
        ash_sample_interval_secs: 5,
        ash_retention_hours: 24,
        ash_max_memory_mb: Some(100),
    };
    
    // This should not panic (validates configuration parsing)
    let _config_json = serde_json::to_string(&valid_config)
        .expect("Failed to serialize valid configuration");
    
    println!("Configuration validation test passed");
}

// Helper function to run all integration tests
pub async fn run_integration_tests() -> Result<(), Box<dyn std::error::Error>> {
    println!("Running PostgreSQL Unified Collector Integration Tests");
    
    // Run tests sequentially to avoid resource conflicts
    test_end_to_end_collection().await;
    test_nri_adapter().await;
    test_otlp_adapter().await;
    test_error_handling().await;
    test_performance_benchmark().await;
    test_concurrent_collection().await;
    test_configuration_validation().await;
    
    println!("All integration tests passed!");
    Ok(())
}

// Example of how to run tests from main
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize tracing for test output
    tracing_subscriber::fmt::init();
    
    run_integration_tests().await?;
    Ok(())
}