#!/bin/bash

set -euo pipefail

# Function to generate random load on MySQL
generate_load() {
    local host=${1:-mysql-primary}
    local iterations=${2:-100}
    
    echo "Generating load on $host..."
    
    for i in $(seq 1 $iterations); do
        # Generate random orders
        docker compose exec -T mysql-primary mysql -uappuser -papppassword ecommerce -e "CALL generate_orders(5);" 2>/dev/null || true
        
        # Run some analytical queries
        docker compose exec -T mysql-primary mysql -uappuser -papppassword ecommerce -e "
            SELECT c.email, COUNT(o.order_id) as order_count, SUM(o.total_amount) as total_spent
            FROM customers c
            LEFT JOIN orders o ON c.customer_id = o.customer_id
            GROUP BY c.customer_id
            ORDER BY total_spent DESC
            LIMIT 10;
        " > /dev/null 2>&1 || true
        
        # Simulate slow query
        if [ $((i % 10)) -eq 0 ]; then
            docker compose exec -T mysql-primary mysql -uappuser -papppassword ecommerce -e "CALL simulate_slow_query();" 2>/dev/null || true
        fi
        
        # Random sleep between queries
        sleep $(awk 'BEGIN{print (rand()*2)}')
        
        echo -ne "\rProgress: $i/$iterations"
    done
    
    echo -e "\nLoad generation completed!"
}

# Parse command line arguments
ITERATIONS=${1:-100}
HOST=${2:-mysql-primary}

echo "MySQL Load Generator"
echo "==================="
echo "Iterations: $ITERATIONS"
echo "Target: $HOST"
echo ""

# Check if services are running
if ! docker compose ps | grep -q "mysql-primary.*running"; then
    echo "Error: MySQL services are not running. Run ./scripts/setup.sh first."
    exit 1
fi

# Start load generation
generate_load $HOST $ITERATIONS

echo ""
echo "Check metrics in New Relic: https://one.newrelic.com/"