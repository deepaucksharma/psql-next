-- Sample data generation for ecommerce database
USE ecommerce;

-- Insert sample customers
INSERT INTO customers (email, name) VALUES
('john.doe@email.com', 'John Doe'),
('jane.smith@email.com', 'Jane Smith'),
('mike.johnson@email.com', 'Mike Johnson'),
('sarah.williams@email.com', 'Sarah Williams'),
('david.brown@email.com', 'David Brown');

-- Insert sample products using a number series
INSERT INTO products (sku, name, description, price, stock_quantity)
SELECT 
    CONCAT('SKU-', LPAD(n, 6, '0')),
    CONCAT('Product ', n, ' - ', ELT(1 + MOD(n, 10), 
        'Laptop', 'Mouse', 'Keyboard', 'Monitor', 'Headphones',
        'Desk', 'Chair', 'Lamp', 'Cable', 'Adapter')),
    CONCAT('High quality ', ELT(1 + MOD(n, 5), 
        'electronic device', 'office furniture', 'computer accessory', 
        'premium item', 'professional equipment')),
    ROUND(10 + RAND() * 990, 2),
    FLOOR(10 + RAND() * 200)
FROM (
    SELECT a.N + b.N * 10 + c.N * 100 AS n
    FROM 
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) a,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) b,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2) c
) numbers
WHERE n BETWEEN 1 AND 250;

-- Generate initial orders
DELIMITER //
CREATE PROCEDURE generate_sample_orders()
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE v_customer_id INT;
    DECLARE v_product_id INT;
    DECLARE v_quantity INT;
    
    WHILE i <= 50 DO
        -- Random customer
        SET v_customer_id = 1 + FLOOR(RAND() * 5);
        
        -- Add 3-5 items to cart
        SET @items = 3 + FLOOR(RAND() * 3);
        WHILE @items > 0 DO
            SET v_product_id = 1 + FLOOR(RAND() * 250);
            SET v_quantity = 1 + FLOOR(RAND() * 3);
            
            INSERT IGNORE INTO shopping_cart (customer_id, product_id, quantity)
            VALUES (v_customer_id, v_product_id, v_quantity);
            
            SET @items = @items - 1;
        END WHILE;
        
        -- Place order
        CALL place_order(v_customer_id);
        
        SET i = i + 1;
    END WHILE;
END//
DELIMITER ;

-- Generate sample data
CALL generate_sample_orders();
DROP PROCEDURE generate_sample_orders;

-- Update some orders to different statuses
UPDATE orders SET status = 'completed' WHERE id % 3 = 0;
UPDATE orders SET status = 'shipped' WHERE id % 5 = 0 AND status != 'completed';

-- Create some historical data by backdating orders
UPDATE orders 
SET order_date = DATE_SUB(order_date, INTERVAL FLOOR(RAND() * 30) DAY)
WHERE id % 2 = 0;