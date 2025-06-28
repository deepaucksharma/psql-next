#!/bin/bash
# Generate database load for testing Database Intelligence MVP

echo "Starting database load generator..."

# Database connection parameters
PG_HOST="postgres-primary"
PG_PORT="5432"
PG_USER="postgres"
PG_PASS="postgres123"
PG_DB="testdb"

MYSQL_HOST="mysql-primary"
MYSQL_PORT="3306"
MYSQL_USER="root"
MYSQL_PASS="mysql123"
MYSQL_DB="testdb"

# Wait for databases to be ready
echo "Waiting for databases to be ready..."
sleep 30

# Function to generate PostgreSQL queries
generate_postgres_load() {
    echo "Generating PostgreSQL load..."
    
    while true; do
        # Generate random orders
        PGPASSWORD=$PG_PASS psql -h $PG_HOST -p $PG_PORT -U $PG_USER -d $PG_DB -c "SELECT generate_random_order();" 2>/dev/null
        
        # Run various queries
        PGPASSWORD=$PG_PASS psql -h $PG_HOST -p $PG_PORT -U $PG_USER -d $PG_DB <<EOF 2>/dev/null
        -- Complex join query
        SELECT c.name, COUNT(o.id) as order_count, SUM(o.total_amount) as total_spent
        FROM customers c
        LEFT JOIN orders o ON c.id = o.customer_id
        GROUP BY c.id, c.name
        ORDER BY total_spent DESC;
        
        -- Subquery
        SELECT p.name, p.price, 
            (SELECT AVG(quantity) FROM order_items WHERE product_id = p.id) as avg_quantity
        FROM products p
        WHERE p.stock_quantity > 0;
        
        -- Window function
        SELECT 
            order_date,
            total_amount,
            SUM(total_amount) OVER (ORDER BY order_date) as running_total
        FROM orders
        WHERE order_date > CURRENT_DATE - INTERVAL '7 days';
        
        -- Update query
        UPDATE products 
        SET stock_quantity = stock_quantity - 1 
        WHERE id = (SELECT id FROM products WHERE stock_quantity > 10 ORDER BY RANDOM() LIMIT 1);
        
        -- Analytics query
        SELECT 
            DATE_TRUNC('hour', created_at) as hour,
            COUNT(*) as orders_per_hour,
            AVG(total_amount) as avg_order_value
        FROM orders
        WHERE created_at > CURRENT_TIMESTAMP - INTERVAL '24 hours'
        GROUP BY DATE_TRUNC('hour', created_at)
        ORDER BY hour DESC;
EOF
        
        # Sleep between 1-5 seconds
        sleep $((RANDOM % 5 + 1))
    done
}

# Function to generate MySQL queries
generate_mysql_load() {
    echo "Generating MySQL load..."
    
    while true; do
        # Generate random orders
        mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB -e "CALL generate_random_order();" 2>/dev/null
        
        # Run various queries
        mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB <<EOF 2>/dev/null
        -- Complex join query
        SELECT c.name, COUNT(o.id) as order_count, SUM(o.total_amount) as total_spent
        FROM customers c
        LEFT JOIN orders o ON c.id = o.customer_id
        GROUP BY c.id, c.name
        ORDER BY total_spent DESC;
        
        -- Subquery
        SELECT p.name, p.price, 
            (SELECT AVG(quantity) FROM order_items WHERE product_id = p.id) as avg_quantity
        FROM products p
        WHERE p.stock_quantity > 0;
        
        -- Update query
        UPDATE products 
        SET stock_quantity = stock_quantity - 1 
        WHERE id = (SELECT id FROM products WHERE stock_quantity > 10 ORDER BY RAND() LIMIT 1);
        
        -- Analytics query
        SELECT 
            DATE_FORMAT(created_at, '%Y-%m-%d %H:00:00') as hour,
            COUNT(*) as orders_per_hour,
            AVG(total_amount) as avg_order_value
        FROM orders
        WHERE created_at > DATE_SUB(NOW(), INTERVAL 24 HOUR)
        GROUP BY DATE_FORMAT(created_at, '%Y-%m-%d %H:00:00')
        ORDER BY hour DESC;
        
        -- Full table scan (intentionally slow)
        SELECT COUNT(DISTINCT oi.product_id) as unique_products,
               SUM(oi.quantity * oi.unit_price) as revenue
        FROM order_items oi
        WHERE oi.created_at > DATE_SUB(NOW(), INTERVAL 30 DAY);
EOF
        
        # Sleep between 1-5 seconds
        sleep $((RANDOM % 5 + 1))
    done
}

# Run both load generators in background
generate_postgres_load &
PG_PID=$!

generate_mysql_load &
MYSQL_PID=$!

echo "Load generators started (PG PID: $PG_PID, MySQL PID: $MYSQL_PID)"

# Keep the script running
wait