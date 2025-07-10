# Database Intelligence - Unified Testing & Operations Guide

This guide provides a **single, consolidated way** to run and test everything in the Database Intelligence project together.

## 🚀 Quick Start (One Command)

```bash
# Complete setup, build, and test everything
make all
```

Or for a quick demonstration:

```bash
# Start complete demo environment
make demo
```

## 📋 Unified Configuration

All configurations are consolidated into a single, comprehensive file:

- **Main Config**: `configs/unified/database-intelligence-complete.yaml`
- **Environment**: `configs/unified/environment-template.env` 
- **Docker Compose**: `docker-compose.unified.yml`
- **Makefile**: `Makefile.unified`

## 🎯 Unified Operations

### Setup & Initialization
```bash
make setup          # Initialize environment and create .env file
make deps           # Install and update all dependencies
```

### Build Everything
```bash
make build-all      # Build all distributions (minimal, standard, enterprise)
make docker-build   # Build all Docker images
```

### Run Everything
```bash
# Local execution
make run-enterprise # Run enterprise distribution locally

# Docker execution (complete system)
make docker-run     # Start PostgreSQL, MySQL, Collector, Prometheus, Grafana
```

### Test Everything
```bash
make test-all       # Run ALL tests: unit, integration, E2E
make test-e2e       # End-to-end tests with real databases
make test-load      # Load testing with traffic generation
```

### Comprehensive Testing
```bash
# Run the complete test suite (everything!)
./tools/scripts/test/run-comprehensive-tests.sh
```

## 🏗️ Complete System Architecture

The unified configuration runs **everything together**:

```
┌─────────────────┬─────────────────┬─────────────────┐
│   DATABASES     │    COLLECTOR    │   MONITORING    │
├─────────────────┼─────────────────┼─────────────────┤
│ PostgreSQL:5432 │ OTLP:4317/4318  │ Prometheus:9090 │
│ MySQL:3306      │ Health:13133    │ Grafana:3000    │
│ Test Data ✓     │ Metrics:8889    │ Dashboards ✓    │
│ PII Data ✓      │ Profiling:1777  │ Alerts ✓        │
└─────────────────┴─────────────────┴─────────────────┘

┌─────────────────────────────────────────────────────────┐
│                  ALL PROCESSORS                         │
├─────────────────┬─────────────────┬─────────────────────┤
│ AdaptiveSampler │ CircuitBreaker  │ CostControl         │
│ PlanExtractor   │ Verification    │ NRErrorMonitor     │
│ QueryCorrelator │ Resource        │ Transform           │
└─────────────────┴─────────────────┴─────────────────────┘
```

## 🧪 What Gets Tested

### 1. Unit Tests
- All 7 custom processors
- All 3 custom receivers  
- Common libraries
- Extensions

### 2. Integration Tests
- Database connectivity (PostgreSQL, MySQL)
- Processor pipeline flow
- Configuration validation
- Feature detection

### 3. End-to-End Tests
- Real database → Collector → New Relic/Prometheus
- Data integrity verification
- PII detection and redaction
- Query plan extraction
- Performance monitoring

### 4. Load Tests
- High-volume query simulation
- Processor performance under load
- Memory and CPU usage
- Circuit breaker activation

### 5. Security Tests
- PII detection accuracy
- Data sanitization
- Configuration security
- Vulnerability scanning

## 📊 Monitoring Everything

Once running (`make docker-run`), access:

- **Collector Health**: http://localhost:13133/health
- **Prometheus Metrics**: http://localhost:9090
- **Grafana Dashboards**: http://localhost:3000 (admin/admin)
- **Collector Metrics**: http://localhost:8889/metrics
- **Performance Profiling**: http://localhost:1777 (if enabled)

## 🔧 Configuration Highlights

The unified config includes **ALL features**:

```yaml
# All Processors Enabled
processors:
  adaptivesampler:      # Intelligent sampling
  circuitbreaker:       # Fault protection  
  costcontrol:          # Budget management
  planattributeextractor: # Query plan analysis
  verification:         # PII detection
  nrerrormonitor:       # Error handling
  querycorrelator:      # Transaction correlation

# All Databases Connected
receivers:
  postgresql:           # PostgreSQL metrics
  mysql:               # MySQL metrics
  sqlquery:            # Custom SQL queries
  enhancedsql:         # Enhanced receiver (custom)

# All Export Destinations
exporters:
  prometheus:          # Local metrics
  otlp/newrelic:      # New Relic export
  debug:              # Debug output
  file:               # File export
```

## 🎮 Demo Mode

For a complete demonstration:

```bash
# Start everything with monitoring
make demo

# Generate realistic load
make test-load

# Watch real-time metrics in:
# - Grafana: http://localhost:3000
# - Prometheus: http://localhost:9090

# Stop when done
make demo-stop
```

## 🛠️ Troubleshooting

### Check System Health
```bash
make health-check    # Verify all services
make logs           # View collector logs
make metrics        # Show current metrics
```

### Verify Data Flow
```bash
make verify-data    # Check metrics are flowing
```

### Clean & Restart
```bash
make docker-clean   # Clean everything
make setup          # Reinitialize
make docker-run     # Restart
```

## 📈 Test Results

The comprehensive test runner provides:

- **Real-time progress** with colored output
- **Detailed logging** to `test-results/`
- **JSON reports** for CI/CD integration
- **Performance profiles** for optimization
- **Security scan results**

## 🎯 Key Features Validated

✅ **Data Pipeline**: PostgreSQL → Collector → Prometheus/New Relic  
✅ **PII Protection**: Email, SSN, Credit Card detection and redaction  
✅ **Query Intelligence**: Execution plan extraction and analysis  
✅ **Cost Management**: Budget tracking and automatic reduction  
✅ **Fault Tolerance**: Circuit breakers and error recovery  
✅ **Performance**: Load handling and resource management  
✅ **Security**: Vulnerability scanning and data sanitization  

## 🚦 Success Criteria

- **Build**: All distributions compile successfully
- **Tests**: >95% test pass rate
- **Performance**: Handle 1000+ QPS sustained load
- **Security**: Zero PII leakage detected
- **Integration**: All databases → exporters data flow verified
- **Monitoring**: Real-time metrics and alerting functional

This unified approach ensures everything works together as a complete, production-ready database intelligence solution.