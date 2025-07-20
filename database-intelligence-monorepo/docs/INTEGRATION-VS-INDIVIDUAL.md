# 🔄 Integration vs Individual Module Usage

## 📋 Overview

The Database Intelligence MySQL Monorepo supports two deployment modes:
1. **Individual Module Mode** - Run modules independently
2. **Integration Mode** - Run all modules together with proper dependencies

## 🏃 **Individual Module Mode**

Use this mode when you:
- Need only specific functionality (e.g., just core metrics)
- Are developing/testing a single module
- Have resource constraints
- Want to scale modules independently

### **How to Run Individual Modules**

```bash
# From module directory
cd modules/core-metrics
docker-compose up -d

# Or from root directory
make run-core-metrics
```

### **Limitations**
- Modules that depend on others won't have full functionality
- No automatic dependency management
- Need to manually ensure dependent modules are running

### **Module Dependencies**

| Module | Depends On | Can Run Standalone |
|--------|------------|-------------------|
| core-metrics | None | ✅ Yes |
| sql-intelligence | None | ✅ Yes |
| wait-profiler | None | ✅ Yes |
| resource-monitor | None | ✅ Yes |
| anomaly-detector | core-metrics, sql-intelligence, wait-profiler, resource-monitor | ❌ No |
| business-impact | sql-intelligence | ❌ No |
| performance-advisor | core-metrics, sql-intelligence, wait-profiler, anomaly-detector | ❌ No |
| replication-monitor | None | ✅ Yes |
| alert-manager | All modules | ❌ No |
| canary-tester | None | ✅ Yes |
| cross-signal-correlator | Multiple modules | ❌ No |

## 🚀 **Integration Mode**

Use this mode when you:
- Need the full database intelligence suite
- Want automatic dependency management
- Are deploying to production
- Need cross-module correlation features

### **How to Run Integration Mode**

```bash
# Validate environment first
make validate-env

# Run all modules with dependencies
make run-all

# Or use docker-compose directly
cd integration
docker-compose -f docker-compose.all.yaml up -d
```

### **Benefits**
- ✅ Automatic dependency ordering
- ✅ Shared MySQL instance
- ✅ Proper network configuration
- ✅ Health checks for dependencies
- ✅ Centralized configuration

## 🔧 **Configuration Differences**

### **Individual Mode**
- Each module has its own `docker-compose.yaml`
- Uses `mysql-test` service defined in each compose file
- Network: module-specific or default

### **Integration Mode**
- Single `integration/docker-compose.all.yaml`
- Shared MySQL service for all modules
- Network: `db-intelligence` shared network
- Environment variables passed consistently

## 📊 **When to Use Which Mode**

### **Use Individual Mode For:**
- Development and testing
- Debugging specific modules
- Limited resource environments
- Microservice deployments
- Custom scaling requirements

### **Use Integration Mode For:**
- Production deployments
- Full feature set requirements
- Demonstration environments
- Integration testing
- Simplified operations

## 🔍 **Monitoring Differences**

### **Individual Mode**
```bash
# Check each module separately
curl http://localhost:8081/metrics  # core-metrics
curl http://localhost:8082/metrics  # sql-intelligence
# etc...
```

### **Integration Mode**
```bash
# Use unified validation
./shared/validation/health-check-all.sh

# Or check all at once
make validate
```

## ⚠️ **Important Notes**

1. **Federation URLs**: In individual mode, federation endpoints may not resolve. Modules will fall back to standalone operation.

2. **Port Conflicts**: When running modules individually, ensure no port conflicts with other running modules.

3. **Data Persistence**: Each mode uses different volume names, so data won't be shared between modes.

4. **Environment Variables**: Integration mode uses `shared/config/service-endpoints.env` for consistent configuration.

## 🛠️ **Troubleshooting**

### **Module Not Getting Data from Dependencies**

**Individual Mode**: Expected behavior - dependent module not running
**Integration Mode**: Check with `docker-compose ps` and ensure all services are healthy

### **Port Already in Use**

**Individual Mode**: Another module might be using the port
**Integration Mode**: Should not happen - all ports are pre-assigned

### **Environment Variables Not Set**

**Individual Mode**: Set manually or use `.env` file in module directory
**Integration Mode**: Check `shared/config/service-endpoints.env`

---

For more details, see:
- [Module Development Standards](MODULE-DEVELOPMENT.md)
- [Main README](../README.md)
- Individual module READMEs in `modules/*/README.md`