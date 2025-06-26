use async_trait::async_trait;
use postgres_collector_core::{UnifiedMetrics, ProcessError};
use postgres_nri_adapter::{NRIAdapter, NRIOutput};
use postgres_otel_adapter::{OTelAdapter, OTelOutput};

use crate::collection_engine::{MetricAdapter, MetricOutput};

/// NRI Adapter wrapper
pub struct NRIMetricAdapter {
    inner: NRIAdapter,
}

impl NRIMetricAdapter {
    pub fn new(entity_key: String) -> Self {
        Self {
            inner: NRIAdapter::new(entity_key),
        }
    }
}

#[async_trait]
impl MetricAdapter for NRIMetricAdapter {
    type Output = NRIOutput;
    
    async fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, ProcessError> {
        self.inner.adapt(metrics)
    }
}

/// OTel Adapter wrapper
pub struct OTelMetricAdapter {
    inner: OTelAdapter,
}

impl OTelMetricAdapter {
    pub fn new(endpoint: String) -> Self {
        Self {
            inner: OTelAdapter::new(endpoint),
        }
    }
}

#[async_trait]
impl MetricAdapter for OTelMetricAdapter {
    type Output = OTelOutput;
    
    async fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, ProcessError> {
        self.inner.adapt(metrics)
    }
}