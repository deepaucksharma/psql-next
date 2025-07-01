# Database Intelligence Helm Chart

This Helm chart deploys the OpenTelemetry-based Database Intelligence Collector for monitoring PostgreSQL and MySQL databases.

## Prerequisites

- Kubernetes 1.21+
- Helm 3.8.0+
- PV provisioner support in the underlying infrastructure (optional, for persistence)

## Installing the Chart

To install the chart with the release name `my-database-intelligence`:

```bash
helm repo add database-intelligence https://database-intelligence-mvp.github.io/helm-charts
helm repo update
helm install my-database-intelligence database-intelligence/database-intelligence \
  --set config.postgres.enabled=true \
  --set config.postgres.endpoint=my-postgres-host \
  --set config.postgres.username=monitoring \
  --set config.postgres.password=secretpassword \
  --set config.newrelic.licenseKey=YOUR_LICENSE_KEY
```

## Configuration

The following table lists the configurable parameters of the Database Intelligence chart and their default values.

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.imageRegistry` | Global Docker image registry | `""` |
| `global.imagePullSecrets` | Global Docker registry secret names as an array | `[]` |
| `global.storageClass` | Global StorageClass for Persistent Volume(s) | `""` |

### Common Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nameOverride` | String to partially override database-intelligence.fullname | `""` |
| `fullnameOverride` | String to fully override database-intelligence.fullname | `""` |
| `replicaCount` | Number of Database Intelligence replicas to deploy | `1` |

### Database Intelligence Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.registry` | Database Intelligence image registry | `docker.io` |
| `image.repository` | Database Intelligence image repository | `database-intelligence-mvp/database-intelligence-collector` |
| `image.tag` | Database Intelligence image tag | `1.0.0` |
| `image.pullPolicy` | Database Intelligence image pull policy | `IfNotPresent` |

### Configuration Parameters

#### PostgreSQL Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.postgres.enabled` | Enable PostgreSQL monitoring | `true` |
| `config.postgres.endpoint` | PostgreSQL endpoint | `postgresql.default.svc.cluster.local` |
| `config.postgres.port` | PostgreSQL port | `5432` |
| `config.postgres.username` | PostgreSQL username | `monitoring` |
| `config.postgres.password` | PostgreSQL password | `changeme` |
| `config.postgres.database` | PostgreSQL database | `postgres` |
| `config.postgres.sslmode` | PostgreSQL SSL mode | `disable` |
| `config.postgres.collectionInterval` | Collection interval for PostgreSQL metrics | `10s` |

#### MySQL Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.mysql.enabled` | Enable MySQL monitoring | `false` |
| `config.mysql.endpoint` | MySQL endpoint | `mysql.default.svc.cluster.local` |
| `config.mysql.port` | MySQL port | `3306` |
| `config.mysql.username` | MySQL username | `monitoring` |
| `config.mysql.password` | MySQL password | `changeme` |
| `config.mysql.database` | MySQL database | `mysql` |
| `config.mysql.collectionInterval` | Collection interval for MySQL metrics | `10s` |

#### New Relic Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.newrelic.licenseKey` | New Relic license key (required) | `""` |
| `config.newrelic.endpoint` | New Relic OTLP endpoint | `otlp.nr-data.net:4317` |
| `config.newrelic.environment` | Environment name for New Relic | `production` |

### Service Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Database Intelligence service type | `ClusterIP` |
| `service.ports.metrics` | Database Intelligence metrics port | `8888` |
| `service.ports.prometheus` | Database Intelligence prometheus port | `8889` |
| `service.ports.health` | Database Intelligence health port | `13133` |

### Resource Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU resource limits | `500m` |
| `resources.limits.memory` | Memory resource limits | `512Mi` |
| `resources.requests.cpu` | CPU resource requests | `200m` |
| `resources.requests.memory` | Memory resource requests | `256Mi` |

### Autoscaling Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `autoscaling.enabled` | Enable autoscaling | `false` |
| `autoscaling.minReplicas` | Minimum number of replicas | `1` |
| `autoscaling.maxReplicas` | Maximum number of replicas | `10` |
| `autoscaling.targetCPU` | Target CPU utilization percentage | `80` |
| `autoscaling.targetMemory` | Target Memory utilization percentage | `80` |

## Examples

### Basic PostgreSQL Monitoring

```bash
helm install my-collector database-intelligence/database-intelligence \
  --set config.postgres.enabled=true \
  --set config.postgres.endpoint=postgres.example.com \
  --set config.postgres.username=monitoring \
  --set config.postgres.password=mypassword \
  --set config.newrelic.licenseKey=YOUR_LICENSE_KEY
```

### MySQL Monitoring

```bash
helm install my-collector database-intelligence/database-intelligence \
  --set config.postgres.enabled=false \
  --set config.mysql.enabled=true \
  --set config.mysql.endpoint=mysql.example.com \
  --set config.mysql.username=monitoring \
  --set config.mysql.password=mypassword \
  --set config.newrelic.licenseKey=YOUR_LICENSE_KEY
```

### Both PostgreSQL and MySQL

```bash
helm install my-collector database-intelligence/database-intelligence \
  --set config.postgres.enabled=true \
  --set config.postgres.endpoint=postgres.example.com \
  --set config.mysql.enabled=true \
  --set config.mysql.endpoint=mysql.example.com \
  --set config.newrelic.licenseKey=YOUR_LICENSE_KEY
```

### With Autoscaling

```bash
helm install my-collector database-intelligence/database-intelligence \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=2 \
  --set autoscaling.maxReplicas=10 \
  --set config.newrelic.licenseKey=YOUR_LICENSE_KEY
```

### With Persistence

```bash
helm install my-collector database-intelligence/database-intelligence \
  --set persistence.enabled=true \
  --set persistence.size=10Gi \
  --set config.newrelic.licenseKey=YOUR_LICENSE_KEY
```

### With ServiceMonitor for Prometheus Operator

```bash
helm install my-collector database-intelligence/database-intelligence \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.namespace=monitoring \
  --set config.newrelic.licenseKey=YOUR_LICENSE_KEY
```

## Upgrading

To upgrade the chart:

```bash
helm upgrade my-database-intelligence database-intelligence/database-intelligence \
  --set config.newrelic.licenseKey=YOUR_LICENSE_KEY
```

## Uninstalling

To uninstall/delete the deployment:

```bash
helm delete my-database-intelligence
```

## Values File Example

Create a `values.yaml` file:

```yaml
replicaCount: 2

config:
  postgres:
    enabled: true
    endpoint: my-postgres.example.com
    port: 5432
    username: monitoring
    password: secretpassword
    database: postgres
    sslmode: require

  newrelic:
    licenseKey: YOUR_LICENSE_KEY_HERE
    environment: production

  sampling:
    defaultRate: 0.5
    rules:
      - name: slow_queries
        expression: 'attributes["db.statement.duration"] > 1000'
        sampleRate: 1.0

  piiDetection:
    enabled: true
    action: redact

resources:
  limits:
    cpu: 1
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPU: 70
  targetMemory: 80

persistence:
  enabled: true
  size: 20Gi
  storageClass: fast-ssd
```

Then install with:

```bash
helm install my-database-intelligence database-intelligence/database-intelligence -f values.yaml
```

## Troubleshooting

### Check Collector Logs

```bash
kubectl logs -n database-intelligence deployment/my-database-intelligence
```

### Check Collector Status

```bash
kubectl describe pod -n database-intelligence -l app.kubernetes.io/name=database-intelligence
```

### Verify Configuration

```bash
kubectl get configmap -n database-intelligence my-database-intelligence-config -o yaml
```

### Test Health Endpoint

```bash
kubectl port-forward -n database-intelligence svc/my-database-intelligence 13133:13133
curl http://localhost:13133/health
```

### Check Metrics

```bash
kubectl port-forward -n database-intelligence svc/my-database-intelligence 8888:8888
curl http://localhost:8888/metrics
```