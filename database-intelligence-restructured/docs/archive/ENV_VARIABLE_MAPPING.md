# Environment Variable Standardization

## Database Connection Variables

### PostgreSQL
- **Standard**: `DB_POSTGRES_HOST`, `DB_POSTGRES_PORT`, `DB_POSTGRES_USER`, `DB_POSTGRES_PASSWORD`, `DB_POSTGRES_DATABASE`
- **Current Issues**:
  - Config uses: `DB_POSTGRES_HOST`, `DB_USERNAME`, `DB_PASSWORD`, `DB_NAME`
  - Docker-compose uses: `DB_POSTGRES_ENDPOINT`, `DB_POSTGRES_USER`, `DB_POSTGRES_PASSWORD`, `DB_POSTGRES_DATABASE`
  - .env.template uses: `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`

### MySQL
- **Standard**: `DB_MYSQL_HOST`, `DB_MYSQL_PORT`, `DB_MYSQL_USER`, `DB_MYSQL_PASSWORD`, `DB_MYSQL_DATABASE`
- **Current Issues**:
  - Config uses: `DB_MYSQL_HOST`, `DB_USERNAME`, `DB_PASSWORD`, `DB_NAME`
  - Docker-compose uses: `DB_MYSQL_ENDPOINT`, `DB_MYSQL_USER`, `DB_MYSQL_PASSWORD`, `DB_MYSQL_DATABASE`
  - .env.template uses: `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DB`

## New Relic Variables
- **Standard**: `NEW_RELIC_LICENSE_KEY` (for OTLP ingest)
- **Current Issues**:
  - Config uses: `NEW_RELIC_API_KEY`
  - .env.template has: `NEW_RELIC_LICENSE_KEY`, `NEW_RELIC_API_KEY`, `NEW_RELIC_INGEST_KEY`

## Service Identification
- **Standard**: `SERVICE_NAME`, `SERVICE_VERSION`, `DEPLOYMENT_ENVIRONMENT`
- **Status**: Consistent across files

## Memory and Performance
- **Standard**: `MEMORY_LIMIT_MIB`, `MEMORY_SPIKE_LIMIT_MIB`
- **Status**: Consistent

## OTLP Configuration
- **Standard**: `OTLP_ENDPOINT`, `OTLP_TIMEOUT`
- **Current Issues**:
  - Config uses: `NEW_RELIC_OTLP_ENDPOINT`
  - .env.template uses: `OTLP_ENDPOINT`