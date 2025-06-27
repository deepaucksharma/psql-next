use anyhow::Result;
use hyper::{Body, Client, Method, Request};
use hyper::client::HttpConnector;
use std::time::Duration;
use tracing::{debug, error, info};

/// Metric exporter responsible for sending serialized metrics to endpoints
pub struct MetricExporter {
    client: Client<HttpConnector>,
}

impl MetricExporter {
    pub fn new() -> Self {
        let client = Client::builder()
            .pool_idle_timeout(Duration::from_secs(30))
            .build_http();
        
        Self { client }
    }
    
    /// Export metrics to an HTTP endpoint
    pub async fn export_http(
        &self,
        endpoint: &str,
        data: Vec<u8>,
        content_type: &str,
        headers: &[(String, String)],
    ) -> Result<()> {
        let mut request = Request::builder()
            .method(Method::POST)
            .uri(endpoint)
            .header("Content-Type", content_type)
            .header("Content-Length", data.len().to_string());
        
        // Add custom headers
        for (key, value) in headers {
            request = request.header(key.as_str(), value.as_str());
        }
        
        let request = request
            .body(Body::from(data))
            .map_err(|e| anyhow::anyhow!("Failed to build request: {}", e))?;
        
        debug!("Sending metrics to {}", endpoint);
        
        match self.client.request(request).await {
            Ok(response) => {
                let status = response.status();
                if status.is_success() {
                    info!("Successfully exported metrics to {} (status: {})", endpoint, status);
                    Ok(())
                } else {
                    error!("Failed to export metrics to {} (status: {})", endpoint, status);
                    Err(anyhow::anyhow!("HTTP error: {}", status))
                }
            }
            Err(e) => {
                error!("Failed to send request to {}: {}", endpoint, e);
                Err(anyhow::anyhow!("Request failed: {}", e))
            }
        }
    }
    
    /// Export metrics to a file (useful for debugging)
    pub async fn export_file(
        &self,
        path: &str,
        data: Vec<u8>,
    ) -> Result<()> {
        use tokio::fs;
        
        fs::write(path, data).await?;
        info!("Exported metrics to file: {}", path);
        Ok(())
    }
}

/// Circuit breaker for handling failures
pub struct CircuitBreaker {
    failure_count: usize,
    threshold: usize,
    is_open: bool,
    last_failure: Option<std::time::Instant>,
    reset_timeout: Duration,
}

impl CircuitBreaker {
    pub fn new(threshold: usize, reset_timeout: Duration) -> Self {
        Self {
            failure_count: 0,
            threshold,
            is_open: false,
            last_failure: None,
            reset_timeout,
        }
    }
    
    pub fn is_open(&self) -> bool {
        if self.is_open {
            if let Some(last_failure) = self.last_failure {
                if last_failure.elapsed() > self.reset_timeout {
                    // Try to reset
                    return false;
                }
            }
        }
        self.is_open
    }
    
    pub fn record_success(&mut self) {
        self.failure_count = 0;
        self.is_open = false;
        self.last_failure = None;
    }
    
    pub fn record_failure(&mut self) {
        self.failure_count += 1;
        self.last_failure = Some(std::time::Instant::now());
        
        if self.failure_count >= self.threshold {
            self.is_open = true;
            error!("Circuit breaker opened after {} failures", self.failure_count);
        }
    }
}