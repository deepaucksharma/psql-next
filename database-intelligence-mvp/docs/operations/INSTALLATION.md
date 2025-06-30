# Installation Guide

This guide covers all installation methods for the Database Intelligence Collector.

## Prerequisites

### System Requirements

#### Minimum Requirements
- **OS**: Linux (x64), macOS (x64/arm64), Windows (x64)
- **CPU**: 2 cores
- **Memory**: 1GB RAM
- **Disk**: 10GB free space
- **Network**: Outbound HTTPS access

#### Recommended Requirements
- **OS**: Linux (x64) - Ubuntu 20.04+ or RHEL 8+
- **CPU**: 4 cores
- **Memory**: 2GB RAM
- **Disk**: 50GB free space
- **Network**: 1Gbps connection

### Software Requirements

#### Required
- Docker 20.10+ and Docker Compose 2.0+ (for containerized deployment)
- OR Go 1.21+ (for building from source)

#### Optional
- Task (task automation tool) - for simplified commands
- Kubernetes 1.24+ (for K8s deployment)
- Helm 3.0+ (for Helm deployment)

## Installation Methods

### Method 1: Docker Compose (Recommended)

#### Step 1: Clone Repository
```bash
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp
```

#### Step 2: Configure Environment
```bash
# Copy example environment file
cp .env.example .env

# Edit with your settings
nano .env
```

Required environment variables:
```bash
# PostgreSQL Connection
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_secure_password
POSTGRES_DB=testdb

# MySQL Connection
MYSQL_HOST=mysql
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=your_secure_password
MYSQL_DB=testdb

# New Relic (Optional)
NEW_RELIC_LICENSE_KEY=your_license_key_here
```

#### Step 3: Start Services
```bash
# Start with minimal configuration
docker-compose up -d

# Or with full configuration
docker-compose --profile full up -d
```

#### Step 4: Verify Installation
```bash
# Check services are running
docker-compose ps

# Check collector health
curl http://localhost:13133/health

# View metrics
curl http://localhost:8888/metrics | head -20
```

### Method 2: Pre-built Binary

#### Step 1: Download Binary
```bash
# Linux x64
wget https://github.com/database-intelligence-mvp/releases/download/v1.0.0/db-intel-collector-linux-amd64.tar.gz
tar -xzf db-intel-collector-linux-amd64.tar.gz

# macOS x64
wget https://github.com/database-intelligence-mvp/releases/download/v1.0.0/db-intel-collector-darwin-amd64.tar.gz
tar -xzf db-intel-collector-darwin-amd64.tar.gz

# macOS ARM64
wget https://github.com/database-intelligence-mvp/releases/download/v1.0.0/db-intel-collector-darwin-arm64.tar.gz
tar -xzf db-intel-collector-darwin-arm64.tar.gz
```

#### Step 2: Install Binary
```bash
# Make executable
chmod +x db-intel-collector

# Move to PATH
sudo mv db-intel-collector /usr/local/bin/

# Verify installation
db-intel-collector --version
```

#### Step 3: Create Configuration
```bash
# Create config directory
mkdir -p /etc/otel-collector

# Download production config
wget -O /etc/otel-collector/config.yaml \
  https://raw.githubusercontent.com/database-intelligence-mvp/main/config/collector-production.yaml

# Edit configuration
nano /etc/otel-collector/config.yaml
```

#### Step 4: Run Collector
```bash
# Run directly
db-intel-collector --config=/etc/otel-collector/config.yaml

# Or create systemd service (see below)
```

### Method 3: Build from Source

#### Step 1: Install Go
```bash
# Download Go 1.21
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz

# Extract and install
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
```

#### Step 2: Install OCB (OpenTelemetry Collector Builder)
```bash
go install go.opentelemetry.io/collector/cmd/builder@v0.127.0
```

#### Step 3: Clone and Build
```bash
# Clone repository
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp

# Build collector
builder --config=ocb-config.yaml

# Binary will be in ./dist/
ls -la ./dist/database-intelligence-collector
```

#### Step 4: Install and Run
```bash
# Install binary
sudo cp ./dist/database-intelligence-collector /usr/local/bin/
sudo chmod +x /usr/local/bin/database-intelligence-collector

# Copy configuration
sudo mkdir -p /etc/otel-collector
sudo cp config/collector-production.yaml /etc/otel-collector/config.yaml

# Run
database-intelligence-collector --config=/etc/otel-collector/config.yaml
```

### Method 4: Kubernetes Deployment

#### Using Helm

```bash
# Add Helm repository
helm repo add db-intel https://database-intelligence-mvp.github.io/helm-charts
helm repo update

# Install with default values
helm install db-intel db-intel/database-intelligence-collector

# Or with custom values
helm install db-intel db-intel/database-intelligence-collector \
  --set postgresql.host=postgres.default.svc.cluster.local \
  --set mysql.host=mysql.default.svc.cluster.local \
  --set newrelic.licenseKey=your_key_here
```

#### Using Kubectl

```bash
# Create namespace
kubectl create namespace db-intelligence

# Create ConfigMap
kubectl create configmap collector-config \
  --from-file=config.yaml=config/collector-production.yaml \
  -n db-intelligence

# Apply deployment
kubectl apply -f deployments/kubernetes/deployment.yaml -n db-intelligence

# Verify deployment
kubectl get pods -n db-intelligence
kubectl logs -f deployment/db-intelligence-collector -n db-intelligence
```

## Post-Installation Setup

### 1. Configure Databases

#### PostgreSQL Setup
```sql
-- Create monitoring user
CREATE USER otel_monitor WITH PASSWORD 'secure_password';
GRANT pg_monitor TO otel_monitor;
GRANT CONNECT ON DATABASE postgres TO otel_monitor;

-- Enable pg_stat_statements
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

#### MySQL Setup
```sql
-- Create monitoring user
CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
```

### 2. Configure Systemd Service (Linux)

Create `/etc/systemd/system/db-intel-collector.service`:
```ini
[Unit]
Description=Database Intelligence Collector
After=network.target

[Service]
Type=simple
User=otel
Group=otel
ExecStart=/usr/local/bin/database-intelligence-collector --config=/etc/otel-collector/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=db-intel-collector

# Resource limits
LimitNOFILE=65536
MemoryLimit=1G
CPUQuota=200%

# Security
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
# Create user
sudo useradd -r -s /bin/false otel

# Set permissions
sudo chown -R otel:otel /etc/otel-collector

# Enable service
sudo systemctl daemon-reload
sudo systemctl enable db-intel-collector
sudo systemctl start db-intel-collector

# Check status
sudo systemctl status db-intel-collector
sudo journalctl -u db-intel-collector -f
```

### 3. Configure Firewall

```bash
# Allow health check port
sudo ufw allow 13133/tcp comment "OTEL Health"

# Allow metrics port (if exposing)
sudo ufw allow 8888/tcp comment "OTEL Metrics"

# Reload firewall
sudo ufw reload
```

### 4. Verify Installation

#### Health Check
```bash
# Basic health check
curl http://localhost:13133/health

# Detailed health with component status
curl http://localhost:13133/health | jq .
```

Expected response:
```json
{
  "healthy": true,
  "components": {
    "adaptive_sampler": {"healthy": true},
    "circuit_breaker": {"healthy": true},
    "plan_extractor": {"healthy": true},
    "verification": {"healthy": true}
  }
}
```

#### Metrics Verification
```bash
# Check PostgreSQL metrics
curl -s http://localhost:8888/metrics | grep postgresql_

# Check MySQL metrics
curl -s http://localhost:8888/metrics | grep mysql_

# Check processor metrics
curl -s http://localhost:8888/metrics | grep otelcol_processor_
```

## Troubleshooting Installation

### Common Issues

#### 1. Port Already in Use
```bash
Error: bind: address already in use
```

Solution:
```bash
# Find process using port
sudo lsof -i :8888
sudo lsof -i :13133

# Change ports in configuration
# Edit config.yaml and change service.telemetry.metrics.address
```

#### 2. Database Connection Failed
```bash
Error: pq: password authentication failed
```

Solution:
```bash
# Verify credentials
psql -h localhost -U otel_monitor -d postgres

# Check network connectivity
telnet postgres-host 5432

# Verify environment variables
env | grep POSTGRES_
```

#### 3. Memory Limit Exceeded
```bash
Error: memory limit exceeded
```

Solution:
```bash
# Increase memory limit in config
memory_limiter:
  check_interval: 1s
  limit_percentage: 80  # Increase from 75
  spike_limit_percentage: 20

# Or increase system limits
sudo systemctl edit db-intel-collector
# Add: MemoryLimit=2G
```

#### 4. Build Failures
```bash
Error: module not found
```

Solution:
```bash
# Clean module cache
go clean -modcache

# Update dependencies
go mod download
go mod tidy

# Rebuild
builder --config=ocb-config.yaml --skip-compilation=false
```

## Next Steps

1. **Configure Monitoring**: Set up dashboards and alerts
2. **Tune Performance**: Adjust sampling rules and resource limits
3. **Enable Processors**: Start with basic processors, add more gradually
4. **Set Up Backups**: Configure configuration backups
5. **Plan Maintenance**: Schedule regular updates

## Uninstallation

### Docker Compose
```bash
# Stop and remove containers
docker-compose down -v

# Remove images
docker rmi database-intelligence/collector:latest
```

### Binary Installation
```bash
# Stop service
sudo systemctl stop db-intel-collector
sudo systemctl disable db-intel-collector

# Remove files
sudo rm /usr/local/bin/database-intelligence-collector
sudo rm -rf /etc/otel-collector
sudo rm /etc/systemd/system/db-intel-collector.service
```

### Kubernetes
```bash
# Helm
helm uninstall db-intel -n db-intelligence

# Kubectl
kubectl delete -f deployments/kubernetes/ -n db-intelligence
kubectl delete namespace db-intelligence
```

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025