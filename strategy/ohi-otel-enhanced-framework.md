# New Relic OHI to OpenTelemetry: Enhanced Strategic Migration Framework

## Table of Contents
1. [Refined Executive Summary](#refined-executive-summary)
2. [Phase 0: Semantic Analysis & Design](#phase-0-semantic-analysis--design)
3. [The Sample-to-Metric Paradigm Shift](#the-sample-to-metric-paradigm-shift)
4. [Robust Entity Correlation Strategy](#robust-entity-correlation-strategy)
5. [Organizational Change Management](#organizational-change-management)
6. [Enhanced Technical Implementation](#enhanced-technical-implementation)
7. [Intelligent Validation Framework](#intelligent-validation-framework)
8. [Cost & Performance Analysis](#cost--performance-analysis)
9. [Migration Guild & Enablement](#migration-guild--enablement)
10. [Semantic Translation Guide](#semantic-translation-guide)

## Refined Executive Summary

Migrating from New Relic's On-Host Integrations (OHIs) to OpenTelemetry is a strategic initiative to modernize our observability practice. Success requires more than achieving metric parity; it demands **true semantic equivalence** between the legacy Sample-based data model and the modern OpenTelemetry Metric-based model.

Our migration framework is built on a five-phase, data-driven approach that prioritizes three pillars of validation:

1. **Semantic Parity**: Ensuring the structure, attributes, and meaning of OTEL metrics are a true replacement for OHI Sample events, not just a name-for-name mapping.

2. **Entity Integrity**: Guaranteeing robust entity correlation through a prioritized hierarchy of unique identifiers, preventing data fragmentation and preserving our correlated observability experience.

3. **Operational Continuity**: Proactively managing the impact on dashboards, alerts, and user workflows through automated discovery, stakeholder enablement, and a managed transition.

By treating this as a **holistic socio-technical challenge**—not just a technical swap—we will mitigate risks related to cost, data quality, and user adoption, ensuring a seamless transition to a more powerful and standardized observability foundation.

## Phase 0: Semantic Analysis & Design

### Understanding the Fundamental Difference

Before any technical work begins, we must understand and document the paradigm shift:

#### OHI Sample Model
```json
// Single MysqlSample event contains ALL metrics
{
  "event_type": "MysqlSample",
  "timestamp": 1701432000,
  "hostname": "prod-mysql-01",
  "entity.guid": "MTIzNDU2Nzg5",
  "mysql.node.net.bytesReceivedPerSecond": 1024,
  "mysql.node.net.bytesSentPerSecond": 2048,
  "mysql.node.query.questionsPerSecond": 50,
  "mysql.node.query.slowQueriesPerSecond": 2,
  "mysql.node.innodb.bufferPoolPagesData": 1000,
  "mysql.node.innodb.bufferPoolPagesFree": 500,
  // ... 40+ more metrics in ONE event
}
```

#### OTEL Metric Model
```json
// Each metric is a SEPARATE data point
[
  {
    "metric": "mysql.net.bytes",
    "timestamp": 1701432000,
    "value": 1024,
    "attributes": {
      "hostname": "prod-mysql-01",
      "entity.guid": "MTIzNDU2Nzg5",
      "direction": "received",
      "otel.scope.name": "mysql"
    }
  },
  {
    "metric": "mysql.net.bytes",
    "timestamp": 1701432000,
    "value": 2048,
    "attributes": {
      "hostname": "prod-mysql-01",
      "entity.guid": "MTIzNDU2Nzg5",
      "direction": "sent",
      "otel.scope.name": "mysql"
    }
  }
  // ... each metric as separate event
]
```

### Semantic Analysis Tools

```python
# semantic_analyzer.py
import json
import subprocess
import yaml
from collections import defaultdict

class OHISemanticsAnalyzer:
    def __init__(self, ohi_name):
        self.ohi_name = ohi_name
        self.sample_structure = {}
        self.metric_groups = defaultdict(list)
        
    def capture_raw_payload(self):
        """Run OHI with verbose flag to capture actual payload"""
        cmd = f"/var/db/newrelic-infra/newrelic-integrations/bin/nri-{self.ohi_name} --verbose --pretty"
        result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
        
        # Parse the JSON output
        payload = json.loads(result.stdout)
        
        # Extract the sample structure
        for entity in payload.get('data', []):
            for metric in entity.get('metrics', []):
                event_type = metric.get('event_type')
                if event_type.endswith('Sample'):
                    self.sample_structure = metric
                    break
                    
        return self.sample_structure
        
    def analyze_metric_relationships(self):
        """Group related metrics that should be modeled together in OTEL"""
        
        for key, value in self.sample_structure.items():
            if key in ['event_type', 'timestamp', 'hostname', 'entity.guid']:
                continue
                
            # Group by common prefixes
            parts = key.split('.')
            if len(parts) >= 3:
                group = '.'.join(parts[:3])  # e.g., "mysql.node.net"
                self.metric_groups[group].append({
                    'original_name': key,
                    'value': value,
                    'type': self.infer_metric_type(key, value)
                })
                
    def infer_metric_type(self, name, value):
        """Determine if metric is gauge, counter, etc."""
        if 'PerSecond' in name or 'rate' in name.lower():
            return 'rate'
        elif 'total' in name.lower() or 'count' in name.lower():
            return 'counter'
        else:
            return 'gauge'
            
    def generate_otel_design(self):
        """Design optimal OTEL metric structure"""
        otel_design = {
            'metrics': [],
            'common_resource_attributes': {
                'service.name': self.ohi_name,
                'ohi.migration.version': '1.0'
            }
        }
        
        for group, metrics in self.metric_groups.items():
            # Can these be combined into one metric with attributes?
            if self._can_combine_metrics(metrics):
                otel_design['metrics'].append(self._design_combined_metric(group, metrics))
            else:
                # Keep as separate metrics
                for metric in metrics:
                    otel_design['metrics'].append(self._design_single_metric(metric))
                    
        return otel_design
        
    def _can_combine_metrics(self, metrics):
        """Determine if metrics can be combined with attributes"""
        # Example: bufferPoolPagesData and bufferPoolPagesFree can become
        # buffer_pool.pages with status="data" or status="free"
        
        names = [m['original_name'] for m in metrics]
        base_parts = [name.rsplit('.', 1)[0] for name in names]
        
        # If all metrics share the same base, they might be combinable
        return len(set(base_parts)) == 1 and len(metrics) > 1
        
    def _design_combined_metric(self, group, metrics):
        """Design a single OTEL metric with attributes"""
        base_name = group.replace('mysql.node.', 'mysql.')
        
        # Extract the varying part as attribute
        attribute_values = []
        for metric in metrics:
            varying_part = metric['original_name'].split('.')[-1]
            attribute_values.append(varying_part.lower())
            
        return {
            'name': f"{base_name}",
            'type': metrics[0]['type'],  # Assume same type
            'unit': self._infer_unit(metrics[0]['original_name']),
            'description': f"Combined metric for {group}",
            'attribute_key': base_name.split('.')[-1] + '_type',
            'attribute_values': attribute_values,
            'original_mappings': {m['original_name']: av for m, av in zip(metrics, attribute_values)}
        }
```

### Semantic Translation Guide Template

```yaml
# mysql-semantic-translation.yaml
ohi_name: mysql
sample_event_type: MysqlSample

semantic_mappings:
  # Network metrics - combine into single metric with direction attribute
  - ohi_group:
      - mysql.node.net.bytesReceivedPerSecond
      - mysql.node.net.bytesSentPerSecond
    otel_metric:
      name: mysql.network.io_bytes
      type: sum
      unit: By/s
      temporality: delta
      attributes:
        - key: direction
          values:
            mysql.node.net.bytesReceivedPerSecond: received
            mysql.node.net.bytesSentPerSecond: sent
            
  # Buffer pool - combine related gauges
  - ohi_group:
      - mysql.node.innodb.bufferPoolPagesData
      - mysql.node.innodb.bufferPoolPagesFree
      - mysql.node.innodb.bufferPoolPagesDirty
    otel_metric:
      name: mysql.innodb.buffer_pool.pages
      type: gauge
      unit: pages
      attributes:
        - key: state
          values:
            mysql.node.innodb.bufferPoolPagesData: data
            mysql.node.innodb.bufferPoolPagesFree: free
            mysql.node.innodb.bufferPoolPagesDirty: dirty
            
  # Query metrics - keep separate due to different meanings
  - ohi_metric: mysql.node.query.questionsPerSecond
    otel_metric:
      name: mysql.queries.executed
      type: sum
      unit: queries/s
      temporality: delta
      
  - ohi_metric: mysql.node.query.slowQueriesPerSecond
    otel_metric:
      name: mysql.queries.slow
      type: sum
      unit: queries/s
      temporality: delta
      attributes:
        - key: query.speed
          value: slow

# Query translation patterns
query_translations:
  - description: "Average queries per second"
    ohi_nrql: |
      SELECT average(mysql.node.query.questionsPerSecond)
      FROM MysqlSample
      WHERE hostname = 'prod-mysql-01'
    otel_nrql: |
      SELECT rate(sum(mysql.queries.executed), 1 second)
      FROM Metric
      WHERE hostname = 'prod-mysql-01'
      AND metricName = 'mysql.queries.executed'
      
  - description: "Buffer pool utilization"
    ohi_nrql: |
      SELECT 
        average(mysql.node.innodb.bufferPoolPagesData) as 'Data Pages',
        average(mysql.node.innodb.bufferPoolPagesFree) as 'Free Pages'
      FROM MysqlSample
    otel_nrql: |
      SELECT 
        filter(average(mysql.innodb.buffer_pool.pages), WHERE state = 'data') as 'Data Pages',
        filter(average(mysql.innodb.buffer_pool.pages), WHERE state = 'free') as 'Free Pages'
      FROM Metric
      WHERE metricName = 'mysql.innodb.buffer_pool.pages'
```

## The Sample-to-Metric Paradigm Shift

### Impact Analysis

```python
# paradigm_shift_analyzer.py
class ParadigmShiftAnalyzer:
    def __init__(self, nr_api_client):
        self.nr_api = nr_api_client
        
    def analyze_query_impact(self):
        """Analyze how the FROM clause change impacts existing queries"""
        
        # Find all dashboards using Sample queries
        dashboards = self.nr_api.get_all_dashboards()
        impacted_queries = []
        
        for dashboard in dashboards:
            for widget in dashboard.get('widgets', []):
                for query in widget.get('queries', []):
                    nrql = query.get('query', '')
                    if 'Sample' in nrql and 'FROM' in nrql:
                        impacted_queries.append({
                            'dashboard': dashboard['name'],
                            'widget': widget['title'],
                            'original_query': nrql,
                            'complexity': self._assess_query_complexity(nrql)
                        })
                        
        return impacted_queries
        
    def _assess_query_complexity(self, nrql):
        """Determine how complex the query translation will be"""
        complexity_score = 0
        
        # Multi-metric queries are harder
        if nrql.count('SELECT') > 1 or ',' in nrql.split('FROM')[0]:
            complexity_score += 2
            
        # Queries with WHERE clauses need attribute mapping
        if 'WHERE' in nrql:
            complexity_score += 1
            
        # FACET queries might need restructuring
        if 'FACET' in nrql:
            complexity_score += 1
            
        # Time-based correlations are complex
        if 'SINCE' in nrql and 'UNTIL' in nrql:
            complexity_score += 1
            
        return {
            'score': complexity_score,
            'level': 'high' if complexity_score >= 3 else 'medium' if complexity_score >= 2 else 'low'
        }
```

### Cost Impact Calculator

```python
# cost_impact_calculator.py
class CostImpactCalculator:
    def __init__(self):
        self.sample_cost_per_million = 0.30  # Hypothetical
        self.metric_cost_per_million = 0.25  # Hypothetical
        
    def calculate_cost_change(self, ohi_samples_per_minute, metrics_per_sample, 
                             collection_interval_seconds=60):
        """
        Calculate the cost difference between Sample and Metric models
        """
        # Current: One sample contains all metrics
        samples_per_month = ohi_samples_per_minute * 60 * 24 * 30
        current_cost = (samples_per_month / 1_000_000) * self.sample_cost_per_million
        
        # Future: Each metric is separate
        metrics_per_minute = ohi_samples_per_minute * metrics_per_sample
        metrics_per_month = metrics_per_minute * 60 * 24 * 30
        future_cost = (metrics_per_month / 1_000_000) * self.metric_cost_per_million
        
        # Additional factors
        attribute_overhead = 1.2  # 20% overhead for repeated attributes
        future_cost *= attribute_overhead
        
        return {
            'current_monthly_cost': current_cost,
            'projected_monthly_cost': future_cost,
            'percentage_change': ((future_cost - current_cost) / current_cost) * 100,
            'monthly_samples': samples_per_month,
            'monthly_metrics': metrics_per_month,
            'break_even_metrics_per_sample': self._calculate_break_even(attribute_overhead)
        }
        
    def _calculate_break_even(self, overhead):
        """How many metrics per sample before OTEL becomes more expensive?"""
        return self.sample_cost_per_million / (self.metric_cost_per_million * overhead)
```

## Robust Entity Correlation Strategy

### Entity Identifier Hierarchy

```yaml
# entity-correlation-config.yaml
entity_correlation:
  # Priority order - first non-empty value wins
  identifier_hierarchy:
    - name: cloud.provider.instance.id
      providers:
        aws: 
          attribute: "host.id"
          source: "ec2:instance-id"
        gcp:
          attribute: "host.id"
          source: "gce:instance-id"
        azure:
          attribute: "host.id"
          source: "azure:vm-id"
          
    - name: container.id
      providers:
        docker:
          attribute: "container.id"
          source: "/proc/self/cgroup"
        containerd:
          attribute: "container.id"
          source: "containerd://id"
          
    - name: kubernetes.pod.uid
      providers:
        k8s:
          attribute: "k8s.pod.uid"
          source: "downward-api"
          
    - name: system.host.id
      providers:
        linux:
          attribute: "host.id"
          source: "/etc/machine-id"
        windows:
          attribute: "host.id"
          source: "registry:MachineGuid"
          
    - name: hostname
      providers:
        all:
          attribute: "host.name"
          source: "os:hostname"
          warning: "Least reliable - use only as last resort"

validation_rules:
  - rule: "Cloud instances MUST have cloud.provider.instance.id"
    query: |
      SELECT count(*) 
      FROM Metric 
      WHERE cloud.provider IS NOT NULL 
      AND cloud.provider.instance.id IS NULL
      
  - rule: "Kubernetes pods MUST have k8s.pod.uid"
    query: |
      SELECT count(*)
      FROM Metric
      WHERE k8s.namespace.name IS NOT NULL
      AND k8s.pod.uid IS NULL
```

### Enhanced Resource Detection

```yaml
# enhanced-resourcedetection.yaml
processors:
  # MANDATORY: Comprehensive resource detection
  resourcedetection/complete:
    detectors: 
      # Cloud providers - MUST be first
      - ec2
      - gcp
      - azure
      
      # Container runtime
      - docker
      - containerd
      
      # Kubernetes - with full metadata
      - k8snode
      
      # System - fallback
      - system
      - env
      
    # Override settings for reliability
    system:
      hostname_sources: ["os", "dns", "lookup"]
      
    ec2:
      tags: ["Name", "Environment", "Application"]
      
    # Custom timeout for cloud metadata services
    timeout: 5s
    
    # Add custom attributes based on detection
    attributes:
      - key: entity.correlation.source
        value: "${detected.source}"
        action: insert
        
  # Additional processor to ensure critical attributes
  resource/entity_guarantee:
    attributes:
      # Ensure we ALWAYS have a host.id
      - key: host.id
        from_attribute: cloud.provider.instance.id
        action: insert
      - key: host.id
        from_attribute: container.id
        action: insert
        # Only if host.id is still not set
      - key: host.id
        from_attribute: host.name
        action: insert
        
      # Add correlation metadata
      - key: ohi.migration.entity.version
        value: "2.0"
        action: insert
```

### Entity Correlation Validator

```python
# entity_correlation_validator.py
class EntityCorrelationValidator:
    def __init__(self, nrql_client):
        self.nrql = nrql_client
        self.critical_failures = []
        
    def validate_entity_uniqueness(self, hostname):
        """Ensure OHI and OTEL create the same entity"""
        
        query = f"""
        SELECT 
          uniques(entity.guid) as entity_guids,
          latest(host.id) as host_id,
          latest(cloud.provider.instance.id) as cloud_id,
          latest(k8s.pod.uid) as k8s_id,
          latest(container.id) as container_id,
          count(*) as data_points
        FROM MysqlSample, Metric
        WHERE hostname = '{hostname}'
        FACET collection.method
        SINCE 1 hour ago
        """
        
        results = self.nrql.query(query)
        
        # Check if both methods produce exactly one entity
        ohi_entities = set()
        otel_entities = set()
        
        for result in results:
            if result['collection.method'] == 'ohi':
                ohi_entities.update(result['entity_guids'])
            else:
                otel_entities.update(result['entity_guids'])
                
        if len(ohi_entities) != 1 or len(otel_entities) != 1:
            self.critical_failures.append({
                'type': 'multiple_entities',
                'hostname': hostname,
                'ohi_entities': list(ohi_entities),
                'otel_entities': list(otel_entities)
            })
            return False
            
        if ohi_entities != otel_entities:
            self.critical_failures.append({
                'type': 'entity_mismatch',
                'hostname': hostname,
                'ohi_entity': list(ohi_entities)[0],
                'otel_entity': list(otel_entities)[0]
            })
            return False
            
        return True
        
    def identify_correlation_source(self, entity_guid):
        """Determine which identifier was used for correlation"""
        
        query = f"""
        SELECT 
          latest(host.id),
          latest(cloud.provider.instance.id),
          latest(k8s.pod.uid),
          latest(container.id),
          latest(host.name),
          latest(entity.correlation.source)
        FROM Metric
        WHERE entity.guid = '{entity_guid}'
        SINCE 1 hour ago
        """
        
        result = self.nrql.query(query)[0]
        
        # Determine primary identifier
        if result.get('cloud.provider.instance.id'):
            return 'cloud_instance', result['cloud.provider.instance.id']
        elif result.get('k8s.pod.uid'):
            return 'kubernetes', result['k8s.pod.uid']
        elif result.get('container.id'):
            return 'container', result['container.id']
        elif result.get('host.id') != result.get('host.name'):
            return 'machine_id', result['host.id']
        else:
            return 'hostname_fallback', result['host.name']
```

## Organizational Change Management

### Migration Guild Structure

```yaml
# migration-guild.yaml
migration_guild:
  structure:
    executive_sponsor:
      role: "VP of Engineering"
      responsibilities:
        - Remove organizational blockers
        - Approve resource allocation
        - Communicate strategic importance
        
    technical_lead:
      role: "Principal SRE"
      responsibilities:
        - Own technical implementation
        - Design validation framework
        - Make architectural decisions
        
    workstream_leads:
      - name: "Dashboard & Alerts Migration"
        owner: "Observability Team Lead"
        responsibilities:
          - Inventory all affected dashboards
          - Coordinate query rewrites
          - Validate alert functionality
          
      - name: "Training & Documentation"
        owner: "Developer Experience Lead"
        responsibilities:
          - Create migration guides
          - Develop training materials
          - Run hands-on workshops
          
      - name: "Application Integration"
        owner: "Platform Team Lead"
        responsibilities:
          - Update CI/CD pipelines
          - Modify deployment configs
          - Ensure smooth rollout
          
  meeting_cadence:
    weekly_standup:
      duration: 30min
      agenda:
        - Progress against milestones
        - Blockers and decisions needed
        - Next week priorities
        
    biweekly_steering:
      duration: 60min
      attendees: ["executive_sponsor", "all_leads"]
      agenda:
        - Strategic decisions
        - Resource needs
        - Risk review
```

### Stakeholder Communication Plan

```python
# communication_automation.py
class MigrationCommunicator:
    def __init__(self, notification_service, nr_api):
        self.notifier = notification_service
        self.nr_api = nr_api
        
    def send_phase_announcement(self, phase, affected_services):
        """Send targeted announcements based on impact"""
        
        # Identify stakeholders
        stakeholders = self.identify_stakeholders(affected_services)
        
        # Customize message by audience
        for group, members in stakeholders.items():
            message = self.craft_message(phase, group, affected_services)
            
            self.notifier.send(
                to=members,
                subject=f"OHI Migration Phase {phase}: Action Required",
                body=message,
                priority='high' if group == 'service_owners' else 'normal'
            )
            
    def identify_stakeholders(self, services):
        """Map services to stakeholder groups"""
        stakeholders = {
            'service_owners': [],
            'sre_on_call': [],
            'dashboard_users': [],
            'alert_recipients': []
        }
        
        for service in services:
            # Get service owners from tags
            entity = self.nr_api.get_entity(service)
            if entity.get('tags', {}).get('team'):
                stakeholders['service_owners'].extend(
                    self.get_team_members(entity['tags']['team'])
                )
                
            # Get dashboard users
            dashboards = self.nr_api.get_dashboards_using_entity(service)
            for dash in dashboards:
                stakeholders['dashboard_users'].extend(
                    self.get_dashboard_users(dash['id'])
                )
                
        return stakeholders
        
    def craft_message(self, phase, audience, services):
        """Create audience-specific messages"""
        
        templates = {
            'service_owners': """
Your service(s) {services} will be migrated from New Relic OHI to OpenTelemetry in Phase {phase}.

**What you need to do:**
1. Review the attached query translation guide
2. Update any custom dashboards using the old MysqlSample queries
3. Verify your alerts are still functioning after migration
4. Join the migration office hours on Thursday at 2 PM

**Resources:**
- Migration Guide: {guide_link}
- Query Translator Tool: {translator_link}
- Slack Channel: #ohi-migration-support
            """,
            
            'dashboard_users': """
Dashboards you use will be affected by the OHI to OpenTelemetry migration in Phase {phase}.

**What's changing:**
- Queries using "FROM MysqlSample" will need to change to "FROM Metric"
- Some metric names are changing (see guide)
- Historical data will remain available

**We'll handle:**
- All standard dashboards will be automatically updated
- Custom dashboards will be backed up before changes

**Resources:**
- New query examples: {examples_link}
- Migration FAQ: {faq_link}
            """
        }
        
        return templates[audience].format(
            services=', '.join(services),
            phase=phase,
            guide_link=self.get_guide_link(),
            translator_link=self.get_translator_link(),
            examples_link=self.get_examples_link(),
            faq_link=self.get_faq_link()
        )
```

### Dashboard & Alert Migration Automation

```python
# dashboard_migrator.py
class DashboardMigrator:
    def __init__(self, nr_api, translation_guide):
        self.nr_api = nr_api
        self.translator = QueryTranslator(translation_guide)
        
    def migrate_dashboard(self, dashboard_id, dry_run=True):
        """Migrate a dashboard from OHI to OTEL queries"""
        
        dashboard = self.nr_api.get_dashboard(dashboard_id)
        migration_report = {
            'dashboard_id': dashboard_id,
            'dashboard_name': dashboard['name'],
            'widgets_migrated': [],
            'warnings': [],
            'errors': []
        }
        
        # Create backup
        backup_id = self.create_backup(dashboard)
        migration_report['backup_id'] = backup_id
        
        # Process each widget
        for widget in dashboard['widgets']:
            widget_result = self.migrate_widget(widget)
            migration_report['widgets_migrated'].append(widget_result)
            
            if widget_result['warnings']:
                migration_report['warnings'].extend(widget_result['warnings'])
            if widget_result['errors']:
                migration_report['errors'].extend(widget_result['errors'])
                
        # Apply changes if not dry run
        if not dry_run and not migration_report['errors']:
            self.nr_api.update_dashboard(dashboard_id, dashboard)
            
        return migration_report
        
    def migrate_widget(self, widget):
        """Migrate queries in a widget"""
        result = {
            'widget_title': widget['title'],
            'queries_translated': 0,
            'warnings': [],
            'errors': []
        }
        
        for i, query in enumerate(widget.get('queries', [])):
            original_nrql = query['query']
            
            try:
                # Check if this needs migration
                if self.needs_migration(original_nrql):
                    translated = self.translator.translate(original_nrql)
                    
                    # Validate the translation
                    validation = self.validate_translation(
                        original_nrql, 
                        translated['nrql']
                    )
                    
                    if validation['is_valid']:
                        query['query'] = translated['nrql']
                        result['queries_translated'] += 1
                        
                        if translated.get('warnings'):
                            result['warnings'].extend(translated['warnings'])
                    else:
                        result['errors'].append({
                            'query_index': i,
                            'error': validation['error']
                        })
                        
            except Exception as e:
                result['errors'].append({
                    'query_index': i,
                    'error': str(e),
                    'original_query': original_nrql
                })
                
        return result
```

## Enhanced Technical Implementation

### Correct Metric Transformation

```yaml
# correct-metric-transformation.yaml
receivers:
  mysql:
    endpoint: "${MYSQL_HOST}:3306"
    username: "${MYSQL_USER}"
    password: "${MYSQL_PASSWORD}"
    collection_interval: 60s
    
processors:
  # Step 1: Ensure resource attributes for correlation
  resourcedetection/full:
    detectors: [ec2, system, docker, k8s]
    override: false
    
  # Step 2: Add migration metadata
  attributes/migration:
    actions:
      - key: instrumentation.source
        value: "opentelemetry"
        action: insert
      - key: migration.phase
        value: "${MIGRATION_PHASE}"
        action: insert
      - key: ohi.name
        value: "mysql"
        action: insert
        
  # Step 3: Handle rate calculations correctly
  # Note: metricstransform does NOT do rate calculation
  # We need to ensure the mysql receiver sends the right types
  
  # Step 4: Rename metrics to match OHI patterns (if needed)
  metricstransform/compatibility:
    transforms:
      # Simple renames
      - include: mysql.locks
        action: update
        new_name: mysql.node.innodb.rowLockWaits
        
      # Metrics that need unit conversion
      - include: mysql.buffer_pool_size
        action: update
        new_name: mysql.node.innodb.bufferPoolSizeBytes
        operations:
          - action: experimental_scale_value
            scale: 1048576  # Convert MB to bytes
            
  # Step 5: Handle OTEL conventions that differ from OHI
  attributes/semantic_convention:
    actions:
      # MySQL receiver uses 'mysql.instance.endpoint'
      # but New Relic expects 'hostname'
      - key: hostname
        from_attribute: mysql.instance.endpoint
        action: insert
        
  # Step 6: Memory management
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 25
    
  # Step 7: Batching aligned with OHI collection interval
  batch:
    timeout: 60s  # Match OHI interval
    send_batch_max_size: 1000

exporters:
  # Debug for validation
  logging/debug:
    loglevel: debug
    sampling_initial: 5
    sampling_thereafter: 100
    
  # New Relic export
  otlp/newrelic:
    endpoint: "https://otlp.nr-data.net"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"
    sending_queue:
      enabled: true
      queue_size: 1000
      storage: file_storage
      
  # Prometheus endpoint for internal metrics
  prometheus:
    endpoint: "0.0.0.0:9090"
    
extensions:
  file_storage:
    directory: /var/lib/otel/storage
    timeout: 10s

service:
  extensions: [file_storage]
  
  pipelines:
    metrics/mysql:
      receivers: [mysql]
      processors: [
        memory_limiter,
        resourcedetection/full,
        attributes/migration,
        attributes/semantic_convention,
        metricstransform/compatibility,
        batch
      ]
      exporters: [logging/debug, otlp/newrelic]
      
    # Monitor the collector itself
    metrics/internal:
      receivers: [prometheus]
      processors: [memory_limiter, batch]
      exporters: [otlp/newrelic]
```

### Handling Metric Types Correctly

```python
# metric_type_handler.py
class MetricTypeHandler:
    def __init__(self, semantic_guide):
        self.guide = semantic_guide
        
    def validate_metric_types(self, ohi_metric, otel_metric):
        """Ensure OTEL metric type matches OHI behavior"""
        
        ohi_type = self.guide.get_metric_type(ohi_metric)
        
        validation_rules = {
            'rate': self.validate_rate_metric,
            'counter': self.validate_counter_metric,
            'gauge': self.validate_gauge_metric
        }
        
        return validation_rules[ohi_type](ohi_metric, otel_metric)
        
    def validate_rate_metric(self, ohi_metric, otel_metric):
        """Rate metrics need special handling"""
        
        # OHI: Reports questionsPerSecond as a gauge (already calculated rate)
        # OTEL: Should report questions as a monotonic sum, let backend calculate rate
        
        checks = {
            'metric_type': otel_metric['type'] == 'Sum',
            'is_monotonic': otel_metric.get('sum', {}).get('is_monotonic', False),
            'temporality': otel_metric.get('sum', {}).get('aggregation_temporality') == 'DELTA',
            'query_uses_rate': 'rate(' in otel_metric['example_query']
        }
        
        return {
            'valid': all(checks.values()),
            'checks': checks,
            'recommendation': 'Use monotonic sum with delta temporality, apply rate() in NRQL'
        }
        
    def validate_gauge_metric(self, ohi_metric, otel_metric):
        """Gauge metrics should remain gauges"""
        
        checks = {
            'metric_type': otel_metric['type'] == 'Gauge',
            'has_timestamp': otel_metric.get('time_unix_nano') is not None
        }
        
        return {
            'valid': all(checks.values()),
            'checks': checks
        }
```

## Intelligent Validation Framework

### Enhanced Validation Suite

```python
# intelligent_validator.py
import numpy as np
from scipy import stats
from datetime import datetime, timedelta

class IntelligentValidator:
    def __init__(self, nrql_client, semantic_guide):
        self.nrql = nrql_client
        self.guide = semantic_guide
        self.validation_results = []
        
    async def validate_semantic_equivalence(self, ohi_metric, otel_metric, duration_hours=4):
        """Validate that OTEL metric semantically matches OHI metric"""
        
        # Collect time series data
        ohi_series = await self.collect_time_series(
            f"SELECT average({ohi_metric}) FROM MysqlSample",
            duration_hours
        )
        
        # Determine correct OTEL query based on metric type
        otel_query = self.build_equivalent_query(ohi_metric, otel_metric)
        otel_series = await self.collect_time_series(otel_query, duration_hours)
        
        # Statistical validation
        validation = {
            'metric': ohi_metric,
            'samples_collected': len(ohi_series),
            'correlation': stats.pearsonr(ohi_series, otel_series)[0],
            'mean_absolute_error': np.mean(np.abs(np.array(ohi_series) - np.array(otel_series))),
            'mean_percentage_error': np.mean(
                np.abs((np.array(ohi_series) - np.array(otel_series)) / np.array(ohi_series)) * 100
            ),
            'max_deviation': np.max(np.abs(np.array(ohi_series) - np.array(otel_series))),
            'passes_validation': None
        }
        
        # Determine if validation passes based on metric type
        metric_type = self.guide.get_metric_type(ohi_metric)
        
        if metric_type == 'gauge':
            # Gauges should match closely
            validation['passes_validation'] = (
                validation['correlation'] > 0.95 and
                validation['mean_percentage_error'] < 5
            )
        elif metric_type == 'rate':
            # Rates can have more variation due to calculation differences
            validation['passes_validation'] = (
                validation['correlation'] > 0.90 and
                validation['mean_percentage_error'] < 10
            )
        elif metric_type == 'counter':
            # Counters should match exactly (within rounding)
            validation['passes_validation'] = (
                validation['correlation'] > 0.99 and
                validation['mean_absolute_error'] < 1
            )
            
        return validation
        
    def build_equivalent_query(self, ohi_metric, otel_metric):
        """Build semantically equivalent OTEL query"""
        
        metric_type = self.guide.get_metric_type(ohi_metric)
        otel_name = self.guide.get_otel_name(ohi_metric)
        
        if metric_type == 'rate':
            # OHI presents as rate, OTEL needs rate() function
            return f"SELECT rate(sum({otel_name}), 1 second) FROM Metric WHERE metricName = '{otel_name}'"
        elif metric_type == 'gauge':
            # Direct average
            return f"SELECT average({otel_name}) FROM Metric WHERE metricName = '{otel_name}'"
        elif metric_type == 'counter':
            # Sum for counters
            return f"SELECT sum({otel_name}) FROM Metric WHERE metricName = '{otel_name}'"
            
    async def validate_collection_performance(self):
        """Compare collection performance between OHI and OTEL"""
        
        query = """
        SELECT 
          latest(otelcol_processor_batch_batch_send_size) as batch_size,
          latest(otelcol_exporter_queue_size) as queue_depth,
          latest(otelcol_exporter_send_failed_metric_points) as failed_points,
          latest(otelcol_processor_refused_metric_points) as refused_points,
          latest(otelcol_receiver_accepted_metric_points) as accepted_points,
          latest(otelcol_exporter_sent_metric_points) as sent_points
        FROM Metric
        WHERE service.name = 'otel-collector'
        SINCE 10 minutes ago
        """
        
        collector_metrics = await self.nrql.query(query)
        
        performance = {
            'throughput': collector_metrics['sent_points'] / 600,  # per second
            'error_rate': collector_metrics['failed_points'] / collector_metrics['accepted_points'],
            'queue_utilization': collector_metrics['queue_depth'] / 1000,  # assuming 1000 queue size
            'batch_efficiency': collector_metrics['batch_size'] / 1000  # assuming 1000 target
        }
        
        return performance
```

## Cost & Performance Analysis

### Comprehensive Cost Model

```python
# comprehensive_cost_analyzer.py
class ComprehensiveCostAnalyzer:
    def __init__(self, nr_api, billing_api):
        self.nr_api = nr_api
        self.billing = billing_api
        
    def analyze_migration_cost_impact(self, integration_name, test_duration_hours=24):
        """Full cost impact analysis including hidden costs"""
        
        analysis = {
            'integration': integration_name,
            'test_duration': test_duration_hours,
            'timestamp': datetime.now().isoformat()
        }
        
        # 1. Data Volume Analysis
        volume_analysis = self.analyze_data_volume(integration_name, test_duration_hours)
        analysis['volume'] = volume_analysis
        
        # 2. Cardinality Analysis
        cardinality_analysis = self.analyze_cardinality_impact(integration_name)
        analysis['cardinality'] = cardinality_analysis
        
        # 3. Query Performance Impact
        query_impact = self.analyze_query_performance()
        analysis['query_performance'] = query_impact
        
        # 4. Storage Impact
        storage_impact = self.estimate_storage_impact(volume_analysis)
        analysis['storage'] = storage_impact
        
        # 5. Total Cost Projection
        analysis['cost_projection'] = self.project_total_cost(analysis)
        
        return analysis
        
    def analyze_data_volume(self, integration, hours):
        """Compare data volumes between OHI and OTEL"""
        
        # OHI Sample volume
        ohi_query = f"""
        SELECT 
          count(*) as total_samples,
          uniqueCount(hostname) as unique_hosts,
          average(numeric(capture(toString(timestamp), r'(\d+)'))) as avg_timestamp
        FROM {integration}Sample
        SINCE {hours} hours ago
        """
        
        ohi_data = self.nr_api.nrql_query(ohi_query)
        
        # OTEL Metric volume
        otel_query = f"""
        SELECT 
          count(*) as total_metrics,
          uniqueCount(metricName) as unique_metrics,
          uniqueCount(hostname) as unique_hosts
        FROM Metric
        WHERE ohi.name = '{integration}'
        SINCE {hours} hours ago
        """
        
        otel_data = self.nr_api.nrql_query(otel_query)
        
        # Calculate ratios
        if ohi_data['total_samples'] > 0:
            expansion_ratio = otel_data['total_metrics'] / ohi_data['total_samples']
        else:
            expansion_ratio = 0
            
        return {
            'ohi_samples': ohi_data['total_samples'],
            'otel_metrics': otel_data['total_metrics'],
            'expansion_ratio': expansion_ratio,
            'unique_metrics': otel_data['unique_metrics'],
            'projected_monthly_ohi': ohi_data['total_samples'] * (720 / hours),
            'projected_monthly_otel': otel_data['total_metrics'] * (720 / hours)
        }
```

## Migration Guild & Enablement

### Training Program Structure

```yaml
# training-program.yaml
training_program:
  modules:
    - name: "OHI to OTEL Fundamentals"
      duration: "2 hours"
      format: "Workshop"
      topics:
        - Sample vs Metric paradigm
        - Entity correlation concepts
        - Cost implications
      hands_on:
        - Convert a simple MysqlSample query
        - Identify entity correlation attributes
        - Calculate cost impact
        
    - name: "Query Translation Deep Dive"
      duration: "3 hours"
      format: "Hands-on Lab"
      prerequisites: ["OHI to OTEL Fundamentals"]
      topics:
        - Complex query patterns
        - Performance optimization
        - Debugging missing data
      exercises:
        - Translate 10 production queries
        - Optimize high-cardinality queries
        - Debug entity correlation issues
        
    - name: "Dashboard Migration Workshop"
      duration: "2 hours"
      format: "Interactive Session"
      topics:
        - Using the migration tools
        - Testing and validation
        - Rollback procedures
      deliverables:
        - Migrated personal dashboard
        - Validation checklist
        - Rollback plan
        
  certification:
    name: "OTEL Migration Specialist"
    requirements:
      - Complete all modules
      - Successfully migrate a production dashboard
      - Pass knowledge check (80%)
    benefits:
      - Priority support during migration
      - Access to beta features
      - Recognition in guild meetings
```

### Change Impact Assessment

```python
# change_impact_assessor.py
class ChangeImpactAssessor:
    def __init__(self, nr_api, org_api):
        self.nr_api = nr_api
        self.org_api = org_api
        
    def assess_full_impact(self, integration_name):
        """Comprehensive impact assessment across technical and organizational dimensions"""
        
        impact = {
            'integration': integration_name,
            'assessment_date': datetime.now(),
            'technical_impact': {},
            'organizational_impact': {},
            'risk_score': 0
        }
        
        # Technical Impact
        tech_impact = self.assess_technical_impact(integration_name)
        impact['technical_impact'] = tech_impact
        
        # Organizational Impact
        org_impact = self.assess_organizational_impact(integration_name)
        impact['organizational_impact'] = org_impact
        
        # Calculate overall risk score
        impact['risk_score'] = self.calculate_risk_score(tech_impact, org_impact)
        
        # Generate recommendations
        impact['recommendations'] = self.generate_recommendations(impact)
        
        return impact
        
    def assess_technical_impact(self, integration):
        """Assess technical complexity and risk"""
        
        return {
            'affected_dashboards': self.count_affected_dashboards(integration),
            'affected_alerts': self.count_affected_alerts(integration),
            'query_complexity': self.assess_query_complexity(integration),
            'data_volume': self.assess_data_volume(integration),
            'cardinality_risk': self.assess_cardinality_risk(integration),
            'entity_correlation_complexity': self.assess_correlation_complexity(integration)
        }
        
    def assess_organizational_impact(self, integration):
        """Assess impact on teams and processes"""
        
        affected_teams = self.identify_affected_teams(integration)
        
        return {
            'affected_teams': affected_teams,
            'affected_users': self.count_affected_users(affected_teams),
            'critical_workflows': self.identify_critical_workflows(integration),
            'training_hours_required': self.estimate_training_hours(affected_teams),
            'support_ticket_projection': self.project_support_tickets(integration)
        }
        
    def generate_recommendations(self, impact):
        """Generate specific recommendations based on impact"""
        
        recommendations = []
        
        # High query complexity
        if impact['technical_impact']['query_complexity'] > 7:
            recommendations.append({
                'priority': 'HIGH',
                'action': 'Assign dedicated query translation expert',
                'reason': 'Complex queries require expert translation'
            })
            
        # Many affected teams
        if len(impact['organizational_impact']['affected_teams']) > 5:
            recommendations.append({
                'priority': 'HIGH',
                'action': 'Create integration-specific working group',
                'reason': 'Coordinate across multiple teams'
            })
            
        # High cardinality risk
        if impact['technical_impact']['cardinality_risk'] > 8:
            recommendations.append({
                'priority': 'CRITICAL',
                'action': 'Design cardinality reduction strategy first',
                'reason': 'Prevent cost explosion'
            })
            
        return recommendations
```

## Semantic Translation Guide

### Complete Translation Patterns

```yaml
# complete-translation-patterns.yaml
translation_patterns:
  # Pattern 1: Direct metric rename
  simple_rename:
    example:
      ohi: "mysql.node.net.maxUsedConnections"
      otel: "mysql.connections.max"
    nrql_pattern:
      ohi: "SELECT latest({metric}) FROM {integration}Sample"
      otel: "SELECT latest({metric}) FROM Metric WHERE metricName = '{metric}'"
      
  # Pattern 2: Rate calculation
  rate_conversion:
    example:
      ohi: "mysql.node.query.questionsPerSecond"  # Already a rate
      otel: "mysql.queries"  # Raw counter
    nrql_pattern:
      ohi: "SELECT average({metric}) FROM {integration}Sample"
      otel: "SELECT rate(sum({metric}), 1 second) FROM Metric WHERE metricName = '{metric}'"
      
  # Pattern 3: Attribute-based split
  attribute_split:
    example:
      ohi: 
        - "mysql.node.net.bytesReceivedPerSecond"
        - "mysql.node.net.bytesSentPerSecond"
      otel: "mysql.network.io"  # With direction attribute
    nrql_pattern:
      ohi: |
        SELECT 
          average(mysql.node.net.bytesReceivedPerSecond) as 'Received',
          average(mysql.node.net.bytesSentPerSecond) as 'Sent'
        FROM MysqlSample
      otel: |
        SELECT 
          filter(average(value), WHERE direction = 'received') as 'Received',
          filter(average(value), WHERE direction = 'sent') as 'Sent'
        FROM Metric
        WHERE metricName = 'mysql.network.io'
        
  # Pattern 4: Multi-metric correlation
  correlated_metrics:
    example:
      ohi: "Multiple metrics in single sample event"
      otel: "Multiple individual metric events"
    nrql_pattern:
      ohi: |
        SELECT 
          average(metric1) as 'm1',
          average(metric2) as 'm2',
          average(metric1) / average(metric2) as 'ratio'
        FROM MysqlSample
      otel: |
        SELECT 
          filter(average(value), WHERE metricName = 'mysql.metric1') as 'm1',
          filter(average(value), WHERE metricName = 'mysql.metric2') as 'm2',
          filter(average(value), WHERE metricName = 'mysql.metric1') / 
          filter(average(value), WHERE metricName = 'mysql.metric2') as 'ratio'
        FROM Metric
        WHERE metricName IN ('mysql.metric1', 'mysql.metric2')
        
  # Pattern 5: Time-based correlation
  temporal_correlation:
    description: "When you need metrics from the same collection moment"
    nrql_pattern:
      ohi: |
        SELECT average(metric1), average(metric2)
        FROM MysqlSample
        WHERE metric1 > 100 AND metric2 < 50
        TIMESERIES
      otel: |
        # More complex - requires subqueries or metric math
        SELECT 
          filter(average(m1.value), WHERE m1.value > 100) as 'metric1',
          average(m2.value) as 'metric2'
        FROM Metric m1, Metric m2
        WHERE m1.metricName = 'mysql.metric1' 
        AND m2.metricName = 'mysql.metric2'
        AND m1.timestamp = m2.timestamp
        AND m1.hostname = m2.hostname
        TIMESERIES
```

### Query Translation Tool

```python
# query_translator.py
import re
from typing import Dict, List, Tuple

class SemanticQueryTranslator:
    def __init__(self, semantic_guide):
        self.guide = semantic_guide
        self.translation_cache = {}
        
    def translate_query(self, ohi_query: str) -> Dict:
        """Translate OHI query to semantically equivalent OTEL query"""
        
        # Parse the OHI query
        parsed = self.parse_nrql(ohi_query)
        
        # Check cache
        cache_key = self.get_cache_key(parsed)
        if cache_key in self.translation_cache:
            return self.translation_cache[cache_key]
            
        # Perform translation
        result = {
            'original': ohi_query,
            'translated': None,
            'warnings': [],
            'requires_manual_review': False,
            'complexity_score': 0
        }
        
        try:
            # Identify the Sample type
            sample_type = self.extract_sample_type(parsed['from_clause'])
            
            # Get metrics used in the query
            metrics_used = self.extract_metrics(parsed['select_clause'])
            
            # Translate each metric
            translated_metrics = []
            for metric in metrics_used:
                translation = self.translate_metric(sample_type, metric)
                translated_metrics.append(translation)
                
                if translation.get('warning'):
                    result['warnings'].append(translation['warning'])
                    
            # Rebuild the query
            result['translated'] = self.rebuild_query(
                parsed, 
                translated_metrics,
                sample_type
            )
            
            # Calculate complexity
            result['complexity_score'] = self.calculate_complexity(
                parsed, 
                translated_metrics
            )
            
            # Flag for manual review if complex
            if result['complexity_score'] > 7:
                result['requires_manual_review'] = True
                result['warnings'].append(
                    "Complex query - manual review recommended"
                )
                
        except Exception as e:
            result['error'] = str(e)
            result['requires_manual_review'] = True
            
        # Cache the result
        self.translation_cache[cache_key] = result
        
        return result
        
    def translate_metric(self, sample_type: str, metric: str) -> Dict:
        """Translate individual metric based on semantic guide"""
        
        integration = sample_type.replace('Sample', '').lower()
        
        # Look up in semantic guide
        mapping = self.guide.get_metric_mapping(integration, metric)
        
        if not mapping:
            return {
                'original': metric,
                'translated': metric,  # Keep original if no mapping
                'warning': f"No mapping found for {metric}",
                'transform_required': False
            }
            
        translation = {
            'original': metric,
            'translated': mapping['otel_name'],
            'transform_required': False
        }
        
        # Check if transformation is needed
        if mapping.get('type') == 'rate' and mapping.get('otel_type') == 'counter':
            translation['transform_required'] = True
            translation['transform'] = 'rate'
            translation['rate_interval'] = mapping.get('rate_interval', '1 second')
            
        # Check for attribute-based metrics
        if mapping.get('attributes'):
            translation['attributes'] = mapping['attributes']
            translation['filter_required'] = True
            
        return translation
        
    def rebuild_query(self, parsed: Dict, translations: List[Dict], 
                     sample_type: str) -> str:
        """Rebuild query with translated metrics"""
        
        # Start with basic structure
        new_query_parts = []
        
        # SELECT clause
        select_parts = []
        for trans in translations:
            if trans.get('transform_required'):
                # Apply transformation
                if trans['transform'] == 'rate':
                    metric_expr = f"rate(sum({trans['translated']}), {trans['rate_interval']})"
                else:
                    metric_expr = trans['translated']
            else:
                metric_expr = f"average({trans['translated']})"
                
            # Apply filters if needed
            if trans.get('filter_required'):
                filters = []
                for attr, value in trans['attributes'].items():
                    filters.append(f"{attr} = '{value}'")
                metric_expr = f"filter({metric_expr}, WHERE {' AND '.join(filters)})"
                
            # Preserve aliases
            if trans.get('alias'):
                select_parts.append(f"{metric_expr} as '{trans['alias']}'")
            else:
                select_parts.append(metric_expr)
                
        new_query_parts.append(f"SELECT {', '.join(select_parts)}")
        
        # FROM clause
        new_query_parts.append("FROM Metric")
        
        # WHERE clause
        where_parts = []
        
        # Add metric name filter if single metric
        if len(translations) == 1 and not translations[0].get('filter_required'):
            where_parts.append(f"metricName = '{translations[0]['translated']}'")
            
        # Translate original WHERE conditions
        if parsed.get('where_clause'):
            translated_conditions = self.translate_where_conditions(
                parsed['where_clause'],
                translations
            )
            where_parts.extend(translated_conditions)
            
        if where_parts:
            new_query_parts.append(f"WHERE {' AND '.join(where_parts)}")
            
        # Other clauses (FACET, SINCE, etc.)
        for clause in ['facet_clause', 'since_clause', 'until_clause', 
                      'timeseries_clause', 'limit_clause']:
            if parsed.get(clause):
                new_query_parts.append(parsed[clause])
                
        return ' '.join(new_query_parts)
```

## Conclusion

This enhanced framework addresses the critical gaps identified in the analysis:

1. **Semantic Understanding**: Phase 0 explicitly analyzes the Sample vs Metric paradigm shift
2. **Robust Entity Correlation**: Hierarchical identifier strategy prevents fragmentation
3. **Organizational Readiness**: Migration Guild and comprehensive training program
4. **Technical Accuracy**: Correct processor usage and metric type handling
5. **Intelligent Validation**: Statistical analysis and semantic equivalence testing
6. **Cost Transparency**: Full cost model including hidden factors
7. **Query Translation**: Sophisticated patterns for complex query migrations

The framework now treats the migration as a holistic socio-technical transformation rather than just a technical swap, significantly increasing the probability of success.