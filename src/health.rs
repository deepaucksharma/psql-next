use hyper::{Body, Request, Response, Server, StatusCode};
use hyper::service::{make_service_fn, service_fn};
use std::convert::Infallible;
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::sync::RwLock;
use tracing::{info, error};

#[derive(Clone)]
pub struct HealthStatus {
    pub is_healthy: bool,
    pub last_collection_time: Option<chrono::DateTime<chrono::Utc>>,
    pub last_collection_error: Option<String>,
    pub metrics_sent: u64,
    pub metrics_failed: u64,
}

impl Default for HealthStatus {
    fn default() -> Self {
        Self {
            is_healthy: true,
            last_collection_time: None,
            last_collection_error: None,
            metrics_sent: 0,
            metrics_failed: 0,
        }
    }
}

pub struct HealthServer {
    status: Arc<RwLock<HealthStatus>>,
}

impl HealthServer {
    pub fn new(status: Arc<RwLock<HealthStatus>>) -> Self {
        Self { status }
    }
    
    pub async fn start(self, addr: SocketAddr) -> Result<(), hyper::Error> {
        let status = self.status.clone();
        
        let make_svc = make_service_fn(move |_conn| {
            let status = status.clone();
            async move {
                Ok::<_, Infallible>(service_fn(move |req| {
                    handle_request(req, status.clone())
                }))
            }
        });
        
        let server = Server::bind(&addr).serve(make_svc);
        info!("Health check server listening on {}", addr);
        
        server.await
    }
}

async fn handle_request(
    req: Request<Body>,
    status: Arc<RwLock<HealthStatus>>,
) -> Result<Response<Body>, Infallible> {
    let response = match req.uri().path() {
        "/health" => health_check(status).await,
        "/ready" => readiness_check(status).await,
        "/metrics" => prometheus_metrics(status).await,
        _ => not_found(),
    };
    
    Ok(response)
}

async fn health_check(status: Arc<RwLock<HealthStatus>>) -> Response<Body> {
    let status = status.read().await;
    
    if status.is_healthy {
        Response::builder()
            .status(StatusCode::OK)
            .body(Body::from(serde_json::json!({
                "status": "healthy",
                "last_collection": status.last_collection_time,
                "metrics_sent": status.metrics_sent,
                "metrics_failed": status.metrics_failed,
            }).to_string()))
            .unwrap()
    } else {
        Response::builder()
            .status(StatusCode::SERVICE_UNAVAILABLE)
            .body(Body::from(serde_json::json!({
                "status": "unhealthy",
                "error": status.last_collection_error,
                "last_collection": status.last_collection_time,
            }).to_string()))
            .unwrap()
    }
}

async fn readiness_check(status: Arc<RwLock<HealthStatus>>) -> Response<Body> {
    let status = status.read().await;
    
    // Consider ready if we've had at least one successful collection
    if status.last_collection_time.is_some() && status.is_healthy {
        Response::builder()
            .status(StatusCode::OK)
            .body(Body::from("ready"))
            .unwrap()
    } else {
        Response::builder()
            .status(StatusCode::SERVICE_UNAVAILABLE)
            .body(Body::from("not ready"))
            .unwrap()
    }
}

async fn prometheus_metrics(status: Arc<RwLock<HealthStatus>>) -> Response<Body> {
    let status = status.read().await;
    
    let metrics = format!(
        "# HELP postgres_collector_up Whether the collector is up and running\n\
         # TYPE postgres_collector_up gauge\n\
         postgres_collector_up {}\n\
         # HELP postgres_collector_metrics_sent_total Total number of metrics successfully sent\n\
         # TYPE postgres_collector_metrics_sent_total counter\n\
         postgres_collector_metrics_sent_total {}\n\
         # HELP postgres_collector_metrics_failed_total Total number of failed metric sends\n\
         # TYPE postgres_collector_metrics_failed_total counter\n\
         postgres_collector_metrics_failed_total {}\n",
        if status.is_healthy { 1 } else { 0 },
        status.metrics_sent,
        status.metrics_failed
    );
    
    Response::builder()
        .status(StatusCode::OK)
        .header("Content-Type", "text/plain; version=0.0.4")
        .body(Body::from(metrics))
        .unwrap()
}

fn not_found() -> Response<Body> {
    Response::builder()
        .status(StatusCode::NOT_FOUND)
        .body(Body::from("404 - Not Found"))
        .unwrap()
}