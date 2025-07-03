#!/bin/bash

# Database Intelligence Dashboard Deployment Script
# This script verifies metrics collection and creates the dashboard

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "========================================"
echo "Database Intelligence Dashboard Deployment"
echo "========================================"
echo ""

# Check for required environment variables
if [ ! -f "$PROJECT_ROOT/.env" ]; then
    echo "❌ Error: .env file not found in project root"
    echo "Please create a .env file with your New Relic credentials"
    exit 1
fi

# Source the environment variables
export $(cat "$PROJECT_ROOT/.env" | grep -v '^#' | xargs)

if [ -z "$NEW_RELIC_USER_KEY" ]; then
    echo "❌ Error: NEW_RELIC_USER_KEY not found in .env file"
    echo "Please add your New Relic User API key to the .env file"
    exit 1
fi

# Check for Node.js
if ! command -v node &> /dev/null; then
    echo "❌ Error: Node.js is not installed"
    echo "Please install Node.js to run the dashboard scripts"
    exit 1
fi

# Install dependencies if needed
if [ ! -d "$PROJECT_ROOT/node_modules" ]; then
    echo "📦 Installing Node.js dependencies..."
    cd "$PROJECT_ROOT"
    npm install dotenv
    cd "$SCRIPT_DIR"
fi

# Make scripts executable
chmod +x "$SCRIPT_DIR/verify-collected-metrics.js"
chmod +x "$SCRIPT_DIR/create-database-dashboard.js"

# Step 1: Verify metrics collection
echo ""
echo "Step 1: Verifying metrics collection..."
echo "========================================"
node "$SCRIPT_DIR/verify-collected-metrics.js"

# Ask user if they want to continue
echo ""
echo -n "Do you want to proceed with dashboard creation? (y/n): "
read -r response

if [[ ! "$response" =~ ^[Yy]$ ]]; then
    echo "Dashboard creation cancelled."
    exit 0
fi

# Step 2: Create the dashboard
echo ""
echo "Step 2: Creating dashboard..."
echo "========================================"
node "$SCRIPT_DIR/create-database-dashboard.js"

echo ""
echo "========================================"
echo "Dashboard deployment complete!"
echo "========================================"