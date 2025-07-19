FROM mysql:8.0

# Install additional tools for monitoring
RUN apt-get update && apt-get install -y \
    curl \
    netcat-traditional \
    && rm -rf /var/lib/apt/lists/*

# Create directory for custom configurations
RUN mkdir -p /etc/mysql/conf.d

# Add custom MySQL configuration for replication
COPY <<EOF /etc/mysql/conf.d/replication.cnf
[mysqld]
# Basic settings
max_connections = 1000
innodb_buffer_pool_size = 256M

# Replication settings
log_bin_trust_function_creators = 1
expire_logs_days = 7
max_binlog_size = 100M
sync_binlog = 1

# Performance schema for monitoring
performance_schema = ON
performance_schema_instrument = '%=ON'

# Slow query log
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 2
EOF

EXPOSE 3306

CMD ["mysqld"]