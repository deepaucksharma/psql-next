#!/bin/bash
# Comprehensive MySQL initialization with realistic data and workload
# Version: 2.0.0

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}      MySQL Comprehensive Initialization v2.0           ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo

# Database connection details
MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-rootpassword}"

# Function to execute MySQL commands
mysql_exec() {
    mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "$1" 2>/dev/null || true
}

# Function to execute MySQL commands with database
mysql_db_exec() {
    local db=$1
    local query=$2
    mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$db" -e "$query" 2>/dev/null || true
}

echo -e "${YELLOW}1. Creating monitoring user...${NC}"
mysql_exec "
CREATE USER IF NOT EXISTS 'otel_monitor'@'%' IDENTIFIED BY 'otel_password';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON mysql.* TO 'otel_monitor'@'%';
GRANT SELECT ON sys.* TO 'otel_monitor'@'%';
FLUSH PRIVILEGES;
"
echo -e "${GREEN}✅ Monitoring user created${NC}"

echo -e "${YELLOW}2. Creating sample databases...${NC}"
# Create multiple databases representing different business functions
for db in ecommerce analytics reporting inventory customers orders payments shipping; do
    mysql_exec "CREATE DATABASE IF NOT EXISTS $db;"
    echo -e "  Created database: $db"
done
echo -e "${GREEN}✅ Sample databases created${NC}"

echo -e "${YELLOW}3. Creating realistic schema...${NC}"

# E-commerce database schema
mysql_db_exec "ecommerce" "
-- Products table
CREATE TABLE IF NOT EXISTS products (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100),
    price DECIMAL(10,2),
    stock_quantity INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_category (category),
    INDEX idx_price (price),
    INDEX idx_stock (stock_quantity)
) ENGINE=InnoDB;

-- Customers table
CREATE TABLE IF NOT EXISTS customers (
    id INT PRIMARY KEY AUTO_INCREMENT,
    email VARCHAR(255) UNIQUE NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP NULL,
    INDEX idx_email (email),
    INDEX idx_name (last_name, first_name)
) ENGINE=InnoDB;

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id INT PRIMARY KEY AUTO_INCREMENT,
    customer_id INT NOT NULL,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status ENUM('pending', 'processing', 'shipped', 'delivered', 'cancelled') DEFAULT 'pending',
    total_amount DECIMAL(10,2),
    shipping_address TEXT,
    INDEX idx_customer (customer_id),
    INDEX idx_status (status),
    INDEX idx_date (order_date),
    FOREIGN KEY (customer_id) REFERENCES customers(id)
) ENGINE=InnoDB;

-- Order items table
CREATE TABLE IF NOT EXISTS order_items (
    id INT PRIMARY KEY AUTO_INCREMENT,
    order_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity INT NOT NULL,
    unit_price DECIMAL(10,2),
    INDEX idx_order (order_id),
    INDEX idx_product (product_id),
    FOREIGN KEY (order_id) REFERENCES orders(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=InnoDB;

-- Shopping cart table (high activity)
CREATE TABLE IF NOT EXISTS shopping_carts (
    id INT PRIMARY KEY AUTO_INCREMENT,
    customer_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity INT DEFAULT 1,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_customer_cart (customer_id),
    FOREIGN KEY (customer_id) REFERENCES customers(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=MEMORY;

-- Reviews table
CREATE TABLE IF NOT EXISTS reviews (
    id INT PRIMARY KEY AUTO_INCREMENT,
    product_id INT NOT NULL,
    customer_id INT NOT NULL,
    rating INT CHECK (rating BETWEEN 1 AND 5),
    comment TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_product_rating (product_id, rating),
    FULLTEXT idx_comment (comment),
    FOREIGN KEY (product_id) REFERENCES products(id),
    FOREIGN KEY (customer_id) REFERENCES customers(id)
) ENGINE=InnoDB;
"

# Analytics database schema
mysql_db_exec "analytics" "
-- Page views tracking
CREATE TABLE IF NOT EXISTS page_views (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    session_id VARCHAR(100),
    user_id INT,
    page_url VARCHAR(500),
    referrer_url VARCHAR(500),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_session (session_id),
    INDEX idx_timestamp (timestamp),
    INDEX idx_page (page_url(100))
) ENGINE=InnoDB;

-- Events tracking
CREATE TABLE IF NOT EXISTS events (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    event_type VARCHAR(50),
    user_id INT,
    event_data JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_type_time (event_type, created_at),
    INDEX idx_user (user_id)
) ENGINE=InnoDB;

-- Aggregated metrics (for heavy queries)
CREATE TABLE IF NOT EXISTS daily_metrics (
    date DATE PRIMARY KEY,
    total_orders INT,
    total_revenue DECIMAL(12,2),
    unique_visitors INT,
    conversion_rate DECIMAL(5,2),
    INDEX idx_date (date)
) ENGINE=InnoDB;
"

echo -e "${GREEN}✅ Realistic schemas created${NC}"

echo -e "${YELLOW}4. Populating with sample data...${NC}"

# Generate sample products
mysql_db_exec "ecommerce" "
INSERT IGNORE INTO products (name, category, price, stock_quantity) VALUES
('Laptop Pro 15', 'Electronics', 1299.99, 50),
('Wireless Mouse', 'Electronics', 29.99, 200),
('Office Chair', 'Furniture', 199.99, 75),
('Standing Desk', 'Furniture', 599.99, 30),
('Coffee Maker', 'Appliances', 89.99, 100),
('Bluetooth Speaker', 'Electronics', 79.99, 150),
('Desk Lamp', 'Furniture', 39.99, 120),
('Monitor 27\"', 'Electronics', 349.99, 80),
('Keyboard Mechanical', 'Electronics', 149.99, 100),
('Webcam HD', 'Electronics', 69.99, 90);
"

# Generate sample customers
mysql_db_exec "ecommerce" "
INSERT IGNORE INTO customers (email, first_name, last_name) 
SELECT 
    CONCAT('user', n, '@example.com'),
    ELT(1 + FLOOR(RAND() * 5), 'John', 'Jane', 'Bob', 'Alice', 'Charlie'),
    ELT(1 + FLOOR(RAND() * 5), 'Smith', 'Johnson', 'Williams', 'Brown', 'Jones')
FROM (
    SELECT a.N + b.N * 10 + c.N * 100 AS n
    FROM 
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) a,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) b,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) c
    LIMIT 1000
) numbers;
"

echo -e "${GREEN}✅ Sample data populated${NC}"

echo -e "${YELLOW}5. Creating stored procedures for workload simulation...${NC}"

# Create procedures for generating workload
mysql_db_exec "ecommerce" "
DELIMITER //

-- Procedure to simulate order placement
CREATE PROCEDURE IF NOT EXISTS place_order(IN cust_id INT)
BEGIN
    DECLARE order_id INT;
    DECLARE total DECIMAL(10,2) DEFAULT 0;
    
    START TRANSACTION;
    
    -- Create order
    INSERT INTO orders (customer_id, status) VALUES (cust_id, 'pending');
    SET order_id = LAST_INSERT_ID();
    
    -- Add random items
    INSERT INTO order_items (order_id, product_id, quantity, unit_price)
    SELECT 
        order_id,
        id,
        FLOOR(1 + RAND() * 3),
        price
    FROM products
    ORDER BY RAND()
    LIMIT FLOOR(1 + RAND() * 5);
    
    -- Calculate total
    SELECT SUM(quantity * unit_price) INTO total
    FROM order_items
    WHERE order_id = order_id;
    
    -- Update order total
    UPDATE orders SET total_amount = total WHERE id = order_id;
    
    COMMIT;
END//

-- Procedure to simulate browsing
CREATE PROCEDURE IF NOT EXISTS browse_products()
BEGIN
    -- Simulate complex product search
    SELECT p.*, 
           COUNT(r.id) as review_count,
           AVG(r.rating) as avg_rating,
           (SELECT COUNT(*) FROM order_items WHERE product_id = p.id) as times_ordered
    FROM products p
    LEFT JOIN reviews r ON p.id = r.product_id
    WHERE p.stock_quantity > 0
    GROUP BY p.id
    ORDER BY RAND()
    LIMIT 10;
END//

-- Procedure to simulate analytics queries
CREATE PROCEDURE IF NOT EXISTS run_analytics()
BEGIN
    -- Heavy aggregation query
    SELECT 
        DATE(o.order_date) as order_day,
        COUNT(DISTINCT o.id) as order_count,
        COUNT(DISTINCT o.customer_id) as unique_customers,
        SUM(o.total_amount) as revenue,
        AVG(o.total_amount) as avg_order_value
    FROM orders o
    WHERE o.order_date >= DATE_SUB(NOW(), INTERVAL 30 DAY)
    GROUP BY DATE(o.order_date)
    ORDER BY order_day DESC;
    
    -- Product performance query
    SELECT 
        p.category,
        COUNT(DISTINCT oi.order_id) as orders,
        SUM(oi.quantity) as units_sold,
        SUM(oi.quantity * oi.unit_price) as revenue
    FROM order_items oi
    JOIN products p ON oi.product_id = p.id
    GROUP BY p.category
    ORDER BY revenue DESC;
END//

-- Procedure to simulate cart operations
CREATE PROCEDURE IF NOT EXISTS manage_cart(IN cust_id INT)
BEGIN
    -- Add items to cart
    INSERT INTO shopping_carts (customer_id, product_id, quantity)
    SELECT cust_id, id, FLOOR(1 + RAND() * 3)
    FROM products
    WHERE stock_quantity > 0
    ORDER BY RAND()
    LIMIT FLOOR(1 + RAND() * 3)
    ON DUPLICATE KEY UPDATE quantity = quantity + 1;
    
    -- Sometimes clear old cart items
    IF RAND() > 0.7 THEN
        DELETE FROM shopping_carts 
        WHERE customer_id = cust_id 
        AND added_at < DATE_SUB(NOW(), INTERVAL 1 HOUR);
    END IF;
END//

DELIMITER ;
"

echo -e "${GREEN}✅ Stored procedures created${NC}"

echo -e "${YELLOW}6. Creating workload generator script...${NC}"

cat > ../scripts/generate-mysql-workload.sh << 'EOF'
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
EOF

chmod +x ../scripts/generate-mysql-workload.sh

echo -e "${GREEN}✅ Workload generator created${NC}"

echo -e "${YELLOW}7. Enabling Performance Schema...${NC}"

mysql_exec "
-- Enable all Performance Schema consumers
UPDATE performance_schema.setup_consumers SET ENABLED = 'YES';

-- Enable all instruments
UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES';

-- Increase history size
SET GLOBAL performance_schema_events_statements_history_size = 100;
SET GLOBAL performance_schema_events_statements_history_long_size = 10000;
"

echo -e "${GREEN}✅ Performance Schema enabled${NC}"

echo
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}      MySQL Initialization Complete!                    ${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo
echo "Next steps:"
echo "1. Start the workload generator: ./generate-mysql-workload.sh"
echo "2. Deploy the collector: ./deploy-master.sh"
echo "3. Check metrics in New Relic"