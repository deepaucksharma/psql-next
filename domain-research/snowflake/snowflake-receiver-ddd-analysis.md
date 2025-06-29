# Snowflake Receiver - Domain-Driven Design Analysis

## Executive Summary

The Snowflake receiver implements a cloud data warehouse monitoring system organized into five primary bounded contexts:

1. **Billing & Credits Context**: Tracks compute and service credit consumption
2. **Warehouse Performance Context**: Monitors query execution and resource utilization
3. **Query Analytics Context**: Analyzes query performance and patterns
4. **Storage Management Context**: Tracks storage usage and costs
5. **Security & Access Context**: Monitors authentication and session activity

## Bounded Contexts

### 1. Billing & Credits Context

**Core Responsibility**: Monitor Snowflake credit consumption across services and warehouses

**Aggregates**:
- **CreditMonitor** (Aggregate Root)
  - Tracks total credits used by service
  - Monitors warehouse-specific consumption
  - Aggregates over 24-hour windows
  - Categorizes by service type

**Value Objects**:
- **CreditAmount**: Decimal credit value
- **ServiceType**: WAREHOUSE_METERING, PIPE, etc.
- **BillingPeriod**: 24-hour aggregation window
- **WarehouseName**: Unique warehouse identifier

**Domain Services**:
- **CreditAggregator**: Sums credits by dimension
- **CostCalculator**: Converts credits to cost
- **UsageTrendAnalyzer**: Tracks consumption patterns

**Invariants**:
- Credits must be non-negative
- Service type must be predefined
- Billing period must be complete hours
- Warehouse names must exist in account

### 2. Warehouse Performance Context

**Core Responsibility**: Monitor virtual warehouse query execution and queueing

**Aggregates**:
- **WarehouseMonitor** (Aggregate Root)
  - Tracks query states (blocked, queued, executed)
  - Monitors warehouse utilization
  - Measures queue depths
  - Analyzes execution patterns

**Value Objects**:
- **QueryState**: BLOCKED, QUEUED, EXECUTED
- **WarehouseSize**: XS, S, M, L, XL, etc.
- **ExecutionTime**: Query duration metrics
- **QueueTime**: Wait time before execution

**Domain Services**:
- **QueueAnalyzer**: Monitors query backlog
- **BlockageDetector**: Identifies blocking queries
- **UtilizationCalculator**: Computes warehouse efficiency

**Invariants**:
- Query counts must be non-negative
- Warehouse must be active for metrics
- Queue time ≤ total execution time
- Blocked implies resource contention

### 3. Query Analytics Context

**Core Responsibility**: Provide detailed query performance analysis and optimization insights

**Aggregates**:
- **QueryAnalyzer** (Aggregate Root)
  - Collects detailed query metrics
  - Tracks compilation and execution times
  - Monitors data scanning volumes
  - Analyzes query complexity

**Value Objects**:
- **QueryType**: SELECT, INSERT, UPDATE, DELETE, etc.
- **BytesScanned**: Data volume processed
- **PartitionsScanned**: Table segments accessed
- **CompilationTime**: Query planning duration
- **ExecutionMetrics**: Detailed performance data

**Domain Services**:
- **QueryProfiler**: Extracts performance metrics
- **DataScanAnalyzer**: Tracks I/O patterns
- **CompilationOptimizer**: Identifies slow compiles
- **PartitionAnalyzer**: Monitors pruning efficiency

**Invariants**:
- Bytes scanned ≥ bytes written
- Compilation time < execution time (usually)
- Partition count must be positive
- Query type must be valid DML/DDL

### 4. Storage Management Context

**Core Responsibility**: Monitor storage utilization, costs, and data lifecycle

**Aggregates**:
- **StorageMonitor** (Aggregate Root)
  - Tracks storage by category
  - Monitors fail-safe storage
  - Measures stage storage
  - Calculates storage costs

**Value Objects**:
- **StorageType**: TABLE, STAGE, FAILSAFE
- **BytesStored**: Storage volume
- **RetentionPeriod**: Time travel/fail-safe duration
- **StorageLocation**: Database/schema/table hierarchy

**Domain Services**:
- **StorageCalculator**: Aggregates by type
- **RetentionAnalyzer**: Tracks time travel usage
- **StageCleaner**: Identifies unused stages
- **CostEstimator**: Projects storage costs

**Invariants**:
- Storage bytes must be non-negative
- Fail-safe ≤ total storage
- Stage storage separate from tables
- Retention period ≤ configured maximum

### 5. Security & Access Context

**Core Responsibility**: Monitor authentication, authorization, and session activity

**Aggregates**:
- **SecurityMonitor** (Aggregate Root)
  - Tracks login attempts
  - Monitors authentication failures
  - Analyzes session patterns
  - Reports security events

**Value Objects**:
- **LoginEvent**: Authentication attempt record
- **ErrorMessage**: Failure reason
- **SessionInfo**: Active session details
- **UserIdentity**: Username and client info

**Domain Services**:
- **AuthenticationTracker**: Records login attempts
- **FailureAnalyzer**: Categorizes auth failures
- **SessionManager**: Monitors active sessions
- **AnomalyDetector**: Identifies suspicious patterns

**Invariants**:
- Login attempts must have outcomes
- Error messages must be categorized
- Sessions require successful auth
- Failed logins tracked separately

## Domain Events

### Billing Events
- `CreditConsumed`: Service used credits
- `BillingThresholdExceeded`: Cost alert triggered
- `WarehouseCreditSpike`: Unusual consumption
- `BillingPeriodClosed`: Daily aggregation complete

### Performance Events
- `QueryQueued`: Query waiting for resources
- `QueryBlocked`: Resource contention detected
- `WarehouseOverloaded`: High queue depth
- `ExecutionDelayed`: Significant wait time

### Storage Events
- `StorageIncreased`: Significant growth detected
- `FailSafeActivated`: Data recovery storage
- `StageUnused`: Temporary storage waste
- `RetentionExpired`: Time travel limit reached

### Security Events
- `LoginFailed`: Authentication failure
- `BruteForceDetected`: Multiple failures
- `SessionAnomalous`: Unusual access pattern
- `PrivilegeEscalation`: Role change detected

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **Credit** | Billing | Snowflake compute unit | 1 credit = 1 hour compute |
| **Warehouse** | Compute | Virtual compute cluster | Must be running for queries |
| **Stage** | Storage | Temporary file storage | For data loading/unloading |
| **Fail-safe** | Storage | Disaster recovery storage | 7-day retention |
| **Time Travel** | Storage | Historical data access | 0-90 days configurable |
| **Query Profile** | Analytics | Execution details | Per-query metrics |
| **Snowpipe** | Ingestion | Continuous data loading | Auto-ingest from stages |
| **Account Usage** | Metadata | System views schema | 24-hour data latency |

## Anti-Patterns Addressed

1. **No Real-Time Queries**: Uses pre-aggregated ACCOUNT_USAGE
2. **No Heavy Scanning**: Queries limited to 24-hour windows
3. **No DDL Operations**: Read-only monitoring
4. **No User Data Access**: Metadata only
5. **No Warehouse Hogging**: Controlled query execution

## Architectural Patterns

### 1. Repository Pattern
```go
type snowflakeClient struct {
    db *sql.DB
}

func (c *snowflakeClient) FetchBillingMetrics() ([]billingMetric, error)
func (c *snowflakeClient) FetchWarehouseBillingMetrics() ([]whBillingMetric, error)
// ... other fetch methods
```

### 2. Parallel Collection Pattern
```go
func (s *snowflakeMetricsScraper) scrape(ctx context.Context) error {
    // Launch all collectors concurrently
    go s.scrapeBillingMetrics(ctx, errChan)
    go s.scrapeWarehouseBillingMetrics(ctx, errChan)
    // ... more concurrent scrapers
}
```

### 3. Error Aggregation Pattern
```go
func errorListener(ctx context.Context, errChan <-chan error) error {
    var errs error
    for err := range errChan {
        errs = multierr.Append(errs, err)
    }
    return errs
}
```

### 4. Time Window Pattern
```sql
WHERE start_time >= DATEADD(hour, -24, CURRENT_TIMESTAMP())
```

## Testing Strategy

### Unit Testing
- Mock snowflakeClient for scraper tests
- Test metric mapping logic
- Validate error aggregation
- Test configuration validation

### Integration Testing
- Test against Snowflake sandbox
- Validate all ACCOUNT_USAGE queries
- Test role permissions
- Verify metric accuracy

### Performance Testing
- Query execution time
- Concurrent collection efficiency
- Memory usage with large results
- Warehouse credit consumption

## Performance Considerations

1. **24-Hour Windows**: Limits data volume per query
2. **Parallel Execution**: Concurrent metric collection
3. **ACCOUNT_USAGE Schema**: Pre-aggregated data
4. **Batch Processing**: All metrics in one cycle
5. **Connection Pooling**: Reuse database connections

## Security Model

1. **Authentication**: Username/password required
2. **Authorization**: ACCOUNTADMIN role by default
3. **Read-Only Access**: SELECT on ACCOUNT_USAGE only
4. **No User Data**: Metadata and metrics only
5. **Audit Trail**: All queries logged by Snowflake

## Evolution Points

1. **Real-Time Metrics**: INFORMATION_SCHEMA integration
2. **Cost Optimization**: Warehouse recommendations
3. **Query Optimization**: Performance suggestions
4. **Multi-Account**: Organization-level monitoring
5. **Data Sharing**: Monitor shared database usage

## Error Handling Philosophy

1. **Partial Success**: Collect available metrics
2. **Clear Attribution**: Error includes query context
3. **Non-Fatal Failures**: Continue other collections
4. **Timeout Protection**: Query time limits
5. **Graceful Degradation**: Reduce functionality safely

## Conclusion

The Snowflake receiver exemplifies cloud-native DDD principles:
- Clear separation of billing, performance, and security concerns
- Rich domain model reflecting Snowflake's unique concepts
- Efficient use of pre-aggregated system data
- Security-conscious design with role-based access
- Scalable architecture for multi-warehouse environments

The design successfully monitors Snowflake deployments comprehensively while minimizing performance impact and credit consumption.