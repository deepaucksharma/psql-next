#!/bin/bash
# Script to organize relevant documentation into the MVP project

set -e

echo "Organizing relevant documentation for Database Intelligence MVP..."

# Create reference directories
mkdir -p database-intelligence-mvp/docs/reference/{metric-mapping,strategy,migration}

# Copy metric mapping files
echo "Copying metric mapping documentation..."
if [ -f enhanced-metric-mapping.md ]; then
    cp enhanced-metric-mapping.md database-intelligence-mvp/docs/reference/metric-mapping/
fi
if [ -f ohi-metric-mapping-implementation.md ]; then
    cp ohi-metric-mapping-implementation.md database-intelligence-mvp/docs/reference/metric-mapping/
fi


# Copy strategy files
echo "Copying strategy documentation..."
cp strategy/00-executive-playbook.md database-intelligence-mvp/docs/reference/strategy/
cp strategy/01-foundation-strategy.md database-intelligence-mvp/docs/reference/strategy/
cp strategy/06-technical-reference.md database-intelligence-mvp/docs/reference/strategy/
cp strategy/07-validation-framework.md database-intelligence-mvp/docs/reference/strategy/

# Copy migration files
echo "Copying migration documentation..."
cp strategy/ohi-otel-enhanced-framework.md database-intelligence-mvp/docs/reference/migration/
cp strategy/ohi-to-otel-migration.md database-intelligence-mvp/docs/reference/migration/

# Create index file
cat > database-intelligence-mvp/docs/reference/README.md << 'EOF'
# Database Intelligence MVP - Reference Documentation

This directory contains essential reference documentation for the Database Intelligence MVP project.

## Directory Structure

### metric-mapping/
- **enhanced-metric-mapping.md** - Enhanced metric structures with OHI compatibility
- **ohi-metric-mapping-implementation.md** - OHI SQL queries and transformation logic

### strategy/
- **00-executive-playbook.md** - Executive-level migration strategy
- **01-foundation-strategy.md** - Foundation phase with metric analysis
- **06-technical-reference.md** - Complete PostgreSQL monitoring technical guide
- **07-validation-framework.md** - Metric validation and comparison tools

### migration/
- **ohi-otel-enhanced-framework.md** - Paradigm shift from samples to metrics
- **ohi-to-otel-migration.md** - Database-specific migration mappings

## Quick Links

- [PostgreSQL Monitoring Setup](strategy/06-technical-reference.md#postgresql-receiver-setup)
- [Metric Validation Tools](strategy/07-validation-framework.md#automated-validation)
- [OHI Compatibility Mapping](metric-mapping/ohi-metric-mapping-implementation.md)
- [Migration Strategy](strategy/00-executive-playbook.md)

## Related Documentation

- [MVP Architecture](../../ARCHITECTURE.md)
- [Configuration Guide](../../CONFIGURATION.md)
- [Deployment Guide](../../DEPLOYMENT.md)
EOF

echo "Documentation organization complete!"
echo "Files have been copied to: database-intelligence-mvp/docs/reference/"