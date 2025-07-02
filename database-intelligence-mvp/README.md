# Database Intelligence Collector

A production-ready OpenTelemetry-based monitoring solution for PostgreSQL and MySQL databases, providing advanced observability with execution plan analysis, Active Session History (ASH), query performance intelligence, and automated regression detection.

## 🚀 Overview

The Database Intelligence Collector extends the OpenTelemetry Collector with specialized processors and receivers for deep database monitoring. It provides enterprise-grade database observability while maintaining the flexibility and openness of OpenTelemetry.

## ✨ Key Features

### 🔍 Plan Intelligence
- **Safe Plan Collection**: Analyzes execution plans from existing data without query overhead
- **pg_querylens Integration**: Advanced plan tracking and regression detection
- **Plan Anonymization**: Multi-layer PII protection with query fingerprinting
- **Regression Detection**: Automatic identification of performance degradations
- **Optimization Recommendations**: AI-driven suggestions for query improvements

### 📊 Active Session History (ASH)
- **High-Frequency Sampling**: 1-second resolution session monitoring
- **Wait Event Analysis**: Comprehensive categorization of database waits
- **Blocking Detection**: Real-time identification of blocking chains
- **Resource Attribution**: CPU, memory, and I/O tracking per session

### 🛡️ Enterprise-Grade Reliability
- **Circuit Breakers**: Prevents cascade failures with per-database protection
- **Adaptive Sampling**: Intelligent data reduction based on system load
- **Cost Control**: Budget-aware data collection with automatic throttling
- **Error Monitoring**: Proactive detection of integration issues

### 🎯 Advanced Processing
- **Query Correlation**: Links related queries and transactions
- **Workload Classification**: Automatic OLTP vs OLAP detection
- **Anomaly Detection**: Statistical analysis for outlier identification
- **Performance Baselines**: Dynamic threshold adjustment

## 📋 Prerequisites

- Go 1.21 or higher
- PostgreSQL 12+ or MySQL 8.0+
- New Relic account with OTLP ingest enabled
- Docker (optional, for containerized deployment)
- Kubernetes 1.21+ (optional, for K8s deployment)

## 🚀 Quick Start

### Option 1: Binary Installation

```bash
# Download the latest release
curl -Lo database-intelligence-collector https://github.com/database-intelligence-mvp/releases/latest/download/database-intelligence-collector-linux-amd64
chmod +x database-intelligence-collector

# Create configuration
cat > config.yaml <<EOF
receivers:
  postgresql:
    endpoint: localhost:5432
    username: monitoring
    password: ${POSTGRES_PASSWORD}
    databases: [postgres]

processors:
  batch:
    timeout: 10s

exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp]
EOF

# Run the collector
./database-intelligence-collector --config=config.yaml
```

### Option 2: Docker

```bash
docker run -d \
  --name database-intelligence \
  -e POSTGRES_HOST=postgres.example.com \
  -e POSTGRES_USER=monitoring \
  -e POSTGRES_PASSWORD=secret \
  -e NEW_RELIC_LICENSE_KEY=your-key \
  -v $(pwd)/config.yaml:/etc/otel/config.yaml \
  ghcr.io/database-intelligence-mvp/database-intelligence-collector:latest
```

### Option 3: Kubernetes with Helm

```bash
helm repo add database-intelligence https://database-intelligence-mvp.github.io/helm-charts
helm install my-collector database-intelligence/database-intelligence \
  --set config.postgres.endpoint=postgres.example.com \
  --set config.newrelic.licenseKey=your-key
```

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    PostgreSQL/MySQL Database                     │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐    │
│  │ pg_stat_*   │  │ pg_querylens │  │ performance_schema │    │
│  └─────────────┘  └──────────────┘  └────────────────────┘    │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────┴────────────────────────────────────┐
│                        Receiver Layer                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐    │
│  │ PostgreSQL  │  │   SQLQuery   │  │       MySQL        │    │
│  │  Receiver   │  │   Receiver   │  │     Receiver       │    │
│  └─────────────┘  └──────────────┘  └────────────────────┘    │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────┴────────────────────────────────────┐
│                      Processing Pipeline                         │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐    │
│  │   Memory    │  │   Adaptive   │  │     Circuit        │    │
│  │   Limiter   │→ │   Sampler    │→ │     Breaker        │    │
│  └─────────────┘  └──────────────┘  └────────────────────┘    │
│                           ↓                                      │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐    │
│  │Plan Extract │  │ Verification │  │   Cost Control     │    │
│  │ (QueryLens) │→ │  Processor   │→ │    Processor       │    │
│  └─────────────┘  └──────────────┘  └────────────────────┘    │
│                           ↓                                      │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐    │
│  │    Query    │  │  Transform   │  │      Batch         │    │
│  │ Correlator  │→ │  Processor   │→ │    Processor       │    │
│  └─────────────┘  └──────────────┘  └────────────────────┘    │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────┴────────────────────────────────────┐
│                        Export Layer                              │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐    │
│  │    OTLP     │  │  Prometheus  │  │      Debug         │    │
│  │  Exporter   │  │   Exporter   │  │    Exporter        │    │
│  └─────────────┘  └──────────────┘  └────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

## 🔧 Configuration

### Basic Configuration

```yaml
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:5432
    transport: tcp
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    databases:
      - ${POSTGRES_DB}
    collection_interval: 10s

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000

exporters:
  otlp:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp]
```

### Advanced Configuration with All Features

```yaml
receivers:
  # PostgreSQL native metrics
  postgresql:
    endpoint: ${POSTGRES_HOST}:5432
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    databases: ["*"]
    collection_interval: 10s
    
  # pg_querylens integration for plan intelligence
  sqlquery:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=5432 user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=postgres"
    queries:
      - sql: |
          SELECT * FROM pg_querylens.current_plans 
          WHERE last_execution > NOW() - INTERVAL '5 minutes'
        collection_interval: 30s

processors:
  # Memory protection
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
    
  # Intelligent sampling
  adaptivesampler:
    in_memory_only: true
    default_sampling_rate: 0.1
    rules:
      - name: slow_queries
        expression: 'metrics["query.duration_ms"] > 1000'
        sample_rate: 1.0
        
  # Plan intelligence with pg_querylens
  planattributeextractor:
    safe_mode: true
    querylens:
      enabled: true
      regression_detection:
        enabled: true
        time_increase: 1.5
        
  # Production protection
  circuitbreaker:
    failure_threshold: 0.5
    timeout: 30s
    
  # Data quality
  verification:
    pii_detection:
      enabled: true
      action: redact
      
  # Cost management
  costcontrol:
    monthly_budget_usd: 100
    pricing_tier: standard
    
  # Error monitoring
  nrerrormonitor:
    enabled: true
    alert_threshold: 0.1

exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    compression: gzip

service:
  pipelines:
    metrics:
      receivers: [postgresql, sqlquery]
      processors: [memory_limiter, adaptivesampler, planattributeextractor, 
                   circuitbreaker, verification, costcontrol, nrerrormonitor, batch]
      exporters: [otlp]
```

## 📊 Custom Processors

### 1. Adaptive Sampler
Intelligently reduces data volume while preserving important events:
- Rule-based sampling with CEL expressions
- Priority-based sampling for anomalies
- Automatic adjustment based on load

### 2. Plan Attribute Extractor
Extracts intelligence from query execution plans:
- pg_querylens integration for advanced plan tracking
- Automatic regression detection
- Query fingerprinting and anonymization
- Optimization recommendations

### 3. Circuit Breaker
Protects databases from monitoring overhead:
- Per-database circuit breakers
- Automatic recovery with exponential backoff
- Integration with New Relic error tracking

### 4. Verification Processor
Ensures data quality and compliance:
- PII detection and redaction
- Cardinality explosion prevention
- Data validation and sanitization

### 5. Cost Control Processor
Manages monitoring costs:
- Monthly budget enforcement
- Intelligent data reduction when over budget
- Support for standard and Data Plus pricing

### 6. NR Error Monitor
Proactive error detection:
- Pattern matching for integration errors
- Semantic convention validation
- Alert generation before data rejection

## 📈 Dashboards

The project includes pre-built New Relic dashboards:

1. **PostgreSQL Overview** - System health and performance metrics
2. **Query Intelligence** - Query performance and plan analysis
3. **Active Session History** - Real-time session monitoring
4. **Plan Regression Analysis** - Automatic regression detection
5. **Resource Utilization** - CPU, memory, and I/O tracking

Import dashboards from the `dashboards/` directory.

## 🧪 Testing

```bash
# Run all tests
make test

# Run specific test suites
make test-unit          # Unit tests only
make test-integration   # Integration tests
make test-e2e          # End-to-end tests
make test-performance  # Performance benchmarks

# Run with coverage
make test-coverage
```

## 🚀 Deployment

### Docker Compose
```bash
cd deployments/docker
docker-compose up -d
```

### Kubernetes
```bash
kubectl apply -f deployments/kubernetes/
```

### Helm
```bash
helm install database-intelligence ./deployments/helm/database-intelligence
```

See the [Production Deployment Guide](docs/production-deployment-guide.md) for detailed instructions.

## 📚 Documentation

- [Architecture Overview](docs/ARCHITECTURE.md)
- [Configuration Guide](docs/CONFIGURATION.md)
- [pg_querylens Integration](docs/PG_QUERYLENS_INTEGRATION.md)
- [Production Deployment](docs/production-deployment-guide.md)
- [Troubleshooting Guide](docs/TROUBLESHOOTING.md)
- [Performance Optimization](docs/PERFORMANCE_OPTIMIZATION.md)

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Built on top of:
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [OpenTelemetry Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib)
- [pg_querylens](https://github.com/pgexperts/pg_querylens) for advanced PostgreSQL plan tracking

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/database-intelligence-mvp/database-intelligence-collector/issues)
- **Discussions**: [GitHub Discussions](https://github.com/database-intelligence-mvp/database-intelligence-collector/discussions)
- **Community**: [New Relic Explorers Hub](https://discuss.newrelic.com)

## 🚦 Status

![Build Status](https://github.com/database-intelligence-mvp/database-intelligence-collector/workflows/CI/badge.svg)
![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)
![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)
![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-v0.128.0-orange.svg)