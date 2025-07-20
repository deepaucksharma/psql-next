FROM mysql:8.0

# WARNING: DO NOT ADD HEALTHCHECK instructions here!
# Health checks have been intentionally removed from production images.
# Use shared/validation/health-check-all.sh for validation purposes.

# MySQL 8.0 image is based on Oracle Linux and uses microdnf
# Skip the installation of extra tools as they're not essential

# Create directory for custom configurations
RUN mkdir -p /etc/mysql/conf.d

# Add custom MySQL configuration for replication
RUN echo "[mysqld]" > /etc/mysql/conf.d/replication.cnf && \
    echo "# Basic settings" >> /etc/mysql/conf.d/replication.cnf && \
    echo "max_connections = 1000" >> /etc/mysql/conf.d/replication.cnf && \
    echo "innodb_buffer_pool_size = 256M" >> /etc/mysql/conf.d/replication.cnf && \
    echo "" >> /etc/mysql/conf.d/replication.cnf && \
    echo "# Replication settings" >> /etc/mysql/conf.d/replication.cnf && \
    echo "log_bin_trust_function_creators = 1" >> /etc/mysql/conf.d/replication.cnf && \
    echo "expire_logs_days = 7" >> /etc/mysql/conf.d/replication.cnf && \
    echo "max_binlog_size = 100M" >> /etc/mysql/conf.d/replication.cnf && \
    echo "sync_binlog = 1" >> /etc/mysql/conf.d/replication.cnf && \
    echo "" >> /etc/mysql/conf.d/replication.cnf && \
    echo "# Performance schema for monitoring" >> /etc/mysql/conf.d/replication.cnf && \
    echo "performance_schema = ON" >> /etc/mysql/conf.d/replication.cnf && \
    echo "performance_schema_instrument = '%=ON'" >> /etc/mysql/conf.d/replication.cnf && \
    echo "" >> /etc/mysql/conf.d/replication.cnf && \
    echo "# Slow query log" >> /etc/mysql/conf.d/replication.cnf && \
    echo "slow_query_log = 1" >> /etc/mysql/conf.d/replication.cnf && \
    echo "slow_query_log_file = /var/log/mysql/slow.log" >> /etc/mysql/conf.d/replication.cnf && \
    echo "long_query_time = 2" >> /etc/mysql/conf.d/replication.cnf

# Expose ports for MySQL functionality only
EXPOSE 3306
# WARNING: DO NOT expose port 13133 (health check port)!

CMD ["mysqld"]