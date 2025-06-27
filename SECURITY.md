# Security Guidelines

## Secrets Management

### Environment Variables Only

All sensitive configuration must be managed through environment variables:

✅ **Correct**:
```bash
# .env file
NEW_RELIC_LICENSE_KEY=your_key_here
NEW_RELIC_API_KEY=your_api_key_here
```

❌ **Incorrect**:
```toml
# config.toml - Never hardcode secrets
headers = [["api-key", "NRAK-actual-key-here"]]
```

### Files to Never Commit

The following files should never be committed to version control:

- `.env` - Contains all secrets
- `docker-config-file.toml` - May contain secrets
- `deployments/kubernetes/secrets.yaml` - Contains Kubernetes secrets
- Any file with actual API keys or license keys

### Configuration Templates

Always use placeholder values in configuration templates:

```toml
# Example configuration
headers = [
    ["api-key", "${NEW_RELIC_LICENSE_KEY}"]
]
```

## Database Security

### Connection Strings

- Use environment variables for database credentials
- Enable SSL/TLS for production deployments
- Use connection pooling with proper timeouts
- Rotate database passwords regularly

### Query Sanitization

The collector includes automatic PII detection:

```toml
# Enable query sanitization
sanitize_query_text = true
sanitization_mode = "smart"  # none, basic, smart
```

Sanitization modes:
- `none`: No sanitization (development only)
- `basic`: Remove literal values
- `smart`: Advanced PII detection and removal

## Network Security

### TLS Configuration

All New Relic communications use TLS encryption:

```toml
# OTLP endpoints are HTTPS by default
endpoint = "https://otlp.nr-data.net:4318"
```

### Firewall Rules

Required outbound connections:
- New Relic OTLP: `otlp.nr-data.net:4318` (US) or `otlp.eu01.nr-data.net:4318` (EU)
- PostgreSQL: Your database host and port

## Kubernetes Security

### Secrets Management

Use Kubernetes Secrets instead of environment variables:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: newrelic-credentials
  namespace: postgres-monitoring
type: Opaque
stringData:
  license-key: "your_license_key_here"
  api-key: "your_api_key_here"
```

### RBAC

Apply principle of least privilege:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: postgres-collector
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
```

## Container Security

### Image Security

- Use minimal base images (Alpine Linux)
- Scan images for vulnerabilities
- Pin specific image versions
- Avoid running as root user

### Resource Limits

Set appropriate resource limits:

```yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

## Monitoring Security

### Audit Logging

Monitor access to sensitive resources:
- Database connection attempts
- Configuration file access
- Secret retrieval from environment

### Alerting

Set up alerts for:
- Authentication failures
- Unusual metric collection patterns
- High resource usage
- Configuration changes

## Incident Response

### Credential Rotation

If credentials are compromised:

1. **Immediately rotate** New Relic license keys
2. **Update** all deployment configurations
3. **Restart** all collector instances
4. **Audit** logs for unauthorized access
5. **Review** git history for exposed secrets

### Security Updates

- Monitor security advisories for dependencies
- Update container base images regularly
- Apply security patches promptly
- Test updates in staging environment first

## Best Practices

### Development

- Never commit secrets to version control
- Use `.env.example` for templates
- Scan code for secrets before commits
- Use git hooks to prevent secret commits

### Production

- Enable query sanitization
- Use TLS for all connections
- Implement proper monitoring and alerting
- Regular security reviews and updates
- Backup and test credential rotation procedures

### Compliance

- Follow your organization's data governance policies
- Ensure query sanitization meets privacy requirements
- Document data flow and retention policies
- Regular compliance audits and reviews