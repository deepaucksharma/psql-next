# Database Prerequisites

## PostgreSQL Requirements

### Essential Prerequisites

1. **pg_stat_statements Extension**
   ```sql
   -- Check if installed
   SELECT * FROM pg_available_extensions WHERE name = 'pg_stat_statements';
   
   -- Enable (requires restart)
   -- In postgresql.conf:
   shared_preload_libraries = 'pg_stat_statements'
   
   -- Then in database:
   CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
   ```

2. **Read Replica**
   - Dedicated read replica required
   - Synchronous or asynchronous (with lag monitoring)
   - Network accessible from collector

3. **Monitoring User**
   ```sql
   -- Create read-only user
   CREATE USER newrelic_monitor WITH PASSWORD 'secure_password';

   -- Grant minimal permissions
   GRANT CONNECT ON DATABASE yourdb TO newrelic_monitor;
   GRANT USAGE ON SCHEMA public TO newrelic_monitor;
   GRANT SELECT ON ALL TABLES IN SCHEMA public TO newrelic_monitor;

   -- Grant pg_stat_statements access
   GRANT SELECT ON pg_stat_statements TO newrelic_monitor;
   ```

### Optional Enhancements

1. **auto_explain Module** (for zero-impact collection)
   ```sql
   -- In postgresql.conf
   shared_preload_libraries = 'pg_stat_statements,auto_explain'

   -- Configuration
   auto_explain.log_min_duration = '100ms'
   auto_explain.log_analyze = false  -- Important: never true on production
   auto_explain.log_format = 'json'
   auto_explain.log_nested_statements = true
   ```

2. **Custom Plan Function** (if not using auto_explain)
   ```sql
   -- Create safe EXPLAIN wrapper
   CREATE OR REPLACE FUNCTION pg_get_json_plan(query_text text)
   RETURNS json AS $$
   DECLARE
     plan json;
   BEGIN
     -- Safety timeout at function level
     SET LOCAL statement_timeout = '2s';
     
     EXECUTE 'EXPLAIN (FORMAT JSON, BUFFERS true) ' || query_text INTO plan;
     RETURN plan;
   EXCEPTION
     WHEN OTHERS THEN
       RETURN json_build_object('error', SQLERRM);
   END;
   $$ LANGUAGE plpgsql SECURITY DEFINER;

   -- Grant execution
   GRANT EXECUTE ON FUNCTION pg_get_json_plan(text) TO newrelic_monitor;
   ```

### Validation Checklist
- [ ] pg_stat_statements shows data
- [ ] Read replica is accessible
- [ ] Monitoring user can connect
- [ ] Replication lag is acceptable (< 30s)
- [ ] No performance impact on primary

## MySQL Requirements

### Essential Prerequisites

1. **Performance Schema Enabled**
   ```sql
   -- Check status
   SHOW VARIABLES LIKE 'performance_schema';

   -- Must be ON (set in my.cnf, requires restart)
   [mysqld]
   performance_schema = ON
   ```

2. **Statement Digests Configured**
   ```sql
   -- Check settings
   SELECT * FROM performance_schema.setup_consumers 
   WHERE NAME LIKE '%statement%';

   -- Enable if needed
   UPDATE performance_schema.setup_consumers 
   SET ENABLED = 'YES' 
   WHERE NAME = 'statements_digest';
   ```

3. **Read Replica or Secondary**
   - Dedicated read replica
   - Or secondary in replica set
   - Must handle additional read load

4. **Monitoring User**
   ```sql
   -- Create user with minimal permissions
   CREATE USER 'newrelic_monitor'@'%' IDENTIFIED BY 'secure_password';

   -- Grant only what's needed
   GRANT SELECT ON performance_schema.* TO 'newrelic_monitor'@'%';
   GRANT SELECT ON mysql.* TO 'newrelic_monitor'@'%';
   GRANT PROCESS ON *.* TO 'newrelic_monitor'@'%';
   ```

### Limitations in MVP

**Important**: MySQL MVP provides query metadata only, not full execution plans. This is because:
- Running EXPLAIN on arbitrary queries is risky
- No safe timeout mechanism like PostgreSQL
- Plans would need to be generated on primary

### Validation Checklist
- [ ] Performance schema is ON
- [ ] Statement digests are collected
- [ ] Read replica is accessible
- [ ] Monitoring user can query performance_schema
- [ ] Digest history is retained (check max size)

## MongoDB Requirements

### For Profiler Collection

1. **Enable Profiling** (on secondary)
   ```javascript
   // Check current level
   db.getProfilingStatus()

   // Enable for slow operations only
   db.setProfilingLevel(1, { slowms: 100 })
   ```

2. **Create Monitoring User**
   ```javascript
   db.createUser({
     user: "newrelic_monitor",
     pwd: "secure_password",
     roles: [
       { role: "read", db: "admin" },
       { role: "clusterMonitor", db: "admin" },
       { role: "read", db: "local" }
     ]
   })
   ```

### For Log Collection

1. **Configure Logging**
   ```yaml
   systemLog:
     destination: file
     path: /var/log/mongodb/mongod.log
     logAppend: true
     verbosity: 1
   ```

2. **Set Slow Operation Threshold**
   ```yaml
   operationProfiling:
     mode: off  # We use logs, not profiler in this mode
     slowOpThresholdMs: 100
   ```

## Common Gotchas

1. **Extension Loading**: Most extensions require server restart
2. **Permission Propagation**: New permissions may need explicit schema refreshes
3. **Replica Lag**: Can make collected data stale
4. **Log Rotation**: Can break filelog receiver mid-parse
5. **Connection Limits**: Monitor connection pool exhaustion