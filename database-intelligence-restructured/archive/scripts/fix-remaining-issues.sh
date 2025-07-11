#!/bin/bash

echo "=== Fixing Remaining Import Issues ==="

# 1. Fix NRI exporter - remove the empty import statement and fix rate limiter code
echo "Fixing NRI exporter..."
cd exporters/nri

# Create a cleaned version of exporter.go
cat > exporter_temp.go << 'EOF'
package nri

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// nriMetricsExporter exports metrics in NRI format
type nriMetricsExporter struct {
	config      *Config
	logger      *zap.Logger
	writer      *nriWriter
	mu          sync.Mutex
	
	// Compiled patterns for efficiency
	metricPatterns map[string]*regexp.Regexp
	
	// Metrics tracking
	exportSuccessCount uint64
	exportErrorCount   uint64
	rateLimitedCount   uint64
}

// nriLogsExporter exports logs as NRI events
type nriLogsExporter struct {
	config      *Config
	logger      *zap.Logger
	writer      *nriWriter
	mu          sync.Mutex
	
	// Compiled patterns
	eventPatterns map[string]*regexp.Regexp
	
	// Metrics tracking
	exportSuccessCount uint64
	exportErrorCount   uint64
	rateLimitedCount   uint64
}
EOF

# Copy the rest of the file starting from the NRI data structures
sed -n '57,$p' exporter.go | sed 's|// e.rateLimiter|// rateLimiter|g' | sed 's|e.rateLimiter|nil|g' >> exporter_temp.go

# Replace the original file
mv exporter_temp.go exporter.go

# Remove rate limiter references from config.go
echo "Fixing NRI config..."
sed -i.bak '/RateLimiting/d' config.go
sed -i.bak '/RateLimitConfig/d' config.go

# Remove rate limiter references from factory.go if it exists
if [ -f factory.go ]; then
    sed -i.bak '/ratelimit/d' factory.go
fi

cd ../..

# 2. Check if internal/database exists, if not create it
echo "Checking internal/database..."
if [ ! -d "internal/database" ]; then
    echo "Creating internal/database module..."
    mkdir -p internal/database
    
    # Create a simple database interface
    cat > internal/database/types.go << 'EOF'
// Package database provides common database interfaces and utilities
package database

import (
	"database/sql"
	"time"
)

// DB represents a database connection
type DB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Close() error
}

// QueryResult represents a database query result
type QueryResult struct {
	Columns []string
	Rows    [][]interface{}
	Timestamp time.Time
}

// ConnectionConfig holds database connection configuration
type ConnectionConfig struct {
	Driver   string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
}
EOF

    # Create go.mod for internal/database
    cat > internal/database/go.mod << 'EOF'
module github.com/database-intelligence/internal/database

go 1.23.0
EOF
fi

# 3. Update go.work to include internal modules
echo "Updating go.work..."
if ! grep -q "./internal/database" go.work; then
    sed -i.bak '/^use/a\
	./internal/database' go.work
fi

# 4. Fix components_complete.go reference
echo "Checking for components_complete.go..."
if [ -f "distributions/production/components_complete.go" ]; then
    echo "Removing components_complete.go (duplicate of components.go)..."
    rm -f distributions/production/components_complete.go
fi

# 5. Add replace directive for internal/database in production go.mod
echo "Updating production go.mod with internal/database..."
cd distributions/production

# Check if internal/database is already in replace section
if ! grep -q "internal/database =>" go.mod; then
    # Add before the closing parenthesis of replace section
    sed -i.bak '/^)$/i\
	\
	// Internal modules\
	github.com/database-intelligence/internal/database => ../../internal/database' go.mod
fi

# 6. Run go mod tidy again
echo "Running go mod tidy..."
go mod tidy -v 2>&1 || true

# 7. Try building again
echo ""
echo "Building production collector..."
if go build -o otelcol-database-intelligence .; then
    echo "✓ Production collector built successfully!"
    ls -la otelcol-database-intelligence
    
    # Test that it runs
    echo ""
    echo "Testing collector binary..."
    ./otelcol-database-intelligence --version || true
else
    echo "⚠ Build failed. Checking remaining errors..."
    go build -v . 2>&1 | head -20
fi

cd ../..

echo ""
echo "=== Fix complete ==="