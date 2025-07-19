#!/bin/bash

# Module testing script
MODULE=$1
MODULE_DIR="modules/$MODULE"

if [ -z "$MODULE" ]; then
    echo "Usage: $0 <module-name>"
    echo "Available modules:"
    ls -1 modules/ | grep -v "^\."
    exit 1
fi

if [ ! -d "$MODULE_DIR" ]; then
    echo "Error: Module '$MODULE' not found in $MODULE_DIR"
    exit 1
fi

echo "Testing module: $MODULE"
cd "$MODULE_DIR"

# Start module in isolation
echo "Starting $MODULE..."
docker-compose up -d

# Get port from docker-compose.yaml
PORT=$(grep -A1 "ports:" docker-compose.yaml | grep -o '[0-9]\{4\}' | head -1)

# Wait for health
echo "Waiting for $MODULE to be healthy..."
timeout 30 bash -c "until curl -f http://localhost:$PORT/metrics 2>/dev/null; do sleep 1; done"

if [ $? -eq 0 ]; then
    echo "✓ $MODULE is running and healthy on port $PORT"
    
    # Run module-specific tests if available
    if [ -f "Makefile" ] && grep -q "test:" "Makefile"; then
        echo "Running module tests..."
        make test
    fi
else
    echo "✗ $MODULE failed to start or become healthy"
    docker-compose logs
    docker-compose down
    exit 1
fi

# Cleanup
echo "Cleaning up..."
docker-compose down

echo "✓ $MODULE test completed successfully"