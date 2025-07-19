#!/bin/bash
# MySQL workload generator
# Runs various queries to generate metrics

MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-rootpassword}"

echo "Starting MySQL workload generation..."

while true; do
    # Random customer ID
    CUSTOMER_ID=$((1 + RANDOM % 1000))
    
    # Place orders (20% chance)
    if [ $((RANDOM % 100)) -lt 20 ]; then
        mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" ecommerce \
            -e "CALL place_order($CUSTOMER_ID);" 2>/dev/null &
    fi
    
    # Browse products (40% chance)
    if [ $((RANDOM % 100)) -lt 40 ]; then
        mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" ecommerce \
            -e "CALL browse_products();" 2>/dev/null &
    fi
    
    # Run analytics (10% chance)
    if [ $((RANDOM % 100)) -lt 10 ]; then
        mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" ecommerce \
            -e "CALL run_analytics();" 2>/dev/null &
    fi
    
    # Cart operations (30% chance)
    if [ $((RANDOM % 100)) -lt 30 ]; then
        mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" ecommerce \
            -e "CALL manage_cart($CUSTOMER_ID);" 2>/dev/null &
    fi
    
    # Performance schema queries
    mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" \
        -e "SELECT * FROM performance_schema.events_statements_summary_by_digest ORDER BY SUM_TIMER_WAIT DESC LIMIT 10;" 2>/dev/null &
    
    # Wait a bit
    sleep 0.5
done
