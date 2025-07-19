# Business Impact Module

The Business Impact module analyzes database operations to identify and score their business impact based on query patterns, affected tables, and operational characteristics.

## Overview

This module receives metrics from other modules via OTLP and applies sophisticated transformations to:
- Score queries based on business impact (0-10 scale)
- Categorize operations by business function
- Identify revenue-impacting queries
- Flag business-critical tables
- Assess SLA impact based on query duration

## Features

### Business Impact Scoring
- **Revenue Operations (Score: 10)**: Orders, payments, transactions, billing
- **Customer Operations (Score: 8)**: User accounts, authentication, sessions
- **Operations (Score: 7)**: Inventory, fulfillment, shipping
- **Analytics (Score: 5)**: Reports, dashboards, aggregations
- **Administrative (Score: 3)**: Configuration, logging, maintenance

### Business Categories
- `revenue`: Direct revenue-impacting operations
- `customer`: Customer-facing operations
- `operations`: Inventory and fulfillment
- `analytics`: Reporting and analytics
- `admin`: Administrative tasks
- `general`: Other operations

### Additional Attributes
- **revenue_impact**: `direct`, `indirect`, or `none`
- **sla_impact**: `critical` (>5s), `warning` (2-5s), or `normal` (<2s)
- **critical_table**: Boolean flag for business-critical tables

## Configuration

The module uses OpenTelemetry Collector with custom transform processors defined in `config/collector.yaml`.

## Ports

- **8085**: Module service port
- **4317**: OTLP gRPC receiver
- **4318**: OTLP HTTP receiver
- **8888**: Prometheus metrics endpoint
- **13133**: Health check endpoint

## Usage

### Starting the Module
```bash
make up
```

### Viewing Logs
```bash
make logs
```

### Checking Health
```bash
make health
```

### Testing
```bash
make test
```

## Integration

To send metrics to this module, configure your OTLP exporter to:
```yaml
exporters:
  otlp:
    endpoint: business-impact:4317
    tls:
      insecure: true
```

## Metrics

The module exposes business impact metrics via Prometheus:
- `business_impact_score`: Business impact score (0-10)
- Attributes include: `business_category`, `revenue_impact`, `sla_impact`, `critical_table`

## Example Query Patterns

### High Impact (Score 10)
- `INSERT INTO orders ...`
- `UPDATE payments SET status = 'completed' ...`
- `SELECT * FROM transactions WHERE ...`

### Medium Impact (Score 7-8)
- `SELECT * FROM users WHERE id = ?`
- `UPDATE inventory SET quantity = ...`
- `INSERT INTO customer_sessions ...`

### Low Impact (Score 3-5)
- `SELECT * FROM reports ...`
- `INSERT INTO audit_logs ...`
- `SELECT COUNT(*) FROM analytics_events ...`