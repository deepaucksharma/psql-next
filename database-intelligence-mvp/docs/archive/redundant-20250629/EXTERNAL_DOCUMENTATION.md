# External Documentation Relevant to Database Intelligence MVP

This document catalogs all markdown files outside the MVP directory that are relevant to the Database Intelligence project.

## Critical Documentation (High Relevance)

### Metric Mapping and Implementation


### Strategy and Planning
- **[strategy/00-executive-playbook.md](../../strategy/00-executive-playbook.md)**
  - Executive framework for PostgreSQL monitoring migration
  - Business drivers and five-phase migration approach
  - Cost optimization targets (30-40% reduction)

- **[strategy/06-technical-reference.md](../../strategy/06-technical-reference.md)**
  - Complete PostgreSQL monitoring technical guide
  - Database user setup, permissions, and security
  - Collector configurations and metric definitions

- **[strategy/07-validation-framework.md](../../strategy/07-validation-framework.md)**
  - Automated metric comparison Python tools
  - PostgreSQL-specific validation tests
  - Alert parity and performance validation

### Migration Frameworks
- **[strategy/ohi-otel-enhanced-framework.md](../../strategy/ohi-otel-enhanced-framework.md)**
  - Analysis of paradigm shift from samples to metrics
  - Semantic equivalence tools and mapping strategies
  - Organizational change management

- **[strategy/ohi-to-otel-migration.md](../../strategy/ohi-to-otel-migration.md)**
  - Database-specific receiver mapping matrix
  - MySQL, PostgreSQL, Redis migration examples
  - Risk assessment by database type

## Supporting Documentation (Medium Relevance)

### OpenTelemetry Integration
- **[otel-newrelic-detailed-guide.md](../../otel-newrelic-detailed-guide.md)**
  - New Relic OTLP endpoint configuration
  - Cardinality management strategies
  - Performance optimization tips

- **[otel-collector-ddd-enhanced.md](../../otel-collector-ddd-enhanced.md)**
  - Collector architecture patterns
  - Pipeline processing context
  - General collector design principles

### Additional Strategy Documents
- **[strategy/02-design-mobilization.md](../../strategy/02-design-mobilization.md)**
  - Design phase activities
  - Team mobilization strategies

- **[strategy/03-parallel-validation.md](../../strategy/03-parallel-validation.md)**
  - Parallel running strategies
  - Validation approaches

- **[strategy/04-cutover-decommission.md](../../strategy/04-cutover-decommission.md)**
  - Cutover planning
  - Legacy system decommission

- **[strategy/05-operationalize-optimize.md](../../strategy/05-operationalize-optimize.md)**
  - Operational excellence
  - Continuous optimization

## Low Relevance Documentation

### General OpenTelemetry Concepts
- **[otel-ddd-enhanced.md](../../otel-ddd-enhanced.md)**
  - General SDK and API patterns
  - Not database-specific

### Governance and Templates
- **[strategy/08-rollback-procedures.md](../../strategy/08-rollback-procedures.md)**
- **[strategy/09-governance-templates.md](../../strategy/09-governance-templates.md)**

## Recommended Reading Order

1. Start with `strategy/00-executive-playbook.md` for strategic overview
2. Review `ohi-metric-mapping-implementation.md` for technical context
3. Study `strategy/06-technical-reference.md` for implementation details
4. Use `strategy/07-validation-framework.md` for testing approach
5. Reference `enhanced-metric-mapping.md` for metric structures

## Integration with MVP

To properly integrate these documents with the MVP:

1. Run the organization script:
   ```bash
   ./scripts/organize-documentation.sh
   ```

2. This will copy relevant files to:
   - `docs/reference/metric-mapping/`
   - `docs/reference/strategy/`
   - `docs/reference/migration/`

3. Update internal links to point to the new locations

## Notes

- These documents provide essential context for the MVP implementation
- The strategy documents contain production-tested approaches
- The metric mapping files ensure backward compatibility
- Consider creating condensed versions focusing on MVP-specific needs