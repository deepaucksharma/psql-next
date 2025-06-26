# Enhanced PostgreSQL Collector Deployment Patterns

## Overview

This document details the enhanced deployment patterns that support the reference architecture's single binary approach with multiple output modes.

## 1. Single Binary Architecture

### 1.1 Binary Structure

```yaml
# Single binary with multiple personalities
binary:
  name: "pgquerylens-collector"
  version: "1.0.0"
  
  modes:
    - name: "standalone"
      description: "Direct execution mode"
      command: "pgquerylens-collector"
      
    - name: "nri"
      description: "New Relic Infrastructure integration mode"
      command: "pgquerylens-collector --mode=nri"
      
    - name: "otel-receiver"
      description: "OpenTelemetry receiver mode"
      command: "pgquerylens-collector --mode=otel"
      
    - name: "library"
      description: "Embedded library mode"
      usage: "import pgquerylens"
```

### 1.2 Mode Detection and Configuration

```go
// cmd/pgquerylens-collector/main.go
package main

import (
    "github.com/newrelic/pgquerylens-core/config"
    "github.com/newrelic/pgquerylens-core/collector"
    "github.com/newrelic/pgquerylens-core/export"
)

func main() {
    // Auto-detect mode based on environment and args
    mode := detectMode()
    
    switch mode {
    case config.ModeNRI:
        runAsNRIIntegration()
    case config.ModeOTel:
        runAsOTelReceiver()
    case config.ModeStandalone:
        runStandalone()
    case config.ModeLibrary:
        // Used when imported as library
        return
    }
}

func detectMode() config.Mode {
    // Check if running under Infrastructure Agent
    if os.Getenv("NRI_INTEGRATION") != "" {
        return config.ModeNRI
    }
    
    // Check command line args
    if flag.Arg("mode") == "otel" {
        return config.ModeOTel
    }
    
    // Check if being imported as library
    if isImportedAsLibrary() {
        return config.ModeLibrary
    }
    
    return config.ModeStandalone
}
```

## 2. Infrastructure Agent Integration

### 2.1 Drop-in Replacement for nri-postgresql

```yaml
# /var/db/newrelic-infra/newrelic-integrations/definitions/postgresql-definition.yml
name: com.newrelic.postgresql
description: Enhanced PostgreSQL monitoring with pgquerylens
protocol_version: 4
os: linux

commands:
  all_data:
    command:
      - ./bin/pgquerylens-collector
      - --mode=nri
      - --config=${config.path}
    interval: ${config.interval}

  metrics:
    command:
      - ./bin/pgquerylens-collector
      - --mode=nri
      - --metrics
    interval: 60

  inventory:
    command:
      - ./bin/pgquerylens-collector
      - --mode=nri
      - --inventory
    interval: 300
    prefix: config/postgresql

  events:
    command:
      - ./bin/pgquerylens-collector
      - --mode=nri
      - --events
    interval: 30
```

### 2.2 Enhanced Integration Configuration

```yaml
# /etc/newrelic-infra/integrations.d/postgresql-config.yml
integrations:
  - name: com.newrelic.postgresql
    
    # Connection settings (OHI compatible)
    env:
      HOSTNAME: ${POSTGRES_HOST}
      PORT: 5432
      USERNAME: ${POSTGRES_USER}
      PASSWORD: ${POSTGRES_PASSWORD}
      DATABASE: postgres
      ENABLE_SSL: true
      
      # OHI compatibility settings
      METRICS: true
      INVENTORY: true
      COLLECTION_LIST: '{"postgres": {"schemas": ["public"]}}'
      TIMEOUT: 30
      
      # Query performance monitoring (OHI compatible)
      QUERY_MONITORING: true
      QUERY_MONITORING_COUNT_THRESHOLD: 20
      QUERY_MONITORING_RESPONSE_TIME_THRESHOLD: 500
      
      # Enhanced features (new)
      ENABLE_PGQUERYLENS_EXTENSION: true
      ENABLE_EBPF: true
      ENABLE_ASH: true
      ASH_SAMPLE_INTERVAL: 1s
      ENABLE_PLAN_CAPTURE: true
      ENABLE_KERNEL_METRICS: true
      
      # Adaptive sampling
      ENABLE_ADAPTIVE_SAMPLING: true
      SAMPLING_BASE_RATE: 1.0
      SAMPLING_SLOW_QUERY_THRESHOLD_MS: 1000
    
    # Multiple instance support
    instances:
      - name: primary
        env:
          HOSTNAME: postgres-primary.example.com
          LABEL_ROLE: primary
          
      - name: replica
        env:
          HOSTNAME: postgres-replica.example.com
          LABEL_ROLE: replica
          ENABLE_EBPF: false  # Disable eBPF on replicas
```

## 3. OpenTelemetry Collector Integration

### 3.1 As OTLP Receiver

```yaml
# otel-collector-config.yaml
receivers:
  pgquerylens:
    # Connection configuration
    endpoints:
      - endpoint: postgres-primary:5432
        username: ${env:POSTGRES_USER}
        password: ${env:POSTGRES_PASSWORD}
        databases: [postgres, app_db]
        
      - endpoint: postgres-replica:5432
        username: ${env:POSTGRES_USER}
        password: ${env:POSTGRES_PASSWORD}
        databases: [postgres]
        disable_ebpf: true
    
    # Collection settings
    collection_interval: 60s
    initial_delay: 10s
    
    # Feature configuration
    features:
      pgquerylens_extension: true
      ebpf_enabled: true
      ash_enabled: true
      plan_capture: true
      kernel_metrics: true
      
    # OHI compatibility
    compatibility:
      ohi_metric_names: true
      ohi_event_types: true
      query_text_limit: 4095
      
    # Resource detection
    resource_attributes:
      service.name: postgresql
      service.namespace: ${env:K8S_NAMESPACE}
      deployment.environment: ${env:ENVIRONMENT}

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000
    
  resource/enhance:
    attributes:
      - key: db.system
        value: postgresql
        action: insert
      - key: cloud.provider
        from_attribute: CLOUD_PROVIDER
        action: insert
        
  transform/compatibility:
    metric_statements:
      - context: metric
        statements:
          # Maintain OHI metric names if needed
          - set(name, "postgres.db.queries.slow_count") 
            where name == "db.postgresql.queries.slow.count"

exporters:
  otlp:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_API_KEY}
      
  prometheus:
    endpoint: 0.0.0.0:9090
    resource_to_telemetry_conversion:
      enabled: true

service:
  pipelines:
    metrics:
      receivers: [pgquerylens]
      processors: [batch, resource/enhance, transform/compatibility]
      exporters: [otlp, prometheus]
    
    logs:
      receivers: [pgquerylens]
      processors: [batch, resource/enhance]
      exporters: [otlp]
    
    traces:
      receivers: [pgquerylens]
      processors: [batch, resource/enhance]
      exporters: [otlp]
```

### 3.2 Custom OTLP Distribution

```dockerfile
# Dockerfile for custom OTel collector with pgquerylens
FROM otel/opentelemetry-collector-contrib:latest as otelcol

FROM golang:1.21 as builder
WORKDIR /app
COPY . .
RUN go build -o pgquerylens-receiver ./receivers/pgquerylens

FROM alpine:3.18
RUN apk add --no-cache ca-certificates

# Copy OTel collector
COPY --from=otelcol /otelcol /otelcol

# Copy pgquerylens receiver
COPY --from=builder /app/pgquerylens-receiver /usr/local/bin/

# Configuration
COPY otel-config.yaml /etc/otel/config.yaml

ENTRYPOINT ["/otelcol", "--config=/etc/otel/config.yaml"]
```

## 4. Kubernetes Deployment Patterns

### 4.1 Sidecar Pattern with Extension

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-init-scripts
data:
  01-install-extension.sql: |
    CREATE EXTENSION IF NOT EXISTS pg_querylens;
    CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
    
    -- Configure pg_querylens
    ALTER SYSTEM SET pg_querylens.enabled = 'on';
    ALTER SYSTEM SET pg_querylens.buffer_size = '32MB';
    ALTER SYSTEM SET pg_querylens.track = 'all';
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres-with-monitoring
spec:
  serviceName: postgres
  replicas: 3
  template:
    spec:
      initContainers:
      # Install pgquerylens extension
      - name: install-extension
        image: postgres:15
        command: ['sh', '-c']
        args:
          - |
            cp /scripts/*.sql /docker-entrypoint-initdb.d/
            chown postgres:postgres /docker-entrypoint-initdb.d/*
        volumeMounts:
        - name: init-scripts
          mountPath: /scripts
        - name: initdb
          mountPath: /docker-entrypoint-initdb.d
          
      containers:
      # PostgreSQL with pgquerylens
      - name: postgres
        image: postgres:15-pgquerylens  # Custom image with extension
        env:
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: password
        - name: POSTGRES_SHARED_PRELOAD_LIBRARIES
          value: "pg_querylens,pg_stat_statements"
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
        - name: postgres-shm
          mountPath: /dev/shm
        - name: initdb
          mountPath: /docker-entrypoint-initdb.d
          
      # Monitoring sidecar
      - name: pgquerylens-collector
        image: pgquerylens-collector:latest
        args:
        - --mode=hybrid
        - --shm-path=/dev/shm/pgquerylens
        env:
        - name: POSTGRES_HOST
          value: localhost
        - name: POSTGRES_USER
          value: postgres
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: password
        - name: OUTPUT_MODE
          value: dual  # Both NRI and OTLP
        - name: NRI_LICENSE_KEY
          valueFrom:
            secretKeyRef:
              name: newrelic-license
              key: key
        - name: OTLP_ENDPOINT
          value: otel-collector.monitoring:4317
        securityContext:
          capabilities:
            add:
            - SYS_ADMIN  # For eBPF
            - SYS_PTRACE
        volumeMounts:
        - name: postgres-shm
          mountPath: /dev/shm
          readOnly: true
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
            
      volumes:
      - name: init-scripts
        configMap:
          name: postgres-init-scripts
      - name: postgres-shm
        emptyDir:
          medium: Memory
          sizeLimit: 128Mi
      - name: initdb
        emptyDir: {}
  
  volumeClaimTemplates:
  - metadata:
      name: postgres-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 100Gi
```

### 4.2 DaemonSet for Node-Wide Monitoring

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pgquerylens-daemonset
  namespace: monitoring
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pgquerylens-daemonset
rules:
- apiGroups: [""]
  resources: ["nodes", "pods", "services"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["statefulsets", "deployments"]
  verbs: ["get", "list"]
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: pgquerylens-node-collector
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: pgquerylens-node-collector
  template:
    metadata:
      labels:
        app: pgquerylens-node-collector
    spec:
      serviceAccountName: pgquerylens-daemonset
      hostNetwork: true
      hostPID: true
      
      containers:
      - name: collector
        image: pgquerylens-collector:latest
        args:
        - --mode=daemonset
        - --discover-postgres  # Auto-discover PostgreSQL instances
        - --ebpf-enabled
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: DISCOVERY_NAMESPACE
          value: "default,production,staging"
        - name: POSTGRES_USER
          value: monitoring
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-monitoring
              key: password
        securityContext:
          privileged: true
          capabilities:
            add:
            - SYS_ADMIN
            - SYS_PTRACE
            - NET_ADMIN
            - SYS_RESOURCE
        volumeMounts:
        - name: sys
          mountPath: /sys
          readOnly: true
        - name: proc
          mountPath: /host/proc
          readOnly: true
        - name: debugfs
          mountPath: /sys/kernel/debug
        - name: cgroup
          mountPath: /sys/fs/cgroup
          readOnly: true
        - name: bpffs
          mountPath: /sys/fs/bpf
          mountPropagation: HostToContainer
        resources:
          requests:
            memory: "512Mi"
            cpu: "200m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
            
      volumes:
      - name: sys
        hostPath:
          path: /sys
      - name: proc
        hostPath:
          path: /proc
      - name: debugfs
        hostPath:
          path: /sys/kernel/debug
      - name: cgroup
        hostPath:
          path: /sys/fs/cgroup
      - name: bpffs
        hostPath:
          path: /sys/fs/bpf
          type: DirectoryOrCreate
```

### 4.3 Operator Pattern

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: pgquerylens-operator
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: postgresqlmonitors.pgquerylens.io
spec:
  group: pgquerylens.io
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              targetRef:
                type: object
                properties:
                  kind: string
                  name: string
                  namespace: string
                  
              connection:
                type: object
                properties:
                  host: string
                  port: integer
                  database: string
                  secretRef:
                    type: object
                    
              collection:
                type: object
                properties:
                  interval: string
                  features:
                    type: object
                    properties:
                      extension: boolean
                      ebpf: boolean
                      ash: boolean
                      planCapture: boolean
                      
              export:
                type: object
                properties:
                  mode: string
                  newrelic:
                    type: object
                  otlp:
                    type: object
                    
              resources:
                type: object
                properties:
                  requests:
                    type: object
                  limits:
                    type: object
---
# Example PostgreSQLMonitor resource
apiVersion: pgquerylens.io/v1
kind: PostgreSQLMonitor
metadata:
  name: production-postgres
  namespace: default
spec:
  targetRef:
    kind: StatefulSet
    name: postgres-cluster
    namespace: default
    
  connection:
    host: postgres-cluster
    port: 5432
    database: postgres
    secretRef:
      name: postgres-credentials
      
  collection:
    interval: 60s
    features:
      extension: true
      ebpf: true
      ash: true
      planCapture: true
      
  export:
    mode: hybrid
    newrelic:
      licenseKeySecret:
        name: newrelic-license
        key: key
      region: US
    otlp:
      endpoint: otel-collector.monitoring:4317
      
  resources:
    requests:
      memory: "256Mi"
      cpu: "100m"
    limits:
      memory: "512Mi"
      cpu: "500m"
```

## 5. Cloud-Specific Deployments

### 5.1 AWS RDS/Aurora

```yaml
# RDS-compatible deployment without extensions
apiVersion: batch/v1
kind: CronJob
metadata:
  name: rds-pgquerylens-collector
spec:
  schedule: "*/1 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: collector
            image: pgquerylens-collector:latest
            args:
            - --mode=rds
            - --fallback-mode  # No extension support
            env:
            - name: RDS_ENDPOINT
              value: postgres.abc123.us-east-1.rds.amazonaws.com
            - name: RDS_USER
              valueFrom:
                secretKeyRef:
                  name: rds-credentials
                  key: username
            - name: RDS_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rds-credentials
                  key: password
            - name: ENABLE_EXPLAIN_SAMPLING
              value: "true"  # Sample EXPLAIN plans
            - name: EXPLAIN_SAMPLE_RATE
              value: "0.01"  # 1% of queries
            - name: EXPLAIN_TIMEOUT_MS
              value: "100"
          restartPolicy: OnFailure
```

### 5.2 Google Cloud SQL

```yaml
# Cloud SQL with Cloud Run sidecar
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: pgquerylens-cloudsql
  annotations:
    run.googleapis.com/cloudsql-instances: PROJECT:REGION:INSTANCE
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/minScale: "1"
    spec:
      containers:
      - name: pgquerylens-collector
        image: gcr.io/PROJECT/pgquerylens-collector:latest
        env:
        - name: POSTGRES_HOST
          value: "/cloudsql/PROJECT:REGION:INSTANCE"
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: cloudsql-credentials
              key: username
        - name: COLLECTION_MODE
          value: "cloudsql"
        - name: EXPORT_TO_CLOUD_MONITORING
          value: "true"
        - name: EXPORT_TO_OTLP
          value: "true"
        - name: OTLP_ENDPOINT
          value: "otel-collector.monitoring:4317"
```

## 6. Systemd Deployment

```ini
# /etc/systemd/system/pgquerylens-collector.service
[Unit]
Description=PostgreSQL Query Lens Collector
Documentation=https://github.com/newrelic/pgquerylens
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=pgquerylens
Group=pgquerylens
ExecStartPre=/usr/local/bin/pgquerylens-collector --check-config
ExecStart=/usr/local/bin/pgquerylens-collector \
  --config /etc/pgquerylens/config.toml \
  --mode ${PGQUERYLENS_MODE}

Restart=always
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pgquerylens

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/pgquerylens /run/pgquerylens
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true
RestrictSUIDSGID=true
RemoveIPC=true

# Resource limits
LimitNOFILE=65536
MemoryLimit=1G
CPUQuota=50%
TasksMax=512

# eBPF capabilities (if needed)
AmbientCapabilities=CAP_SYS_ADMIN CAP_SYS_PTRACE
CapabilityBoundingSet=CAP_SYS_ADMIN CAP_SYS_PTRACE

[Install]
WantedBy=multi-user.target
```

## 7. Container Image Build

```dockerfile
# Multi-stage build for pgquerylens-collector
FROM golang:1.21 as builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION}" \
    -o pgquerylens-collector ./cmd/pgquerylens-collector

# eBPF assets
FROM ubuntu:22.04 as ebpf-builder
RUN apt-get update && apt-get install -y \
    clang llvm libbpf-dev linux-headers-generic
COPY bpf/ /build/bpf/
WORKDIR /build/bpf
RUN make

# Final image
FROM alpine:3.18
RUN apk add --no-cache ca-certificates

# Create user
RUN addgroup -g 1000 pgquerylens && \
    adduser -u 1000 -G pgquerylens -s /bin/sh -D pgquerylens

# Copy binaries
COPY --from=builder /build/pgquerylens-collector /usr/local/bin/
COPY --from=ebpf-builder /build/bpf/*.o /usr/local/lib/pgquerylens/bpf/

# Configuration
COPY config/default.toml /etc/pgquerylens/config.toml

USER pgquerylens
ENTRYPOINT ["pgquerylens-collector"]
CMD ["--config", "/etc/pgquerylens/config.toml"]
```

## 8. Helm Chart

```yaml
# helm/pgquerylens/values.yaml
replicaCount: 1

image:
  repository: pgquerylens-collector
  pullPolicy: IfNotPresent
  tag: "1.0.0"

mode: hybrid  # standalone, nri, otel, hybrid

postgresql:
  host: postgres.default.svc.cluster.local
  port: 5432
  database: postgres
  username: monitoring
  existingSecret: postgres-credentials
  passwordKey: password

collection:
  interval: 60s
  features:
    extension: true
    ebpf: true
    ash: true
    planCapture: true
    kernelMetrics: true

export:
  newrelic:
    enabled: true
    licenseKey:
      secretName: newrelic-license
      secretKey: key
    region: US
    
  otlp:
    enabled: true
    endpoint: otel-collector.monitoring:4317

resources:
  requests:
    memory: 256Mi
    cpu: 100m
  limits:
    memory: 512Mi
    cpu: 500m

securityContext:
  capabilities:
    add:
    - SYS_ADMIN
    - SYS_PTRACE

# Pod assignment
nodeSelector: {}
tolerations: []
affinity: {}
```

## 9. Ansible Deployment

```yaml
---
- name: Deploy pgquerylens-collector
  hosts: postgres_servers
  become: yes
  vars:
    pgquerylens_version: "1.0.0"
    pgquerylens_mode: "hybrid"
    
  tasks:
    - name: Create pgquerylens user
      user:
        name: pgquerylens
        system: yes
        shell: /sbin/nologin
        home: /var/lib/pgquerylens
        
    - name: Download and install pgquerylens
      unarchive:
        src: "https://github.com/newrelic/pgquerylens/releases/download/v{{ pgquerylens_version }}/pgquerylens_{{ pgquerylens_version }}_linux_amd64.tar.gz"
        dest: /usr/local/bin
        remote_src: yes
        owner: root
        group: root
        mode: '0755'
        
    - name: Create configuration directory
      file:
        path: /etc/pgquerylens
        state: directory
        owner: pgquerylens
        mode: '0755'
        
    - name: Deploy configuration
      template:
        src: pgquerylens-config.toml.j2
        dest: /etc/pgquerylens/config.toml
        owner: pgquerylens
        mode: '0600'
      notify: restart pgquerylens
        
    - name: Install PostgreSQL extension
      postgresql_ext:
        name: pg_querylens
        db: "{{ item }}"
        login_host: localhost
        login_user: postgres
        login_password: "{{ postgres_password }}"
      loop: "{{ databases }}"
      when: pgquerylens_extension_enabled
        
    - name: Configure shared_preload_libraries
      postgresql_set:
        name: shared_preload_libraries
        value: 'pg_querylens,pg_stat_statements'
      notify: restart postgresql
        
    - name: Deploy systemd service
      template:
        src: pgquerylens-collector.service.j2
        dest: /etc/systemd/system/pgquerylens-collector.service
      notify:
        - reload systemd
        - restart pgquerylens
        
    - name: Enable and start service
      systemd:
        name: pgquerylens-collector
        enabled: yes
        state: started
        daemon_reload: yes
        
  handlers:
    - name: reload systemd
      systemd:
        daemon_reload: yes
        
    - name: restart pgquerylens
      systemd:
        name: pgquerylens-collector
        state: restarted
        
    - name: restart postgresql
      systemd:
        name: postgresql
        state: restarted
```

## 10. Migration Strategy

```yaml
# Phased migration from existing deployments
migration:
  phase1_validation:
    description: "Deploy alongside existing monitoring"
    steps:
      - Deploy pgquerylens in shadow mode
      - Compare metrics with existing solution
      - Validate no performance impact
      
  phase2_dual_running:
    description: "Run both collectors in parallel"
    steps:
      - Enable dual export mode
      - Monitor both metric streams
      - Build confidence in new metrics
      
  phase3_cutover:
    description: "Switch to pgquerylens as primary"
    steps:
      - Update dashboards to use new metrics
      - Disable old collector
      - Enable advanced features
      
  rollback:
    description: "Emergency rollback procedure"
    steps:
      - Re-enable old collector
      - Disable pgquerylens
      - Investigate issues
```

## Summary

The enhanced deployment patterns provide:

1. **Single Binary**: One binary supports all deployment modes
2. **Auto-Detection**: Automatically detects running environment
3. **Drop-in Replacement**: Works as direct replacement for nri-postgresql
4. **Cloud Native**: First-class Kubernetes support with operators
5. **Multi-Cloud**: Optimized for AWS RDS, Google Cloud SQL, Azure Database
6. **Security Hardened**: Minimal privileges, security contexts
7. **Observable**: Self-monitoring and health checks
8. **Gradual Rollout**: Safe migration path from existing solutions
9. **Infrastructure as Code**: Full automation support
10. **Backward Compatible**: Preserves all existing deployment patterns