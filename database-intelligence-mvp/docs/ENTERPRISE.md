# Database Intelligence Collector - Enterprise Edition

## Overview

The Database Intelligence Collector Enterprise Edition implements the 2025 best practices for OpenTelemetry and New Relic integration, providing a production-ready observability solution for database monitoring at scale.

### Key Enterprise Features

- **🏗️ Layered Architecture**: Agent → Gateway → New Relic pipeline
- **💰 Cost Control**: Intelligent data reduction with budget enforcement
- **🔒 Security**: mTLS, PII redaction, compliance-ready
- **🎯 Smart Sampling**: Tail-based sampling with business logic
- **📊 Integration Monitoring**: Proactive NrIntegrationError detection
- **🔧 Production Hardened**: Circuit breakers, health checks, graceful degradation

## Architecture

### Three-Tier Design

```
Applications → Agents (DaemonSet) → Gateway (Deployment) → New Relic
```

1. **Agent Layer**: Lightweight collectors on every node
2. **Gateway Layer**: Central processing and policy enforcement  
3. **New Relic**: AI-driven analysis and insights

### Custom Processors

#### Database-Specific (Original)
- **Adaptive Sampler**: Intelligent rule-based sampling
- **Circuit Breaker**: Database overload protection
- **Plan Extractor**: Query plan analysis with anonymization
- **Verification**: Data quality and PII detection

#### Enterprise (New)
- **Cost Control**: Budget enforcement and cardinality management
- **NR Error Monitor**: Proactive integration error detection

## Quick Start

### 1. Deploy Gateway

```bash
# Deploy the enterprise gateway
kubectl apply -f config/collector-gateway-enterprise.yaml

# For mTLS setup
./scripts/generate-mtls-certs.sh
kubectl apply -f certs/mtls-secrets.yaml
```

### 2. Deploy Agents

```bash
# Deploy agent DaemonSet
kubectl apply -f config/collector-agent-k8s.yaml
```

### 3. Configure Applications

```yaml
# Application OTLP configuration
exporters:
  otlp:
    endpoint: otel-agent:4317  # Local agent
    insecure: true  # Agent handles security
```

## Configuration Examples

### Cost Control

```yaml
processors:
  costcontrol:
    monthly_budget_usd: 5000
    price_per_gb: 0.35  # or 0.55 for Data Plus
    metric_cardinality_limit: 10000
    aggressive_mode: false  # Auto-enables when over budget
```

### Semantic Convention Enforcement

```yaml
processors:
  resource:
    attributes:
      # Critical for New Relic entity synthesis
      - key: service.name
        action: insert
      - key: host.id
        from_attribute: host.name
        action: insert
      - key: k8s.pod.uid
        action: insert
```

### PII Redaction

```yaml
processors:
  transform/pii:
    log_statements:
      - context: log
        statements:
          - replace_pattern(body, "\\b\\d{3}-\\d{2}-\\d{4}\\b", "[REDACTED-SSN]")
          - replace_pattern(body, "\\b(?:\\d{4}[\\s-]?){3}\\d{4}\\b", "[REDACTED-CC]")
```

## Monitoring

### Health Checks

```bash
# Gateway health
curl http://gateway:13133/health

# View live pipeline (zPages)
curl http://gateway:55679/debug/tracez
```

### Cost Monitoring

```sql
-- NRQL: Current month spend
SELECT sum(GigabytesIngested) * 0.35 as CostUSD
FROM NrMTD 
WHERE productLine = 'DataPlatform'
SINCE this month
```

### Integration Errors

```sql
-- NRQL: Check for data rejection
SELECT count(*) 
FROM NrIntegrationError 
WHERE newRelicFeature IN ('Metrics', 'Distributed Tracing')
FACET category, message
SINCE 1 hour ago
```

## Production Deployment Checklist

### Pre-Deployment
- [ ] Generate mTLS certificates
- [ ] Set monthly budget in cost control
- [ ] Configure semantic conventions
- [ ] Enable PII redaction patterns
- [ ] Set up tail sampling policies

### Deployment
- [ ] Deploy gateway with 3+ replicas
- [ ] Deploy agents as DaemonSet
- [ ] Configure applications for local OTLP
- [ ] Verify health endpoints
- [ ] Check data flow in zPages

### Post-Deployment
- [ ] Monitor NrIntegrationError events
- [ ] Validate entity correlation
- [ ] Check cost projections
- [ ] Verify sampling effectiveness
- [ ] Test circuit breaker triggers

## Troubleshooting

### No Data in New Relic

1. Check health endpoint: `curl http://collector:13133/health`
2. Verify OTLP connection: Check for 4xx/5xx responses
3. Query NrIntegrationError: Look for validation failures
4. Check semantic conventions: Ensure service.name is set

### High Costs

1. Check cardinality: Look for high-cardinality attributes
2. Review sampling: Ensure tail sampling is working
3. Enable aggressive mode: Activate cost control
4. Check data types: Logs often cost more than metrics

### Performance Issues

1. Monitor memory: Check memory_limiter metrics
2. Review batch sizes: Adjust for throughput
3. Check CPU: Profile with pprof endpoint
4. Scale horizontally: Add more replicas

## Best Practices

### Do's
- ✅ Use semantic conventions consistently
- ✅ Implement cost controls early
- ✅ Monitor integration errors
- ✅ Use layered architecture
- ✅ Enable health checks

### Don'ts
- ❌ Skip PII redaction
- ❌ Ignore cardinality
- ❌ Process in agents
- ❌ Hardcode secrets
- ❌ Disable health monitoring

## Migration from Standard Edition

1. **Week 1**: Deploy enterprise gateway alongside existing
2. **Week 2**: Route 10% traffic through new gateway
3. **Week 3**: Increase to 50%, monitor costs
4. **Week 4**: Complete migration, decommission old

## Support

- Documentation: See `/docs` directory
- Issues: GitHub Issues
- Community: #database-intelligence Slack

## License

Licensed under Apache 2.0. See LICENSE file.

---

**Version**: 2.0.0  
**Last Updated**: June 30, 2025  
**Status**: Production Ready