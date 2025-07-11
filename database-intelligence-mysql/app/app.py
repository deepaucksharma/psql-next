#!/usr/bin/env python3
"""
Sample application to generate MySQL traffic for monitoring demonstration
"""

import os
import time
import random
import logging
import mysql.connector
from mysql.connector import Error
from faker import Faker
import schedule
from datetime import datetime, timedelta

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

fake = Faker()

class MySQLApp:
    def __init__(self):
        self.config = {
            'host': os.environ.get('MYSQL_HOST', 'mysql-primary'),
            'port': int(os.environ.get('MYSQL_PORT', 3306)),
            'user': os.environ.get('MYSQL_USER', 'appuser'),
            'password': os.environ.get('MYSQL_PASSWORD', 'apppassword'),
            'database': os.environ.get('MYSQL_DATABASE', 'ecommerce'),
            'autocommit': True
        }
        self.connection = None
        
    def connect(self):
        """Establish MySQL connection"""
        try:
            if self.connection and self.connection.is_connected():
                return
            
            self.connection = mysql.connector.connect(**self.config)
            logger.info("Connected to MySQL database")
        except Error as e:
            logger.error(f"Error connecting to MySQL: {e}")
            time.sleep(5)
            self.connect()
    
    def ensure_connection(self):
        """Ensure database connection is active"""
        if not self.connection or not self.connection.is_connected():
            self.connect()
    
    def create_customer(self):
        """Create a new customer"""
        self.ensure_connection()
        cursor = self.connection.cursor()
        
        try:
            email = fake.email()
            first_name = fake.first_name()
            last_name = fake.last_name()
            
            query = """
                INSERT INTO customers (email, first_name, last_name)
                VALUES (%s, %s, %s)
            """
            cursor.execute(query, (email, first_name, last_name))
            customer_id = cursor.lastrowid
            logger.info(f"Created customer: {first_name} {last_name} (ID: {customer_id})")
            return customer_id
        except Error as e:
            logger.error(f"Error creating customer: {e}")
            return None
        finally:
            cursor.close()
    
    def create_order(self):
        """Create a new order with random items"""
        self.ensure_connection()
        cursor = self.connection.cursor()
        
        try:
            # Get random customer
            cursor.execute("SELECT customer_id FROM customers ORDER BY RAND() LIMIT 1")
            result = cursor.fetchone()
            if not result:
                logger.warning("No customers found, creating one")
                customer_id = self.create_customer()
                if not customer_id:
                    return
            else:
                customer_id = result[0]
            
            # Create order
            cursor.execute(
                "INSERT INTO orders (customer_id, total_amount) VALUES (%s, 0)",
                (customer_id,)
            )
            order_id = cursor.lastrowid
            
            # Add random items
            num_items = random.randint(1, 5)
            total_amount = 0
            
            for _ in range(num_items):
                cursor.execute(
                    "SELECT product_id, price FROM products ORDER BY RAND() LIMIT 1"
                )
                product = cursor.fetchone()
                if product:
                    product_id, price = product
                    quantity = random.randint(1, 3)
                    
                    cursor.execute(
                        """INSERT INTO order_items 
                           (order_id, product_id, quantity, unit_price)
                           VALUES (%s, %s, %s, %s)""",
                        (order_id, product_id, quantity, price)
                    )
                    total_amount += quantity * float(price)
            
            # Update order total
            cursor.execute(
                "UPDATE orders SET total_amount = %s WHERE order_id = %s",
                (total_amount, order_id)
            )
            
            logger.info(f"Created order {order_id} with {num_items} items, total: ${total_amount:.2f}")
            
        except Error as e:
            logger.error(f"Error creating order: {e}")
        finally:
            cursor.close()
    
    def run_analytics_query(self):
        """Run analytics queries to generate read load"""
        self.ensure_connection()
        cursor = self.connection.cursor()
        
        queries = [
            # Top customers by order value
            """
            SELECT c.email, COUNT(o.order_id) as order_count, 
                   SUM(o.total_amount) as total_spent
            FROM customers c
            LEFT JOIN orders o ON c.customer_id = o.customer_id
            WHERE o.order_date >= DATE_SUB(NOW(), INTERVAL 30 DAY)
            GROUP BY c.customer_id
            ORDER BY total_spent DESC
            LIMIT 10
            """,
            
            # Product sales analysis
            """
            SELECT p.name, SUM(oi.quantity) as units_sold,
                   SUM(oi.quantity * oi.unit_price) as revenue
            FROM products p
            JOIN order_items oi ON p.product_id = oi.product_id
            JOIN orders o ON oi.order_id = o.order_id
            WHERE o.order_date >= DATE_SUB(NOW(), INTERVAL 7 DAY)
            GROUP BY p.product_id
            ORDER BY revenue DESC
            """,
            
            # Order status distribution
            """
            SELECT status, COUNT(*) as count,
                   AVG(total_amount) as avg_order_value
            FROM orders
            GROUP BY status
            """,
            
            # Inventory check (intentionally complex)
            """
            SELECT p.sku, p.name, p.stock_quantity,
                   COALESCE(SUM(oi.quantity), 0) as pending_orders
            FROM products p
            LEFT JOIN order_items oi ON p.product_id = oi.product_id
            LEFT JOIN orders o ON oi.order_id = o.order_id
            WHERE o.status IN ('pending', 'processing')
               OR o.order_id IS NULL
            GROUP BY p.product_id
            HAVING p.stock_quantity < 20
               OR pending_orders > p.stock_quantity * 0.5
            """
        ]
        
        try:
            query = random.choice(queries)
            start_time = time.time()
            cursor.execute(query)
            results = cursor.fetchall()
            execution_time = (time.time() - start_time) * 1000
            
            logger.info(f"Analytics query returned {len(results)} rows in {execution_time:.2f}ms")
            
        except Error as e:
            logger.error(f"Error running analytics query: {e}")
        finally:
            cursor.close()
    
    def simulate_slow_query(self):
        """Intentionally run a slow query for testing"""
        self.ensure_connection()
        cursor = self.connection.cursor()
        
        try:
            # This query will be slow due to lack of proper indexes and complexity
            query = """
            SELECT 
                c1.customer_id,
                c1.email,
                (SELECT COUNT(*) FROM orders o1 WHERE o1.customer_id = c1.customer_id) as order_count,
                (SELECT SUM(total_amount) FROM orders o2 WHERE o2.customer_id = c1.customer_id) as total_spent,
                (SELECT GROUP_CONCAT(DISTINCT p.name) 
                 FROM products p
                 JOIN order_items oi ON p.product_id = oi.product_id
                 JOIN orders o ON oi.order_id = o.order_id
                 WHERE o.customer_id = c1.customer_id) as products_purchased
            FROM customers c1
            WHERE EXISTS (
                SELECT 1 FROM orders o3 
                WHERE o3.customer_id = c1.customer_id 
                AND o3.order_date >= DATE_SUB(NOW(), INTERVAL 90 DAY)
            )
            ORDER BY total_spent DESC
            """
            
            start_time = time.time()
            cursor.execute(query)
            results = cursor.fetchall()
            execution_time = (time.time() - start_time) * 1000
            
            logger.warning(f"Slow query executed in {execution_time:.2f}ms, returned {len(results)} rows")
            
        except Error as e:
            logger.error(f"Error running slow query: {e}")
        finally:
            cursor.close()
    
    def update_order_status(self):
        """Update random order statuses"""
        self.ensure_connection()
        cursor = self.connection.cursor()
        
        try:
            # Get pending orders
            cursor.execute(
                """SELECT order_id FROM orders 
                   WHERE status = 'pending' 
                   AND order_date < DATE_SUB(NOW(), INTERVAL 1 HOUR)
                   LIMIT 10"""
            )
            orders = cursor.fetchall()
            
            statuses = ['processing', 'shipped', 'delivered']
            for order in orders:
                new_status = random.choice(statuses)
                cursor.execute(
                    "UPDATE orders SET status = %s WHERE order_id = %s",
                    (new_status, order[0])
                )
                logger.info(f"Updated order {order[0]} to status: {new_status}")
                
        except Error as e:
            logger.error(f"Error updating order status: {e}")
        finally:
            cursor.close()
    
    def generate_traffic(self):
        """Generate mixed traffic patterns"""
        operations = [
            (self.create_order, 40),          # 40% chance
            (self.run_analytics_query, 30),   # 30% chance
            (self.update_order_status, 20),   # 20% chance
            (self.create_customer, 10),       # 10% chance
        ]
        
        # Weighted random selection
        total_weight = sum(weight for _, weight in operations)
        rand = random.randint(1, total_weight)
        
        cumulative = 0
        for operation, weight in operations:
            cumulative += weight
            if rand <= cumulative:
                operation()
                break
        
        # Occasionally run slow query (5% chance)
        if random.random() < 0.05:
            self.simulate_slow_query()
    
    def run(self):
        """Main application loop"""
        logger.info("Starting MySQL monitoring demo application...")
        
        # Initial connection
        self.connect()
        
        # Schedule regular operations
        schedule.every(2).seconds.do(self.generate_traffic)
        schedule.every(30).seconds.do(self.simulate_slow_query)
        
        while True:
            try:
                schedule.run_pending()
                time.sleep(1)
            except KeyboardInterrupt:
                logger.info("Shutting down...")
                break
            except Exception as e:
                logger.error(f"Unexpected error: {e}")
                time.sleep(5)
        
        if self.connection and self.connection.is_connected():
            self.connection.close()
            logger.info("MySQL connection closed")

if __name__ == "__main__":
    # Wait for MySQL to be ready
    logger.info("Waiting for MySQL to be ready...")
    time.sleep(10)
    
    # Install dependencies if needed
    try:
        import mysql.connector
        from faker import Faker
        import schedule
    except ImportError:
        logger.info("Installing required packages...")
        os.system("pip install -r requirements.txt")
    
    app = MySQLApp()
    app.run()