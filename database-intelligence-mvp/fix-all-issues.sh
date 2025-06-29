#!/bin/bash

set -e

echo "=== Comprehensive Fix Script for Database Intelligence Collector ==="
echo ""

# 1. Fix Taskfile.yml emoji issues
echo "1. Fixing Taskfile.yml emoji issues..."
if [ -f "Taskfile.yml" ]; then
    # Remove all emojis from Taskfile.yml
    sed -i '' 's/✅/[OK]/g' Taskfile.yml
    sed -i '' 's/🔍/[CHECK]/g' Taskfile.yml
    sed -i '' 's/❌/[ERROR]/g' Taskfile.yml
    sed -i '' 's/🚀/[START]/g' Taskfile.yml
    sed -i '' 's/⏳/[WAIT]/g' Taskfile.yml
    sed -i '' 's/🐳/[DOCKER]/g' Taskfile.yml
    sed -i '' 's/🛠/[BUILD]/g' Taskfile.yml
    sed -i '' 's/📦/[PACKAGE]/g' Taskfile.yml
    sed -i '' 's/🔧/[FIX]/g' Taskfile.yml
    sed -i '' 's/🎯/[TARGET]/g' Taskfile.yml
    sed -i '' 's/⚠️/[WARNING]/g' Taskfile.yml
    sed -i '' 's/🧪/[TEST]/g' Taskfile.yml
    echo "   ✓ Removed emojis from Taskfile.yml"
fi

# Also fix task files
for taskfile in tasks/*.yml; do
    if [ -f "$taskfile" ]; then
        sed -i '' 's/✅/[OK]/g' "$taskfile"
        sed -i '' 's/🔍/[CHECK]/g' "$taskfile"
        sed -i '' 's/❌/[ERROR]/g' "$taskfile"
        sed -i '' 's/🚀/[START]/g' "$taskfile"
        sed -i '' 's/⏳/[WAIT]/g' "$taskfile"
        sed -i '' 's/🐳/[DOCKER]/g' "$taskfile"
        sed -i '' 's/🛠/[BUILD]/g' "$taskfile"
        sed -i '' 's/📦/[PACKAGE]/g' "$taskfile"
        sed -i '' 's/🔧/[FIX]/g' "$taskfile"
        sed -i '' 's/🎯/[TARGET]/g' "$taskfile"
        sed -i '' 's/⚠️/[WARNING]/g' "$taskfile"
        sed -i '' 's/🧪/[TEST]/g' "$taskfile"
    fi
done
echo "   ✓ Fixed emoji issues in all task files"

# 2. Fix module path inconsistencies
echo ""
echo "2. Fixing module path inconsistencies..."
# Fix in YAML files
find . -name "*.yaml" -type f -exec sed -i '' \
  's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' {} \;

# Fix in Go files
find . -name "*.go" -type f -exec sed -i '' \
  's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' {} \;

echo "   ✓ Fixed module paths"

# 3. Fix Helm chart values issues
echo ""
echo "3. Adding missing Helm values..."
if ! grep -q "^ingress:" deployments/helm/db-intelligence/values.yaml; then
    cat >> deployments/helm/db-intelligence/values.yaml << 'EOF'

# Ingress configuration
ingress:
  enabled: false
  className: nginx
  annotations: {}
  hosts:
    - host: db-intelligence.local
      paths:
        - path: /
          pathType: Prefix
  tls: []
EOF
    echo "   ✓ Added ingress configuration to values.yaml"
fi

# 4. Fix Docker Compose profile dependencies
echo ""
echo "4. Fixing Docker Compose profile dependencies..."
# Update collector service to not depend on databases when using collector profile alone
sed -i '' '/^  collector:/,/^  [^ ]/ {
    /depends_on:/,/^    [^ ]/ {
        /postgres:/d
        /mysql:/d
    }
}' docker-compose.yaml || true
echo "   ✓ Fixed Docker Compose dependencies"

# 5. Fix go.mod replace directives format
echo ""
echo "5. Fixing go.mod replace directives..."
# Ensure replace block exists properly
if grep -q "replace (" go.mod; then
    # Update existing replace block
    sed -i '' '/^replace (/,/^)/ {
        s|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g
    }' go.mod
else
    # Add replace block if missing
    echo "" >> go.mod
    echo "replace (" >> go.mod
    echo "	github.com/database-intelligence-mvp/processors/adaptivesampler => ./processors/adaptivesampler" >> go.mod
    echo "	github.com/database-intelligence-mvp/processors/circuitbreaker => ./processors/circuitbreaker" >> go.mod
    echo "	github.com/database-intelligence-mvp/processors/planattributeextractor => ./processors/planattributeextractor" >> go.mod
    echo "	github.com/database-intelligence-mvp/processors/verification => ./processors/verification" >> go.mod
    echo "	github.com/database-intelligence-mvp/exporters/otlpexporter => ./exporters/otlpexporter" >> go.mod
    echo ")" >> go.mod
fi
echo "   ✓ Fixed go.mod replace directives"

# 6. Fix otelcol-builder.yaml
echo ""
echo "6. Fixing otelcol-builder.yaml..."
# Remove the replaces section since it's handled in go.mod
sed -i '' '/^replaces:/,/^[^[:space:]]/ {
    /^replaces:/d
    /^[[:space:]]/d
}' otelcol-builder.yaml
echo "   ✓ Removed duplicate replaces from otelcol-builder.yaml"

# 7. Fix ocb-config.yaml module references
echo ""
echo "7. Fixing ocb-config.yaml..."
if [ -f "ocb-config.yaml" ]; then
    sed -i '' 's|github.com/database-intelligence/|github.com/database-intelligence-mvp/|g' ocb-config.yaml
    echo "   ✓ Fixed ocb-config.yaml module references"
fi

# 8. Create .env file if missing
echo ""
echo "8. Setting up environment files..."
if [ ! -f ".env" ]; then
    cp .env.development .env
    echo "   ✓ Created .env file from .env.development"
else
    echo "   ✓ .env file already exists"
fi

# 9. Fix permissions
echo ""
echo "9. Fixing file permissions..."
chmod +x *.sh 2>/dev/null || true
chmod +x scripts/*.sh 2>/dev/null || true
echo "   ✓ Fixed executable permissions"

# 10. Validate fixes
echo ""
echo "10. Validating fixes..."
echo ""

# Check for remaining module path issues
REMAINING_ISSUES=$(grep -r "github.com/newrelic/database-intelligence-mvp" . \
    --include="*.yaml" --include="*.go" --include="*.mod" \
    --exclude-dir=".git" --exclude-dir="target" 2>/dev/null | grep -v "postgres-collector" || true)

if [ -z "$REMAINING_ISSUES" ]; then
    echo "   ✓ No module path inconsistencies found"
else
    echo "   ⚠ Some module path issues remain:"
    echo "$REMAINING_ISSUES" | head -5
fi

# Check Taskfile
if command -v task &> /dev/null; then
    if task --list-all &>/dev/null; then
        echo "   ✓ Taskfile is valid"
    else
        echo "   ⚠ Taskfile has syntax errors"
    fi
else
    echo "   ⚠ Task not installed - cannot validate Taskfile"
fi

# Check Docker Compose
if docker-compose config &>/dev/null; then
    echo "   ✓ Docker Compose file is valid"
else
    echo "   ⚠ Docker Compose has issues"
fi

# Check Helm chart
if command -v helm &> /dev/null; then
    if helm lint deployments/helm/db-intelligence/ --quiet 2>&1 | grep -q "1 chart(s) linted, 0 chart(s) failed"; then
        echo "   ✓ Helm chart is valid"
    else
        echo "   ⚠ Helm chart has issues (run 'helm lint deployments/helm/db-intelligence/' for details)"
    fi
else
    echo "   ⚠ Helm not installed - cannot validate chart"
fi

echo ""
echo "=== Fix Script Complete ==="
echo ""
echo "Next steps:"
echo "1. Install Task if not already installed:"
echo "   brew install go-task/tap/go-task"
echo ""
echo "2. Install OpenTelemetry Collector Builder:"
echo "   go install go.opentelemetry.io/collector/cmd/builder@latest"
echo ""
echo "3. Run the quickstart:"
echo "   task quickstart"
echo ""
echo "Or manually:"
echo "   task setup"
echo "   task build"
echo "   task dev:up"