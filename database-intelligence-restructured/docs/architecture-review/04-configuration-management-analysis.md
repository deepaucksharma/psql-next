# Configuration Management Analysis

## Critical Configuration Problems

### 1. Configuration File Chaos
```
configs/
├── 25+ example configurations (why?)
├── collector.yaml
├── collector-simplified.yaml  
├── collector-secure.yaml
├── collector-e2e-test.yaml
├── collector-end-to-end-test.yaml (duplicate?)
└── ... 20 more similar files
```
**Impact**: Nobody knows which config to use, deployment errors

### 2. No Validation
```yaml
# This will crash at runtime
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}  # What if not set?
    username: ${POSTGRES_USER}                   # No validation
    password: ${POSTGRES_PASSWORD}               # Could be empty
    collection_interval: "not-a-duration"        # Will crash
```
**Impact**: Runtime failures, silent errors, data loss

### 3. Environment Variable Chaos
```yaml
# No documentation of required variables
# No defaults
# No validation
password: ${POSTGRES_PASSWORD}  # Fails silently if not set
host: ${POSTGRES_HOST:-}       # Empty default = crash
```

### 4. Duplicate Configuration Logic
- Base configs incomplete
- Overlays don't properly extend
- Manual copying between environments
- No clear inheritance

## Required Fixes

### Fix 1: Single Configuration Structure
```
configs/
├── base.yaml           # Core configuration
├── overlays/
│   ├── dev.yaml       # Dev-specific changes only
│   ├── staging.yaml   # Staging-specific changes only
│   └── prod.yaml      # Prod-specific changes only
└── README.md          # Documentation
```

### Fix 2: Add Schema Validation
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "receivers": {
      "type": "object",
      "properties": {
        "postgresql": {
          "required": ["endpoint", "username", "password"],
          "properties": {
            "endpoint": {
              "type": "string",
              "pattern": "^[^:]+:[0-9]+$"
            },
            "collection_interval": {
              "type": "string",
              "pattern": "^[0-9]+(s|m|h)$"
            }
          }
        }
      }
    }
  }
}
```

### Fix 3: Environment Documentation
```bash
# .env.template
# REQUIRED
POSTGRES_HOST=      # PostgreSQL host (e.g., localhost)
POSTGRES_PORT=      # PostgreSQL port (e.g., 5432)
POSTGRES_USER=      # PostgreSQL username
POSTGRES_PASSWORD=  # PostgreSQL password

# OPTIONAL
COLLECTOR_LOG_LEVEL=info  # Log level (debug, info, warn, error)
```

### Fix 4: Configuration Validator
```go
type ConfigValidator struct {
    schema *jsonschema.Schema
}

func (v *ConfigValidator) Validate(config []byte) error {
    var cfg map[string]interface{}
    if err := yaml.Unmarshal(config, &cfg); err != nil {
        return fmt.Errorf("invalid YAML: %w", err)
    }
    
    if err := v.schema.Validate(cfg); err != nil {
        return fmt.Errorf("schema validation failed: %w", err)
    }
    
    return nil
}
```

## Migration Steps

### Step 1: Consolidate Configs
```bash
# Keep only these files
configs/base.yaml
configs/schema.json
configs/.env.template
configs/overlays/{dev,staging,prod}.yaml
```

### Step 2: Add Validation
```go
// Add to main.go
if err := validateConfig(configPath); err != nil {
    log.Fatal("Config validation failed:", err)
}
```

### Step 3: Document Variables
- List all required environment variables
- Provide example values
- Document validation rules

## Success Metrics
- Single source of truth for config
- All configs validate before deployment
- Clear environment variable documentation
- No runtime config errors
- Reduced from 25+ configs to <5