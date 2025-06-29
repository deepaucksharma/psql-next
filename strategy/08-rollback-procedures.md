# Rollback Procedures: Emergency & Planned Rollback

## Executive Summary

This document provides comprehensive rollback procedures for the PostgreSQL to OpenTelemetry migration, covering both emergency scenarios requiring immediate action and planned rollbacks based on predetermined criteria. These procedures ensure monitoring continuity and minimize business impact during any rollback scenario.

## Rollback Strategy Overview

### Rollback Principles

1. **Monitor First**: Never lose monitoring visibility
2. **Data Preservation**: Maintain all historical metrics
3. **Staged Approach**: Roll back incrementally when possible
4. **Documentation**: Record all actions and decisions
5. **Communication**: Keep all stakeholders informed

### Rollback Scenarios

```
Rollback Triggers
├── Emergency Rollback
│   ├── Complete monitoring failure
│   ├── Critical data loss
│   ├── Security breach
│   └── Severe performance impact
│
├── PostgreSQL-Specific Triggers
│   ├── Replication monitoring failure
│   ├── Transaction wraparound undetected
│   ├── Connection pool exhaustion by monitoring
│   ├── Slow query detection missing
│   └── WAL accumulation undetected
│
└── Planned Rollback
    ├── Validation failures
    ├── Performance thresholds exceeded
    ├── Integration incompatibilities
    └── Business decision
```

### PostgreSQL-Specific Rollback Criteria

| Scenario | Detection | Threshold | Action |
|----------|-----------|-----------|--------|
| Replication Lag Undetected | Compare with pg_stat_replication | >30s undetected | Immediate rollback |
| Connection Saturation | Monitor connections used by OTel | >10% of max_connections | Gradual rollback |
| Query Performance Impact | pg_stat_statements analysis | >5% degradation | Investigation then rollback |
| Vacuum Metrics Missing | Check autovacuum progress | Critical tables not monitored | Immediate rollback |
| Lock Detection Failure | pg_locks monitoring gap | Deadlocks undetected | Immediate rollback |

## Emergency Rollback Procedures

### Immediate Response Protocol

```yaml
emergency_response:
  detection:
    automated_triggers:
      - metric_collection_failure: ">50% databases affected"
      - data_loss_detected: "Any critical metric gaps >5 minutes"
      - performance_impact: "Database latency increased >100%"
      - security_alert: "Unauthorized access detected"
    
  notification:
    immediate:
      - on_call_engineer: "PagerDuty alert"
      - team_lead: "Phone call + Slack"
      - ops_manager: "Email + SMS"
    
    within_15_minutes:
      - steering_committee: "Emergency meeting invite"
      - affected_teams: "Slack broadcast"
      - vendor_support: "P1 ticket"
  
  decision_tree:
    severity_assessment:
      critical: "Immediate rollback"
      high: "Attempt quick fix (15 min), then rollback"
      medium: "Troubleshoot (1 hour), assess, then decide"
```

### Emergency Rollback Execution

#### Phase 1: Immediate Stabilization (0-15 minutes)

```bash
#!/bin/bash
# emergency_rollback_phase1.sh

set -e
ALERT_WEBHOOK="${SLACK_WEBHOOK}"
LOG_FILE="/var/log/emergency_rollback_$(date +%Y%m%d_%H%M%S).log"

# Function to log and alert
log_alert() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
    curl -X POST "$ALERT_WEBHOOK" -d "{\"text\": \"EMERGENCY ROLLBACK: $1\"}"
}

log_alert "Starting emergency rollback procedure"

# Step 1: Revert load balancer to legacy system
log_alert "Reverting traffic to legacy system"
cat > /tmp/haproxy_emergency.cfg << EOF
backend monitoring_backend
    server legacy_pool weight 100
    server otel_pool weight 0 disabled
EOF

sudo cp /tmp/haproxy_emergency.cfg /etc/haproxy/conf.d/
sudo systemctl reload haproxy

# Step 2: Verify legacy system is receiving traffic
sleep 10
LEGACY_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://legacy-prometheus:9090/-/healthy)
if [ "$LEGACY_STATUS" != "200" ]; then
    log_alert "ERROR: Legacy system not responding! Manual intervention required!"
    exit 1
fi

# Step 3: Stop OTel collectors to prevent dual collection
log_alert "Stopping OpenTelemetry collectors"
kubectl scale deployment otel-collector --replicas=0 -n monitoring

# Step 4: Verify metrics are flowing through legacy
METRIC_CHECK=$(curl -s http://legacy-prometheus:9090/api/v1/query?query=up | jq '.data.result | length')
if [ "$METRIC_CHECK" -lt 1 ]; then
    log_alert "WARNING: No metrics detected in legacy system"
fi

log_alert "Phase 1 complete - Traffic reverted to legacy system"
```

#### Phase 2: System Verification (15-30 minutes)

```python
#!/usr/bin/env python3
# verify_rollback.py

import requests
import json
import time
from datetime import datetime, timedelta
import sys

class RollbackVerifier:
    def __init__(self, legacy_url, databases):
        self.legacy_url = legacy_url
        self.databases = databases
        self.verification_results = {}
    
    def verify_all_databases(self):
        """Verify all databases are being monitored by legacy system"""
        print(f"Verifying {len(self.databases)} databases...")
        
        for db in self.databases:
            query = f'pg_stat_database_numbackends{{datname="{db}"}}'
            result = self.query_prometheus(query)
            
            if result and len(result) > 0:
                self.verification_results[db] = {
                    'status': 'OK',
                    'last_value': result[0]['value'][1],
                    'timestamp': datetime.fromtimestamp(result[0]['value'][0])
                }
            else:
                self.verification_results[db] = {
                    'status': 'MISSING',
                    'last_value': None,
                    'timestamp': None
                }
        
        return self.verification_results
    
    def query_prometheus(self, query):
        """Query Prometheus and return results"""
        try:
            response = requests.get(
                f"{self.legacy_url}/api/v1/query",
                params={'query': query}
            )
            if response.status_code == 200:
                return response.json()['data']['result']
        except Exception as e:
            print(f"Error querying Prometheus: {e}")
        return None
    
    def generate_report(self):
        """Generate verification report"""
        total = len(self.databases)
        monitored = sum(1 for v in self.verification_results.values() if v['status'] == 'OK')
        
        report = {
            'timestamp': datetime.now().isoformat(),
            'total_databases': total,
            'monitored_databases': monitored,
            'missing_databases': total - monitored,
            'success_rate': (monitored / total * 100) if total > 0 else 0,
            'details': self.verification_results
        }
        
        with open('rollback_verification.json', 'w') as f:
            json.dump(report, f, indent=2, default=str)
        
        return report

if __name__ == "__main__":
    # Load database list
    with open('/etc/monitoring/databases.json', 'r') as f:
        databases = json.load(f)
    
    verifier = RollbackVerifier('http://legacy-prometheus:9090', databases)
    results = verifier.verify_all_databases()
    report = verifier.generate_report()
    
    print(f"\nVerification Complete:")
    print(f"Success Rate: {report['success_rate']:.1f}%")
    print(f"Missing Databases: {report['missing_databases']}")
    
    sys.exit(0 if report['success_rate'] >= 95 else 1)
```

#### Phase 3: Data Recovery (30-60 minutes)

```bash
#!/bin/bash
# data_recovery.sh

set -e

# Configuration
OTEL_PROMETHEUS="http://otel-prometheus:9090"
LEGACY_PROMETHEUS="http://legacy-prometheus:9090"
S3_BUCKET="s3://metrics-backup/emergency-recovery"
RECOVERY_WINDOW="2h"  # Recover last 2 hours of data

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Step 1: Identify data gaps
log "Identifying data gaps in legacy system"

END_TIME=$(date +%s)
START_TIME=$((END_TIME - 7200))  # 2 hours ago

# Step 2: Export data from OTel system if still accessible
if curl -s -f "$OTEL_PROMETHEUS/-/healthy" > /dev/null; then
    log "OTel system accessible, exporting recent data"
    
    # Export critical metrics
    for metric in "pg_stat_database_*" "pg_stat_user_tables_*" "pg_replication_*"; do
        curl -G "$OTEL_PROMETHEUS/api/v1/query_range" \
            --data-urlencode "query={__name__=~\"$metric\"}" \
            --data-urlencode "start=$START_TIME" \
            --data-urlencode "end=$END_TIME" \
            --data-urlencode "step=15s" \
            -o "/tmp/otel_export_$(echo $metric | tr '*' '_').json"
    done
    
    # Archive to S3
    tar -czf /tmp/otel_metrics_export.tar.gz /tmp/otel_export_*.json
    aws s3 cp /tmp/otel_metrics_export.tar.gz "$S3_BUCKET/rollback_$(date +%Y%m%d_%H%M%S).tar.gz"
    log "Metrics exported to S3"
else
    log "WARNING: OTel system not accessible, checking for recent backups"
    
    # List recent backups
    aws s3 ls "$S3_BUCKET/" --recursive | tail -10
fi

# Step 3: Export critical PostgreSQL metrics before shutdown
log "Exporting PostgreSQL-specific metrics"

# Export replication status at time of rollback
curl -G "$OTEL_PROMETHEUS/api/v1/query" \
    --data-urlencode 'query=postgresql_replication_lag_bytes' \
    --data-urlencode "time=$END_TIME" \
    -o "/tmp/replication_status_at_rollback.json"

# Export connection states
curl -G "$OTEL_PROMETHEUS/api/v1/query" \
    --data-urlencode 'query=sum by (database, state) (postgresql_database_connections)' \
    --data-urlencode "time=$END_TIME" \
    -o "/tmp/connection_status_at_rollback.json"

# Export any active long-running queries
curl -G "$OTEL_PROMETHEUS/api/v1/query" \
    --data-urlencode 'query=postgresql_locks_waiting_sessions > 0' \
    --data-urlencode "time=$END_TIME" \
    -o "/tmp/locks_at_rollback.json"

# Step 4: Identify and document any permanent data loss
log "Analyzing potential data loss"
python3 << 'EOF'
import json
import requests
from datetime import datetime, timedelta

# Check for gaps in legacy system
legacy_url = "http://legacy-prometheus:9090"
now = datetime.now()
check_points = [now - timedelta(minutes=x) for x in [5, 15, 30, 60, 120]]

gaps = []
critical_metrics_lost = []

for timestamp in check_points:
    # Check basic connectivity
    query = f'up{{job="postgres_exporter"}} @ {int(timestamp.timestamp())}'
    response = requests.get(f"{legacy_url}/api/v1/query", params={'query': query})
    
    if response.status_code == 200:
        result = response.json()['data']['result']
        if len(result) == 0:
            gaps.append({
                'timestamp': timestamp.isoformat(),
                'duration_minutes': int((now - timestamp).total_seconds() / 60)
            })
    
    # Check critical PostgreSQL metrics
    critical_queries = [
        'pg_replication_lag',
        'pg_stat_database_conflicts',
        'pg_stat_database_deadlocks'
    ]
    
    for metric in critical_queries:
        metric_query = f'{metric} @ {int(timestamp.timestamp())}'
        metric_response = requests.get(f"{legacy_url}/api/v1/query", params={'query': metric_query})
        
        if metric_response.status_code == 200:
            metric_result = metric_response.json()['data']['result']
            if len(metric_result) == 0:
                critical_metrics_lost.append({
                    'metric': metric,
                    'timestamp': timestamp.isoformat(),
                    'impact': 'HIGH' if 'replication' in metric else 'MEDIUM'
                })

# Generate gap report
gap_report = {
    'analysis_time': now.isoformat(),
    'gaps_detected': len(gaps),
    'gap_details': gaps,
    'critical_metrics_lost': critical_metrics_lost,
    'max_gap_minutes': max([g['duration_minutes'] for g in gaps]) if gaps else 0,
    'recovery_required': len(critical_metrics_lost) > 0
}

with open('/tmp/data_gap_analysis.json', 'w') as f:
    json.dump(gap_report, f, indent=2)

print(f"Data gap analysis complete: {len(gaps)} gaps found, {len(critical_metrics_lost)} critical metrics lost")
EOF

log "Data recovery phase complete"
```

### Post-Rollback Stabilization

```yaml
stabilization_checklist:
  immediate_actions:
    - verify_all_alerts_firing: "Check critical alerts are active"
    - confirm_dashboard_access: "Ensure all teams can access dashboards"
    - validate_integrations: "Test downstream system connections"
    - review_performance: "Confirm no performance degradation"
  
  within_1_hour:
    - conduct_retrospective: "Initial incident review"
    - update_status_page: "Communicate system status"
    - notify_stakeholders: "Send detailed update"
    - plan_forward: "Define next steps"
  
  within_24_hours:
    - complete_rca: "Root cause analysis"
    - update_runbooks: "Document lessons learned"
    - plan_remediation: "Address identified issues"
    - schedule_retry: "Plan migration retry if appropriate"
```

## Planned Rollback Procedures

### Rollback Decision Matrix

| Scenario | Trigger Threshold | Decision Time | Rollback Type |
|----------|------------------|---------------|---------------|
| Performance Impact | >10% degradation | 1 hour | Gradual |
| Metric Discrepancy | >5% variance | 4 hours | Immediate |
| Alert Failures | >3 critical alerts | 30 minutes | Immediate |
| Integration Issues | Any critical system | 2 hours | Gradual |
| Cost Overrun | >150% projected | 1 day | Planned |

### Gradual Rollback Process

```python
#!/usr/bin/env python3
# gradual_rollback.py

import time
import subprocess
import json
from datetime import datetime
import requests

class GradualRollback:
    def __init__(self, config_file):
        with open(config_file, 'r') as f:
            self.config = json.load(f)
        
        self.rollback_stages = [
            (90, 10),  # 90% legacy, 10% OTel
            (75, 25),  # 75% legacy, 25% OTel
            (50, 50),  # 50% legacy, 50% OTel
            (25, 75),  # 25% legacy, 75% OTel
            (0, 100),  # 0% legacy, 100% OTel (current state)
        ]
        self.current_stage = len(self.rollback_stages) - 1
    
    def execute_rollback(self, target_stage=0):
        """Execute gradual rollback to target stage"""
        print(f"Starting gradual rollback from stage {self.current_stage} to {target_stage}")
        
        while self.current_stage > target_stage:
            next_stage = self.current_stage - 1
            legacy_weight, otel_weight = self.rollback_stages[next_stage]
            
            print(f"\nRolling back to stage {next_stage}: {legacy_weight}% legacy, {otel_weight}% OTel")
            
            # Update load balancer weights
            if self.update_load_balancer(legacy_weight, otel_weight):
                # Wait for stabilization
                print("Waiting for stabilization...")
                time.sleep(300)  # 5 minutes
                
                # Verify health
                if self.verify_health():
                    self.current_stage = next_stage
                    print(f"Stage {next_stage} completed successfully")
                else:
                    print(f"Health check failed at stage {next_stage}, stopping rollback")
                    return False
            else:
                print("Failed to update load balancer")
                return False
        
        print(f"\nRollback completed successfully to stage {target_stage}")
        return True
    
    def update_load_balancer(self, legacy_weight, otel_weight):
        """Update HAProxy weights"""
        config = f"""
backend monitoring_backend
    server legacy_pool weight {legacy_weight}
    server otel_pool weight {otel_weight}
"""
        
        try:
            with open('/tmp/haproxy_weights.cfg', 'w') as f:
                f.write(config)
            
            subprocess.run([
                'sudo', 'cp', '/tmp/haproxy_weights.cfg',
                '/etc/haproxy/conf.d/weights.cfg'
            ], check=True)
            
            subprocess.run(['sudo', 'systemctl', 'reload', 'haproxy'], check=True)
            return True
        except subprocess.CalledProcessError as e:
            print(f"Error updating load balancer: {e}")
            return False
    
    def verify_health(self):
        """Verify system health after weight change"""
        checks = {
            'legacy_prometheus': self.check_prometheus(self.config['legacy_prometheus']),
            'otel_prometheus': self.check_prometheus(self.config['otel_prometheus']),
            'metric_collection': self.check_metric_collection(),
            'alert_manager': self.check_alertmanager()
        }
        
        failed_checks = [k for k, v in checks.items() if not v]
        
        if failed_checks:
            print(f"Health checks failed: {failed_checks}")
            return False
        
        return True
    
    def check_prometheus(self, url):
        """Check Prometheus health"""
        try:
            response = requests.get(f"{url}/-/healthy", timeout=5)
            return response.status_code == 200
        except:
            return False
    
    def check_metric_collection(self):
        """Verify metrics are being collected"""
        query = 'up{job=~"postgres.*"}'
        
        for prometheus_url in [self.config['legacy_prometheus'], self.config['otel_prometheus']]:
            try:
                response = requests.get(
                    f"{prometheus_url}/api/v1/query",
                    params={'query': query}
                )
                if response.status_code == 200:
                    results = response.json()['data']['result']
                    if len(results) > 0:
                        return True
            except:
                pass
        
        return False
    
    def check_alertmanager(self):
        """Check AlertManager health"""
        try:
            response = requests.get(
                f"{self.config['alertmanager']}/api/v1/status",
                timeout=5
            )
            return response.status_code == 200
        except:
            return False

if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: gradual_rollback.py <target_stage>")
        sys.exit(1)
    
    target = int(sys.argv[1])
    rollback = GradualRollback('/etc/rollback/config.json')
    
    success = rollback.execute_rollback(target)
    sys.exit(0 if success else 1)
```

### Component-Specific Rollback

#### Collector Rollback

```bash
#!/bin/bash
# rollback_collectors.sh

# Function to rollback collectors on a specific host
rollback_host_collector() {
    local host=$1
    
    echo "Rolling back collector on $host"
    
    # Stop OTel collector
    ssh "$host" "sudo systemctl stop otel-collector"
    
    # Start legacy exporter
    ssh "$host" "sudo systemctl start postgres_exporter"
    
    # Verify exporter is running
    sleep 5
    if ssh "$host" "sudo systemctl is-active postgres_exporter"; then
        echo "Successfully rolled back $host"
        return 0
    else
        echo "Failed to rollback $host"
        return 1
    fi
}

# Rollback collectors in waves
WAVE1_HOSTS=("db-host-01" "db-host-02" "db-host-03")
WAVE2_HOSTS=("db-host-04" "db-host-05" "db-host-06")
WAVE3_HOSTS=("db-host-07" "db-host-08" "db-host-09")

echo "Starting collector rollback in waves"

# Wave 1
echo "Rolling back Wave 1..."
for host in "${WAVE1_HOSTS[@]}"; do
    rollback_host_collector "$host" &
done
wait

# Check metrics before proceeding
./verify_metrics.sh || exit 1

# Wave 2
echo "Rolling back Wave 2..."
for host in "${WAVE2_HOSTS[@]}"; do
    rollback_host_collector "$host" &
done
wait

# Final verification
./verify_metrics.sh || exit 1

echo "Collector rollback completed"
```

#### Storage Rollback

```yaml
storage_rollback:
  prometheus_data:
    - step: "Stop writes to new storage"
      commands:
        - "kubectl scale deployment otel-prometheus --replicas=0"
    
    - step: "Export recent data"
      commands:
        - "promtool tsdb dump /data/prometheus --min-time=2h"
        - "tar -czf prometheus_dump.tar.gz /tmp/prometheus_dump"
    
    - step: "Reconfigure legacy storage"
      commands:
        - "sudo mount /dev/vg_metrics/lv_prometheus /mnt/prometheus"
        - "sudo systemctl start prometheus"
    
    - step: "Import recent data if needed"
      commands:
        - "promtool tsdb create-blocks-from dump /tmp/prometheus_dump"
  
  long_term_storage:
    - step: "Pause S3 uploads"
      commands:
        - "kubectl patch cronjob s3-backup -p '{\"spec\":{\"suspend\":true}}'"
    
    - step: "Copy recent data"
      commands:
        - "aws s3 sync s3://otel-metrics/ s3://legacy-metrics/ --since=24h"
```

## Rollback Testing

### Rollback Test Plan

```yaml
rollback_test_scenarios:
  scenario_1:
    name: "Emergency rollback - Complete failure"
    setup:
      - "Deploy to test environment"
      - "Simulate 100% collector failure"
    execution:
      - "Trigger emergency rollback"
      - "Measure recovery time"
      - "Verify metric continuity"
    success_criteria:
      - "Recovery time < 15 minutes"
      - "No metric gaps > 5 minutes"
      - "All alerts functioning"
  
  scenario_2:
    name: "Gradual rollback - Performance issue"
    setup:
      - "Deploy with artificial latency"
      - "Generate normal load"
    execution:
      - "Detect performance degradation"
      - "Execute gradual rollback"
      - "Monitor each stage"
    success_criteria:
      - "Performance restored at each stage"
      - "No service disruption"
      - "Gradual transition smooth"
  
  scenario_3:
    name: "Partial rollback - Integration failure"
    setup:
      - "Break specific integration"
      - "Keep core monitoring functional"
    execution:
      - "Identify affected components"
      - "Rollback only affected parts"
      - "Maintain partial OTel operation"
    success_criteria:
      - "Affected integration restored"
      - "Unaffected components unchanged"
      - "Minimal disruption"
```

### Rollback Drill Execution

```bash
#!/bin/bash
# rollback_drill.sh

set -e

DRILL_TYPE="${1:-emergency}"
DRILL_ENV="${2:-staging}"
DRILL_LOG="/var/log/rollback_drill_$(date +%Y%m%d_%H%M%S).log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$DRILL_LOG"
}

# Pre-drill checklist
pre_drill_checks() {
    log "Starting pre-drill checks"
    
    # Verify test environment
    if [ "$DRILL_ENV" == "production" ]; then
        log "ERROR: Cannot run drill in production!"
        exit 1
    fi
    
    # Backup current state
    log "Backing up current configuration"
    kubectl get all -n monitoring -o yaml > /tmp/monitoring_backup.yaml
    
    # Notify team
    log "Notifying team about drill"
    ./notify_team.sh "Rollback drill starting in $DRILL_ENV"
    
    log "Pre-drill checks complete"
}

# Execute drill based on type
execute_drill() {
    case "$DRILL_TYPE" in
        "emergency")
            log "Executing emergency rollback drill"
            time ./emergency_rollback_phase1.sh
            time ./verify_rollback.py
            ;;
        
        "gradual")
            log "Executing gradual rollback drill"
            ./gradual_rollback.py 2  # Rollback to 50/50 split
            ;;
        
        "component")
            log "Executing component rollback drill"
            ./rollback_collectors.sh
            ;;
        
        *)
            log "ERROR: Unknown drill type: $DRILL_TYPE"
            exit 1
            ;;
    esac
}

# Post-drill analysis
post_drill_analysis() {
    log "Starting post-drill analysis"
    
    # Collect metrics
    DURATION=$(($(date +%s) - START_TIME))
    log "Drill duration: $DURATION seconds"
    
    # Check for issues
    ERROR_COUNT=$(grep -c ERROR "$DRILL_LOG" || true)
    WARNING_COUNT=$(grep -c WARNING "$DRILL_LOG" || true)
    
    log "Errors: $ERROR_COUNT, Warnings: $WARNING_COUNT"
    
    # Generate report
    cat > /tmp/drill_report.md << EOF
# Rollback Drill Report

**Date**: $(date)
**Type**: $DRILL_TYPE
**Environment**: $DRILL_ENV
**Duration**: $DURATION seconds

## Results
- Errors: $ERROR_COUNT
- Warnings: $WARNING_COUNT
- Success: $([[ $ERROR_COUNT -eq 0 ]] && echo "YES" || echo "NO")

## Lessons Learned
- [Add observations here]

## Action Items
- [Add follow-up tasks here]
EOF
    
    log "Drill report generated: /tmp/drill_report.md"
}

# Main execution
START_TIME=$(date +%s)

log "Starting rollback drill: Type=$DRILL_TYPE, Environment=$DRILL_ENV"

pre_drill_checks
execute_drill
post_drill_analysis

# Restore environment
log "Restoring environment to pre-drill state"
kubectl apply -f /tmp/monitoring_backup.yaml

log "Rollback drill completed"
```

## Communication During Rollback

### Stakeholder Communication Matrix

| Audience | When | What | How |
|----------|------|------|-----|
| On-Call Team | Immediate | Rollback initiated, actions needed | PagerDuty + Slack |
| Management | Within 5 min | Status, impact, ETA | Phone + Email |
| Affected Teams | Within 10 min | Service impact, workarounds | Slack + Email |
| All Engineering | Within 30 min | Detailed update | Email + Wiki |
| Customers | If impact >30 min | Service status | Status Page |

### Communication Templates

#### Initial Notification

```
Subject: [URGENT] Monitoring System Rollback in Progress

We are currently executing a rollback of the OpenTelemetry monitoring system due to [ISSUE].

**Status**: Rollback in progress
**Impact**: Potential delays in metrics/alerts
**ETA**: 30 minutes
**Action Required**: Use legacy dashboards at [URL]

Updates will be provided every 15 minutes.

Incident Commander: [Name]
Bridge: [Phone/URL]
```

#### Status Update

```
Subject: [UPDATE] Monitoring Rollback - Status at [TIME]

**Current Status**: [Phase X of Y complete]
**Systems Affected**: [List]
**Systems Restored**: [List]
**Next Steps**: [Actions]
**Revised ETA**: [Time]

No action required from most teams. DBA team please standby.

Questions? Join bridge: [URL]
```

#### Completion Notice

```
Subject: [RESOLVED] Monitoring Rollback Complete

The monitoring system rollback has been completed successfully.

**Duration**: [X] minutes
**Impact**: [Summary]
**Root Cause**: Under investigation
**Follow-up**: RCA scheduled for [Date/Time]

All systems are now operating normally on the legacy monitoring platform.

Thank you for your patience.
```

## Post-Rollback Activities

### Immediate Actions (First 24 Hours)

```yaml
post_rollback_immediate:
  hour_1:
    - verify_stability: "Confirm all systems stable"
    - document_timeline: "Create detailed timeline"
    - preserve_evidence: "Collect logs and metrics"
    - notify_vendors: "Inform relevant vendors"
  
  hour_2_4:
    - initial_rca: "Draft preliminary RCA"
    - identify_fixes: "List immediate fixes needed"
    - plan_forward: "Decide on retry timeline"
    - update_docs: "Update runbooks with findings"
  
  hour_4_24:
    - detailed_analysis: "Deep dive into root cause"
    - fix_implementation: "Implement identified fixes"
    - test_fixes: "Verify fixes in test environment"
    - stakeholder_update: "Send comprehensive update"
```

### Root Cause Analysis

```markdown
# RCA Template - Monitoring System Rollback

## Incident Summary
- **Date/Time**: [Timestamp]
- **Duration**: [Minutes]
- **Impact**: [Business impact]
- **Severity**: [P1/P2/P3]

## Timeline
- HH:MM - First indication of issue
- HH:MM - Alert triggered
- HH:MM - Rollback decision made
- HH:MM - Rollback initiated
- HH:MM - Service restored
- HH:MM - Incident closed

## Root Cause
[Detailed explanation of what went wrong]

## Contributing Factors
1. [Factor 1]
2. [Factor 2]

## Resolution
[Steps taken to resolve]

## Lessons Learned
### What Went Well
- [Item 1]
- [Item 2]

### What Went Wrong
- [Item 1]
- [Item 2]

### Where We Got Lucky
- [Item 1]

## Action Items
| Action | Owner | Due Date | Priority |
|--------|-------|----------|----------|
| [Action 1] | [Name] | [Date] | [High/Med/Low] |

## Prevention
[Long-term fixes to prevent recurrence]
```

### Recovery Planning

```yaml
recovery_planning:
  short_term:
    fix_immediate_issues:
      - description: "Address root cause"
      - timeline: "1-2 weeks"
      - owner: "Technical Lead"
    
    improve_monitoring:
      - description: "Add missing monitors"
      - timeline: "1 week"
      - owner: "SRE Team"
    
    update_procedures:
      - description: "Revise rollback procedures"
      - timeline: "3 days"
      - owner: "Operations Manager"
  
  medium_term:
    enhance_testing:
      - description: "Expand test coverage"
      - timeline: "1 month"
      - owner: "QA Team"
    
    automation:
      - description: "Automate rollback procedures"
      - timeline: "6 weeks"
      - owner: "Platform Team"
  
  long_term:
    architecture_review:
      - description: "Review and improve architecture"
      - timeline: "3 months"
      - owner: "Architecture Team"
    
    retry_migration:
      - description: "Plan and execute retry"
      - timeline: "To be determined"
      - owner: "Steering Committee"
```

## Automation Scripts

### Rollback Automation Framework

```python
#!/usr/bin/env python3
# rollback_automation.py

import asyncio
import yaml
import logging
from datetime import datetime
from typing import Dict, List, Optional
from enum import Enum

class RollbackType(Enum):
    EMERGENCY = "emergency"
    GRADUAL = "gradual"
    COMPONENT = "component"
    PLANNED = "planned"

class RollbackAutomation:
    def __init__(self, config_path: str):
        with open(config_path, 'r') as f:
            self.config = yaml.safe_load(f)
        
        self.logger = self._setup_logging()
        self.state = {"status": "ready", "stage": None}
        
    def _setup_logging(self):
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
            handlers=[
                logging.FileHandler(f'/var/log/rollback_{datetime.now():%Y%m%d_%H%M%S}.log'),
                logging.StreamHandler()
            ]
        )
        return logging.getLogger(__name__)
    
    async def execute_rollback(self, rollback_type: RollbackType, reason: str):
        """Main rollback execution coordinator"""
        self.logger.info(f"Initiating {rollback_type.value} rollback. Reason: {reason}")
        self.state["status"] = "in_progress"
        self.state["start_time"] = datetime.now()
        
        try:
            # Pre-rollback checks
            if not await self._pre_rollback_checks():
                raise Exception("Pre-rollback checks failed")
            
            # Execute rollback based on type
            if rollback_type == RollbackType.EMERGENCY:
                await self._emergency_rollback()
            elif rollback_type == RollbackType.GRADUAL:
                await self._gradual_rollback()
            elif rollback_type == RollbackType.COMPONENT:
                await self._component_rollback()
            else:
                await self._planned_rollback()
            
            # Post-rollback verification
            if await self._verify_rollback():
                self.state["status"] = "completed"
                self.logger.info("Rollback completed successfully")
            else:
                self.state["status"] = "failed"
                self.logger.error("Rollback verification failed")
                
        except Exception as e:
            self.state["status"] = "failed"
            self.logger.error(f"Rollback failed: {str(e)}")
            raise
        finally:
            self.state["end_time"] = datetime.now()
            await self._generate_report()
    
    async def _pre_rollback_checks(self) -> bool:
        """Perform pre-rollback validation"""
        checks = [
            self._check_legacy_system_health(),
            self._check_team_availability(),
            self._check_backup_availability(),
            self._check_communication_channels()
        ]
        
        results = await asyncio.gather(*checks, return_exceptions=True)
        
        for i, result in enumerate(results):
            if isinstance(result, Exception):
                self.logger.error(f"Pre-check {i} failed: {result}")
                return False
            if not result:
                self.logger.error(f"Pre-check {i} returned False")
                return False
        
        return True
    
    async def _emergency_rollback(self):
        """Execute emergency rollback procedure"""
        self.logger.info("Executing emergency rollback")
        
        # Stage 1: Immediate traffic switch
        self.state["stage"] = "traffic_switch"
        await self._switch_traffic_to_legacy(weight=100)
        
        # Stage 2: Stop OTel collectors
        self.state["stage"] = "stop_collectors"
        await self._stop_otel_collectors()
        
        # Stage 3: Verify legacy operation
        self.state["stage"] = "verify_legacy"
        await self._verify_legacy_operation()
        
        # Stage 4: Preserve data
        self.state["stage"] = "preserve_data"
        await self._preserve_otel_data()
    
    async def _gradual_rollback(self):
        """Execute gradual rollback procedure"""
        self.logger.info("Executing gradual rollback")
        
        stages = [(90, 10), (75, 25), (50, 50), (25, 75), (0, 100)]
        current_stage = 0
        
        for legacy_weight, otel_weight in stages:
            self.state["stage"] = f"gradual_{legacy_weight}_{otel_weight}"
            self.logger.info(f"Stage: {legacy_weight}% legacy, {otel_weight}% OTel")
            
            await self._switch_traffic_to_legacy(weight=legacy_weight)
            await asyncio.sleep(300)  # 5 minute stabilization
            
            if not await self._verify_health():
                self.logger.error(f"Health check failed at stage {current_stage}")
                break
            
            current_stage += 1
    
    async def _verify_rollback(self) -> bool:
        """Verify rollback was successful"""
        verifications = [
            self._verify_metric_collection(),
            self._verify_alert_functionality(),
            self._verify_dashboard_access(),
            self._verify_no_data_loss()
        ]
        
        results = await asyncio.gather(*verifications)
        return all(results)
    
    async def _generate_report(self):
        """Generate rollback report"""
        duration = (self.state["end_time"] - self.state["start_time"]).total_seconds()
        
        report = {
            "rollback_id": f"RB-{datetime.now():%Y%m%d-%H%M%S}",
            "type": self.state.get("type", "unknown"),
            "status": self.state["status"],
            "duration_seconds": duration,
            "stages_completed": self.state.get("stages_completed", []),
            "errors": self.state.get("errors", []),
            "timestamp": datetime.now().isoformat()
        }
        
        report_path = f"/var/log/rollback_report_{report['rollback_id']}.json"
        with open(report_path, 'w') as f:
            json.dump(report, f, indent=2)
        
        self.logger.info(f"Report generated: {report_path}")
        
        # Send notification
        await self._send_notification(
            f"Rollback {report['status']}: Duration {duration}s"
        )

if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 3:
        print("Usage: rollback_automation.py <type> <reason>")
        sys.exit(1)
    
    rollback_type = RollbackType(sys.argv[1])
    reason = sys.argv[2]
    
    automation = RollbackAutomation('/etc/rollback/config.yaml')
    asyncio.run(automation.execute_rollback(rollback_type, reason))
```

## Rollback Success Criteria

### Technical Success Metrics

```yaml
rollback_success_metrics:
  immediate_success:
    - metric_collection_restored: "100% databases reporting"
    - alerts_functional: "All critical alerts active"
    - dashboard_access: "All teams can access dashboards"
    - performance_acceptable: "Query latency <5s"
    - no_data_loss: "Gap <5 minutes"
  
  stability_success:
    - uptime_24h: ">99.9%"
    - no_repeated_issues: "Original issue not recurring"
    - team_confidence: "Ready to operate legacy system"
    - integrations_stable: "All downstream systems functioning"
  
  operational_success:
    - documentation_updated: "All procedures reflect current state"
    - team_trained: "All operators comfortable with legacy"
    - vendor_support: "Support contracts reactivated if needed"
    - monitoring_coverage: "No blind spots identified"
```

### Business Success Criteria

```yaml
business_success_criteria:
  customer_impact:
    - sla_maintained: "No SLA breaches during rollback"
    - customer_notifications: "Proactive communication sent"
    - support_tickets: "No increase in monitoring-related tickets"
  
  financial_impact:
    - rollback_cost: "Within allocated emergency budget"
    - ongoing_costs: "Legacy system costs understood"
    - remediation_budget: "Funding for fixes approved"
  
  strategic_impact:
    - timeline_impact: "Recovery plan established"
    - confidence_maintained: "Stakeholder trust preserved"
    - lessons_documented: "Value extracted from experience"
```

## Appendix: Quick Reference

### Emergency Contacts for Rollback

```yaml
rollback_contacts:
  internal:
    - role: "Incident Commander"
      primary: "+1-XXX-XXX-XXXX"
      backup: "+1-XXX-XXX-XXXX"
    
    - role: "Platform Lead"
      primary: "+1-XXX-XXX-XXXX"
      backup: "+1-XXX-XXX-XXXX"
    
    - role: "Database Lead"
      primary: "+1-XXX-XXX-XXXX"
      backup: "+1-XXX-XXX-XXXX"
  
  vendors:
    - name: "Legacy Monitoring Vendor"
      support: "+1-800-XXX-XXXX"
      account: "Premium-12345"
      
    - name: "Cloud Provider"
      support: "+1-800-XXX-XXXX"
      account: "ENT-67890"
  
  escalation:
    - level: "Director"
      name: "[Name]"
      phone: "+1-XXX-XXX-XXXX"
      
    - level: "VP Engineering"
      name: "[Name]"
      phone: "+1-XXX-XXX-XXXX"
```

### Rollback Command Cheatsheet

```bash
# Emergency Commands
## Switch all traffic to legacy
echo "server otel_pool weight 0" | sudo socat stdio /var/run/haproxy.sock

## Stop all OTel collectors
kubectl scale deployment otel-collector --replicas=0 -n monitoring

## Start legacy exporters
ansible postgres_hosts -m service -a "name=postgres_exporter state=started"

# Verification Commands
## Check legacy metrics
curl -s http://legacy-prometheus:9090/api/v1/query?query=up | jq '.data.result | length'

## Check for gaps
promtool tsdb analyze /var/lib/prometheus

## Verify alerts
amtool alert --alertmanager.url=http://alertmanager:9093

# Recovery Commands
## Export recent OTel data
curl -G http://otel-prometheus:9090/api/v1/export \
  --data-urlencode 'match[]={__name__=~"postgres.*"}' \
  --data-urlencode 'start=2h' \
  > otel_export.json

## Import to legacy if needed
promtool tsdb create-blocks-from openmetrics otel_export.json
```