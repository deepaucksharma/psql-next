# Production Security Configuration Guide

This guide provides comprehensive security configuration recommendations for deploying the Database Intelligence MVP in production environments.

## Security Overview

The Database Intelligence MVP implements defense-in-depth security across multiple layers:

1. **Application Security** - Secure coding practices and input validation
2. **Container Security** - Hardened container images and runtime security
3. **Network Security** - TLS encryption and network isolation
4. **Secrets Management** - Secure handling of credentials and API keys
5. **Access Control** - Authentication and authorization mechanisms
6. **Monitoring & Auditing** - Security event logging and alerting

## Configuration Files Overview

### Available Security Configurations

| Configuration File | Purpose | Security Level | Use Case |
|-------------------|---------|----------------|----------|
| `collector-secure.yaml` | Main secure collector config | High | Development with security |
| `production-secure.yaml` | Production-ready config | Maximum | Production deployment |
| `docker-collector-secure.yaml` | Docker-specific secure config | High | Docker development |
| `docker-compose.production-secure.yml` | Secure Docker Compose | Maximum | Production containers |

## Security Features Implemented

### 1. **Memory Protection**

All configurations include memory limiters to prevent OOM attacks:

```yaml
processors:
  memory_limiter:
    check_interval: 5s
    limit_mib: 512
    spike_limit_mib: 128
```

### 2. **Circuit Breaker Protection**

Prevents cascade failures and provides resilience:

```yaml
processors:
  circuitbreaker:
    failure_threshold: 5
    success_threshold: 3
    open_state_timeout: 60s
    max_concurrent_requests: 25
    memory_threshold_mb: 256
    cpu_threshold_percent: 85
```

### 3. **Secure Hash Algorithms**

Only SHA-256 is permitted for cryptographic operations:

```yaml
processors:
  planattributeextractor:
    hash_config:
      algorithm: "sha256"  # Only secure algorithm
```

### 4. **Data Sanitization**

Automatic removal of sensitive information:

```yaml
processors:
  transform:
    log_statements:
      - context: log
        statements:
          - delete_key(attributes, "password")
          - delete_key(attributes, "secret")
          - delete_key(attributes, "token")
```

### 5. **Query Anonymization**

Automatic anonymization of database queries:

```yaml
processors:
  planattributeextractor:
    query_anonymization:
      enabled: true
      attributes_to_anonymize:
        - "db.statement"
        - "db.query.text"
```

### 6. **Connection Security**

Secure database connections with proper timeouts:

```yaml
receivers:
  postgresql:
    timeout: 30s
    tls:
      insecure: false
      cert_file: "/etc/ssl/certs/postgres-client.crt"
      key_file: "/etc/ssl/private/postgres-client.key"
```

## Production Deployment Security

### 1. **Secrets Management**

#### Using Docker Secrets

```yaml
services:
  otel-collector:
    secrets:
      - postgres_password
      - mysql_password
      - new_relic_license_key
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password
```

#### Using External Secret Management

For production, integrate with:
- **HashiCorp Vault**
- **AWS Secrets Manager**
- **Azure Key Vault**
- **Google Secret Manager**
- **Kubernetes Secrets**

### 2. **TLS/SSL Configuration**

#### Database TLS Configuration

```yaml
receivers:
  postgresql:
    tls:
      insecure: false
      insecure_skip_verify: false
      cert_file: "/etc/ssl/certs/postgres-client.crt"
      key_file: "/etc/ssl/private/postgres-client.key"
      ca_file: "/etc/ssl/certs/postgres-ca.crt"
      server_name: "postgres.example.com"
```

#### Export TLS Configuration

```yaml
exporters:
  otlp/newrelic:
    tls:
      insecure: false
      insecure_skip_verify: false
```

### 3. **Network Security**

#### Port Binding Restrictions

```yaml
# Development
ports:
  - "127.0.0.1:8888:8888"  # Localhost only

# Production
ports:
  - "8888"  # Internal network only
```

#### Network Isolation

```yaml
networks:
  default:
    driver: bridge
    ipam:
      config:
        - subnet: 172.25.0.0/16
    driver_opts:
      com.docker.network.bridge.enable_icc: "false"
```

### 4. **Container Security**

#### Security Options

```yaml
services:
  otel-collector:
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
    user: "1000:1000"
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
```

## Security Validation Checklist

### Pre-Deployment Security Checklist

- [ ] **Secrets Management**
  - [ ] No hardcoded credentials in configurations
  - [ ] All secrets stored in external secret management
  - [ ] Secret rotation procedures implemented
  - [ ] Secrets properly scoped and permissioned

- [ ] **Network Security**
  - [ ] TLS enabled for all database connections
  - [ ] Certificate validation enabled
  - [ ] Network policies configured
  - [ ] Firewall rules implemented
  - [ ] Port exposure minimized

- [ ] **Container Security**
  - [ ] Non-root user configured
  - [ ] Read-only file systems where possible
  - [ ] Security contexts applied
  - [ ] Resource limits configured
  - [ ] Base images regularly updated

- [ ] **Application Security**
  - [ ] Input validation implemented
  - [ ] Memory limits configured
  - [ ] Circuit breakers enabled
  - [ ] Query timeouts set
  - [ ] Sanitization rules applied

- [ ] **Monitoring Security**
  - [ ] Security logs enabled
  - [ ] Anomaly detection configured
  - [ ] Alert rules defined
  - [ ] Incident response procedures documented

## Security Monitoring

### Key Security Metrics to Monitor

1. **Authentication Failures**
   - Failed database connections
   - Invalid API keys
   - Certificate validation failures

2. **Resource Usage**
   - Memory consumption spikes
   - CPU utilization anomalies
   - Network traffic patterns

3. **Error Patterns**
   - Circuit breaker activations
   - Connection timeouts
   - Export failures

4. **Data Privacy**
   - PII detection alerts
   - Query anonymization failures
   - Sensitive data leaks

### Alert Configuration

```yaml
# Example alert rules (adapt to your monitoring system)
alerts:
  - name: "High Authentication Failures"
    condition: "postgresql_connection_errors > 10/5m"
    severity: "warning"
    
  - name: "Memory Limit Exceeded"
    condition: "memory_usage > 90%"
    severity: "critical"
    
  - name: "Circuit Breaker Open"
    condition: "circuit_breaker_state == 'open'"
    severity: "warning"
```

## Security Incident Response

### Incident Classification

1. **P0 - Critical Security Incident**
   - Data breach or exposure
   - Unauthorized access detected
   - Service compromise

2. **P1 - High Security Incident**
   - Authentication bypass
   - Privilege escalation
   - Suspicious activity patterns

3. **P2 - Medium Security Incident**
   - Configuration drift
   - Certificate expiration
   - Anomalous usage patterns

### Response Procedures

#### Immediate Actions

1. **Assess and Contain**
   - Identify scope of incident
   - Isolate affected systems
   - Stop data collection if necessary

2. **Preserve Evidence**
   - Capture logs and metrics
   - Take system snapshots
   - Document timeline

3. **Notify Stakeholders**
   - Security team
   - Operations team
   - Management (for P0/P1)

#### Recovery Actions

1. **Remediation**
   - Apply security patches
   - Rotate compromised credentials
   - Update configurations

2. **Validation**
   - Verify fix effectiveness
   - Test security controls
   - Monitor for recurrence

3. **Documentation**
   - Update incident report
   - Document lessons learned
   - Update procedures

## Compliance and Auditing

### Regulatory Compliance

The security configurations support compliance with:

- **SOC 2 Type II**
- **ISO 27001**
- **GDPR** (with data anonymization)
- **HIPAA** (with additional controls)
- **PCI DSS** (for payment data)

### Audit Trail

Security events are logged for audit purposes:

```yaml
service:
  telemetry:
    logs:
      level: warn  # Log security events
      development: false
```

### Security Testing

Regular security testing should include:

1. **Vulnerability Scanning**
   - Container image scanning
   - Dependency vulnerability checks
   - Configuration security analysis

2. **Penetration Testing**
   - Network penetration testing
   - Application security testing
   - Social engineering assessments

3. **Compliance Audits**
   - Configuration compliance checks
   - Access control reviews
   - Data handling audits

## Best Practices Summary

### Development Environment

1. Use `collector-secure.yaml` for security-aware development
2. Enable query anonymization and data sanitization
3. Use localhost-only port binding
4. Implement proper logging and monitoring

### Staging Environment

1. Mirror production security configurations
2. Use production-like secrets management
3. Enable all security processors and filters
4. Perform security testing and validation

### Production Environment

1. Use `production-secure.yaml` configuration
2. Implement external secrets management
3. Enable comprehensive monitoring and alerting
4. Follow strict change management procedures
5. Maintain security documentation and procedures

### Continuous Security

1. **Regular Updates**
   - Keep base images updated
   - Apply security patches promptly
   - Update dependencies regularly

2. **Security Monitoring**
   - Monitor security metrics continuously
   - Review logs for anomalies
   - Investigate security alerts promptly

3. **Access Management**
   - Implement least privilege access
   - Regular access reviews
   - Strong authentication mechanisms

4. **Documentation**
   - Keep security documentation current
   - Document all security procedures
   - Maintain incident response playbooks

## Additional Resources

- [OWASP Container Security Guide](https://owasp.org/www-project-container-security/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [CIS Controls](https://www.cisecurity.org/controls/)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [OpenTelemetry Security Best Practices](https://opentelemetry.io/docs/concepts/security/)