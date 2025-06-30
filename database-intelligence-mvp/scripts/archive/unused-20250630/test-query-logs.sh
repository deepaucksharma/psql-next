#!/bin/bash

# Create sample PostgreSQL query log
cat > /tmp/postgresql-test.log << 'EOF'
2025-06-30 12:45:00.123 UTC [12345] LOG:  duration: 15.234 ms  statement: SELECT * FROM users WHERE id = 1
2025-06-30 12:45:01.456 UTC [12346] LOG:  duration: 250.567 ms  statement: SELECT COUNT(*) FROM orders WHERE status = 'pending'
2025-06-30 12:45:02.789 UTC [12347] LOG:  duration: 1234.890 ms  statement: WITH RECURSIVE order_hierarchy AS (SELECT id, user_id, order_date, total_amount, 1 as level FROM orders WHERE user_id = 1 UNION ALL SELECT o.id, o.user_id, o.order_date, o.total_amount, oh.level + 1 FROM orders o JOIN order_hierarchy oh ON o.user_id = oh.user_id WHERE o.order_date > oh.order_date) SELECT * FROM order_hierarchy
2025-06-30 12:45:03.012 UTC [12348] LOG:  duration: 45.678 ms  plan:
	Query Text: SELECT u.username, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id
	HashAggregate  (cost=250.00..275.00 rows=100 width=32) (actual time=45.123..45.456 rows=3 loops=1)
	  Group Key: u.id
	  ->  Hash Left Join  (cost=100.00..200.00 rows=1000 width=16) (actual time=30.123..40.456 rows=50 loops=1)
	        Hash Cond: (o.user_id = u.id)
	        ->  Seq Scan on orders o  (cost=0.00..75.00 rows=1000 width=8) (actual time=0.123..10.456 rows=1000 loops=1)
	        ->  Hash  (cost=50.00..50.00 rows=100 width=16) (actual time=5.123..5.123 rows=100 loops=1)
	              Buckets: 128  Batches: 1  Memory Usage: 5kB
	              ->  Seq Scan on users u  (cost=0.00..50.00 rows=100 width=16) (actual time=0.123..2.456 rows=100 loops=1)
EOF

# Create sample MySQL query log
cat > /tmp/mysql-test.log << 'EOF'
2025-06-30T12:45:00.123456Z	    8 Query	SELECT * FROM users WHERE id = 1
# Time: 2025-06-30T12:45:01.456789Z
# User@Host: root[root] @ localhost []  Id:     8
# Query_time: 0.250567  Lock_time: 0.000123 Rows_sent: 1  Rows_examined: 1000
SET timestamp=1719754321;
SELECT COUNT(*) FROM products WHERE category = 'Electronics';
# Time: 2025-06-30T12:45:02.789012Z
# User@Host: root[root] @ localhost []  Id:     9
# Query_time: 1.234890  Lock_time: 0.001234 Rows_sent: 10  Rows_examined: 50000
SET timestamp=1719754322;
SELECT p.name, p.category, COUNT(oi.id) as times_ordered, SUM(oi.quantity) as total_quantity FROM products p LEFT JOIN order_items oi ON p.id = oi.product_id GROUP BY p.id, p.name, p.category ORDER BY times_ordered DESC LIMIT 10;
EOF

# Create test configuration for log processing
cat > /tmp/test-logs-config.yaml << 'EOF'
receivers:
  filelog/postgres:
    include: [ "/tmp/postgresql-test.log" ]
    start_at: beginning
    operators:
      - type: regex_parser
        regex: '^(?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} \w+) \[(?P<pid>\d+)\] (?P<level>\w+):  duration: (?P<duration>[\d.]+) ms  (?P<rest>.*)'
        timestamp:
          parse_from: attributes.timestamp
          layout: '2006-01-02 15:04:05.000 MST'
      - type: regex_parser
        regex: 'statement: (?P<query>.*)'
        parse_from: attributes.rest
        parse_to: attributes
      - type: remove
        field: attributes.rest
        
  filelog/mysql:
    include: [ "/tmp/mysql-test.log" ]
    start_at: beginning
    multiline:
      line_start_pattern: '^# Time:'
    operators:
      - type: regex_parser
        regex: '# Query_time: (?P<query_time>[\d.]+).*Rows_sent: (?P<rows_sent>\d+).*Rows_examined: (?P<rows_examined>\d+)'
      - type: regex_parser
        regex: 'SET timestamp=\d+;\n(?P<query>.*)'

processors:
  planattributeextractor:
    safe_mode: true
    timeout_ms: 1000
    error_mode: ignore
    enable_debug_logging: true
    
  adaptivesampler:
    in_memory_only: true
    default_sampling_rate: 100
    rules:
      - name: "slow_queries"
        condition: 'float(attributes["duration"]) > 100'
        sampling_rate: 100
      - name: "fast_queries"
        condition: 'float(attributes["duration"]) < 10'
        sampling_rate: 10
        
  circuitbreaker:
    failure_threshold: 5
    timeout: 10s
    half_open_requests: 3
    
  batch:
    timeout: 5s

exporters:
  debug:
    verbosity: detailed
    
  file:
    path: /tmp/processed-logs.json

service:
  telemetry:
    logs:
      level: debug
      
  pipelines:
    logs:
      receivers: [filelog/postgres, filelog/mysql]
      processors: [planattributeextractor, adaptivesampler, circuitbreaker, batch]
      exporters: [debug, file]
EOF

echo "Testing custom processors with query logs..."
./dist/database-intelligence-collector --config=/tmp/test-logs-config.yaml