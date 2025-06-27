# ADR-0002: Query Sanitization Strategy

## Status
Accepted

## Context
PostgreSQL query text may contain sensitive information including:
- Personal Identifiable Information (PII)
- Authentication credentials
- Business-sensitive data values
- Internal system details

The collector must balance:
1. **Security**: Protect sensitive data from exposure
2. **Utility**: Maintain query structure for analysis
3. **Performance**: Minimize sanitization overhead
4. **Compliance**: Meet data protection regulations (GDPR, CCPA, etc.)

## Decision
Implement a multi-mode query sanitization system:

### Sanitization Modes
1. **None**: No sanitization (internal environments only)
2. **Smart** (Default): Intelligent parameter replacement
3. **Full**: Aggressive sanitization with minimal data retention

### Implementation Strategy
```rust
pub enum SanitizationMode {
    None,
    Smart,
    Full,
}

pub struct QuerySanitizer {
    mode: SanitizationMode,
    pii_patterns: Vec<Regex>,
    preserve_patterns: Vec<Regex>,
}
```

## Rationale

### 1. Smart Sanitization (Default)
**Approach**: Replace literal values while preserving query structure

**Transformations:**
```sql
-- Original
SELECT * FROM users WHERE email = 'john@example.com' AND age = 25;

-- Sanitized (Smart)
SELECT * FROM users WHERE email = $1 AND age = $2;
```

**Benefits:**
- Preserves query structure for performance analysis
- Removes sensitive literal values
- Maintains readability for debugging

### 2. Full Sanitization
**Approach**: Aggressive sanitization for high-security environments

**Transformations:**
```sql
-- Original
SELECT name, email FROM users WHERE id = 123;

-- Sanitized (Full)
SELECT [FIELD], [FIELD] FROM [TABLE] WHERE [FIELD] = [VALUE];
```

**Benefits:**
- Maximum data protection
- Compliant with strictest security policies
- Zero risk of data exposure

### 3. PII Detection
**Pattern-Based Detection**:
```rust
const PII_PATTERNS: &[&str] = &[
    r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b", // Email
    r"\b\d{3}-\d{2}-\d{4}\b",                                 // SSN
    r"\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b",             // Credit Card
    r"\b\d{3}-\d{3}-\d{4}\b",                                 // Phone
];
```

**Warning System**:
- Log detected PII patterns
- Alert on potential data exposure
- Metrics on sanitization effectiveness

## Implementation Details

### Query Text Processing
```rust
impl QuerySanitizer {
    pub fn sanitize(&self, query: &str) -> String {
        match self.mode {
            SanitizationMode::None => query.to_string(),
            SanitizationMode::Smart => self.smart_sanitize(query),
            SanitizationMode::Full => self.full_sanitize(query),
        }
    }
    
    fn smart_sanitize(&self, query: &str) -> String {
        // Replace string literals with parameters
        let mut sanitized = self.replace_string_literals(query);
        
        // Replace numeric literals
        sanitized = self.replace_numeric_literals(&sanitized);
        
        // Handle IN clauses
        sanitized = self.replace_in_clauses(&sanitized);
        
        sanitized
    }
    
    pub fn get_pii_warning(&self, query: &str) -> Option<String> {
        for pattern in &self.pii_patterns {
            if pattern.is_match(query) {
                return Some(format!("Potential PII detected: {}", pattern.as_str()));
            }
        }
        None
    }
}
```

### Performance Optimization
```rust
// Compiled regex cache
lazy_static! {
    static ref STRING_LITERAL_REGEX: Regex = Regex::new(r"'[^']*'").unwrap();
    static ref NUMERIC_LITERAL_REGEX: Regex = Regex::new(r"\b\d+\b").unwrap();
    static ref EMAIL_REGEX: Regex = Regex::new(
        r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b"
    ).unwrap();
}
```

### Configuration
```toml
[collection]
sanitize_query_text = true
sanitization_mode = "smart"  # "none", "smart", "full"

[sanitization]
# Custom PII patterns
custom_patterns = [
    "\\b\\d{4}-\\d{4}-\\d{4}-\\d{4}\\b",  # Custom card format
]

# Preserve specific patterns
preserve_patterns = [
    "pg_stat_statements",  # Function names
    "information_schema",  # Schema names
]
```

## Consequences

### Positive
- **Security**: Protects sensitive data from exposure
- **Compliance**: Meets data protection requirements
- **Flexibility**: Multiple modes for different environments
- **Awareness**: PII detection and alerting

### Negative
- **Performance**: Additional processing overhead
- **Utility Loss**: Some query details are lost
- **False Positives**: May over-sanitize legitimate queries

### Performance Impact
- **Smart Mode**: ~5ms per query (acceptable)
- **Full Mode**: ~10ms per query (acceptable)
- **Memory**: ~2MB for compiled regexes (minimal)

## Security Considerations

### Threat Model
1. **Data Leakage**: Query text exposure in logs/exports
2. **Insider Threats**: Unauthorized access to query data
3. **Compliance Violations**: Accidental PII transmission

### Mitigation Strategies
1. **Default Sanitization**: Smart mode enabled by default
2. **Audit Logging**: Track sanitization decisions
3. **Configurable Strictness**: Environment-appropriate modes
4. **PII Detection**: Proactive identification and alerting

### Validation
```rust
#[cfg(test)]
mod tests {
    #[test]
    fn test_email_sanitization() {
        let sanitizer = QuerySanitizer::new(SanitizationMode::Smart);
        let query = "SELECT * FROM users WHERE email = 'user@example.com'";
        let result = sanitizer.sanitize(query);
        
        assert!(!result.contains("user@example.com"));
        assert!(result.contains("email = $1"));
    }
    
    #[test]
    fn test_pii_detection() {
        let sanitizer = QuerySanitizer::new(SanitizationMode::Smart);
        let query = "SELECT * FROM users WHERE ssn = '123-45-6789'";
        let warning = sanitizer.get_pii_warning(query);
        
        assert!(warning.is_some());
        assert!(warning.unwrap().contains("Potential PII detected"));
    }
}
```

## Alternatives Considered

### 1. No Sanitization
**Rejected**: Unacceptable security risk

### 2. Hash-Based Sanitization
**Rejected**: Irreversible, limited utility for debugging

### 3. Encryption-Based Sanitization
**Rejected**: Complex key management, performance overhead

### 4. External Sanitization Service
**Rejected**: Network dependency, latency concerns

## Monitoring Success

### Metrics
- `sanitization_duration_ms`: Processing time per query
- `pii_detections_total`: Count of PII pattern matches
- `sanitization_errors_total`: Failed sanitization attempts

### Alerts
- High PII detection rates (>10% of queries)
- Sanitization performance degradation (>50ms)
- Sanitization failures

## Related Decisions
- ADR-0001: Unified Collector Architecture
- ADR-0005: Logging and Observability Strategy