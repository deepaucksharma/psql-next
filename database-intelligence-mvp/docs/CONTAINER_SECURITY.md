# Container Security Configuration

This document outlines the comprehensive security measures implemented in the Database Intelligence MVP container configurations.

## Security Overview

The Database Intelligence MVP implements multiple layers of container security to protect against common attack vectors and ensure secure operation in both development and production environments.

## Security Measures Implemented

### 1. **Multi-Stage Dockerfile Security**

#### Base Image Security
- Uses Alpine Linux 3.19 (minimal attack surface)
- Regular security updates with `apk upgrade --no-cache`
- Removes package cache and temporary files
- Certificate updates for secure connections

#### Non-Root User
- Creates dedicated user `otel` (UID: 1000, GID: 1000)
- Runs all processes as non-root user
- Strict directory permissions (750)
- Home directory protection (700)

#### File System Security
- Read-only configuration files (`--chmod=640`)
- Secure file ownership (`--chown=otel:otel`)
- Removal of sensitive files from root

### 2. **Container Runtime Security**

#### Security Options
```yaml
security_opt:
  - no-new-privileges:true  # Prevents privilege escalation
```

#### Read-Only File Systems
```yaml
read_only: true
tmpfs:
  - /tmp                   # Writable temp directory
  - /var/run/postgresql    # Runtime directories
```

#### User Security
```yaml
user: "1000:1000"          # Run as non-root user
```

#### Capability Restrictions
```yaml
cap_drop:
  - ALL                    # Drop all capabilities
cap_add:
  - NET_BIND_SERVICE       # Only add necessary capabilities
```

### 3. **Network Security**

#### Port Binding
- **Development**: Bind to `127.0.0.1` (localhost only)
- **Production**: Use Docker secrets and internal networks
- No unnecessary port exposure

#### Network Isolation
```yaml
networks:
  default:
    driver: bridge
    ipam:
      config:
        - subnet: 172.24.0.0/16  # Isolated subnet
```

### 4. **Secrets Management**

#### Development Environment
- Uses stronger default passwords than typical examples
- Environment variables for configuration
- Clear separation from production

#### Production Environment (`docker-compose.production-secure.yml`)
- Docker secrets for all sensitive data
- External secret files (not in repository)
- No hardcoded credentials

```yaml
secrets:
  postgres_password:
    file: ./secrets/postgres_password.txt
  mysql_password:
    file: ./secrets/mysql_password.txt
  new_relic_license_key:
    file: ./secrets/new_relic_license_key.txt
```

### 5. **Database Security**

#### PostgreSQL Security
- **Development**: Improved default passwords
- **Production**: 
  - SCRAM-SHA-256 password encryption
  - SSL/TLS enforcement
  - Certificate-based authentication
  - Reduced logging for security

#### MySQL Security
- **Development**: Secure authentication plugins
- **Production**:
  - `caching_sha2_password` authentication
  - `require_secure_transport=ON`
  - SSL certificate configuration
  - Connection limits

### 6. **Monitoring Security**

#### Grafana Security
- Disabled user registration
- Disabled external services (Gravatar, analytics)
- Secure session configuration
- HTTPS enforcement in production
- Strong admin passwords

#### Prometheus Security
- Data retention limits
- External URL restrictions
- Non-root user execution

### 7. **Health Check Security**

#### Secure Health Checks
```yaml
healthcheck:
  test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:13133/"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 30s
```

## Container Configurations

### Available Configurations

1. **`docker-compose.yml`** - Development with security improvements
2. **`docker-compose.secure.yml`** - Development with Docker secrets
3. **`docker-compose.production-secure.yml`** - Production-ready secure configuration

### Configuration Comparison

| Feature | Development | Development + Secrets | Production Secure |
|---------|-------------|----------------------|-------------------|
| Passwords | Strong defaults | Docker secrets | Docker secrets |
| Port Binding | Localhost only | Localhost only | Internal + limited external |
| SSL/TLS | Optional | Optional | Required |
| Monitoring | All enabled | All enabled | Security-focused |
| User Privileges | Non-root | Non-root | Non-root + capabilities |
| File System | Read-only where possible | Read-only | Fully read-only |

## Security Best Practices

### Development
1. Use the improved `docker-compose.yml` for basic development
2. Use `docker-compose.secure.yml` to test secrets management
3. Never commit actual secret files
4. Regularly update base images

### Production
1. Always use `docker-compose.production-secure.yml`
2. Generate strong, unique secrets for each deployment
3. Use external secret management (Vault, AWS Secrets Manager, etc.)
4. Implement network policies and firewalls
5. Regular security scanning and updates

### Secret Management

#### Setting Up Secrets for Production

```bash
# Create secrets directory
mkdir -p secrets
chmod 700 secrets

# Generate strong passwords
openssl rand -base64 32 > secrets/postgres_password.txt
openssl rand -base64 32 > secrets/mysql_root_password.txt
openssl rand -base64 32 > secrets/mysql_password.txt
openssl rand -base64 32 > secrets/grafana_admin_password.txt

# Set usernames
echo "postgres" > secrets/postgres_user.txt
echo "dbuser" > secrets/mysql_user.txt

# Add your New Relic license key
echo "YOUR_ACTUAL_LICENSE_KEY" > secrets/new_relic_license_key.txt

# Set secure permissions
chmod 600 secrets/*.txt

# Add to .gitignore
echo "secrets/*.txt" >> .gitignore
```

## Security Monitoring

### Container Security Scanning

Regular scanning should be performed using tools like:
- Docker Security Scanning
- Trivy
- Clair
- Snyk

### Runtime Security

Monitor for:
- Privilege escalation attempts
- Unusual network activity
- File system modifications
- Resource exhaustion

## Compliance and Standards

The container security configuration addresses requirements from:

- **CIS Docker Benchmark**
- **NIST Container Security Guidelines**
- **OWASP Container Security Top 10**
- **SOC 2 Type II Controls**

## Security Incident Response

In case of security incidents:

1. **Immediate Actions**:
   - Stop affected containers
   - Isolate network access
   - Preserve logs for analysis

2. **Investigation**:
   - Analyze container logs
   - Check for privilege escalation
   - Verify secret integrity

3. **Recovery**:
   - Rotate all secrets
   - Update container images
   - Apply security patches

## Security Validation

### Testing Security Configuration

```bash
# Test container security
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd):/tmp/app:ro \
  aquasecurity/trivy config /tmp/app

# Validate Docker Compose security
docker-compose config --quiet

# Test network isolation
docker network inspect dbintel-network-dev
```

### Security Checklist

- [ ] No hardcoded secrets in images or configurations
- [ ] All containers run as non-root users
- [ ] Read-only file systems where possible
- [ ] Minimal network exposure
- [ ] Security scanning implemented
- [ ] Secrets rotation procedures in place
- [ ] Monitoring and alerting configured
- [ ] Incident response plan documented

## Additional Resources

- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Container Security Guide](https://kubernetes.io/docs/concepts/security/)
- [OWASP Container Security](https://owasp.org/www-project-container-security/)
- [CIS Docker Benchmark](https://www.cisecurity.org/benchmark/docker)