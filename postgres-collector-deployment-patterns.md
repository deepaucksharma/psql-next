# PostgreSQL Unified Collector - Deployment Patterns

## Deployment Architecture Overview

```yaml
deployment_patterns:
  - standalone_binary      # Direct binary installation
  - infrastructure_agent   # New Relic Infrastructure integration
  - otel_collector        # OpenTelemetry collector component
  - kubernetes_operator   # K8s native deployment
  - container_sidecar     # Sidecar pattern
  - daemonset_ebpf       # eBPF-enabled DaemonSet
  - library_embedding    # Embedded in applications
```

## 1. New Relic Infrastructure Agent Integration

### Binary Package Structure
```
/var/db/newrelic-infra/
├── newrelic-integrations/
│   ├── bin/
│   │   └── nri-postgresql         # Main binary
│   └── definitions/
│       └── postgresql-definition.yml
├── integrations.d/
│   └── postgresql-config.yml      # User configuration
└── custom-attributes.d/
    └── postgresql-attrs.yml       # Custom attributes
```

### Integration Definition
```yaml
# postgresql-definition.yml
name: com.newrelic.postgresql
description: PostgreSQL performance monitoring with enhanced metrics
protocol_version: 4
os: linux
data:
  - event_type: PostgresSlowQueries
    commands:
      - ./bin/nri-postgresql --metrics --mode slow_queries
  - event_type: PostgresWaitEvents
    commands:
      - ./bin/nri-postgresql --metrics --mode wait_events
  - event_type: PostgresBlockingSessions
    commands:
      - ./bin/nri-postgresql --metrics --mode blocking_sessions
  - event_type: PostgresIndividualQueries
    commands:
      - ./bin/nri-postgresql --metrics --mode individual_queries
  - event_type: PostgresExecutionPlanMetrics
    commands:
      - ./bin/nri-postgresql --metrics --mode execution_plans
  # Extended metrics (optional)
  - event_type: PostgresExtendedMetrics
    commands:
      - ./bin/nri-postgresql --metrics --mode extended
    when:
      env_exists: ENABLE_EXTENDED_METRICS
```

### Infrastructure Agent Configuration
```yaml
# /etc/newrelic-infra.yml
license_key: YOUR_LICENSE_KEY
enable_process_metrics: true
integrations_config_refresh: 60s

# Custom attributes for PostgreSQL monitoring
custom_attributes:
  environment: production
  service: postgresql
  cluster: primary

# Integration-specific configuration
integrations:
  - name: nri-postgresql
    interval: 60s
    timeout: 30s
    working_dir: /var/db/newrelic-infra/newrelic-integrations
```

## 2. OpenTelemetry Collector Deployment

### Collector Configuration
```yaml
# otel-collector-config.yaml
receivers:
  postgresql_unified:
    # Connection settings
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DB}
    
    # Collection settings
    collection_interval: 60s
    initial_delay: 10s
    
    # Feature configuration
    features:
      ohi_compatibility: true  # Maintain OHI metric names
      extended_metrics: true
      ebpf_enabled: ${env:ENABLE_EBPF}
      active_session_history: true
      
    # Sampling configuration
    sampling:
      mode: adaptive
      rules:
        - condition: "query_time > 1000ms"
          sample_rate: 1.0
        - condition: "query_count > 1000/min"
          sample_rate: 0.1

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000
  
  resource:
    attributes:
      - key: service.name
        value: postgresql
        action: upsert
      - key: service.namespace
        value: ${env:K8S_NAMESPACE}
        action: upsert
        
  transform/ohi_compat:
    metric_statements:
      - context: metric
        statements:
          # Transform to OHI-compatible names if needed
          - set(name, "postgres.slow_queries.count") where name == "postgresql_slow_query_count"
          - set(name, "postgres.wait_events.time") where name == "postgresql_wait_event_duration"

exporters:
  otlp:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_API_KEY}
  
  prometheus:
    endpoint: 0.0.0.0:9090
    namespace: postgresql
    const_labels:
      environment: ${env:ENVIRONMENT}

service:
  pipelines:
    metrics:
      receivers: [postgresql_unified]
      processors: [batch, resource, transform/ohi_compat]
      exporters: [otlp, prometheus]
```

### Kubernetes Deployment
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-collector-config
data:
  collector-config.yaml: |
    # OTel collector config from above
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres-collector
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres-collector
  template:
    metadata:
      labels:
        app: postgres-collector
    spec:
      serviceAccountName: postgres-collector
      containers:
      - name: collector
        image: postgres-unified-collector:latest
        command: ["postgres-unified-collector"]
        args: ["--config", "/etc/collector/config.yaml"]
        env:
        - name: POSTGRES_HOST
          value: "postgres-primary"
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: postgres-credentials
              key: username
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-credentials
              key: password
        volumeMounts:
        - name: config
          mountPath: /etc/collector
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: config
        configMap:
          name: postgres-collector-config
```

## 3. Kubernetes Operator Pattern

### Custom Resource Definition
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: postgresmonitors.monitoring.io
spec:
  group: monitoring.io
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
              postgresRef:
                type: object
                properties:
                  name: string
                  namespace: string
              monitoringConfig:
                type: object
                properties:
                  mode: string  # "otel", "nri", "hybrid"
                  interval: string
                  enableExtendedMetrics: boolean
                  enableEbpf: boolean
              outputConfig:
                type: object
                properties:
                  newrelic:
                    type: object
                  otlp:
                    type: object
```

### PostgresMonitor Resource
```yaml
apiVersion: monitoring.io/v1
kind: PostgresMonitor
metadata:
  name: production-postgres
spec:
  postgresRef:
    name: postgres-cluster
    namespace: database
  
  monitoringConfig:
    mode: hybrid
    interval: 60s
    enableExtendedMetrics: true
    enableEbpf: true
    
  outputConfig:
    newrelic:
      licenseKey:
        secretKeyRef:
          name: newrelic-license
          key: key
      region: US
    
    otlp:
      endpoint: otel-collector.monitoring:4317
      headers:
        - name: api-key
          valueFrom:
            secretKeyRef:
              name: otlp-credentials
              key: api-key
```

## 4. Container Sidecar Pattern

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  serviceName: postgres
  replicas: 3
  template:
    spec:
      containers:
      # Main PostgreSQL container
      - name: postgres
        image: postgres:15
        env:
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: password
        volumeMounts:
        - name: postgres-socket
          mountPath: /var/run/postgresql
        - name: pgdata
          mountPath: /var/lib/postgresql/data
          
      # Monitoring sidecar
      - name: postgres-monitor
        image: postgres-unified-collector:latest
        args:
        - --mode=sidecar
        - --socket=/var/run/postgresql/.s.PGSQL.5432
        env:
        - name: POSTGRES_USER
          value: postgres
        - name: OUTPUT_MODE
          value: hybrid
        - name: NRI_LICENSE_KEY
          valueFrom:
            secretKeyRef:
              name: newrelic-license
              key: key
        volumeMounts:
        - name: postgres-socket
          mountPath: /var/run/postgresql
          readOnly: true
        resources:
          requests:
            memory: "128Mi"
            cpu: "50m"
          limits:
            memory: "256Mi"
            cpu: "200m"
            
      volumes:
      - name: postgres-socket
        emptyDir: {}
      - name: pgdata
        persistentVolumeClaim:
          claimName: postgres-data
```

## 5. eBPF-Enabled DaemonSet

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: postgres-ebpf-collector
  namespace: monitoring
spec:
  selector:
    matchLabels:
      name: postgres-ebpf-collector
  template:
    metadata:
      labels:
        name: postgres-ebpf-collector
    spec:
      hostNetwork: true
      hostPID: true
      serviceAccountName: postgres-ebpf-collector
      containers:
      - name: ebpf-collector
        image: postgres-unified-collector:ebpf
        securityContext:
          privileged: true
          capabilities:
            add:
            - SYS_ADMIN
            - SYS_PTRACE
            - NET_ADMIN
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: COLLECTOR_MODE
          value: "ebpf"
        volumeMounts:
        - name: sys
          mountPath: /sys
          readOnly: true
        - name: proc
          mountPath: /proc
          readOnly: true
        - name: debug
          mountPath: /sys/kernel/debug
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: sys
        hostPath:
          path: /sys
      - name: proc
        hostPath:
          path: /proc
      - name: debug
        hostPath:
          path: /sys/kernel/debug
```

## 6. Docker Compose Deployment

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: mysecretpassword
      POSTGRES_DB: myapp
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - postgres_socket:/var/run/postgresql
    networks:
      - monitoring

  postgres-collector:
    image: postgres-unified-collector:latest
    depends_on:
      - postgres
    environment:
      POSTGRES_HOST: postgres
      POSTGRES_PORT: 5432
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: mysecretpassword
      POSTGRES_DB: myapp
      
      # Collection configuration
      COLLECTION_MODE: hybrid
      COLLECTION_INTERVAL: 60s
      ENABLE_EXTENDED_METRICS: "true"
      ENABLE_EBPF: "false"  # Not supported in container
      
      # Output configuration
      NRI_ENABLED: "true"
      NRI_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
      
      OTLP_ENABLED: "true"
      OTLP_ENDPOINT: http://otel-collector:4317
    volumes:
      - postgres_socket:/var/run/postgresql:ro
    networks:
      - monitoring
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '0.5'

  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command: ["--config=/etc/otel-collector-config.yaml"]
    volumes:
      - ./otel-collector-config.yaml:/etc/otel-collector-config.yaml
    ports:
      - "4317:4317"  # OTLP gRPC
      - "4318:4318"  # OTLP HTTP
      - "9090:9090"  # Prometheus metrics
    networks:
      - monitoring

volumes:
  postgres_data:
  postgres_socket:

networks:
  monitoring:
    driver: bridge
```

## 7. Systemd Service Deployment

```ini
# /etc/systemd/system/postgres-unified-collector.service
[Unit]
Description=PostgreSQL Unified Collector
Documentation=https://github.com/your-org/postgres-unified-collector
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=postgres-collector
Group=postgres-collector
ExecStart=/usr/local/bin/postgres-unified-collector \
  --config /etc/postgres-collector/config.toml
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=postgres-collector

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/postgres-collector

# Resource limits
LimitNOFILE=65536
MemoryLimit=512M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

## 8. Ansible Deployment Playbook

```yaml
---
- name: Deploy PostgreSQL Unified Collector
  hosts: postgres_servers
  become: yes
  vars:
    collector_version: "1.0.0"
    collector_mode: "hybrid"
    
  tasks:
    - name: Create collector user
      user:
        name: postgres-collector
        system: yes
        shell: /sbin/nologin
        home: /var/lib/postgres-collector
        
    - name: Download collector binary
      get_url:
        url: "https://github.com/your-org/postgres-unified-collector/releases/download/v{{ collector_version }}/postgres-unified-collector_linux_amd64"
        dest: /usr/local/bin/postgres-unified-collector
        mode: '0755'
        checksum: sha256:{{ collector_checksum }}
        
    - name: Create configuration directory
      file:
        path: /etc/postgres-collector
        state: directory
        owner: postgres-collector
        mode: '0755'
        
    - name: Deploy configuration
      template:
        src: collector-config.toml.j2
        dest: /etc/postgres-collector/config.toml
        owner: postgres-collector
        mode: '0600'
      notify: restart collector
        
    - name: Deploy systemd service
      copy:
        src: postgres-unified-collector.service
        dest: /etc/systemd/system/
      notify:
        - reload systemd
        - restart collector
        
    - name: Enable and start collector
      systemd:
        name: postgres-unified-collector
        enabled: yes
        state: started
        daemon_reload: yes
        
  handlers:
    - name: reload systemd
      systemd:
        daemon_reload: yes
        
    - name: restart collector
      systemd:
        name: postgres-unified-collector
        state: restarted
```

## 9. Terraform Module

```hcl
# modules/postgres-collector/main.tf
resource "kubernetes_namespace" "monitoring" {
  metadata {
    name = var.namespace
  }
}

resource "kubernetes_config_map" "collector_config" {
  metadata {
    name      = "${var.name}-config"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
  }
  
  data = {
    "config.yaml" = templatefile("${path.module}/templates/config.yaml.tpl", {
      postgres_host     = var.postgres_host
      postgres_port     = var.postgres_port
      collection_mode   = var.collection_mode
      enable_ebpf      = var.enable_ebpf
      otlp_endpoint    = var.otlp_endpoint
    })
  }
}

resource "kubernetes_deployment" "collector" {
  metadata {
    name      = var.name
    namespace = kubernetes_namespace.monitoring.metadata[0].name
  }
  
  spec {
    replicas = var.replicas
    
    selector {
      match_labels = {
        app = var.name
      }
    }
    
    template {
      metadata {
        labels = {
          app = var.name
        }
      }
      
      spec {
        service_account_name = kubernetes_service_account.collector.metadata[0].name
        
        container {
          name  = "collector"
          image = "${var.image_repository}:${var.image_tag}"
          
          env {
            name = "POSTGRES_PASSWORD"
            value_from {
              secret_key_ref {
                name = var.postgres_secret_name
                key  = "password"
              }
            }
          }
          
          volume_mount {
            name       = "config"
            mount_path = "/etc/collector"
          }
          
          resources {
            requests = {
              memory = var.memory_request
              cpu    = var.cpu_request
            }
            limits = {
              memory = var.memory_limit
              cpu    = var.cpu_limit
            }
          }
        }
        
        volume {
          name = "config"
          config_map {
            name = kubernetes_config_map.collector_config.metadata[0].name
          }
        }
      }
    }
  }
}
```

## Migration Strategy

```yaml
# Phased migration from OHI to Unified Collector
migration_phases:
  phase1_validation:
    duration: "1 week"
    steps:
      - Deploy unified collector in shadow mode
      - Compare metrics with existing OHI
      - Validate data accuracy
      - Performance testing
      
  phase2_pilot:
    duration: "2 weeks"
    steps:
      - Enable on 10% of instances
      - Monitor for issues
      - Gather feedback
      - Tune configuration
      
  phase3_rollout:
    duration: "4 weeks"
    steps:
      - Gradual rollout to 50%
      - Enable extended metrics
      - Monitor performance impact
      - Full rollout to 100%
      
  phase4_optimization:
    duration: "2 weeks"
    steps:
      - Enable eBPF features
      - Optimize sampling rules
      - Deprecate old OHI
      - Documentation update
```