# Infrastructure Modernization Plan

## Overview

This document outlines a comprehensive plan to modernize the Database Intelligence Collector's build, deployment, and operational infrastructure. Currently, functionality is scattered across 30+ shell scripts, multiple docker-compose files, and raw Kubernetes YAMLs. We'll consolidate this into a modern, maintainable infrastructure.

## Current State Analysis

### Problems with Current Infrastructure

1. **Scattered Scripts** (30+ .sh files)
   - Duplicate functionality across scripts
   - No consistent error handling
   - Hard to discover what's available
   - No unified interface

2. **Multiple Docker Compose Files** (10+ files)
   - Overlapping configurations
   - Environment-specific files not clearly organized
   - No variable management system

3. **Raw Kubernetes YAMLs**
   - No templating or package management
   - Environment values hardcoded
   - No versioning or rollback strategy

4. **Makefile Limitations**
   - Growing too complex
   - No dependency management
   - Limited cross-platform support

## Proposed Modern Infrastructure

### 1. Unified Task Runner with Taskfile

Replace Makefile and shell scripts with [Taskfile](https://taskfile.dev/):

```yaml
# Taskfile.yml
version: '3'

includes:
  build: ./tasks/build.yml
  deploy: ./tasks/deploy.yml
  test: ./tasks/test.yml
  dev: ./tasks/dev.yml

env:
  PROJECT_NAME: database-intelligence-collector
  DEFAULT_ENV: development

tasks:
  default:
    desc: Show available tasks
    cmds:
      - task --list

  setup:
    desc: Complete development environment setup
    deps: [setup:tools, setup:deps, setup:env]
    
  setup:tools:
    desc: Install required tools
    cmds:
      - go install go.opentelemetry.io/collector/cmd/ocb@latest
      - go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
      - curl -sSfL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    status:
      - command -v ocb
      - command -v golangci-lint
      - command -v helm

  build:
    desc: Build the collector with proper module fixes
    deps: [fix:modules]
    vars:
      OUTPUT: '{{.OUTPUT | default "dist/otelcol-db-intelligence"}}'
    cmds:
      - ocb --config=build/ocb-config.yaml --output-path={{.OUTPUT}}
    sources:
      - processors/**/*.go
      - go.mod
      - go.sum
    generates:
      - "{{.OUTPUT}}"

  fix:modules:
    desc: Fix module path inconsistencies
    cmds:
      - |
        echo "Fixing module paths..."
        sed -i.bak 's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' build/*.yaml
        find . -name "*.go" -type f -exec sed -i.bak 's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' {} \;
    status:
      - '! grep -r "github.com/newrelic/database-intelligence-mvp" build/'

  dev:
    desc: Start development environment
    deps: [dev:databases]
    cmds:
      - task: build
      - task: run
        vars: {CONFIG: "config/dev.yaml"}

  test:all:
    desc: Run all tests with coverage
    deps: [test:unit, test:integration, test:e2e]
    cmds:
      - go tool cover -html=coverage.out -o coverage.html
      - echo "Coverage report: coverage.html"
```

### 2. Helm Chart for Kubernetes

Complete Helm chart with proper structure:

```yaml
# deployments/helm/db-intelligence/Chart.yaml
apiVersion: v2
name: database-intelligence
description: OpenTelemetry-based database monitoring with custom processors
type: application
version: 1.0.0
appVersion: "0.1.0"

dependencies:
  - name: postgresql
    version: "~12.0.0"
    repository: https://charts.bitnami.com/bitnami
    condition: postgresql.enabled
  - name: mysql
    version: "~9.0.0"
    repository: https://charts.bitnami.com/bitnami
    condition: mysql.enabled
```

```yaml
# deployments/helm/db-intelligence/values.yaml
replicaCount: 1

image:
  repository: database-intelligence
  pullPolicy: IfNotPresent
  tag: ""

config:
  mode: standard  # or experimental
  
  receivers:
    postgresql:
      enabled: true
      endpoint: "{{ .Values.postgresql.primary.service.name }}:5432"
      databases: []
      
    mysql:
      enabled: false
      endpoint: "{{ .Values.mysql.primary.service.name }}:3306"
      
    sqlquery:
      enabled: true
      queries:
        ashSampling: true
        pgStatStatements: true

  processors:
    standard:
      - memory_limiter
      - batch
      - resource
    experimental:
      - adaptive_sampler
      - circuit_breaker
      - plan_extractor
      - verification

  exporters:
    newrelic:
      enabled: true
      endpoint: otlp.nr-data.net:4317
      # License key from secret
    prometheus:
      enabled: true
      port: 8889

monitoring:
  serviceMonitor:
    enabled: true
  grafanaDashboard:
    enabled: true

autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 5
  targetCPUUtilizationPercentage: 80
  targetMemoryUtilizationPercentage: 80

postgresql:
  enabled: true
  auth:
    postgresPassword: postgres
    database: testdb
  primary:
    initdb:
      scripts:
        01-init.sql: |
          CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
          CREATE USER monitoring_user WITH PASSWORD 'monitoring';
          GRANT pg_monitor TO monitoring_user;
```

### 3. Docker Compose with Profiles

Single docker-compose.yaml with profiles:

```yaml
# docker-compose.yaml
version: '3.9'

x-common-env: &common-env
  NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
  ENVIRONMENT: ${ENVIRONMENT:-development}

services:
  # Core collector service
  collector:
    build:
      context: .
      dockerfile: build/Dockerfile
      target: ${BUILD_TARGET:-production}
    image: database-intelligence:${VERSION:-latest}
    environment:
      <<: *common-env
      POSTGRES_HOST: postgres
      POSTGRES_USER: monitoring_user
      POSTGRES_PASSWORD: monitoring
    ports:
      - "13133:13133"  # Health check
      - "8888:8888"    # Metrics
      - "8889:8889"    # Prometheus
    volumes:
      - ./config/${CONFIG_FILE:-collector.yaml}:/etc/otel/config.yaml:ro
      - collector-data:/var/lib/otel
    depends_on:
      postgres:
        condition: service_healthy
    profiles: ["collector", "all"]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:13133/"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Development databases
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: testdb
    volumes:
      - ./scripts/sql/init-postgres.sql:/docker-entrypoint-initdb.d/01-init.sql:ro
      - postgres-data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    profiles: ["databases", "dev", "all"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: mysql
      MYSQL_DATABASE: testdb
    volumes:
      - ./scripts/sql/init-mysql.sql:/docker-entrypoint-initdb.d/01-init.sql:ro
      - mysql-data:/var/lib/mysql
    ports:
      - "3306:3306"
    profiles: ["databases", "dev", "all"]
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Monitoring stack
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./deploy/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    ports:
      - "9090:9090"
    profiles: ["monitoring", "all"]

  grafana:
    image: grafana/grafana:latest
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
      GF_USERS_ALLOW_SIGN_UP: false
    volumes:
      - ./deploy/monitoring/grafana/datasources:/etc/grafana/provisioning/datasources:ro
      - ./deploy/monitoring/grafana/dashboards:/etc/grafana/provisioning/dashboards:ro
      - grafana-data:/var/lib/grafana
    ports:
      - "3000:3000"
    profiles: ["monitoring", "all"]

  # Load testing
  k6:
    image: grafana/k6:latest
    volumes:
      - ./tests/load:/scripts:ro
    command: run /scripts/database-load.js
    environment:
      K6_POSTGRES_URL: postgres://postgres:postgres@postgres:5432/testdb
      K6_MYSQL_URL: mysql://root:mysql@mysql:3306/testdb
    profiles: ["load-test"]
    depends_on:
      - postgres
      - mysql

volumes:
  collector-data:
  postgres-data:
  mysql-data:
  prometheus-data:
  grafana-data:
```

### 4. Configuration Management System

Environment-specific configuration with validation:

```yaml
# config/base/collector.yaml
# Base configuration - environment agnostic
receivers:
  postgresql:
    collection_interval: 60s
    resource_attributes:
      db.system: postgresql
      
  mysql:
    collection_interval: 60s
    resource_attributes:
      db.system: mysql
      
  sqlquery:
    collection_interval: 300s
    timeout: 30s

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 30
    
  batch:
    send_batch_size: 8192
    timeout: 10s
    
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: upsert
      - key: service.version
        from_attribute: SERVICE_VERSION
        action: insert

exporters:
  debug:
    verbosity: basic
    sampling_initial: 5
    sampling_thereafter: 20
```

```yaml
# config/overlays/development.yaml
# Development-specific overrides
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases: [testdb]
    tls:
      insecure: true

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 50
    
service:
  telemetry:
    logs:
      level: debug
```

### 5. CI/CD Pipeline

GitHub Actions workflow:

```yaml
# .github/workflows/ci.yml
name: CI/CD Pipeline

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

env:
  GO_VERSION: '1.21'
  DOCKER_REGISTRY: ghcr.io

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          
      - name: Install Task
        uses: arduino/setup-task@v1
        
      - name: Validate
        run: |
          task setup:tools
          task lint
          task validate:config
          task validate:modules

  test:
    needs: validate
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
          
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          
      - name: Run Tests
        run: |
          task test:all
          
      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  build:
    needs: test
    runs-on: ubuntu-latest
    strategy:
      matrix:
        platform: [linux/amd64, linux/arm64]
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3
        
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
        
      - name: Build Collector
        run: task build:multi PLATFORM=${{ matrix.platform }}
        
      - name: Build Docker Image
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: ${{ matrix.platform }}
          tags: ${{ env.DOCKER_REGISTRY }}/${{ github.repository }}:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

  helm:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Lint Helm Chart
        run: |
          helm lint deployments/helm/db-intelligence
          
      - name: Test Helm Chart
        run: |
          helm unittest deployments/helm/db-intelligence
          
      - name: Package Helm Chart
        if: github.ref == 'refs/heads/main'
        run: |
          helm package deployments/helm/db-intelligence
          helm repo index . --url https://${{ github.repository_owner }}.github.io/helm-charts/
```

### 6. Unified CLI Tool

Go-based CLI for all operations:

```go
// cmd/dbintel/main.go
package main

import (
    "github.com/spf13/cobra"
    "github.com/database-intelligence-mvp/internal/cli"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "dbintel",
        Short: "Database Intelligence Collector CLI",
        Long:  `Unified CLI for managing the Database Intelligence Collector`,
    }

    rootCmd.AddCommand(
        cli.NewBuildCommand(),
        cli.NewDeployCommand(),
        cli.NewConfigCommand(),
        cli.NewValidateCommand(),
        cli.NewDiagnoseCommand(),
    )

    rootCmd.Execute()
}
```

```go
// internal/cli/deploy.go
package cli

func NewDeployCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "deploy",
        Short: "Deploy the collector",
    }

    cmd.AddCommand(
        &cobra.Command{
            Use:   "local",
            Short: "Deploy locally with Docker Compose",
            RunE: func(cmd *cobra.Command, args []string) error {
                return deployLocal(cmd.Context())
            },
        },
        &cobra.Command{
            Use:   "kubernetes",
            Short: "Deploy to Kubernetes with Helm",
            RunE: func(cmd *cobra.Command, args []string) error {
                return deployKubernetes(cmd.Context())
            },
        },
    )

    return cmd
}
```

### 7. Development Environment Automation

DevContainer configuration:

```json
// .devcontainer/devcontainer.json
{
    "name": "Database Intelligence Collector",
    "dockerComposeFile": ["../docker-compose.yaml"],
    "service": "devcontainer",
    "workspaceFolder": "/workspace",
    
    "features": {
        "ghcr.io/devcontainers/features/go:1": {
            "version": "1.21"
        },
        "ghcr.io/devcontainers/features/docker-in-docker:2": {},
        "ghcr.io/devcontainers/features/kubectl-helm-minikube:1": {}
    },
    
    "customizations": {
        "vscode": {
            "extensions": [
                "golang.go",
                "ms-azuretools.vscode-docker",
                "ms-kubernetes-tools.vscode-kubernetes-tools",
                "redhat.vscode-yaml"
            ],
            "settings": {
                "go.toolsManagement.checkForUpdates": "local",
                "go.useLanguageServer": true,
                "go.lintTool": "golangci-lint"
            }
        }
    },
    
    "postCreateCommand": "task setup",
    "remoteUser": "vscode"
}
```

### 8. Infrastructure as Code

Terraform module for cloud deployments:

```hcl
# terraform/main.tf
module "database_intelligence" {
  source = "./modules/database-intelligence"
  
  environment = var.environment
  region      = var.region
  
  # Kubernetes cluster
  kubernetes_version = "1.28"
  node_pools = {
    default = {
      size         = "t3.medium"
      min_nodes    = 1
      max_nodes    = 5
      desired_nodes = 2
    }
  }
  
  # Database targets
  postgresql_instances = var.postgresql_instances
  mysql_instances      = var.mysql_instances
  
  # New Relic configuration
  newrelic_license_key = var.newrelic_license_key
  newrelic_account_id  = var.newrelic_account_id
  
  # Monitoring
  enable_prometheus = true
  enable_grafana    = true
  
  tags = {
    Project     = "database-intelligence"
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}
```

## Migration Plan

### Phase 1: Foundation (Week 1-2)
1. Implement Taskfile structure
2. Consolidate shell scripts into tasks
3. Create unified configuration system
4. Set up development environment automation

### Phase 2: Containerization (Week 3-4)
1. Create unified Docker Compose with profiles
2. Build multi-platform Docker images
3. Implement proper health checks
4. Create development containers

### Phase 3: Kubernetes (Week 5-6)
1. Complete Helm chart implementation
2. Add monitoring integrations
3. Create Terraform modules
4. Test auto-scaling scenarios

### Phase 4: CI/CD (Week 7-8)
1. Implement GitHub Actions workflows
2. Add comprehensive testing
3. Set up automated releases
4. Create deployment pipelines

## Benefits of Modernization

1. **Developer Experience**
   - Single command setup: `task setup`
   - Consistent interface across all operations
   - Self-documenting commands
   - Cross-platform support

2. **Reliability**
   - Proper dependency management
   - Health checks at every level
   - Automated rollback capabilities
   - Comprehensive testing

3. **Scalability**
   - Easy horizontal scaling
   - Resource optimization
   - Cloud-native patterns
   - Multi-region support

4. **Maintainability**
   - Single source of truth
   - Version-controlled infrastructure
   - Automated documentation
   - Clear separation of concerns

5. **Security**
   - Secrets management
   - RBAC implementation
   - Network policies
   - Vulnerability scanning

## Next Steps

1. Review and approve modernization plan
2. Create implementation tickets
3. Begin Phase 1 implementation
4. Set up tracking and metrics
5. Plan team training

This modernization will transform the Database Intelligence Collector from a collection of scripts into a professional, enterprise-ready system.