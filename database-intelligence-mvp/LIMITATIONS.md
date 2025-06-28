# Honest Limitations & Workarounds

## Architectural Limitations

### 1. Single Instance Constraint

**Limitation**: The collector MUST run as a single instance due to file-based state storage.

**Impact**: 
- No horizontal scaling
- Single point of failure
- Limited throughput (~1000 plans/second max)

**Workaround**: 
- Use multiple collectors sharded by database
- Each collector manages a subset of databases
- No shared state between collectors

**Future Solution**: 
- External state store (Redis/Memcached)
- Distributed cache with consistent hashing
- Target: Q3 2024

### 2. No Automatic APM Correlation

**Limitation**: Cannot automatically link database queries to APM traces.

**Why It's Hard**:
```
Application → Connection Pool → Database Connection → Query
    ↓              ↓                    ↓              ↓
Trace ID      Lost Here            Lost Here    Not Propagated
```

**Current Workaround**:
- Manual correlation using timestamps
- Query fingerprint matching
- Duration-based correlation

**What's Required for True Solution**:
1. Database driver changes (all languages)
2. SQL comment injection standard
3. Database engine trace context support
4. Industry-wide adoption

**Realistic Timeline**: 2+ years

### 3. Limited MySQL Support

**Limitation**: No safe way to get execution plans in real-time.

**Technical Reasons**:
- No statement-level timeout in MySQL
- EXPLAIN can acquire metadata locks
- No read-safe EXPLAIN option
- Must run on primary (dangerous)

**Current Capability**: 
- Query digest metadata only
- Performance schema statistics
- Row examination metrics
- No actual execution plan details

**Workaround**:
- Use slow query log with full logging
- Performance schema events_statements_history
- Periodic manual EXPLAIN during maintenance
- Third-party tools (pt-query-digest)

**Future Enhancement**: 
- Safe stored procedure approach (Phase 2)
- MySQL 8.0+ specific optimizations
- Query rewrite suggestions

## Functional Limitations

### 4. Query Sampling Granularity

**Limitation**: Only analyzes single worst query per cycle.

**Rationale**:
- Minimize database impact
- Reduce data volume
- Focus on highest impact

**What You Miss**:
- Moderate performance issues
- Patterns across multiple queries
- Workload distribution insights

**Workaround**:
- Reduce collection interval (carefully)
- Multiple collectors for different query classes
- Rotate focus via configuration updates

### 5. Historical Data Gaps

**Limitation**: State storage is ephemeral and limited.

**Impact**:
- Deduplication window: 5 minutes max
- No long-term trending without New Relic
- State loss on pod restart

**Workaround**:
- Increase deduplication window (uses more memory)
- Export raw data for external analysis
- Frequent New Relic dashboard snapshots

### 6. PII Detection Limitations

**Limitation**: Regex-based PII detection is imperfect.

**What Can Leak**:
- Non-standard formats
- International phone numbers
- Custom identifiers
- Base64 encoded values

**Additional Safeguards**:
- Network isolation
- Encryption in transit
- Access logging
- Regular audit reviews

## Operational Limitations

### 7. Database Prerequisites

**Limitation**: Requires significant database configuration.

**PostgreSQL Requirements**:
- `pg_stat_statements` extension
- Possible restart needed
- DBA involvement required
- Read replica recommended

**MySQL Requirements**:
- Performance Schema enabled
- Restart required to enable
- Additional memory overhead
- No plan details available

**Workaround**:
- Partner with DBA team early
- Automate prerequisite checking
- Provide setup scripts
- Document rollback procedures

### 8. Resource Scaling

**Limitation**: Memory usage grows with database activity.

**Scaling Factors**:
- Plan size (can be large)
- Collection frequency
- Number of databases
- Deduplication cache

**Practical Limits**:
- ~10 databases per collector
- ~1GB memory per collector
- ~1000 plans/second throughput

**Workaround**:
- Multiple collector instances
- Increase memory limits
- Reduce sampling rates
- Shorter cache windows

### 9. Error Handling Gaps

**Limitation**: Some errors are silent failures.

**Examples**:
- Malformed plans not parsed
- State storage corruption
- Partial receiver failures

**Detection**:
- Monitor data flow rates
- Check collector error logs
- Validate data in New Relic

**Workaround**:
- Comprehensive logging
- Metric alerts on data flow
- Regular validation scripts

## Security Limitations

### 10. Credential Management

**Limitation**: Database credentials in environment variables.

**Risk**:
- Credentials visible in pod specs
- No automatic rotation
- Shared across databases

**Mitigation**:
- Use Kubernetes secrets
- Implement secret rotation
- Separate credentials per database
- Audit access regularly

### 11. Query Visibility

**Limitation**: Full query text is collected.

**Privacy Concern**:
- Even with PII scrubbing
- Query patterns reveal logic
- Potential competitive information

**Mitigation**:
- Implement query allowlists
- Additional scrubbing rules
- Data retention limits
- Access controls in New Relic

## Performance Limitations

### 12. Collection Latency

**Limitation**: Not real-time monitoring.

**Current Reality**:
- 60-second collection interval
- Processing delay: 5-10 seconds  
- New Relic ingestion: 10-30 seconds
- Total latency: ~2 minutes

**Not Suitable For**:
- Real-time alerting
- Interactive debugging
- Sub-minute issues

**Workaround**:
- Combine with APM for real-time
- Use for post-incident analysis
- Focus on trending and patterns

## Data Fidelity Limitations

### 13. Sampling Accuracy

**Limitation**: Statistical sampling may miss issues.

**What's Missed**:
- Rare but critical queries
- Intermittent problems
- Edge case scenarios

**Mitigation**:
- Adjust sampling rules regularly
- Always sample known problems
- Combine with other monitoring

### 14. Plan Format Variations

**Limitation**: Plan formats vary by version and configuration.

**Challenge Examples**:
- JSON vs XML vs Text formats
- Version-specific fields
- Custom PostgreSQL builds
- Cloud provider variations

**Current Handling**:
- Best-effort parsing
- Fallback to raw storage
- Manual analysis required

## Integration Limitations

### 15. Dashboard Capabilities

**Limitation**: Raw JSON plans in New Relic.

**User Experience**:
- No visual plan trees
- Manual JSON inspection
- Limited query correlation

**Workaround**:
- Export to external tools
- Custom parsing scripts
- Wait for UI enhancements

## Planning Around Limitations

### Honest Communication

When presenting to stakeholders:
1. Lead with capabilities
2. Be upfront about limitations
3. Provide clear workarounds
4. Set realistic timelines

### Incremental Value

Despite limitations, the MVP provides:
- Visibility into slow queries
- Basic performance metrics
- Foundation for enhancement
- Production-safe collection

### Clear Evolution Path

Each limitation has a path forward:
- Short-term workarounds
- Medium-term enhancements
- Long-term solutions
- Community contributions

Remember: These limitations make the MVP achievable. Perfect is the enemy of good, and good database observability is desperately needed now.