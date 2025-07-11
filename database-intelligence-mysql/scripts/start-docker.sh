#!/bin/bash

echo "🐳 Starting Docker Daemon"
echo "========================"

# Try different methods to start Docker depending on the system
if command -v systemctl &> /dev/null; then
    echo "Attempting to start Docker with systemctl..."
    if sudo systemctl start docker 2>/dev/null; then
        echo "✅ Docker started with systemctl"
        sudo systemctl enable docker
    else
        echo "❌ Failed to start with systemctl"
    fi
elif command -v service &> /dev/null; then
    echo "Attempting to start Docker with service..."
    if sudo service docker start 2>/dev/null; then
        echo "✅ Docker started with service command"
    else
        echo "❌ Failed to start with service command"
    fi
else
    echo "❓ Could not find systemctl or service command"
fi

# Check if Docker Desktop is available (macOS/Windows)
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "Checking for Docker Desktop on macOS..."
    if [ -d "/Applications/Docker.app" ]; then
        echo "Starting Docker Desktop..."
        open -a Docker
        echo "⏳ Waiting for Docker Desktop to start..."
        sleep 10
    fi
fi

# Wait a moment and test
sleep 3
if docker info &> /dev/null; then
    echo "✅ Docker is now running!"
    docker info | head -10
else
    echo "❌ Docker is still not accessible"
    echo ""
    echo "Manual steps to try:"
    echo "1. Linux: sudo systemctl start docker"
    echo "2. macOS: Start Docker Desktop application"  
    echo "3. Windows: Start Docker Desktop application"
    echo "4. Check Docker documentation for your OS"
    echo ""
    echo "Then run: ./scripts/diagnose.sh to verify"
fi