# Project Structure

```
database-intelligence-mysql/
│
├── 📋 Configuration Files
│   ├── .env.example                    # Example environment variables
│   ├── docker-compose.yml              # Main orchestration file
│   │
│   └── config/                         # All configuration files
│       ├── collector/
│       │   └── master.yaml             # OpenTelemetry collector config (1,442 lines)
│       ├── mysql/
│       │   ├── primary.cnf             # MySQL primary server config
│       │   └── replica.cnf             # MySQL replica server config
│       └── newrelic/
│           └── dashboards.json         # Consolidated New Relic dashboards
│
├── 🚀 Deployment & Operations
│   ├── deploy/                         # Deployment scripts
│   │   ├── deploy.sh                   # Main deployment script with modes
│   │   ├── setup.sh                    # Initial environment setup
│   │   └── docker-init.sh              # Docker daemon helper
│   │
│   └── operate/                        # Operational scripts
│       ├── generate-workload.sh        # MySQL workload generator
│       ├── validate-metrics.sh         # New Relic metric validation
│       ├── validate-config.sh          # Configuration validation
│       ├── diagnose.sh                 # System diagnostics
│       ├── test-connection.sh          # MySQL connection testing
│       └── full-test.sh                # Comprehensive test suite
│
├── 💾 MySQL Setup
│   └── mysql/
│       └── init/                       # MySQL initialization
│           ├── 01-schema.sql           # Database schema & procedures
│           ├── 02-sample-data.sql      # Sample data generation
│           ├── initialize.sh           # MySQL initialization script
│           └── populate.sh             # Data population script
│
├── 📚 Documentation
│   └── docs/
│       ├── getting-started.md          # Quick start guide
│       ├── configuration.md            # Configuration reference
│       ├── operations.md               # Operational procedures
│       └── troubleshooting.md          # Common issues & solutions
│
└── 📦 Examples
    └── examples/
        ├── python-app/                 # Sample Python application
        │   ├── app.py                  # MySQL workload generator app
        │   └── requirements.txt        # Python dependencies
        └── configurations/             # Example configurations
```

## Key Components

### 1. Master Configuration (`config/collector/master.yaml`)
- **Size**: 1,442 lines
- **Features**: All MySQL metrics, SQL intelligence, ML processors
- **Modes**: minimal, standard, advanced, debug

### 2. Deployment Script (`deploy/deploy.sh`)
- **Primary deployment tool**
- **Supports all deployment modes**
- **Optional workload generation**

### 3. Docker Compose (`docker-compose.yml`)
- **Services**: MySQL primary, replica, collector, sample app
- **Volumes**: Persistent data, configurations
- **Networks**: Isolated MySQL network

### 4. Operational Scripts
- **Validation**: Metrics, configuration, connections
- **Monitoring**: Workload generation, diagnostics
- **Testing**: Full test suite with health checks

## Deployment Flow

```
1. Environment Setup
   └── .env configuration
   
2. MySQL Initialization
   └── mysql/init/*.sql
   
3. Collector Deployment
   └── config/collector/master.yaml
   
4. Metric Validation
   └── operate/validate-metrics.sh
   
5. Dashboard Import
   └── config/newrelic/dashboards.json
```

## Configuration Hierarchy

```
Environment Variables (.env)
    ↓
Docker Compose (docker-compose.yml)
    ↓
Collector Config (config/collector/master.yaml)
    ↓
MySQL Configs (config/mysql/*.cnf)
```