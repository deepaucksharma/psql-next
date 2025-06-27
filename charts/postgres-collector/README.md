# PostgreSQL Unified Collector Helm Chart

This Helm chart deploys the PostgreSQL Unified Collector for monitoring PostgreSQL databases with support for both New Relic Infrastructure (NRI) and OpenTelemetry (OTLP) output formats.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.8+
- PostgreSQL 12+ with `pg_stat_statements` enabled
- New Relic account with license key

## Installation

### Add the Helm repository (when published)

```bash
helm repo add newrelic https://helm.newrelic.com
helm repo update
```

### Install the chart

```bash
# Install with default values
helm install postgres-collector newrelic/postgres-collector \
  --set newrelic.licenseKey=YOUR_LICENSE_KEY \
  --set postgresql.password=YOUR_PASSWORD

# Install with custom values file
helm install postgres-collector newrelic/postgres-collector -f values.yaml
```

### Install from local directory

```bash
# From the charts directory
helm install postgres-collector ./postgres-collector \
  --set newrelic.licenseKey=YOUR_LICENSE_KEY \
  --set postgresql.password=YOUR_PASSWORD
```

## Configuration

See [values.yaml](values.yaml) for all available configuration options.

### Common Configuration Examples

#### Basic deployment with NRI mode

```yaml
collectorMode: nri
postgresql:
  host: my-postgres.example.com
  port: 5432
  user: monitoring
  password: "secure-password"
newrelic:
  licenseKey: "YOUR_LICENSE_KEY"
```

#### OTLP mode with custom endpoint

```yaml
collectorMode: otel
otlp:
  enabled: true
  endpoint: "http://my-otel-collector:4317"
  compression: gzip
```

#### Hybrid mode (both NRI and OTLP)

```yaml
collectorMode: hybrid
nri:
  enabled: true
otlp:
  enabled: true
  endpoint: "http://otel-collector:4317"
```

#### Multi-instance monitoring

```yaml
instances:
  - name: primary
    host: primary.example.com
    port: 5432
    database: postgres
    user: monitoring
    password: "password1"
    enabled: true
  - name: replica
    host: replica.example.com
    port: 5432
    database: postgres
    user: monitoring
    password: "password2"
    enabled: true
```

#### With PgBouncer monitoring

```yaml
pgbouncer:
  enabled: true
  host: pgbouncer.example.com
  port: 6432
  adminUser: pgbouncer
  password: "pgbouncer-password"
```

#### Using existing secrets

```yaml
postgresql:
  existingSecret: "my-postgres-secret"
  existingSecretPasswordKey: "postgres-password"
newrelic:
  existingSecret: "my-newrelic-secret"
  existingSecretLicenseKey: "license"
```

#### DaemonSet deployment

```yaml
deploymentMode: daemonset
nodeSelector:
  node-role.kubernetes.io/database: "true"
```

#### Resource limits and autoscaling

```yaml
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
```

## Deployment Modes

### Deployment (default)
- Standard Kubernetes deployment
- Suitable for monitoring remote PostgreSQL instances
- Supports autoscaling

### DaemonSet
- Runs on every node (or selected nodes)
- Ideal for monitoring PostgreSQL instances on the same nodes
- No autoscaling support

### Sidecar
- Deploy as a sidecar container in your PostgreSQL pod
- Configured separately in your PostgreSQL deployment

## Monitoring and Troubleshooting

### Check deployment status

```bash
helm status postgres-collector
kubectl get all -l app.kubernetes.io/name=postgres-collector
```

### View logs

```bash
kubectl logs -f deployment/postgres-collector
```

### Access health endpoint

```bash
kubectl port-forward deployment/postgres-collector 8080:8080
curl http://localhost:8080/health
```

### Access metrics endpoint

```bash
kubectl port-forward deployment/postgres-collector 9090:9090
curl http://localhost:9090/metrics
```

## Upgrading

```bash
helm upgrade postgres-collector newrelic/postgres-collector \
  --set newrelic.licenseKey=YOUR_LICENSE_KEY \
  --set postgresql.password=YOUR_PASSWORD
```

## Uninstalling

```bash
helm uninstall postgres-collector
```

## Values Reference

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `replicaCount` | int | `1` | Number of replicas (for deployment mode) |
| `image.repository` | string | `"newrelic/postgres-unified-collector"` | Image repository |
| `image.tag` | string | `""` | Image tag (defaults to chart appVersion) |
| `collectorMode` | string | `"hybrid"` | Collection mode: nri, otel, or hybrid |
| `deploymentMode` | string | `"deployment"` | Deployment type: deployment or daemonset |
| `postgresql.host` | string | `"postgresql"` | PostgreSQL host |
| `postgresql.port` | int | `5432` | PostgreSQL port |
| `postgresql.database` | string | `"postgres"` | Database to connect to |
| `postgresql.user` | string | `"monitoring"` | PostgreSQL user |
| `postgresql.password` | string | `""` | PostgreSQL password |
| `newrelic.licenseKey` | string | `""` | New Relic license key |
| `resources` | object | See values.yaml | Resource requests and limits |
| `autoscaling.enabled` | bool | `false` | Enable horizontal pod autoscaling |

See [values.yaml](values.yaml) for the complete list of configurable values.

## Support

For issues and feature requests, please open an issue in the [GitHub repository](https://github.com/newrelic/postgres-unified-collector).