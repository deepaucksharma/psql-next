# New Relic OpenTelemetry Support for MySQL Entity Synthesis with Dimensional Metrics

New Relic provides comprehensive native support for OpenTelemetry dimensional metrics through its OTLP endpoints, with sophisticated entity synthesis capabilities for database monitoring. For MySQL instances and schemas, the platform offers configuration-driven entity creation, high-cardinality management, and production-ready implementation patterns that leverage OTEL semantic conventions.

The research reveals that New Relic's architecture is fundamentally optimized for delta metrics with exponential histograms, supporting up to 10.2M data points per minute for enterprise accounts. Entity synthesis relies on resource attributes matching predefined rules, while datapoint attributes enable dimensional analysis. The platform enforces cardinality limits through intelligent pruning rather than data dropping, ensuring observability continuity even under high-cardinality scenarios.

## Best practices for dimensional metric attributes in OTLP configurations

New Relic's OTLP endpoint performs optimally with specific configuration patterns for dimensional metrics. **Delta temporality** should be configured as the default (`OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta`) because New Relic's architecture is fundamentally a delta metrics system. Using cumulative temporality requires stateful translation, increases memory usage by 5-10x, and results in higher data ingest volumes. For histograms, **exponential bucket aggregation** (`OTEL_EXPORTER_OTLP_METRICS_DEFAULT_HISTOGRAM_AGGREGATION=base2_exponential_bucket_histogram`) provides automatic bucket boundary adjustment, highly compressed wire representation, and better alignment with New Relic's internal representation.

Resource attributes must follow specific patterns for entity synthesis. The critical attributes include `service.name` as the primary service identifier, `server.address` and `server.port` for database identification, and `db.system=mysql` to trigger MySQL-specific entity rules. Additional recommended attributes include `service.instance.id` for instance differentiation, `host.id` for infrastructure correlation, and `service.namespace` for environment classification. All resource attributes should remain constant for telemetry from the same entity.

Metric point attributes should focus on analytical dimensions while avoiding high-cardinality values. Follow OpenTelemetry semantic conventions like `db.operation`, `db.collection.name`, and `db.statement` for query-specific context. Exclude unbounded values such as user IDs, session IDs, or timestamps that could exponentially increase cardinality. The platform enforces a 255-byte limit on attribute names and 4095-byte limit on values, with automatic truncation for oversized data.

## How New Relic handles high-cardinality dimensions and metric aggregation

New Relic implements sophisticated cardinality management with account-level limits ranging from **3M to 10.2M data points per minute**, depending on subscription tier. These limits can be extended to 15M upon request. Cardinality is evaluated per UTC day (00:00:00 to 23:59:59), with per-metric limits defaulting to 100,000 unique time series but adjustable up to 1 million. When limits are exceeded, the platform continues storing raw data but halts creation of aggregated rollups (1-minute, 5-minute intervals) for the remainder of the day.

The enforcement approach prioritizes data retention over rejection. Rather than dropping data when cardinality limits are reached, New Relic preserves raw metrics for 30 days while stopping aggregate generation. This allows queries using the RAW keyword or time windows of 60 minutes or less to access all data. For sustained high-cardinality scenarios, the platform provides pruning capabilities to exclude specific attributes from long-term aggregates while maintaining raw data access.

Client-side management strategies include configuring OpenTelemetry SDK cardinality limits (default 2000 per instrument), using Views to drop high-cardinality attributes before export, and implementing transform processors in the OpenTelemetry Collector. A practical configuration example demonstrates dropping problematic attributes:

```yaml
processors:
  transform:
    metric_statements:
      - truncate_all(datapoint.attributes, 4095)
      - truncate_all(resource.attributes, 4095)
  
  # Remove high-cardinality attributes
  attributes:
    actions:
      - key: user.id
        action: delete
      - key: session.id
        action: delete
```

## Entity synthesis rules for custom database entities using OTEL metrics

Entity synthesis in New Relic operates through a configuration-driven framework that matches OpenTelemetry resource attributes against predefined rules to create entity GUIDs. For **MYSQL_INSTANCE** entities, the synthesis requires `server.address` as the primary identifier combined with `db.system=mysql` as a matching condition. The system generates unique entity identifiers based on these attributes and automatically creates relationships to host and service entities when corresponding attributes are present.

For **MYSQL_SCHEMA** entities, synthesis uses composite identifiers combining server address and database namespace:

```yaml
synthesis:
  rules:
    - compositeIdentifier:
        separator: "/"
        attributes:
          - server.address
          - db.namespace
      name: db.namespace
      conditions:
        - attribute: db.system
          value: "mysql"
```

The distinction between resource and datapoint attributes is critical for entity synthesis. Resource attributes attached to the OpenTelemetry Resource object identify the entity producing telemetry and remain constant across all metrics from that entity. These attributes drive entity creation, relationships, and tags. Datapoint attributes provide metric-specific context and dimensions for analysis but don't influence entity identity. This separation ensures efficient entity management while enabling rich dimensional analysis.

Entity lifecycle management defaults to 8 days after last telemetry received, with configurable options ranging from 1 day to 3 months. Tags can be added through resource attributes with the `tags.` prefix, resulting in searchable entity metadata. The framework automatically creates relationships when standard attributes match between entities, such as service-to-database connections through `db.connection_string` or host-to-database links via `host.id`.

## Specific examples of successful OTEL-based MySQL monitoring implementations

Production implementations demonstrate comprehensive MySQL monitoring through OpenTelemetry Collector configurations. A complete monitoring setup includes MySQL metric collection, host metrics correlation, and log aggregation:

```yaml
receivers:
  mysql:
    endpoint: localhost:3306
    username: ${MYSQL_USERNAME}
    password: ${MYSQL_PASSWORD}
    collection_interval: 60s
    statement_events:
      digest_text_limit: 120
      time_limit: 24h
      limit: 250

processors:
  resourcedetection:
    detectors: ["system", "env"]
  
  cumulativetodelta:  # Critical for New Relic compatibility
  
  attributes:
    actions:
      - key: service.name
        value: "mysql-prod-cluster"
        action: upsert
      - key: db.system
        value: "mysql"
        action: upsert

exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    compression: gzip
```

Kubernetes deployments leverage the OpenTelemetry Operator for automated collector management with MySQL monitoring. The configuration includes Kubernetes attribute enrichment for proper entity relationships and namespace-aware monitoring. Docker Compose environments provide simplified local development and testing setups with proper service dependencies and volume management.

Golden metrics configuration for MySQL entities includes queries per second, slow query rates, connection usage, and buffer pool statistics. These metrics appear in entity summary views and drive alerting policies:

```sql
-- High-value dashboard query examples
SELECT rate(sum(mysql.queries), 1 SECOND) as 'Queries/sec' 
FROM Metric WHERE db.system = 'mysql' TIMESERIES AUTO

SELECT percentage(latest(mysql.connection.count), latest(mysql.connection.max)) 
FROM Metric WHERE db.system = 'mysql' FACET service.name
```

## Performance considerations and limits for dimensional metrics

New Relic's OTLP endpoint enforces several performance-related limits. The **maximum payload size is 1MB** per POST request, with compression (gzip or zstd) highly recommended to maximize throughput. The platform supports up to **100,000 payloads per minute** per account, adjustable on a case-by-case basis. When rate limits are exceeded, the API returns HTTP 429 responses with `Retry-After` headers indicating when to resume sending data.

Batch processing configuration significantly impacts performance. Optimal batch sizes range from 1000-2000 metrics per batch with 1-10 second timeout windows. Memory management becomes critical at scale, with recommended memory limits of 256-512MB for collectors handling high-volume MySQL monitoring. The platform recommends OTLP/HTTP binary protobuf over gRPC for better robustness without performance reduction.

Cost implications scale with cardinality. A 200-node cluster with typical dimensional attributes can generate 1.8M custom metrics, potentially adding $68,000/month in costs. Cardinality management tools help identify expensive attributes, with pruning options to exclude specific dimensions from long-term storage while preserving 30-day raw access. Regular monitoring of cardinality contributors and strategic attribute selection optimize cost-to-value ratios.

## Configuring resource attributes vs datapoint attributes for entity creation

Resource attributes define the identity and relationships of MySQL entities within New Relic's entity model. These attributes must be configured at the OpenTelemetry Resource level and include:

```yaml
# Required for MySQL instance entity creation
server.address: "mysql-prod-01.company.com"
server.port: 3306
db.system: "mysql"
service.name: "mysql-cluster-prod"

# Recommended for enhanced entity context
service.instance.id: "mysql-node-1"
service.namespace: "production"
host.id: "prod-mysql-host-01"  # Links to infrastructure
db.version: "8.0.28"
```

Datapoint attributes provide dimensional context for metrics without affecting entity identity. These should be used for query-specific information, operational context, and analytical dimensions:

```yaml
# Query-level attributes (datapoint)
db.operation: "SELECT"
db.collection.name: "users"
db.statement: "SELECT * FROM users WHERE active = true"
db.rows_affected: 42

# Connection context (datapoint)
db.user: "app_user"
db.connection.pool: "primary"
mysql.connection.status: "active"
```

The OpenTelemetry Collector can enforce this separation through processor configuration, ensuring resource attributes remain at the resource level while metric-specific attributes stay with datapoints. This distinction is crucial for proper entity synthesis, as only resource attributes participate in entity creation rules.

## New Relic's support for OTEL semantic conventions for databases

New Relic fully supports OpenTelemetry database semantic conventions, with **MySQL achieving stable status** alongside PostgreSQL, Microsoft SQL Server, and MariaDB. The implementation follows the official OTEL specification for attribute naming, span generation, and metric semantics. Key stable attributes include `db.system.name`, `db.collection.name`, `db.namespace`, `db.operation.name`, and `db.query.summary`.

The platform's APM UI automatically recognizes database spans conforming to semantic conventions, providing dedicated database operations views with query analysis, performance trending, and dependency mapping. Query sanitization occurs by default, replacing literals with `?` placeholders to protect sensitive data while maintaining query structure for analysis. The span naming convention follows the pattern `{db.query.summary}` → `{db.operation.name} {target}` → `{target}` → `{db.system.name}`, with automatic fallbacks ensuring meaningful span names.

Migration to stable conventions is managed through the `OTEL_SEMCONV_STABILITY_OPT_IN` environment variable. Setting it to `database` emits only stable conventions, while `database/dup` provides a phased rollout approach by emitting both experimental and stable attributes. This ensures backward compatibility during the transition period while encouraging adoption of standardized attributes.

## Practical definition.yml configurations for database entities

Entity definition files configure how New Relic synthesizes MySQL entities from OpenTelemetry metrics. A complete MySQL instance definition includes synthesis rules, golden metrics, and dashboard configuration:

```yaml
# definitions/infra-mysql/definition.yml
domain: INFRA
type: MYSQL_INSTANCE
configuration:
  entityExpirationTime: EIGHT_DAYS
  alertable: true

synthesis:
  rules:
    - identifier: server.address
      name: server.address
      conditions:
        - attribute: db.system
          value: mysql
        - attribute: instrumentation.provider
          value: opentelemetry
      tags:
        - environment
        - cluster.name
        - db.version
```

Golden metrics configuration defines key performance indicators displayed in entity views:

```yaml
# definitions/infra-mysql/golden_metrics.yml
queries:
  - title: "Query Rate"
    query: "FROM Metric SELECT rate(sum(mysql.queries), 1 SECOND) WHERE entity.guid = '{entityGuid}'"
    unit: OPERATIONS_PER_SECOND
    
  - title: "Connection Usage"
    query: "FROM Metric SELECT percentage(average(mysql.connection.count), average(mysql.connection.max)) WHERE entity.guid = '{entityGuid}'"
    unit: PERCENTAGE
    
  - title: "InnoDB Buffer Pool Hit Rate"
    query: "FROM Metric SELECT 100 * (1 - rate(sum(mysql.innodb.buffer_pool.pages.read), 1 MINUTE) / rate(sum(mysql.innodb.buffer_pool.read_requests), 1 MINUTE)) WHERE entity.guid = '{entityGuid}'"
    unit: PERCENTAGE
```

Summary metrics appear in the Entity Explorer list view, providing at-a-glance health indicators:

```yaml
# definitions/infra-mysql/summary_metrics.yml
- title: "Active Connections"
  query: "FROM Metric SELECT latest(mysql.connection.count) WHERE entity.guid = '{entityGuid}'"
  unit: COUNT
  
- title: "Slow Queries/min"
  query: "FROM Metric SELECT rate(sum(mysql.slow_queries), 1 MINUTE) WHERE entity.guid = '{entityGuid}'"
  unit: RATE
```

These configurations create a complete observability experience for MySQL entities, from automatic discovery through OpenTelemetry metrics to rich visualization and alerting capabilities within New Relic's unified platform.