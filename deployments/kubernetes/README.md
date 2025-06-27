# PostgreSQL Unified Collector - Kubernetes Deployment

This directory contains Kubernetes manifests and deployment scripts for the PostgreSQL Unified Collector.

## Prerequisites

- Kubernetes cluster (1.19+)
- kubectl configured to access your cluster
- docker for building images
- kustomize for resource management
- New Relic account with valid license key

## Quick Start

1. Ensure your `.env` file in the project root contains:
   ```
   NEW_RELIC_LICENSE_KEY=<your-license-key>
   NEW_RELIC_ACCOUNT_ID=<your-account-id>
   NEW_RELIC_API_KEY=<your-api-key>
   NEW_RELIC_REGION=US
   ```

2. Run the deployment script:
   ```bash
   ../../scripts/deploy-k8s.sh
   ```

This script will:
- Build the Docker image
- Update Kubernetes secrets with your New Relic license key
- Deploy all Kubernetes resources
- Wait for the deployment to be ready
- Verify the deployment health

## Manual Deployment

If you prefer to deploy manually:

1. Build the Docker image:
   ```bash
   docker build -f ../docker/Dockerfile -t postgres-unified-collector:latest ../..
   ```

2. Update the New Relic license key in kustomization.yaml:
   ```bash
   sed -i "s/NEWRELIC_LICENSE_KEY_PLACEHOLDER/your-actual-license-key/" kustomization.yaml
   ```

3. Apply the resources:
   ```bash
   kustomize build . | kubectl apply -f -
   ```

## Architecture

The deployment creates:

- **Namespace**: `postgres-monitoring`
- **Deployment**: Single replica of the postgres-unified-collector
- **ConfigMap**: Collector configuration
- **Secrets**: PostgreSQL credentials and New Relic license key
- **Service**: Exposes health check (8080) and metrics (9090) endpoints
- **ServiceAccount**: With necessary RBAC permissions

## Health Checks

The collector exposes health endpoints on port 8080:

- `/health` - Liveness probe endpoint
- `/ready` - Readiness probe endpoint
- `/metrics` - Prometheus metrics (on port 9090)

## Configuration

The collector configuration is stored in a ConfigMap and can be modified by editing `configmap-patch.yaml`.

Key configuration options:
- `connection_string`: PostgreSQL connection details
- `collection_interval_secs`: How often to collect metrics
- `collection_mode`: Collection strategy (hybrid recommended)
- `outputs.nri.enabled`: Enable New Relic Infrastructure output
- `outputs.otlp.enabled`: Enable OpenTelemetry output

## Monitoring

### View Logs
```bash
kubectl logs -n postgres-monitoring -l app=postgres-collector -f
```

### Check Health
```bash
kubectl port-forward -n postgres-monitoring svc/postgres-collector-metrics 8080:8080
curl http://localhost:8080/health
```

### View Metrics
```bash
kubectl port-forward -n postgres-monitoring svc/postgres-collector-metrics 9090:9090
curl http://localhost:9090/metrics
```

## Troubleshooting

### Pod not starting
```bash
kubectl describe pod -n postgres-monitoring -l app=postgres-collector
```

### Check events
```bash
kubectl get events -n postgres-monitoring --sort-by='.lastTimestamp'
```

### Verify secrets
```bash
kubectl get secrets -n postgres-monitoring
```

### Test database connectivity
```bash
kubectl exec -it -n postgres-monitoring deployment/postgres-collector -- /bin/sh
# Then test connection manually
```

## Customization

### Using Kustomize

The deployment uses Kustomize for configuration management. You can customize:

1. Create an overlay directory:
   ```bash
   mkdir -p overlays/production
   ```

2. Create a kustomization.yaml in the overlay:
   ```yaml
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization
   
   bases:
     - ../../base
   
   patchesStrategicMerge:
     - deployment-patch.yaml
   ```

3. Apply your customized deployment:
   ```bash
   kustomize build overlays/production | kubectl apply -f -
   ```

## Security Considerations

- Secrets are managed via Kubernetes Secrets
- The collector runs as a non-root user (uid: 1000)
- Network policies can be added to restrict traffic
- Consider using sealed-secrets or external secret management for production