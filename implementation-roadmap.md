# PostgreSQL Query Lens - Implementation Roadmap

## Executive Summary

This roadmap outlines the implementation plan for PostgreSQL Query Lens (pgquerylens), a unified collector that supersets all nri-postgresql QPM metrics while adding advanced capabilities including live plan capture, kernel-level metrics, and Active Session History.

## Phase 1: Foundation (Q1 2025)

### 1.1 Core Module Development (Weeks 1-4)

**Objective**: Establish the modular Go architecture

```yaml
deliverables:
  - module: core/model
    tasks:
      - Define Common Plan Model (CPM) structures
      - Implement UnifiedMetrics interface
      - Create metric conversion utilities
    
  - module: core/pgsql
    tasks:
      - PostgreSQL connection management
      - Capability detection system
      - Version compatibility layer
    
  - module: config
    tasks:
      - Unified configuration parser (YAML/TOML/HCL)
      - Environment variable support
      - Validation framework

milestones:
  - week_2: Basic module structure and interfaces
  - week_4: Unit tests achieving 80% coverage
```

### 1.2 OHI Compatibility Layer (Weeks 5-8)

**Objective**: Ensure 100% backward compatibility with nri-postgresql

```yaml
deliverables:
  - compatibility_tests:
      - Port all OHI test cases
      - Metric name preservation
      - Query text anonymization
      - Parameter validation matching
      
  - nri_adapter:
      - JSON output formatting
      - Entity key generation
      - Event type mapping
      - Integration protocol v4 support
      
validation:
  - Run parallel with existing OHI
  - Compare outputs byte-for-byte
  - Performance benchmarking
```

### 1.3 PostgreSQL Extension Development (Weeks 9-12)

**Objective**: Create pg_querylens extension for high-performance metric collection

```c
// Key features to implement
features:
  - shared_memory_ring:
      size: "32MB default"
      type: "lock-free ring buffer"
      
  - executor_hooks:
      - ExecutorStart: capture plan
      - ExecutorRun: track progress
      - ExecutorEnd: collect metrics
      
  - plan_dictionary:
      - In-memory plan cache
      - Plan hash calculation
      - Regression detection
```

## Phase 2: Enhanced Metrics (Q2 2025)

### 2.1 eBPF Integration (Weeks 13-16)

**Objective**: Add kernel-level metrics without modifying PostgreSQL

```yaml
ebpf_programs:
  - query_latency:
      probes:
        - uprobe: exec_simple_query
        - kprobe: tcp_sendmsg
      metrics:
        - cpu_time_ns
        - io_wait_ns
        - context_switches
        
  - wait_analysis:
      probes:
        - tracepoint: sched_switch
        - tracepoint: block_rq_complete
      metrics:
        - scheduler_delay
        - block_io_delay
        
security:
  - CO-RE compatible
  - Minimal kernel version: 5.4
  - Graceful fallback
```

### 2.2 Active Session History (Weeks 17-20)

**Objective**: Implement 1-second resolution session sampling

```yaml
ash_implementation:
  - sampler:
      interval: "1s"
      retention: "1h default"
      storage: "ring buffer"
      
  - enrichment:
      - Query ID correlation
      - Wait event tracking
      - CPU state from eBPF
      - Blocking session graph
      
  - aggregation:
      - Top wait events
      - Session state timeline
      - Resource consumption
```

### 2.3 Live Plan Capture (Weeks 21-24)

**Objective**: Automatic execution plan collection

```yaml
plan_capture:
  - strategies:
      extension_mode:
        - Hook-based capture
        - Zero-overhead
        - Every execution
        
      fallback_mode:
        - EXPLAIN sampling
        - Top-N queries
        - Timeout protection
        
  - storage:
      - Plan dictionary table
      - Deduplication by hash
      - Change detection
      
  - analysis:
      - Cost tracking
      - Regression detection
      - Plan stability metrics
```

## Phase 3: Export Integration (Q3 2025)

### 3.1 OpenTelemetry Support (Weeks 25-28)

**Objective**: Full OTLP compliance

```yaml
otel_integration:
  - receiver_mode:
      - gRPC endpoint: 4317
      - HTTP endpoint: 4318
      - Batching support
      
  - semantic_conventions:
      - db.* attributes
      - Resource detection
      - Instrumentation scope
      
  - signals:
      - metrics: histograms, gauges, counters
      - traces: query spans with plans
      - logs: plan changes, regressions
```

### 3.2 Dual Export Mode (Weeks 29-30)

**Objective**: Simultaneous NRI and OTLP output

```yaml
dual_mode:
  - implementation:
      - Parallel export pipelines
      - Independent buffering
      - Failure isolation
      
  - configuration:
      - Per-exporter settings
      - Selective metric routing
      - Format transformation
```

## Phase 4: Production Readiness (Q4 2025)

### 4.1 Performance Optimization (Weeks 31-34)

**Objective**: Minimize overhead

```yaml
optimization_targets:
  - extension_overhead: "< 1μs per query"
  - collector_memory: "< 256MB typical"
  - cpu_usage: "< 2% of core"
  
techniques:
  - adaptive_sampling:
      - Dynamic rate adjustment
      - Priority-based collection
      - Resource-aware throttling
      
  - efficient_aggregation:
      - Streaming algorithms
      - Memory-bounded structures
      - Lock-free data structures
```

### 4.2 Cloud Provider Support (Weeks 35-38)

**Objective**: Optimize for managed PostgreSQL services

```yaml
cloud_support:
  aws_rds:
    - No-extension mode
    - CloudWatch integration
    - IAM authentication
    
  google_cloud_sql:
    - Cloud SQL proxy support
    - Cloud Monitoring export
    
  azure_database:
    - Managed identity auth
    - Azure Monitor integration
    
features:
  - Auto-detection of environment
  - Graceful degradation
  - Cloud-specific optimizations
```

### 4.3 Kubernetes Operator (Weeks 39-42)

**Objective**: Native Kubernetes integration

```yaml
operator_features:
  - crd: PostgreSQLMonitor
  - capabilities:
      - Auto-discovery
      - Configuration management
      - Rolling updates
      - Multi-cluster support
      
  - integrations:
      - Prometheus ServiceMonitor
      - Grafana dashboards
      - Alert rules
```

## Phase 5: GA Release (Q1 2026)

### 5.1 Documentation & Training (Weeks 43-46)

```yaml
documentation:
  - user_guide:
      - Installation procedures
      - Configuration reference
      - Troubleshooting guide
      
  - migration_guide:
      - From nri-postgresql
      - From pg_stat_monitor
      - From custom solutions
      
  - api_reference:
      - Metric definitions
      - Extension SQL functions
      - eBPF programs
```

### 5.2 Testing & Certification (Weeks 47-50)

```yaml
testing:
  - compatibility:
      - PostgreSQL 12-16
      - All major Linux distros
      - Kubernetes 1.24+
      
  - performance:
      - Load testing
      - Long-term stability
      - Resource consumption
      
  - security:
      - Penetration testing
      - CVE scanning
      - Compliance validation
```

### 5.3 Launch Activities (Weeks 51-52)

```yaml
launch:
  - releases:
      - Binary packages (RPM, DEB)
      - Container images
      - Helm charts
      - Terraform modules
      
  - announcements:
      - Blog posts
      - Documentation
      - Migration tools
      - Support channels
```

## Implementation Principles

### Technical Decisions

```yaml
principles:
  - single_binary: "One binary, multiple modes"
  - zero_breaking_changes: "100% backward compatible"
  - performance_first: "< 1% overhead target"
  - cloud_native: "Kubernetes-first design"
  - vendor_neutral: "Works with any OTLP backend"
```

### Success Metrics

```yaml
metrics:
  adoption:
    - target: "1000 deployments in 6 months"
    - measurement: "Telemetry opt-in"
    
  performance:
    - query_overhead: "< 1μs p99"
    - memory_usage: "< 256MB p95"
    
  compatibility:
    - ohi_parity: "100% metric coverage"
    - dashboard_compatibility: "Zero changes required"
    
  reliability:
    - uptime: "99.9% collector availability"
    - data_loss: "< 0.01% metric loss"
```

## Risk Mitigation

### Technical Risks

```yaml
risks:
  - risk: "Extension installation barriers"
    mitigation: "Robust fallback mode"
    
  - risk: "eBPF kernel compatibility"
    mitigation: "Graceful degradation"
    
  - risk: "Performance regression"
    mitigation: "Extensive benchmarking"
    
  - risk: "Breaking changes"
    mitigation: "Comprehensive testing"
```

### Rollback Plan

```yaml
rollback:
  - feature_flags: "Disable new features remotely"
  - version_pinning: "Support specific versions"
  - parallel_running: "Run old and new side-by-side"
  - quick_revert: "< 5 minute rollback time"
```

## Resource Requirements

### Team Structure

```yaml
team:
  - engineering:
      senior_engineers: 3
      engineers: 4
      sre: 2
      
  - product:
      product_manager: 1
      technical_writer: 1
      
  - support:
      solutions_architect: 2
      support_engineers: 2
```

### Infrastructure

```yaml
infrastructure:
  - development:
      - CI/CD pipeline
      - Test PostgreSQL clusters
      - Performance lab
      
  - production:
      - Package repositories
      - Container registries
      - Documentation hosting
```

## Conclusion

This roadmap provides a structured path from the current OHI implementation to a comprehensive, vendor-neutral PostgreSQL monitoring solution. By maintaining 100% backward compatibility while adding significant new capabilities, pgquerylens will provide a smooth migration path for existing users while attracting new adopters with its advanced features.

The phased approach ensures that each component is thoroughly tested before moving to the next, minimizing risk while maximizing the value delivered at each stage.