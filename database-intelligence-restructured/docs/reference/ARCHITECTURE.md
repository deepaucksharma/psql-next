# Database Intelligence - Architecture Reference

## 🏗️ System Architecture

### Overview
Database Intelligence is a specialized OpenTelemetry Collector distribution designed for comprehensive database monitoring with intelligent analysis capabilities.

```
┌─────────────────────────────────────────────────────────────────┐
│                     Database Intelligence Platform               │
├─────────────────────────────────────────────────────────────────┤
│                         Receivers Layer                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │PostgreSQL│ │  MySQL   │ │ MongoDB  │ │  Redis   │          │
│  │ Receiver │ │ Receiver │ │ Receiver │ │ Receiver │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                       │
│  │   ASH    │ │Enhanced  │ │  Kernel  │                       │
│  │ Receiver │ │   SQL    │ │ Metrics  │                       │
│  └──────────┘ └──────────┘ └──────────┘                       │
├─────────────────────────────────────────────────────────────────┤
│                        Processors Layer                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │ Adaptive │ │ Circuit  │ │  Query   │ │   Cost   │          │
│  │ Sampler  │ │ Breaker  │ │Correlator│ │ Control  │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                       │
│  │Plan Attr │ │  Memory  │ │  Batch   │                       │
│  │Extractor │ │ Limiter  │ │Processor │                       │
│  └──────────┘ └──────────┘ └──────────┘                       │
├─────────────────────────────────────────────────────────────────┤
│                         Exporters Layer                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                       │
│  │   OTLP   │ │   NRI    │ │Prometheus│                       │
│  │ Exporter │ │ Exporter │ │ Exporter │                       │
│  └──────────┘ └──────────┘ └──────────┘                       │
└─────────────────────────────────────────────────────────────────┘
```

## 🎯 Operating Modes

### 1. Config-Only Mode (Production Ready)
Uses standard OpenTelemetry components configured via YAML.

**Components**:
- Standard OTel receivers (postgresql, mysql)
- Standard processors (batch, memory_limiter, resource)
- Standard exporters (otlp, prometheus)

**Characteristics**:
- ✅ Production-ready with any OTel Collector
- 📊 35+ metrics per database type
- 💾 <512MB memory usage
- ⚡ <5% CPU overhead
- 🔧 Simple YAML configuration

### 2. Enhanced Mode (Advanced Features)
Includes custom components for database intelligence.

**Additional Components**:
- **ASH Receiver**: Active Session History for PostgreSQL
- **Enhanced SQL**: Advanced metrics with feature detection
- **Adaptive Sampler**: Load-based sampling adjustment
- **Circuit Breaker**: Overload protection
- **Query Correlator**: Cross-metric correlation
- **Plan Extractor**: Query plan analysis

**Characteristics**:
- 🧠 50+ advanced metrics
- 📈 Query intelligence features
- 💾 <2GB memory usage
- ⚡ <20% CPU overhead
- 🔧 Advanced configuration options

## 📊 Data Flow Architecture

### Standard Pipeline
```
Database → Receiver → Processor → Exporter → Backend
   ↓          ↓           ↓           ↓          ↓
Metrics   Collect    Transform    Buffer    New Relic
```

### Enhanced Pipeline
```
Database → Receiver → Intelligence → Processor → Exporter
   ↓          ↓            ↓            ↓           ↓
Metrics   Collect    Analyze/Enrich  Optimize   New Relic
```

## 🔧 Component Architecture

### Receivers

#### Standard Receivers
- **PostgreSQL Receiver**
  - Connection pooling
  - Multi-database support
  - Custom query execution
  - SSL/TLS support

- **MySQL Receiver**
  - Performance schema integration
  - Replication monitoring
  - InnoDB metrics
  - Custom query support

#### Custom Receivers
- **ASH Receiver**
  - Real-time session sampling
  - Wait event analysis
  - Query fingerprinting
  - Circular buffer storage

- **Enhanced SQL Receiver**
  - Feature detection
  - Version-aware queries
  - Extended metrics
  - Query plan capture

### Processors

#### Intelligence Processors
- **Adaptive Sampler**
  ```go
  // Adjusts sampling based on system load
  if systemLoad > threshold {
      samplingRate = max(minRate, currentRate * 0.9)
  }
  ```

- **Circuit Breaker**
  ```go
  // Protects against cascading failures
  if errorRate > 0.8 || latency > maxLatency {
      circuit.Open()
  }
  ```

- **Query Correlator**
  ```go
  // Adds correlation metadata
  metric.Attributes["query_hash"] = hashQuery(sql)
  metric.Attributes["table"] = extractTable(sql)
  ```

### Exporters

- **OTLP Exporter**
  - Compression support
  - Retry with backoff
  - Connection pooling
  - Header injection

- **NRI Exporter**
  - New Relic specific format
  - Batch optimization
  - Metric aggregation
  - Cost tracking

## 🏗️ Deployment Architecture

### Single Instance
```
┌─────────────────┐
│   Collector     │
│  ┌───────────┐  │
│  │ Config    │  │──→ New Relic
│  │ Components│  │
│  └───────────┘  │
└─────────────────┘
        ↑
    Databases
```

### High Availability
```
┌─────────────────┐     ┌─────────────────┐
│  Collector 1    │     │  Collector 2    │
│   (Primary)     │     │   (Standby)     │
└─────────────────┘     └─────────────────┘
         ↑                       ↑
         └───────────┬───────────┘
                     │
                 Databases
```

### Distributed
```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Agent 1     │    │  Agent 2     │    │  Agent 3     │
│ (PostgreSQL) │    │   (MySQL)    │    │  (MongoDB)   │
└──────────────┘    └──────────────┘    └──────────────┘
        ↓                   ↓                   ↓
┌─────────────────────────────────────────────────────┐
│                  Gateway Collector                   │
│              (Aggregation & Routing)                │
└─────────────────────────────────────────────────────┘
                           ↓
                      New Relic

```

## 🔒 Security Architecture

### Authentication & Authorization
- Database credentials via environment variables
- New Relic API key protection
- TLS/SSL for all connections
- Role-based access control (RBAC)

### Data Protection
- Sensitive data redaction
- Query parameter masking
- Encryption in transit
- No persistent storage

### Network Security
```
Database ←[TLS]→ Collector ←[HTTPS/TLS]→ New Relic
```

## 📈 Performance Characteristics

### Resource Usage by Mode

| Mode | CPU | Memory | Network | Latency |
|------|-----|--------|---------|---------|
| Config-Only | <5% | <512MB | Low | <5ms |
| Enhanced | <20% | <2GB | Medium | <10ms |
| Distributed | <10% | <1GB | High | <15ms |

### Scalability Patterns

#### Vertical Scaling
- Increase memory for larger buffers
- More CPU cores for parallel processing
- NVMe storage for temporary data

#### Horizontal Scaling
- Multiple collectors per database cluster
- Load balancing across instances
- Sharding by database or schema

## 🔄 Lifecycle Management

### Configuration Hot Reload
```yaml
# Supports live configuration updates
extensions:
  configreload:
    interval: 30s
```

### Health Monitoring
```yaml
# Health check endpoints
extensions:
  health_check:
    endpoint: localhost:13133
```

### Graceful Shutdown
- Flush all buffers
- Complete in-flight requests
- Close database connections
- Save state if applicable

## 🎯 Design Principles

1. **Zero Persistence**
   - All state in memory
   - No local storage dependencies
   - Clean restart capability

2. **Defense in Depth**
   - Multiple protection layers
   - Graceful degradation
   - Circuit breakers everywhere

3. **OpenTelemetry First**
   - Standard components preferred
   - Custom only when necessary
   - Ecosystem compatibility

4. **Performance Focused**
   - Minimal overhead
   - Efficient resource usage
   - Adaptive behavior

## 🔗 Integration Points

### Database Integration
- Native driver connections
- Connection pooling
- Prepared statements
- Batch operations

### Backend Integration
- OTLP protocol support
- Vendor-specific formats
- Compression options
- Retry mechanisms

### Monitoring Integration
- Prometheus metrics
- Health endpoints
- Logging framework
- Trace correlation

---

For implementation details, see [Development Guide](../development/SETUP.md).  
For configuration options, see [Configuration Reference](../guides/CONFIGURATION.md).