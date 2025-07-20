#!/usr/bin/env python3
"""
Load Generator for Database Intelligence System Benchmarking

Generates realistic database workloads to test monitoring system performance
under various conditions including:
- High connection rates
- Complex queries
- Lock contention
- Replication lag simulation
- Resource stress scenarios
"""

import time
import threading
import random
import string
import mysql.connector
from mysql.connector import pooling
from typing import Dict, List, Any, Optional
from dataclasses import dataclass
from datetime import datetime, timedelta
import json
import os
import sys
from pathlib import Path
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


@dataclass
class LoadProfile:
    """Defines a load generation profile"""
    name: str
    duration: int  # seconds
    connections: int
    queries_per_second: int
    read_ratio: float  # 0.0 to 1.0
    slow_query_ratio: float  # 0.0 to 1.0
    lock_contention_ratio: float  # 0.0 to 1.0
    batch_operations: bool = False
    long_transactions: bool = False
    metadata: Dict[str, Any] = None


class LoadGenerator:
    """Main load generation class"""
    
    def __init__(self, db_config: Dict[str, Any], max_connections: int = 50):
        self.db_config = db_config
        self.max_connections = max_connections
        self.running = False
        self.stats = {
            'queries_executed': 0,
            'errors': 0,
            'slow_queries': 0,
            'locks_created': 0,
            'transactions': 0
        }
        
        # Create connection pool
        self.pool = pooling.MySQLConnectionPool(
            pool_name="load_generator_pool",
            pool_size=max_connections,
            pool_reset_session=True,
            **db_config
        )
        
        # Test data
        self.test_tables_created = False
        self.test_data = []
        
    def setup_test_environment(self):
        """Create test tables and data"""
        logger.info("Setting up test environment...")
        
        conn = self.pool.get_connection()
        cursor = conn.cursor()
        
        try:
            # Create test database if it doesn't exist
            cursor.execute("CREATE DATABASE IF NOT EXISTS benchmark_test")
            cursor.execute("USE benchmark_test")
            
            # Create test tables
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS users (
                    id INT PRIMARY KEY AUTO_INCREMENT,
                    username VARCHAR(50) UNIQUE,
                    email VARCHAR(100),
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
                    INDEX idx_username (username),
                    INDEX idx_email (email)
                )
            """)
            
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS orders (
                    id INT PRIMARY KEY AUTO_INCREMENT,
                    user_id INT,
                    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    total_amount DECIMAL(10, 2),
                    status VARCHAR(20),
                    FOREIGN KEY (user_id) REFERENCES users(id),
                    INDEX idx_user_id (user_id),
                    INDEX idx_status (status),
                    INDEX idx_order_date (order_date)
                )
            """)
            
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS products (
                    id INT PRIMARY KEY AUTO_INCREMENT,
                    name VARCHAR(100),
                    price DECIMAL(10, 2),
                    stock INT,
                    category VARCHAR(50),
                    INDEX idx_category (category),
                    INDEX idx_price (price)
                )
            """)
            
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS order_items (
                    id INT PRIMARY KEY AUTO_INCREMENT,
                    order_id INT,
                    product_id INT,
                    quantity INT,
                    price DECIMAL(10, 2),
                    FOREIGN KEY (order_id) REFERENCES orders(id),
                    FOREIGN KEY (product_id) REFERENCES products(id),
                    INDEX idx_order_id (order_id)
                )
            """)
            
            # Insert initial test data
            logger.info("Inserting test data...")
            
            # Insert users
            for i in range(1000):
                username = f"user_{i}_{self._random_string(5)}"
                email = f"{username}@example.com"
                cursor.execute(
                    "INSERT INTO users (username, email) VALUES (%s, %s)",
                    (username, email)
                )
                
            # Insert products
            categories = ['Electronics', 'Books', 'Clothing', 'Food', 'Toys']
            for i in range(500):
                cursor.execute(
                    "INSERT INTO products (name, price, stock, category) VALUES (%s, %s, %s, %s)",
                    (f"Product_{i}", random.uniform(10, 1000), random.randint(0, 1000), random.choice(categories))
                )
                
            conn.commit()
            self.test_tables_created = True
            logger.info("Test environment setup complete")
            
        except Exception as e:
            logger.error(f"Error setting up test environment: {e}")
            conn.rollback()
            
        finally:
            cursor.close()
            conn.close()
            
    def generate_load(self, profile: LoadProfile):
        """Generate load based on the specified profile"""
        if not self.test_tables_created:
            self.setup_test_environment()
            
        logger.info(f"Starting load generation: {profile.name}")
        logger.info(f"Duration: {profile.duration}s, Connections: {profile.connections}, QPS: {profile.queries_per_second}")
        
        self.running = True
        self.stats = {
            'queries_executed': 0,
            'errors': 0,
            'slow_queries': 0,
            'locks_created': 0,
            'transactions': 0
        }
        
        start_time = time.time()
        threads = []
        
        # Start worker threads
        for i in range(profile.connections):
            thread = threading.Thread(
                target=self._worker_thread,
                args=(profile, start_time),
                name=f"Worker-{i}"
            )
            thread.daemon = True
            threads.append(thread)
            thread.start()
            
        # Monitor progress
        while time.time() - start_time < profile.duration and self.running:
            time.sleep(1)
            elapsed = time.time() - start_time
            qps = self.stats['queries_executed'] / elapsed if elapsed > 0 else 0
            logger.info(f"Progress: {elapsed:.0f}s, Queries: {self.stats['queries_executed']}, QPS: {qps:.1f}, Errors: {self.stats['errors']}")
            
        # Stop load generation
        self.running = False
        
        # Wait for threads to complete
        for thread in threads:
            thread.join(timeout=5)
            
        # Final statistics
        total_time = time.time() - start_time
        logger.info(f"\nLoad generation complete for profile: {profile.name}")
        logger.info(f"Total duration: {total_time:.2f}s")
        logger.info(f"Total queries: {self.stats['queries_executed']}")
        logger.info(f"Average QPS: {self.stats['queries_executed'] / total_time:.2f}")
        logger.info(f"Errors: {self.stats['errors']}")
        logger.info(f"Slow queries: {self.stats['slow_queries']}")
        logger.info(f"Lock operations: {self.stats['locks_created']}")
        logger.info(f"Transactions: {self.stats['transactions']}")
        
        return self.stats
        
    def _worker_thread(self, profile: LoadProfile, start_time: float):
        """Worker thread that executes queries"""
        conn = None
        
        try:
            conn = self.pool.get_connection()
            cursor = conn.cursor()
            cursor.execute("USE benchmark_test")
            
            queries_per_connection = profile.queries_per_second / profile.connections
            query_interval = 1.0 / queries_per_connection if queries_per_connection > 0 else 1.0
            
            while self.running and time.time() - start_time < profile.duration:
                try:
                    # Determine query type based on profile
                    is_read = random.random() < profile.read_ratio
                    is_slow = random.random() < profile.slow_query_ratio
                    create_lock = random.random() < profile.lock_contention_ratio
                    
                    if profile.long_transactions and random.random() < 0.1:
                        self._execute_long_transaction(cursor, conn)
                    elif profile.batch_operations and random.random() < 0.2:
                        self._execute_batch_operation(cursor, conn)
                    elif create_lock:
                        self._execute_lock_query(cursor, conn)
                    elif is_read:
                        self._execute_read_query(cursor, is_slow)
                    else:
                        self._execute_write_query(cursor, conn)
                        
                    self.stats['queries_executed'] += 1
                    
                    # Rate limiting
                    time.sleep(query_interval)
                    
                except mysql.connector.Error as e:
                    self.stats['errors'] += 1
                    logger.debug(f"Query error: {e}")
                    
        except Exception as e:
            logger.error(f"Worker thread error: {e}")
            
        finally:
            if conn and conn.is_connected():
                conn.close()
                
    def _execute_read_query(self, cursor, is_slow: bool = False):
        """Execute a read query"""
        queries = [
            # Simple queries
            "SELECT COUNT(*) FROM users",
            "SELECT * FROM users ORDER BY created_at DESC LIMIT 10",
            "SELECT * FROM products WHERE category = %s LIMIT 20",
            "SELECT u.username, COUNT(o.id) as order_count FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id LIMIT 10",
            
            # Complex queries (slower)
            """
            SELECT u.username, SUM(oi.quantity * oi.price) as total_spent
            FROM users u
            JOIN orders o ON u.id = o.user_id
            JOIN order_items oi ON o.id = oi.order_id
            WHERE o.order_date > DATE_SUB(NOW(), INTERVAL 30 DAY)
            GROUP BY u.id
            ORDER BY total_spent DESC
            LIMIT 100
            """,
            
            """
            SELECT p.category, AVG(p.price) as avg_price, COUNT(*) as product_count
            FROM products p
            WHERE p.stock > 0
            GROUP BY p.category
            HAVING COUNT(*) > 10
            ORDER BY avg_price DESC
            """
        ]
        
        if is_slow:
            # Force a slow query with large result set and no index usage
            query = """
                SELECT u1.*, u2.*
                FROM users u1
                CROSS JOIN users u2
                WHERE SUBSTRING(u1.username, 1, 3) = SUBSTRING(u2.username, 1, 3)
                LIMIT 1000
            """
            self.stats['slow_queries'] += 1
        else:
            query = random.choice(queries)
            
        # Execute query with parameters if needed
        if '%s' in query:
            cursor.execute(query, (random.choice(['Electronics', 'Books', 'Clothing']),))
        else:
            cursor.execute(query)
            
        # Fetch results to complete the query
        cursor.fetchall()
        
    def _execute_write_query(self, cursor, conn):
        """Execute a write query"""
        query_type = random.choice(['insert', 'update', 'delete'])
        
        if query_type == 'insert':
            # Insert new order
            cursor.execute("SELECT id FROM users ORDER BY RAND() LIMIT 1")
            user_id = cursor.fetchone()[0]
            
            cursor.execute(
                "INSERT INTO orders (user_id, total_amount, status) VALUES (%s, %s, %s)",
                (user_id, random.uniform(10, 1000), random.choice(['pending', 'completed', 'cancelled']))
            )
            
        elif query_type == 'update':
            # Update product stock
            cursor.execute(
                "UPDATE products SET stock = stock + %s WHERE id = %s",
                (random.randint(-10, 10), random.randint(1, 500))
            )
            
        else:  # delete
            # Delete old orders
            cursor.execute(
                "DELETE FROM orders WHERE status = 'cancelled' AND order_date < DATE_SUB(NOW(), INTERVAL 90 DAY) LIMIT 10"
            )
            
        conn.commit()
        
    def _execute_lock_query(self, cursor, conn):
        """Execute queries that create lock contention"""
        self.stats['locks_created'] += 1
        
        # Start transaction
        conn.start_transaction()
        
        try:
            # Lock a random user row
            user_id = random.randint(1, 100)  # Focus on first 100 users for contention
            cursor.execute("SELECT * FROM users WHERE id = %s FOR UPDATE", (user_id,))
            
            # Simulate some work
            time.sleep(random.uniform(0.01, 0.1))
            
            # Update the locked row
            cursor.execute(
                "UPDATE users SET updated_at = NOW() WHERE id = %s",
                (user_id,)
            )
            
            conn.commit()
            
        except:
            conn.rollback()
            
    def _execute_long_transaction(self, cursor, conn):
        """Execute a long-running transaction"""
        self.stats['transactions'] += 1
        
        conn.start_transaction()
        
        try:
            # Multiple operations in one transaction
            for _ in range(random.randint(5, 20)):
                operation = random.choice(['insert', 'update', 'select'])
                
                if operation == 'insert':
                    cursor.execute(
                        "INSERT INTO order_items (order_id, product_id, quantity, price) VALUES (%s, %s, %s, %s)",
                        (random.randint(1, 100), random.randint(1, 500), random.randint(1, 10), random.uniform(10, 100))
                    )
                elif operation == 'update':
                    cursor.execute(
                        "UPDATE products SET stock = stock - %s WHERE id = %s AND stock >= %s",
                        (1, random.randint(1, 500), 1)
                    )
                else:
                    cursor.execute("SELECT COUNT(*) FROM order_items WHERE order_id = %s", (random.randint(1, 100),))
                    cursor.fetchall()
                    
                # Simulate processing time
                time.sleep(random.uniform(0.01, 0.05))
                
            conn.commit()
            
        except:
            conn.rollback()
            
    def _execute_batch_operation(self, cursor, conn):
        """Execute batch operations"""
        batch_size = random.randint(10, 100)
        
        # Batch insert
        values = []
        for _ in range(batch_size):
            username = f"batch_user_{self._random_string(10)}"
            email = f"{username}@example.com"
            values.append((username, email))
            
        cursor.executemany(
            "INSERT INTO users (username, email) VALUES (%s, %s)",
            values
        )
        
        conn.commit()
        self.stats['queries_executed'] += batch_size - 1  # Account for additional queries
        
    def _random_string(self, length: int) -> str:
        """Generate a random string"""
        return ''.join(random.choices(string.ascii_lowercase + string.digits, k=length))
        
    def cleanup(self):
        """Cleanup test environment"""
        logger.info("Cleaning up test environment...")
        
        conn = self.pool.get_connection()
        cursor = conn.cursor()
        
        try:
            cursor.execute("DROP DATABASE IF EXISTS benchmark_test")
            conn.commit()
            logger.info("Test environment cleaned up")
            
        except Exception as e:
            logger.error(f"Error during cleanup: {e}")
            
        finally:
            cursor.close()
            conn.close()


# Predefined load profiles
LOAD_PROFILES = {
    "light": LoadProfile(
        name="Light Load",
        duration=60,
        connections=5,
        queries_per_second=50,
        read_ratio=0.8,
        slow_query_ratio=0.05,
        lock_contention_ratio=0.01
    ),
    
    "moderate": LoadProfile(
        name="Moderate Load",
        duration=120,
        connections=20,
        queries_per_second=200,
        read_ratio=0.7,
        slow_query_ratio=0.1,
        lock_contention_ratio=0.05,
        batch_operations=True
    ),
    
    "heavy": LoadProfile(
        name="Heavy Load",
        duration=180,
        connections=50,
        queries_per_second=500,
        read_ratio=0.6,
        slow_query_ratio=0.15,
        lock_contention_ratio=0.1,
        batch_operations=True,
        long_transactions=True
    ),
    
    "read_heavy": LoadProfile(
        name="Read-Heavy Load",
        duration=120,
        connections=30,
        queries_per_second=400,
        read_ratio=0.95,
        slow_query_ratio=0.05,
        lock_contention_ratio=0.01
    ),
    
    "write_heavy": LoadProfile(
        name="Write-Heavy Load",
        duration=120,
        connections=30,
        queries_per_second=300,
        read_ratio=0.2,
        slow_query_ratio=0.05,
        lock_contention_ratio=0.15,
        batch_operations=True
    ),
    
    "lock_contention": LoadProfile(
        name="Lock Contention Test",
        duration=60,
        connections=40,
        queries_per_second=200,
        read_ratio=0.3,
        slow_query_ratio=0.1,
        lock_contention_ratio=0.4,
        long_transactions=True
    )
}


def main():
    """Main entry point"""
    import argparse
    
    parser = argparse.ArgumentParser(description="Database Load Generator")
    parser.add_argument("--profile", choices=list(LOAD_PROFILES.keys()), default="moderate",
                        help="Load profile to use")
    parser.add_argument("--duration", type=int, help="Override profile duration (seconds)")
    parser.add_argument("--connections", type=int, help="Override profile connections")
    parser.add_argument("--qps", type=int, help="Override profile queries per second")
    parser.add_argument("--cleanup", action="store_true", help="Cleanup test environment")
    
    args = parser.parse_args()
    
    # Database configuration
    db_config = {
        'host': os.getenv('MYSQL_HOST', 'localhost'),
        'port': int(os.getenv('MYSQL_PORT', '3306')),
        'user': os.getenv('MYSQL_USER', 'root'),
        'password': os.getenv('MYSQL_PASSWORD', ''),
        'raise_on_warnings': True
    }
    
    # Create load generator
    generator = LoadGenerator(db_config)
    
    if args.cleanup:
        generator.cleanup()
        return
        
    # Get load profile
    profile = LOAD_PROFILES[args.profile]
    
    # Override profile settings if specified
    if args.duration:
        profile.duration = args.duration
    if args.connections:
        profile.connections = args.connections
    if args.qps:
        profile.queries_per_second = args.qps
        
    # Generate load
    try:
        stats = generator.generate_load(profile)
        
        # Save statistics
        with open(f"load_stats_{profile.name.lower().replace(' ', '_')}.json", 'w') as f:
            json.dump({
                'profile': profile.name,
                'timestamp': datetime.now().isoformat(),
                'duration': profile.duration,
                'stats': stats
            }, f, indent=2)
            
    except KeyboardInterrupt:
        logger.info("\nLoad generation interrupted by user")
        generator.running = False


if __name__ == "__main__":
    main()