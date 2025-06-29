# Phase 1: Design & Mobilization — The "How" and "Who"

## Overview

Phase 1 transforms the strategic vision from Phase 0 into concrete technical designs, organizational structures, and actionable plans. This phase creates the detailed blueprint for execution.

## Table of Contents

1. [Phase Objectives](#phase-objectives)
2. [Semantic Translation Design](#semantic-translation-design)
3. [Technical Architecture](#technical-architecture)
4. [Validation Framework Design](#validation-framework-design)
5. [Organizational Mobilization](#organizational-mobilization)
6. [Training & Enablement Plan](#training--enablement-plan)
7. [Migration Work Queue](#migration-work-queue)
8. [Phase Artifacts](#phase-artifacts)
9. [Exit Criteria & Gate 1](#exit-criteria--gate-1)

## Phase Objectives

### Primary Objectives
1. Create comprehensive Semantic Translation Guide
2. Design scalable OTEL collector architecture
3. Establish validation and testing framework
4. Mobilize and train the Migration Guild
5. Build automated migration tooling

### Success Criteria
- 100% of REPLICATE/REDESIGN metrics have semantic mappings
- Architecture approved by review board
- Test plan covers all critical scenarios
- Migration Guild fully staffed and trained
- Automation tools tested and ready

## Semantic Translation Design

### Translation Guide Structure

```yaml
# semantic-translation-guide-v1.yaml
metadata:
  version: "1.0"
  last_updated: "2024-01-15"
  approved_by: "Technical Lead"
  
integrations:
  mysql:
    sample_event_type: "MysqlSample"
    otel_scope_name: "otelcol/mysqlreceiver"
    
    metrics:
      # Direct translations (REPLICATE)
      - ohi_metric: "mysql.node.net.maxUsedConnections"
        disposition: "REPLICATE"
        otel_metric: "mysql.connections.max"
        type: "gauge"
        unit: "connections"
        description: "Maximum number of used connections"
        
      # Rate conversions
      - ohi_metric: "mysql.node.query.questionsPerSecond"
        disposition: "REPLICATE"
        otel_metric: "mysql.statements"
        type: "sum"
        unit: "statements"
        temporality: "delta"
        monotonic: true
        notes: "OHI provides rate, OTEL provides counter. Apply rate() in queries"
        
      # Redesigned metrics with attributes
      - ohi_metric_group:
          - "mysql.node.net.bytesReceivedPerSecond"
          - "mysql.node.net.bytesSentPerSecond"
        disposition: "REDESIGN"
        otel_metric: "mysql.network.io"
        type: "sum"
        unit: "By"
        temporality: "delta"
        monotonic: true
        attributes:
          - key: "direction"
            values:
              "mysql.node.net.bytesReceivedPerSecond": "received"
              "mysql.node.net.bytesSentPerSecond": "sent"
              
      # Complex transformations
      - ohi_metric_group:
          - "mysql.node.innodb.bufferPoolPagesData"
          - "mysql.node.innodb.bufferPoolPagesFree"
          - "mysql.node.innodb.bufferPoolPagesDirty"
        disposition: "REDESIGN"
        otel_metric: "mysql.innodb.buffer_pool.pages"
        type: "gauge"
        unit: "pages"
        attributes:
          - key: "state"
            values:
              "mysql.node.innodb.bufferPoolPagesData": "data"
              "mysql.node.innodb.bufferPoolPagesFree": "free"
              "mysql.node.innodb.bufferPoolPagesDirty": "dirty"
```

### Query Translation Patterns

```python
# query_pattern_generator.py
class QueryPatternGenerator:
    def __init__(self, semantic_guide):
        self.guide = semantic_guide
        self.patterns = []
        
    def generate_query_patterns(self, integration):
        """Generate before/after query examples"""
        
        patterns = []
        metrics = self.guide['integrations'][integration]['metrics']
        
        for metric_def in metrics:
            if metric_def['disposition'] == 'REPLICATE':
                pattern = self._generate_simple_pattern(metric_def)
            elif metric_def['disposition'] == 'REDESIGN':
                pattern = self._generate_complex_pattern(metric_def)
            
            patterns.append(pattern)
            
        return patterns
    
    def _generate_simple_pattern(self, metric_def):
        """Generate pattern for simple metric translation"""
        
        ohi_metric = metric_def['ohi_metric']
        otel_metric = metric_def['otel_metric']
        
        if metric_def['type'] == 'gauge':
            return {
                'description': f'Average {ohi_metric}',
                'ohi_query': f"""
                    SELECT average({ohi_metric})
                    FROM MysqlSample
                    WHERE hostname = 'prod-mysql-01'
                    SINCE 5 minutes ago
                """,
                'otel_query': f"""
                    SELECT average({otel_metric})
                    FROM Metric
                    WHERE metricName = '{otel_metric}'
                    AND hostname = 'prod-mysql-01'
                    SINCE 5 minutes ago
                """
            }
        elif metric_def['type'] == 'sum' and 'PerSecond' in ohi_metric:
            return {
                'description': f'Rate of {ohi_metric}',
                'ohi_query': f"""
                    SELECT average({ohi_metric})
                    FROM MysqlSample
                    WHERE hostname = 'prod-mysql-01'
                    SINCE 5 minutes ago
                """,
                'otel_query': f"""
                    SELECT rate(sum({otel_metric}), 1 second)
                    FROM Metric
                    WHERE metricName = '{otel_metric}'
                    AND hostname = 'prod-mysql-01'
                    SINCE 5 minutes ago
                """
            }
    
    def _generate_complex_pattern(self, metric_def):
        """Generate pattern for redesigned metrics with attributes"""
        
        ohi_metrics = metric_def['ohi_metric_group']
        otel_metric = metric_def['otel_metric']
        attributes = metric_def['attributes'][0]
        
        # Build OTEL query with filters
        otel_queries = []
        for ohi_m in ohi_metrics:
            attr_value = attributes['values'][ohi_m]
            otel_queries.append(
                f"filter(average({otel_metric}), WHERE {attributes['key']} = '{attr_value}') as '{ohi_m}'"
            )
        
        return {
            'description': f'Multiple metrics from {otel_metric}',
            'ohi_query': f"""
                SELECT {', '.join([f'average({m})' for m in ohi_metrics])}
                FROM MysqlSample
                WHERE hostname = 'prod-mysql-01'
                SINCE 5 minutes ago
            """,
            'otel_query': f"""
                SELECT {', '.join(otel_queries)}
                FROM Metric
                WHERE metricName = '{otel_metric}'
                AND hostname = 'prod-mysql-01'
                SINCE 5 minutes ago
            """
        }
```

### Semantic Validation Rules

```yaml
semantic_validation_rules:
  metric_type_consistency:
    - rule: "Gauge metrics must remain gauges"
      validation: "metric.type == 'gauge' implies otel.type == 'gauge'"
      
    - rule: "Rate metrics become monotonic sums"
      validation: "metric.name.contains('PerSecond') implies otel.type == 'sum' and otel.monotonic == true"
      
  unit_preservation:
    - rule: "Units must be explicitly defined"
      validation: "otel.unit != null and otel.unit in UCUM"
      
    - rule: "Byte metrics use 'By' not 'B'"
      validation: "metric.name.contains('bytes') implies otel.unit == 'By'"
      
  attribute_completeness:
    - rule: "All group members must have attribute mappings"
      validation: "len(ohi_metric_group) == len(attributes.values)"
      
    - rule: "Attribute keys must be semantic"
      validation: "attributes.key matches [a-z_.]+"
```

## Technical Architecture

### OTEL Collector Architecture

```yaml
# collector-architecture.yaml
architecture:
  deployment_model: "Hierarchical Gateway"
  
  tiers:
    edge_collectors:
      deployment: "DaemonSet on every host"
      responsibilities:
        - "Receive metrics from local integrations"
        - "Initial processing and filtering"
        - "Forward to gateway tier"
      configuration:
        - "Minimal processing for low overhead"
        - "Local buffering for resilience"
        - "Health reporting"
        
    gateway_collectors:
      deployment: "StatefulSet with horizontal scaling"
      responsibilities:
        - "Receive from edge collectors"
        - "Advanced processing and transformation"
        - "Export to multiple destinations"
      configuration:
        - "High availability (3+ replicas)"
        - "Persistent queue for reliability"
        - "Complex routing rules"
        
    management_plane:
      components:
        - "Configuration management (GitOps)"
        - "Service discovery"
        - "Load balancing"
        - "Monitoring and alerting"
```

### Collector Configuration Template

```yaml
# otel-collector-base-config.yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    
  pprof:
    endpoint: 0.0.0.0:1777
    
  zpages:
    endpoint: 0.0.0.0:55679
    
  file_storage:
    directory: /var/lib/otel/storage
    timeout: 10s
    
receivers:
  # Integration-specific receivers configured per host
  mysql:
    endpoint: ${env:MYSQL_ENDPOINT}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    collection_interval: 60s
    initial_delay: 10s
    
  # Collector's own metrics
  prometheus:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          scrape_interval: 30s
          static_configs:
            - targets: ['0.0.0.0:8888']
            
processors:
  # Memory management
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 25
    
  # Resource detection for entity correlation
  resourcedetection:
    detectors: [env, system, ec2, gcp, azure, docker, k8snode]
    timeout: 5s
    override: false
    system:
      hostname_sources: ["os", "dns", "lookup"]
      
  # Ensure entity correlation attributes
  resource/entity_synthesis:
    attributes:
      - key: host.id
        from_attribute: cloud.instance.id
        action: insert
      - key: host.id
        from_attribute: k8s.node.uid
        action: insert
      - key: host.id
        from_attribute: host.name
        action: insert
      - key: entity.guid
        value: "infrastructure:host:${host.id}"
        action: insert
      - key: instrumentation.source
        value: "opentelemetry"
        action: insert
      - key: instrumentation.version
        value: ${env:OTEL_VERSION}
        action: insert
        
  # Metric transformations per semantic guide
  metricstransform/semantic:
    transforms:
      # Example: MySQL metric transformations
      - include: mysql.locks
        action: update
        new_name: mysql.node.innodb.rowLockWaits
        
  # Batching for efficiency
  batch:
    send_batch_size: 1000
    timeout: 60s
    send_batch_max_size: 1500
    
exporters:
  # Primary New Relic export
  otlp/newrelic:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
      storage: file_storage
      
  # Debug export for validation
  logging/debug:
    loglevel: debug
    sampling_initial: 5
    sampling_thereafter: 100
    
service:
  extensions: [health_check, pprof, zpages, file_storage]
  
  pipelines:
    metrics/infrastructure:
      receivers: [mysql]
      processors: [
        memory_limiter,
        resourcedetection,
        resource/entity_synthesis,
        metricstransform/semantic,
        batch
      ]
      exporters: [otlp/newrelic, logging/debug]
      
    metrics/internal:
      receivers: [prometheus]
      processors: [memory_limiter, batch]
      exporters: [otlp/newrelic]
      
  telemetry:
    logs:
      level: info
      development: false
      encoding: json
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

### Deployment Architecture

```python
# deployment_generator.py
class DeploymentGenerator:
    def __init__(self, cluster_config):
        self.cluster = cluster_config
        
    def generate_edge_collector_daemonset(self):
        """Generate DaemonSet for edge collectors"""
        
        return {
            'apiVersion': 'apps/v1',
            'kind': 'DaemonSet',
            'metadata': {
                'name': 'otel-collector-edge',
                'namespace': 'observability'
            },
            'spec': {
                'selector': {
                    'matchLabels': {
                        'app': 'otel-collector',
                        'tier': 'edge'
                    }
                },
                'template': {
                    'metadata': {
                        'labels': {
                            'app': 'otel-collector',
                            'tier': 'edge'
                        }
                    },
                    'spec': {
                        'serviceAccountName': 'otel-collector',
                        'containers': [{
                            'name': 'otel-collector',
                            'image': 'otel/opentelemetry-collector-contrib:0.92.0',
                            'args': ['--config=/conf/otel-collector-config.yaml'],
                            'resources': {
                                'limits': {
                                    'cpu': '200m',
                                    'memory': '512Mi'
                                },
                                'requests': {
                                    'cpu': '100m',
                                    'memory': '256Mi'
                                }
                            },
                            'volumeMounts': [{
                                'name': 'config',
                                'mountPath': '/conf'
                            }],
                            'env': [
                                {
                                    'name': 'NODE_NAME',
                                    'valueFrom': {
                                        'fieldRef': {
                                            'fieldPath': 'spec.nodeName'
                                        }
                                    }
                                },
                                {
                                    'name': 'POD_NAME',
                                    'valueFrom': {
                                        'fieldRef': {
                                            'fieldPath': 'metadata.name'
                                        }
                                    }
                                }
                            ]
                        }],
                        'volumes': [{
                            'name': 'config',
                            'configMap': {
                                'name': 'otel-collector-config'
                            }
                        }],
                        'hostNetwork': True,
                        'dnsPolicy': 'ClusterFirstWithHostNet'
                    }
                }
            }
        }
    
    def generate_gateway_statefulset(self):
        """Generate StatefulSet for gateway collectors"""
        
        return {
            'apiVersion': 'apps/v1',
            'kind': 'StatefulSet',
            'metadata': {
                'name': 'otel-collector-gateway',
                'namespace': 'observability'
            },
            'spec': {
                'serviceName': 'otel-collector-gateway',
                'replicas': 3,
                'selector': {
                    'matchLabels': {
                        'app': 'otel-collector',
                        'tier': 'gateway'
                    }
                },
                'template': {
                    'metadata': {
                        'labels': {
                            'app': 'otel-collector',
                            'tier': 'gateway'
                        }
                    },
                    'spec': {
                        'containers': [{
                            'name': 'otel-collector',
                            'image': 'otel/opentelemetry-collector-contrib:0.92.0',
                            'args': ['--config=/conf/otel-collector-config.yaml'],
                            'resources': {
                                'limits': {
                                    'cpu': '2',
                                    'memory': '4Gi'
                                },
                                'requests': {
                                    'cpu': '1',
                                    'memory': '2Gi'
                                }
                            },
                            'volumeMounts': [
                                {
                                    'name': 'config',
                                    'mountPath': '/conf'
                                },
                                {
                                    'name': 'storage',
                                    'mountPath': '/var/lib/otel/storage'
                                }
                            ]
                        }],
                        'volumes': [{
                            'name': 'config',
                            'configMap': {
                                'name': 'otel-collector-gateway-config'
                            }
                        }]
                    }
                },
                'volumeClaimTemplates': [{
                    'metadata': {
                        'name': 'storage'
                    },
                    'spec': {
                        'accessModes': ['ReadWriteOnce'],
                        'resources': {
                            'requests': {
                                'storage': '10Gi'
                            }
                        }
                    }
                }]
            }
        }
```

## Validation Framework Design

### Master Test Plan

```yaml
# master-test-plan.yaml
test_strategy:
  approach: "Risk-based testing with continuous validation"
  
  test_levels:
    unit_tests:
      scope: "Individual metric translations"
      tools: ["pytest", "custom validators"]
      coverage_target: "100% of semantic mappings"
      
    integration_tests:
      scope: "End-to-end collection pipeline"
      tools: ["docker-compose", "synthetic data"]
      coverage_target: "All critical paths"
      
    system_tests:
      scope: "Production-like validation"
      tools: ["parallel collection", "statistical analysis"]
      coverage_target: "All integrations in real environment"
      
    acceptance_tests:
      scope: "Business validation"
      tools: ["dashboard comparison", "alert testing"]
      coverage_target: "User-facing functionality"
      
test_types:
  functional_testing:
    metric_accuracy:
      description: "Verify metric values match OHI"
      method: "Statistical comparison"
      acceptance_criteria: "95% correlation, <5% deviation"
      
    entity_correlation:
      description: "Verify same entities created"
      method: "GUID comparison"
      acceptance_criteria: "100% match"
      
    query_compatibility:
      description: "Verify queries work correctly"
      method: "Query translation validation"
      acceptance_criteria: "Same results ±5%"
      
  non_functional_testing:
    performance:
      description: "Collection overhead acceptable"
      method: "Resource monitoring"
      acceptance_criteria: "<10% CPU increase, <20% memory increase"
      
    reliability:
      description: "No data loss under stress"
      method: "Chaos testing"
      acceptance_criteria: "99.9% delivery rate"
      
    scalability:
      description: "Handles growth"
      method: "Load testing"
      acceptance_criteria: "Linear scaling to 10x volume"
```

### Validation Test Suite

```python
# validation_test_suite.py
import pytest
import numpy as np
from scipy import stats
import asyncio

class MetricValidationTestSuite:
    def __init__(self, nrql_client, semantic_guide):
        self.nrql = nrql_client
        self.guide = semantic_guide
        
    @pytest.mark.parametrize("metric_mapping", semantic_guide.get_all_mappings())
    async def test_metric_accuracy(self, metric_mapping):
        """Test that OTEL metric values match OHI within tolerance"""
        
        ohi_metric = metric_mapping['ohi_metric']
        otel_metric = metric_mapping['otel_metric']
        
        # Collect parallel data for 1 hour
        duration_minutes = 60
        samples = await self._collect_parallel_samples(
            ohi_metric, 
            otel_metric, 
            duration_minutes
        )
        
        # Statistical validation
        correlation = stats.pearsonr(
            samples['ohi_values'], 
            samples['otel_values']
        )[0]
        
        mean_error = np.mean(np.abs(
            samples['ohi_values'] - samples['otel_values']
        ))
        
        percent_error = (mean_error / np.mean(samples['ohi_values'])) * 100
        
        # Assertions
        assert correlation > 0.95, f"Correlation {correlation} below threshold"
        assert percent_error < 5, f"Error {percent_error}% exceeds tolerance"
        
    async def test_entity_correlation(self, hostname):
        """Test that same entity GUID is generated"""
        
        query = f"""
        SELECT 
          uniques(entity.guid) as guids,
          uniques(entity.type) as types,
          count(*) as data_points
        FROM MysqlSample, Metric
        WHERE hostname = '{hostname}'
        FACET collection.method
        SINCE 1 hour ago
        """
        
        results = await self.nrql.query(query)
        
        ohi_guids = []
        otel_guids = []
        
        for result in results:
            if result['collection.method'] == 'ohi':
                ohi_guids = result['guids']
            else:
                otel_guids = result['guids']
                
        # Must produce exactly one entity each
        assert len(ohi_guids) == 1, f"OHI created {len(ohi_guids)} entities"
        assert len(otel_guids) == 1, f"OTEL created {len(otel_guids)} entities"
        
        # Must be the same entity
        assert ohi_guids[0] == otel_guids[0], "Entity GUIDs don't match"
        
    async def test_query_translation(self, query_pattern):
        """Test that translated queries produce equivalent results"""
        
        ohi_result = await self.nrql.query(query_pattern['ohi_query'])
        otel_result = await self.nrql.query(query_pattern['otel_query'])
        
        # Extract numeric values for comparison
        ohi_value = self._extract_numeric_value(ohi_result)
        otel_value = self._extract_numeric_value(otel_result)
        
        # Allow 5% tolerance for query differences
        if ohi_value > 0:
            percent_diff = abs(ohi_value - otel_value) / ohi_value * 100
            assert percent_diff < 5, f"Query results differ by {percent_diff}%"
        else:
            assert otel_value == 0, "Both values should be zero"
```

### Dual-Signal Alerting

```yaml
# dual-signal-alert-config.yaml
alert_validation:
  approach: "Create duplicate alerts using OTEL metrics"
  
  implementation:
    - step: "Clone existing OHI-based alerts"
      action: "Create OTEL versions with _validation suffix"
      
    - step: "Configure muted notifications"
      action: "Prevent duplicate pages during validation"
      
    - step: "Compare firing patterns"
      action: "Dashboard showing both alert states"
      
  validation_queries:
    - name: "Alert firing comparison"
      nrql: |
        SELECT 
          filter(count(*), WHERE policy_name LIKE '%_original') as 'OHI Alerts',
          filter(count(*), WHERE policy_name LIKE '%_validation') as 'OTEL Alerts',
          percentage(
            filter(count(*), WHERE policy_name LIKE '%_validation'),
            filter(count(*), WHERE policy_name LIKE '%_original')
          ) as 'Match Rate'
        FROM NrAiIncident
        FACET condition_name
        SINCE 24 hours ago
```

## Organizational Mobilization

### Migration Guild Structure

```yaml
migration_guild:
  charter:
    purpose: "Execute the OHI to OTEL migration with excellence"
    duration: "6 months (Phase 1 through Phase 4)"
    authority: "Make technical decisions within approved scope"
    
  roles:
    guild_lead:
      person: "Jane Smith"
      responsibilities:
        - "Chair guild meetings"
        - "Escalate blockers"
        - "Report to steering committee"
        - "Approve technical decisions"
      time_commitment: "50%"
      
    technical_architect:
      person: "Bob Johnson"
      responsibilities:
        - "Design collector architecture"
        - "Review semantic mappings"
        - "Approve technical patterns"
        - "Mentor implementation team"
      time_commitment: "75%"
      
    automation_lead:
      person: "Sarah Chen"
      responsibilities:
        - "Build migration tooling"
        - "Automate validation"
        - "Create reusable libraries"
        - "Train on tools"
      time_commitment: "100%"
      
    integration_leads:
      mysql:
        person: "Mike Wilson"
        responsibilities: ["Own MySQL migration end-to-end"]
      postgresql:
        person: "Lisa Anderson"
        responsibilities: ["Own PostgreSQL migration end-to-end"]
      redis:
        person: "Tom Garcia"
        responsibilities: ["Own Redis migration end-to-end"]
        
  meeting_structure:
    weekly_standup:
      day: "Monday"
      time: "10:00 AM"
      duration: "30 minutes"
      agenda:
        - "Progress against sprint goals"
        - "Blockers and dependencies"
        - "This week's priorities"
        
    technical_deep_dive:
      day: "Wednesday"
      time: "2:00 PM"
      duration: "90 minutes"
      agenda:
        - "Architecture reviews"
        - "Semantic mapping decisions"
        - "Tool demonstrations"
        
    stakeholder_sync:
      day: "Friday"
      time: "3:00 PM"
      duration: "60 minutes"
      agenda:
        - "Status to service owners"
        - "Upcoming migrations"
        - "Feedback and concerns"
```

### RACI Matrix for Phase 1

| Activity | Guild Lead | Tech Architect | Auto Lead | Int. Leads | Service Owners |
|----------|------------|----------------|-----------|------------|----------------|
| Semantic Design | A | R | C | C | I |
| Architecture | C | A/R | I | I | I |
| Tool Development | I | C | A/R | C | I |
| Test Planning | A | R | R | C | C |
| Training Design | R | C | R | C | I |

## Training & Enablement Plan

### Curriculum Design

```yaml
training_program:
  target_audiences:
    service_owners:
      profile: "Owns services using OHI"
      training_needs:
        - "Understanding migration impact"
        - "Query translation basics"
        - "How to validate their metrics"
      delivery_method: "Workshop + documentation"
      
    sre_teams:
      profile: "On-call for infrastructure"
      training_needs:
        - "OTEL collector operations"
        - "Troubleshooting guide"
        - "Emergency procedures"
      delivery_method: "Hands-on labs"
      
    developers:
      profile: "Build dashboards and alerts"
      training_needs:
        - "Query translation patterns"
        - "OTEL metric semantics"
        - "Migration tools usage"
      delivery_method: "Self-paced + office hours"
      
  modules:
    module_1:
      title: "OHI to OTEL Fundamentals"
      duration: "2 hours"
      format: "Interactive workshop"
      objectives:
        - "Understand Sample vs Metric paradigm"
        - "Learn entity correlation concepts"
        - "Practice basic query translation"
      materials:
        - "slides/01-fundamentals.pptx"
        - "labs/01-query-translation.md"
        - "reference/semantic-guide.pdf"
        
    module_2:
      title: "Hands-on Migration Workshop"
      duration: "3 hours"
      format: "Lab session"
      prerequisites: ["Module 1"]
      objectives:
        - "Migrate a sample dashboard"
        - "Update alert conditions"
        - "Validate results"
      materials:
        - "labs/02-dashboard-migration.md"
        - "tools/migration-toolkit.zip"
        - "sandbox/test-environment.yaml"
        
    module_3:
      title: "OTEL Operations Training"
      duration: "4 hours"
      format: "Technical deep-dive"
      prerequisites: ["Module 1"]
      objectives:
        - "Deploy OTEL collectors"
        - "Configure integrations"
        - "Monitor and troubleshoot"
      materials:
        - "labs/03-collector-operations.md"
        - "runbooks/otel-troubleshooting.md"
        - "configs/reference-configs.yaml"
```

### Training Delivery Schedule

```python
# training_scheduler.py
class TrainingScheduler:
    def __init__(self, team_roster, curriculum):
        self.teams = team_roster
        self.curriculum = curriculum
        
    def generate_training_schedule(self):
        """Create personalized training schedules"""
        
        schedule = []
        
        # Week 1-2: Guild members (train the trainers)
        for member in self.teams['migration_guild']:
            schedule.append({
                'participant': member['name'],
                'role': member['role'],
                'modules': ['all'],
                'dates': 'Week 1-2',
                'format': 'Instructor-led'
            })
            
        # Week 3-4: Early adopters
        for team in self.teams['pilot_teams']:
            schedule.append({
                'participant': team['name'],
                'role': 'Pilot Team',
                'modules': ['module_1', 'module_2'],
                'dates': 'Week 3-4',
                'format': 'Workshop'
            })
            
        # Week 5-6: Broader rollout
        for team in self.teams['all_teams']:
            schedule.append({
                'participant': team['name'],
                'role': team['role'],
                'modules': self._select_modules(team['role']),
                'dates': 'Week 5-6',
                'format': 'Mixed'
            })
            
        return schedule
    
    def _select_modules(self, role):
        """Select appropriate modules based on role"""
        
        role_modules = {
            'service_owner': ['module_1'],
            'sre': ['module_1', 'module_3'],
            'developer': ['module_1', 'module_2'],
            'architect': ['module_1', 'module_2', 'module_3']
        }
        
        return role_modules.get(role, ['module_1'])
```

## Migration Work Queue

### Automated Discovery

```python
# work_queue_builder.py
class MigrationWorkQueueBuilder:
    def __init__(self, nr_api, semantic_guide):
        self.api = nr_api
        self.guide = semantic_guide
        self.work_items = []
        
    def build_complete_work_queue(self):
        """Discover all work items that need migration"""
        
        # Discover dashboards
        dashboards = self._discover_affected_dashboards()
        
        # Discover alerts
        alerts = self._discover_affected_alerts()
        
        # Discover SLIs
        slis = self._discover_affected_slis()
        
        # Build prioritized queue
        self.work_items = self._prioritize_work_items(
            dashboards + alerts + slis
        )
        
        return self.work_items
    
    def _discover_affected_dashboards(self):
        """Find all dashboards using OHI metrics"""
        
        work_items = []
        
        dashboards = self.api.list_dashboards()
        
        for dashboard in dashboards:
            affected_widgets = []
            
            for page in dashboard.get('pages', []):
                for widget in page.get('widgets', []):
                    for query in widget.get('rawConfiguration', {}).get('queries', []):
                        nrql = query.get('query', '')
                        
                        # Check if uses OHI events
                        if self._uses_ohi_events(nrql):
                            affected_widgets.append({
                                'widget_id': widget['id'],
                                'widget_title': widget['title'],
                                'query': nrql,
                                'complexity': self._assess_complexity(nrql)
                            })
            
            if affected_widgets:
                work_items.append({
                    'type': 'dashboard',
                    'id': dashboard['guid'],
                    'name': dashboard['name'],
                    'owner': dashboard.get('owner', {}).get('email', 'unknown'),
                    'widgets': affected_widgets,
                    'priority': self._calculate_priority(dashboard),
                    'estimated_effort': len(affected_widgets) * 15  # 15 min per widget
                })
                
        return work_items
    
    def _discover_affected_alerts(self):
        """Find all alerts using OHI metrics"""
        
        work_items = []
        
        policies = self.api.list_alert_policies()
        
        for policy in policies:
            affected_conditions = []
            
            for condition in policy.get('conditions', []):
                nrql = condition.get('nrql', {}).get('query', '')
                
                if self._uses_ohi_events(nrql):
                    affected_conditions.append({
                        'condition_id': condition['id'],
                        'condition_name': condition['name'],
                        'query': nrql,
                        'threshold': condition.get('threshold'),
                        'complexity': self._assess_complexity(nrql)
                    })
            
            if affected_conditions:
                work_items.append({
                    'type': 'alert_policy',
                    'id': policy['id'],
                    'name': policy['name'],
                    'conditions': affected_conditions,
                    'priority': 'CRITICAL',  # Alerts always critical
                    'estimated_effort': len(affected_conditions) * 30  # 30 min per condition
                })
                
        return work_items
    
    def _prioritize_work_items(self, work_items):
        """Sort work items by priority and dependencies"""
        
        # Priority order: alerts > production dashboards > other
        priority_scores = {
            'CRITICAL': 1,
            'HIGH': 2,
            'MEDIUM': 3,
            'LOW': 4
        }
        
        return sorted(
            work_items,
            key=lambda x: (priority_scores.get(x['priority'], 5), x['estimated_effort'])
        )
    
    def export_work_queue(self, format='jira'):
        """Export work queue to tracking system"""
        
        if format == 'jira':
            return self._export_to_jira()
        elif format == 'csv':
            return self._export_to_csv()
        else:
            return self._export_to_json()
```

### Work Queue Dashboard

```yaml
work_queue_dashboard:
  widgets:
    - title: "Migration Progress Overview"
      query: |
        SELECT 
          count(*) as 'Total Items',
          filter(count(*), WHERE status = 'complete') as 'Completed',
          filter(count(*), WHERE status = 'in_progress') as 'In Progress',
          filter(count(*), WHERE status = 'not_started') as 'Not Started'
        FROM MigrationWorkQueue
        SINCE 1 week ago
        
    - title: "Work by Type"
      query: |
        SELECT count(*)
        FROM MigrationWorkQueue
        FACET type
        
    - title: "Effort Burn-down"
      query: |
        SELECT 
          sum(estimated_effort) as 'Total Effort',
          sum(actual_effort) as 'Actual Effort'
        FROM MigrationWorkQueue
        FACET dateOf(timestamp)
        SINCE 30 days ago
        TIMESERIES
```

## Phase Artifacts

### Deliverable Checklist

```yaml
phase_1_deliverables:
  semantic_translation_guide:
    status: "REQUIRED"
    format: "YAML + Documentation"
    location: "git://otel-migration/semantic-guide/"
    acceptance_criteria:
      - "100% of REPLICATE metrics mapped"
      - "100% of REDESIGN metrics specified"
      - "Query patterns documented"
      - "Peer reviewed and approved"
      
  technical_architecture:
    status: "REQUIRED"
    format: "Architecture diagrams + configs"
    location: "git://otel-migration/architecture/"
    acceptance_criteria:
      - "Deployment architecture approved"
      - "Config templates created"
      - "Scaling strategy defined"
      - "Security review passed"
      
  validation_framework:
    status: "REQUIRED"
    format: "Test plans + code"
    location: "git://otel-migration/validation/"
    acceptance_criteria:
      - "Test strategy documented"
      - "Automated tests implemented"
      - "Success criteria defined"
      - "Test data prepared"
      
  migration_tooling:
    status: "REQUIRED"
    format: "Code + documentation"
    location: "git://otel-migration/tools/"
    acceptance_criteria:
      - "Dashboard migration tool tested"
      - "Alert migration tool tested"
      - "Validation scripts ready"
      - "Documentation complete"
      
  organizational_readiness:
    status: "REQUIRED"
    format: "Documentation + schedules"
    location: "sharepoint://migration/phase1/"
    acceptance_criteria:
      - "Guild fully staffed"
      - "Training materials created"
      - "Schedule published"
      - "Work queue populated"
```

## Exit Criteria & Gate 1

### Gate 1 Requirements

```yaml
gate_1_criteria:
  technical_readiness:
    - criterion: "Semantic guide 100% complete"
      measurement: "All metrics mapped and reviewed"
      verifier: "Technical Architect"
      
    - criterion: "Architecture approved"
      measurement: "ARB sign-off obtained"
      verifier: "Architecture Review Board"
      
    - criterion: "Validation framework ready"
      measurement: "All test scenarios implemented"
      verifier: "QA Lead"
      
    - criterion: "Tools tested and documented"
      measurement: "Successful pilot migrations"
      verifier: "Automation Lead"
      
  organizational_readiness:
    - criterion: "Guild operational"
      measurement: "All roles filled and active"
      verifier: "Guild Lead"
      
    - criterion: "Training program ready"
      measurement: "Materials created and reviewed"
      verifier: "Training Lead"
      
    - criterion: "Stakeholders engaged"
      measurement: "Kickoff meetings completed"
      verifier: "Program Manager"
      
  risk_management:
    - criterion: "Technical risks mitigated"
      measurement: "All HIGH risks have controls"
      verifier: "Risk Manager"
      
    - criterion: "Rollback procedures defined"
      measurement: "Documented and tested"
      verifier: "Technical Lead"
```

### Gate Review Package

```python
# gate_review_generator.py
class GateReviewPackageGenerator:
    def __init__(self, phase_artifacts, criteria):
        self.artifacts = phase_artifacts
        self.criteria = criteria
        
    def generate_gate_package(self):
        """Generate comprehensive gate review package"""
        
        package = {
            'executive_summary': self._generate_executive_summary(),
            'criteria_assessment': self._assess_all_criteria(),
            'artifact_status': self._check_artifact_completion(),
            'risk_assessment': self._update_risk_assessment(),
            'recommendation': self._generate_recommendation(),
            'appendices': self._collect_supporting_docs()
        }
        
        return package
    
    def _assess_all_criteria(self):
        """Evaluate each gate criterion"""
        
        assessments = []
        
        for category, criteria_list in self.criteria.items():
            for criterion in criteria_list:
                assessment = {
                    'category': category,
                    'criterion': criterion['criterion'],
                    'measurement': criterion['measurement'],
                    'status': self._evaluate_criterion(criterion),
                    'evidence': self._collect_evidence(criterion),
                    'verifier': criterion['verifier'],
                    'verified_date': None
                }
                assessments.append(assessment)
                
        return assessments
    
    def _generate_recommendation(self):
        """Generate go/no-go recommendation"""
        
        all_criteria = self._assess_all_criteria()
        
        passed = sum(1 for c in all_criteria if c['status'] == 'PASS')
        total = len(all_criteria)
        
        if passed == total:
            return {
                'recommendation': 'GO',
                'confidence': 'HIGH',
                'notes': 'All criteria met, team ready to proceed'
            }
        elif passed / total > 0.9:
            return {
                'recommendation': 'CONDITIONAL GO',
                'confidence': 'MEDIUM',
                'notes': 'Minor items to complete in parallel with Phase 2'
            }
        else:
            return {
                'recommendation': 'NO-GO',
                'confidence': 'HIGH',
                'notes': f'Only {passed}/{total} criteria met. Remediation required.'
            }
```

---

*Phase 1 transforms strategy into actionable plans. Quality here determines success in execution.*