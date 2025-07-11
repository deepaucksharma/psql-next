# Database Intelligence - User & Session Analytics Dashboard

## Overview
Successfully deployed a comprehensive user and session-focused dashboard for Database Intelligence monitoring.

## Dashboard Details
- **Name:** Database Intelligence - User & Session Analytics
- **Dashboard URL:** https://one.newrelic.com/redirect/entity/MzYzMDA3MnxWSVp8REFTSEJPQVJEfGRhOjEwNDU1MTQx
- **Account ID:** 3630072
- **Total Pages:** 6
- **Total Widgets:** 43

## Dashboard Pages

### 1. User Activity Overview
Real-time view of user sessions and activity patterns
- Active Users by Database
- Sessions by User
- Current Active Sessions
- Blocked User Sessions
- User Session States Distribution
- Top Users by Session Count

### 2. Session Performance
Detailed session performance metrics by user
- Query Execution Time by User
- User Query Volume
- Slow Queries by User
- Session Resource Usage
- Transaction Rate by User
- User Connection Pool Usage

### 3. User Behavior Analysis
Analysis of user behavior patterns and anomalies
- User Activity Heatmap
- Failed Login Attempts
- User Error Rate
- Unusual User Activity
- User Data Access Patterns
- Session Termination Reasons

### 4. User Impact Analysis
Impact of database performance on users
- Users Affected by Slow Queries
- Users with Blocked Sessions
- Average User Wait Time
- User Experience Score
- Resource Consumption by User Group
- User Session Health Status

### 5. User Security & Compliance
Security monitoring and compliance tracking for user sessions
- Privileged User Activity
- Suspicious Session Patterns
- User Access Violations
- Session Audit Trail
- Data Access Compliance
- User Session Encryption Status

### 6. Session Cost Analysis
Cost analysis and optimization opportunities by user
- Resource Cost by User
- Query Cost Distribution
- Expensive User Sessions
- Cost Optimization Opportunities
- User Efficiency Score
- Session Cost Trends

## Key Metrics Being Tracked

### User Metrics
- `user_id` - Unique user identifiers
- `user_name` - User names for tracking
- `user.query.count` - Query counts per user
- `user.transaction.commits/rollbacks` - Transaction metrics
- `user.lock.wait_time_ms` - Lock wait times
- `user.connection.pool.active/idle` - Connection pool usage
- `user.total_wait_time_ms` - Total wait times
- `user.is_privileged` - Privileged user tracking
- `user.data.rows_read` - Data access patterns

### Session Metrics
- `db.ash.active_sessions` - Active session counts by state
- `session.duration.seconds` - Session durations
- `session.cpu_usage_percent` - CPU usage per session
- `session.memory_mb` - Memory usage
- `session.io_read_mb` - I/O metrics
- `session.is_blocked` - Blocked session tracking
- `session.health` - Session health status
- `session.queries_per_second` - Query rates
- `session.recovery_time_ms` - Recovery times
- `session.encryption_enabled` - Security tracking

### Performance Metrics
- `query.execution_time_ms` - Query performance
- `wait_time_ms` - Wait event tracking
- `user.query.queue_depth` - Queue depths

### Cost Metrics
- `session.estimated_cost_usd` - Session costs
- `query.estimated_cost_usd` - Query costs
- `session.cpu_seconds` - Resource consumption
- `session.memory_gb_hours` - Memory usage over time
- `session.io_gb` - I/O consumption

## Data Generation Scripts

### Send User Session Data
```bash
./send-user-session-data.sh
```

### Continuous Data Generation
```bash
./continuous-data-generator.sh
```

## Sample Users
The test data includes the following sample users:
- john_doe
- jane_smith
- bob_johnson
- alice_williams
- charlie_brown
- david_miller
- emma_davis
- frank_wilson

## User Groups
- developers
- analysts
- administrators
- applications

## Next Steps

1. **Monitor Real User Activity**: Configure your actual database collectors to send user and session metrics
2. **Set Up Alerts**: Create alerts for:
   - High number of blocked sessions per user
   - Users with excessive resource consumption
   - Failed authentication attempts
   - Unusual query patterns
3. **Optimize Based on Insights**: Use the dashboard to identify:
   - Users with poor query performance
   - Sessions consuming excessive resources
   - Security compliance issues
   - Cost optimization opportunities

## Integration with Existing Setup
The user-focused dashboard complements your existing Database Intelligence setup by providing:
- User-centric views of database performance
- Session-level granularity for troubleshooting
- Security and compliance monitoring
- Cost attribution by user/session

## Technical Details
- Uses NRQL queries with user/session dimensions
- Leverages OTEL attributes for user tracking
- Supports both metrics and logs for comprehensive monitoring
- Includes security audit trail capabilities