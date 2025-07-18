# Secrets Management Guide

## Overview

The Database Intelligence platform includes comprehensive secrets management to ensure sensitive information like passwords, API keys, and certificates are never stored in plain text configuration files.

## Features

- **Multiple Provider Support**: Environment variables, Kubernetes secrets, HashiCorp Vault
- **Automatic Resolution**: Secrets are resolved at runtime from placeholders
- **Secure Caching**: Temporary caching with automatic expiration
- **Fallback Mechanism**: Tries multiple providers in order of preference

## Configuration Syntax

### Basic Secret Reference

Use the `${secret:KEY_NAME}` syntax in any configuration value:

```yaml
receivers:
  postgresql:
    username: ${secret:POSTGRES_USER}
    password: ${secret:POSTGRES_PASSWORD}
    
exporters:
  newrelic:
    api_key: ${secret:NEW_RELIC_API_KEY}
```

### Complex Secret References

Secrets can be embedded within strings:

```yaml
receivers:
  sqlquery:
    datasource: postgres://${secret:POSTGRES_USER}:${secret:POSTGRES_PASSWORD}@${secret:POSTGRES_HOST}:5432/mydb
```

## Supported Providers

### 1. Environment Variables (Default)

The simplest method - set environment variables before running the collector:

```bash
export POSTGRES_PASSWORD=my-secure-password
export NEW_RELIC_API_KEY=NRAK-XXXXXXXXXX
```

Or use a `.env` file:

```bash
# .env
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=my-secure-password
NEW_RELIC_API_KEY=NRAK-XXXXXXXXXX
```

### 2. Kubernetes Secrets

When running in Kubernetes, secrets are automatically read from mounted volumes:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: database-secrets
data:
  POSTGRES_PASSWORD: <base64-encoded-password>
  NEW_RELIC_API_KEY: <base64-encoded-api-key>
```

Mount in your deployment:

```yaml
volumeMounts:
  - name: secrets
    mountPath: /etc/secrets
    readOnly: true
volumes:
  - name: secrets
    secret:
      secretName: database-secrets
```

### 3. HashiCorp Vault (Future)

Vault integration for enterprise environments:

```bash
export VAULT_ADDR=https://vault.example.com
export VAULT_TOKEN=s.XXXXXXXXXX
```

## Running with Secrets

### Using the Secure Run Script

The easiest way to run with secrets:

```bash
# Create .env file with your secrets
cat > .env << EOF
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=my-password
POSTGRES_DATABASE=mydb
NEW_RELIC_API_KEY=NRAK-XXXXXXXXXX
EOF

# Run the collector
./scripts/run-with-secrets.sh enterprise configs/postgresql-maximum-extraction.yaml
```

### Manual Execution

```bash
# Export required variables
export POSTGRES_PASSWORD=my-password
export NEW_RELIC_API_KEY=NRAK-XXXXXXXXXX

# Run collector
./database-intelligence-collector --config configs/postgresql-maximum-extraction.yaml
```

## Best Practices

### 1. Never Commit Secrets

Add to `.gitignore`:

```
.env
.env.*
*.key
*.pem
*-secrets.yaml
```

### 2. Use Different Secrets Per Environment

```bash
# Development
./scripts/run-with-secrets.sh --env .env.development

# Production
./scripts/run-with-secrets.sh --env .env.production
```

### 3. Rotate Secrets Regularly

The platform supports dynamic secret updates without restart:

- Secrets are cached for only 5 minutes
- After cache expiration, new values are fetched
- No collector restart required

### 4. Audit Secret Access

Enable debug logging to track secret resolution:

```yaml
service:
  telemetry:
    logs:
      level: debug
```

## Common Secret Keys

### Database Connections

- `POSTGRES_HOST`: PostgreSQL hostname
- `POSTGRES_PORT`: PostgreSQL port (default: 5432)
- `POSTGRES_USER`: Database username
- `POSTGRES_PASSWORD`: Database password
- `POSTGRES_DATABASE`: Database name

### New Relic Integration

- `NEW_RELIC_API_KEY`: Account API key (NRAK-...)
- `NEW_RELIC_INGEST_KEY`: Ingest/Insert key (NRII-...)
- `NEW_RELIC_ACCOUNT_ID`: Account ID number
- `NEW_RELIC_REGION`: US or EU (default: US)

### OTLP Endpoints

- `OTLP_ENDPOINT`: OTLP receiver endpoint
- `OTLP_AUTH_TOKEN`: Authentication token
- `OTLP_CERT_PATH`: TLS certificate path
- `OTLP_KEY_PATH`: TLS key path

## Troubleshooting

### Secret Not Found

If you see errors like `failed to resolve secret 'KEY_NAME'`:

1. Check the environment variable is set:
   ```bash
   echo $KEY_NAME
   ```

2. Verify the exact key name (case-sensitive)

3. Check provider availability:
   ```bash
   # For Kubernetes
   ls /etc/secrets/
   
   # For Vault
   echo $VAULT_ADDR
   ```

### Invalid Secret Format

Ensure secrets don't contain:
- Unescaped special characters in URLs
- Newlines or carriage returns
- Leading/trailing whitespace

### Performance

If secret resolution is slow:

1. Check network connectivity to Vault
2. Verify Kubernetes secret mount permissions
3. Consider increasing cache duration (currently 5 minutes)

## Security Considerations

### 1. Minimal Permissions

Grant only necessary permissions:

```yaml
# Kubernetes RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: secret-reader
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]
```

### 2. Encryption at Rest

Ensure your secret storage encrypts data:

- Kubernetes: Enable encryption at rest
- Environment: Use encrypted filesystems
- Vault: Always encrypted

### 3. Network Security

- Use TLS for all connections
- Restrict network access to secret stores
- Monitor secret access patterns

### 4. Compliance

The secrets management system helps with:

- PCI DSS: No plaintext cardholder data
- HIPAA: Encrypted PHI storage
- SOC 2: Access controls and audit trails

## Examples

### Complete PostgreSQL Configuration

```yaml
receivers:
  postgresql:
    endpoint: ${secret:POSTGRES_HOST}:${secret:POSTGRES_PORT}
    username: ${secret:POSTGRES_USER}
    password: ${secret:POSTGRES_PASSWORD}
    databases:
      - ${secret:POSTGRES_DATABASE}
    tls:
      insecure: ${secret:POSTGRES_TLS_INSECURE}
      ca_file: ${secret:POSTGRES_CA_CERT}
      cert_file: ${secret:POSTGRES_CLIENT_CERT}
      key_file: ${secret:POSTGRES_CLIENT_KEY}
```

### Multi-Database Setup

```yaml
receivers:
  sqlquery/prod:
    datasource: ${secret:PROD_DB_CONNECTION_STRING}
    
  sqlquery/staging:
    datasource: ${secret:STAGING_DB_CONNECTION_STRING}
    
  sqlquery/dev:
    datasource: ${secret:DEV_DB_CONNECTION_STRING}
```

### Rate Limiting with Secrets

```yaml
exporters:
  nri:
    rate_limiting:
      database_limits:
        ${secret:PROD_DB_NAME}:
          rps: ${secret:PROD_RATE_LIMIT_RPS}
          burst: ${secret:PROD_RATE_LIMIT_BURST}