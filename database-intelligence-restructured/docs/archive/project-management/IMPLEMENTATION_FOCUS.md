# Implementation Focus (Tests Deferred)

## 🎯 Revised Priorities

### Immediate Focus Areas

#### 1. Fix Module Paths (Critical)
All imports use `github.com/deepaksharma/db-otel` which needs to be updated to the correct module path.

**Action**: Update all go.mod files and imports to use consistent module naming.

#### 2. Fix README.md (High Priority)
README claims support for databases that don't exist (Oracle, MSSQL).

**Action**: Update README to accurately reflect:
- ✅ PostgreSQL (fully supported)
- ✅ MySQL (fully supported)
- ⚠️ MongoDB (basic support, no custom features)
- ⚠️ Redis (basic support, no custom features)
- ❌ Oracle (not implemented)
- ❌ SQL Server (not implemented)

#### 3. Standardize Go Versions
Mix of go 1.21, 1.22, and 1.23 across modules.

**Action**: Update all go.mod files to use go 1.22.0

#### 4. MongoDB Enhanced Receiver
Create custom MongoDB receiver with:
- Replica set support
- Sharding awareness
- Performance metrics
- Query analysis

#### 5. Redis Enhanced Receiver
Create custom Redis receiver with:
- Cluster support
- Pub/sub monitoring
- Memory analysis
- Command patterns

#### 6. Multi-Database Dashboards
Create Grafana/New Relic dashboards for:
- Unified database overview
- Per-database deep dives
- Cross-database correlations

## 📋 Deferred Items (Testing)

The following testing tasks are deferred:
- ❌ MongoDB E2E test suite completion
- ❌ Redis E2E test suite
- ❌ NRI exporter tests
- ❌ ASH receiver tests
- ❌ Enhanced SQL receiver tests
- ❌ Kernel metrics receiver tests
- ❌ Test data factory pattern
- ❌ Cross-database test scenarios

## 🚀 Next Actions

### Today
1. Fix module paths across all files
2. Update README.md with accurate information
3. Standardize Go versions

### This Week
1. Implement MongoDB enhanced receiver
2. Implement Redis enhanced receiver
3. Create dashboard templates
4. Update processors for MongoDB/Redis support

### Next Week
1. Add Oracle receiver (if needed)
2. Add SQL Server receiver (if needed)
3. Implement cross-database correlation
4. Performance optimization

## 📊 Success Metrics (Without Tests)

1. **Module Health**
   - All builds passing
   - Consistent module versions
   - No import errors

2. **Database Support**
   - MongoDB custom receiver operational
   - Redis custom receiver operational
   - All processors support MongoDB/Redis

3. **Documentation**
   - README reflects reality
   - Configuration examples work
   - Architecture diagrams updated

4. **Dashboards**
   - Multi-database overview available
   - Individual database dashboards
   - Performance metrics visible

## 🔧 Technical Implementation

### MongoDB Enhanced Receiver
```go
// components/receivers/mongodb/
├── receiver.go      # Main receiver implementation
├── scraper.go       # Metrics scraper
├── config.go        # Configuration
├── factory.go       # Factory pattern
└── metrics.go       # Metric definitions
```

### Redis Enhanced Receiver
```go
// components/receivers/redis/
├── receiver.go      # Main receiver implementation
├── scraper.go       # Metrics scraper
├── config.go        # Configuration
├── factory.go       # Factory pattern
└── metrics.go       # Metric definitions
```

### Dashboard Structure
```yaml
dashboards/
├── unified/
│   └── overview.json
├── databases/
│   ├── postgresql.json
│   ├── mysql.json
│   ├── mongodb.json
│   └── redis.json
└── correlations/
    └── cross-database.json
```

## 🏁 Definition of Done (No Tests)

### For Each Component
- [ ] Code compiles without errors
- [ ] Configuration documented
- [ ] Example usage provided
- [ ] Metrics exposed to Prometheus
- [ ] Dashboard visualizes metrics

### For Overall Project
- [ ] All claimed databases have receivers
- [ ] Documentation matches implementation
- [ ] Dashboards provide insights
- [ ] Performance overhead < 5%
- [ ] Production deployment guide

---

*Focus shifted from testing to implementation*
*Priority: Get working implementations first*