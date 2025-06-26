# PostgreSQL Unified Collector - Architecture Overview

## Table of Contents
1. [Introduction](#introduction)
2. [System Architecture](#system-architecture)
3. [Core Components](#core-components)
4. [Data Flow](#data-flow)
5. [Design Principles](#design-principles)
6. [Technology Stack](#technology-stack)

## Introduction

The PostgreSQL Unified Collector is a comprehensive monitoring solution that combines the proven reliability of New Relic's OHI (On-Host Integration) with modern observability standards. It provides a single binary that can operate in multiple modes while maintaining 100% backward compatibility with existing nri-postgresql deployments.

### Key Features
- **Single Binary, Multiple Modes**: One executable supports NRI, OpenTelemetry, and hybrid outputs
- **100% OHI Compatibility**: Drop-in replacement for nri-postgresql with identical metrics
- **Extended Metrics**: Adds histogram support, Active Session History, execution plans, and kernel-level metrics
- **Cloud-Native**: First-class support for Kubernetes, RDS, Cloud SQL, and containerized deployments
- **Performance Optimized**: <1% overhead with adaptive sampling and efficient aggregation

### Reference Architecture Alignment
This implementation follows the reference-grade architecture that supersets every metric from nri-postgresql QPM while adding:
- Plan/wait-state/OS-delay/ASH depth
- Live execution plan capture
- Kernel-level metrics via eBPF
- Plan regression detection

## System Architecture

```
┌─────────────────────── PostgreSQL Unified Collector ──────────────────────┐
│                                                                           │
│  ┌─────────── Input Layer ───────────┐  ┌──────── Output Layer ────────┐ │
│  │                                   │  │                               │ │
│  │  PostgreSQL Connection            │  │  NRI Adapter                  │ │
│  │  ├─ pg_stat_statements           │  │  ├─ JSON v4 Protocol         │ │
│  │  ├─ pg_wait_sampling             │  │  └─ Infrastructure Agent      │ │
│  │  ├─ pg_stat_monitor              │  │                               │ │
│  │  └─ pg_querylens (future)        │  │  OTLP Adapter                 │ │
│  │                                   │  │  ├─ Metrics                   │ │
│  │  eBPF Probes (optional)          │  │  ├─ Traces                    │ │
│  │  └─ Kernel metrics               │  │  └─ Logs                      │ │
│  │                                   │  │                               │ │
│  │  Active Session History          │  │  Prometheus (future)          │ │
│  │  └─ 1-second sampling            │  │  └─ /metrics endpoint         │ │
│  └───────────────────────────────────┘  └───────────────────────────────┘ │
│                                                                           │
│  ┌────────────────────── Core Engine ─────────────────────────────────┐  │
│  │                                                                    │  │
│  │  Collection Engine          Aggregation           Export Manager   │  │
│  │  ├─ Query Executor    →    ├─ Time Bucketing  →  ├─ Mode Detection│  │
│  │  ├─ Extension Manager       ├─ Adaptive Sample    ├─ Dual Export  │  │
│  │  └─ Capability Detect       └─ Metric Enrichment  └─ Batching     │  │
│  │                                                                    │  │
│  └────────────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Collection Engine (`crates/core/`)
The heart of the system, responsible for:
- **Capability Detection**: Automatically detects PostgreSQL version, extensions, and environment
- **Query Execution**: Maintains OHI-compatible queries with version-specific variations
- **Metric Collection**: Gathers all metrics with minimal overhead
- **RDS Compatibility**: Falls back gracefully when extensions are unavailable

### 2. Unified Metrics Model
```rust
pub struct UnifiedMetrics {
    // OHI-compatible base metrics
    pub slow_queries: Vec<SlowQueryMetric>,
    pub wait_events: Vec<WaitEventMetric>,
    pub blocking_sessions: Vec<BlockingSessionMetric>,
    pub individual_queries: Vec<IndividualQueryMetric>,
    pub execution_plans: Vec<ExecutionPlanMetric>,
    
    // Extended metrics (optional)
    pub per_execution_traces: Option<Vec<ExecutionTrace>>,
    pub kernel_metrics: Option<Vec<KernelMetric>>,
    pub active_session_history: Option<Vec<ASHSample>>,
    pub plan_changes: Option<Vec<PlanChangeEvent>>,
    
    pub collection_metadata: CollectionMetadata,
}
```

### 3. Adapter Pattern
Clean separation between collection and output formatting:

```rust
#[async_trait]
pub trait MetricAdapter: Send + Sync {
    async fn export(&self, metrics: &UnifiedMetrics) -> Result<Vec<u8>, Box<dyn Error>>;
    fn content_type(&self) -> &'static str;
    fn name(&self) -> &str;
}
```

### 4. Extension Management
Dynamic detection and configuration of PostgreSQL extensions:
- **pg_stat_statements**: Query performance metrics (required for OHI compatibility)
- **pg_wait_sampling**: Enhanced wait event tracking
- **pg_stat_monitor**: Individual query correlation
- **pg_querylens**: Future shared-memory ring buffer integration

### 5. Active Session History (ASH)
Provides Oracle-like session sampling:
- 1-second resolution sampling
- Wait event tracking
- CPU state correlation (when eBPF is available)
- Configurable retention period

### 6. eBPF Integration (Optional)
Kernel-level metrics without modifying PostgreSQL:
- CPU vs I/O time breakdown
- Context switch tracking
- Scheduler delay measurement
- System call analysis

## Data Flow

### 1. Collection Phase
```
PostgreSQL → SQL Queries → Collection Engine → Raw Metrics
     ↓                                              ↓
Extensions → Capability Detection → Version-Specific Queries
     ↓                                              ↓
eBPF Probes → Kernel Events → Enrichment → Enhanced Metrics
```

### 2. Processing Phase
```
Raw Metrics → Validation → Aggregation → Sampling → Unified Metrics
                  ↓            ↓            ↓
              OHI Rules    Time Buckets  Adaptive Rules
```

### 3. Export Phase
```
Unified Metrics → Export Manager → Parallel Export
                        ↓                ↓
                  NRI Adapter      OTLP Adapter
                        ↓                ↓
                 Infrastructure    OpenTelemetry
                     Agent           Collector
```

## Design Principles

### 1. Backward Compatibility First
- Maintains exact OHI metric names and schemas
- Preserves all existing dashboards and alerts
- Drop-in replacement requires no configuration changes

### 2. Performance by Design
- Target: <1% overhead on PostgreSQL
- Adaptive sampling reduces data volume
- Efficient aggregation algorithms
- Connection pooling and query batching

### 3. Cloud-Native Architecture
- Kubernetes-first design
- Service discovery support
- Horizontal scaling ready
- Cloud provider optimizations

### 4. Extensibility
- Pluggable adapter pattern
- Feature flags for optional components
- Clean interfaces for future extensions
- Vendor-neutral core

### 5. Operational Excellence
- Comprehensive logging and self-monitoring
- Graceful degradation
- Circuit breakers for external dependencies
- Health check endpoints

## Technology Stack

### Core Technologies
- **Language**: Rust 1.70+
  - Chosen for performance, safety, and reliability
  - Async/await with Tokio runtime
  - Zero-cost abstractions

- **Database**: PostgreSQL 12+
  - Full support for versions 12-16
  - Extension-aware architecture
  - Prepared statement caching

- **Serialization**: 
  - Serde for data structures
  - Protocol Buffers for future SHM integration
  - JSON for NRI compatibility

### Dependencies
```toml
[dependencies]
tokio = { version = "1", features = ["full"] }
sqlx = { version = "0.7", features = ["postgres", "runtime-tokio-native-tls"] }
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
opentelemetry = "0.20"
opentelemetry-otlp = "0.13"
tracing = "0.1"
anyhow = "1.0"
```

### Optional Features
```toml
[features]
default = ["otlp", "nri"]
ebpf = ["aya", "aya-bpf"]
extended-metrics = []
```

## Performance Characteristics

### Resource Usage
- **Memory**: 128-256MB typical, 512MB maximum
- **CPU**: <2% of one core under normal load
- **Network**: Configurable batching, typically <1MB/min
- **Disk**: Minimal, only for configuration and logs

### Scalability
- **Databases**: Tested up to 100 databases per instance
- **Queries**: Handles 10,000+ unique queries
- **Connections**: Efficient pooling, typically 2-5 connections
- **Metrics**: Adaptive sampling prevents overload

## Security Considerations

### Authentication
- PostgreSQL native authentication
- SSL/TLS support with certificate validation
- AWS IAM authentication for RDS
- Kubernetes service account integration

### Authorization
- Minimal privileges required (pg_monitor role)
- No superuser access needed
- Read-only operations only
- Extension creation optional

### Data Protection
- Query text anonymization
- No sensitive data in metrics
- Configurable literal suppression
- Audit trail support

## Next Steps

For detailed implementation guidance, see:
- [Implementation Guide](02-implementation-guide.md) - Building and extending the collector
- [Deployment Guide](03-deployment-operations.md) - Installation and configuration
- [Metrics Reference](04-metrics-reference.md) - Complete metric documentation
- [Migration Guide](05-migration-guide.md) - Upgrading from nri-postgresql