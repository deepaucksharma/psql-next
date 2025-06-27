use regex::Regex;
use lazy_static::lazy_static;
use std::collections::HashSet;

/// Query text sanitizer for PII protection
pub struct QuerySanitizer {
    enabled: bool,
    mode: SanitizationMode,
    custom_patterns: Vec<Regex>,
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum SanitizationMode {
    /// Replace all literals with placeholders
    Full,
    /// Only sanitize potentially sensitive patterns
    Smart,
    /// No sanitization
    None,
}

lazy_static! {
    // Common patterns that might contain PII
    static ref EMAIL_PATTERN: Regex = Regex::new(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b").unwrap();
    static ref PHONE_PATTERN: Regex = Regex::new(r"\b(?:\+?1[-.\s]?)?\(?([0-9]{3})\)?[-.\s]?([0-9]{3})[-.\s]?([0-9]{4})\b").unwrap();
    static ref SSN_PATTERN: Regex = Regex::new(r"\b\d{3}-\d{2}-\d{4}\b").unwrap();
    static ref CREDIT_CARD_PATTERN: Regex = Regex::new(r"\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b").unwrap();
    static ref IP_PATTERN: Regex = Regex::new(r"\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b").unwrap();
    
    // SQL literals
    static ref STRING_LITERAL: Regex = Regex::new(r"'([^']*(?:''[^']*)*)'").unwrap();
    static ref NUMBER_LITERAL: Regex = Regex::new(r"\b\d+\.?\d*\b").unwrap();
    static ref HEX_LITERAL: Regex = Regex::new(r"0x[0-9a-fA-F]+").unwrap();
    static ref QUOTED_IDENTIFIER: Regex = Regex::new(r#""([^"]+)""#).unwrap();
    
    // Common PII column names
    static ref PII_COLUMNS: HashSet<&'static str> = {
        let mut set = HashSet::new();
        set.insert("email");
        set.insert("e_mail");
        set.insert("emailaddress");
        set.insert("email_address");
        set.insert("phone");
        set.insert("phonenumber");
        set.insert("phone_number");
        set.insert("mobile");
        set.insert("cell");
        set.insert("ssn");
        set.insert("social_security");
        set.insert("socialsecurity");
        set.insert("password");
        set.insert("passwd");
        set.insert("pwd");
        set.insert("creditcard");
        set.insert("credit_card");
        set.insert("cc_number");
        set.insert("cardnumber");
        set.insert("card_number");
        set.insert("cvv");
        set.insert("address");
        set.insert("street");
        set.insert("city");
        set.insert("state");
        set.insert("zip");
        set.insert("zipcode");
        set.insert("postal");
        set.insert("firstname");
        set.insert("first_name");
        set.insert("lastname");
        set.insert("last_name");
        set.insert("fullname");
        set.insert("full_name");
        set.insert("dob");
        set.insert("dateofbirth");
        set.insert("date_of_birth");
        set.insert("birthdate");
        set.insert("birth_date");
        set
    };
}

impl QuerySanitizer {
    pub fn new(mode: SanitizationMode) -> Self {
        Self {
            enabled: mode != SanitizationMode::None,
            mode,
            custom_patterns: Vec::new(),
        }
    }
    
    pub fn with_custom_patterns(mut self, patterns: Vec<String>) -> Result<Self, regex::Error> {
        for pattern in patterns {
            self.custom_patterns.push(Regex::new(&pattern)?);
        }
        Ok(self)
    }
    
    pub fn sanitize(&self, query: &str) -> String {
        if !self.enabled {
            return query.to_string();
        }
        
        match self.mode {
            SanitizationMode::None => query.to_string(),
            SanitizationMode::Full => self.full_sanitize(query),
            SanitizationMode::Smart => self.smart_sanitize(query),
        }
    }
    
    /// Full sanitization - replace all literals
    fn full_sanitize(&self, query: &str) -> String {
        let mut result = query.to_string();
        
        // Replace string literals
        result = STRING_LITERAL.replace_all(&result, "'?'").to_string();
        
        // Replace hex literals first (before number literals)
        result = HEX_LITERAL.replace_all(&result, "?").to_string();
        
        // Replace number literals
        result = NUMBER_LITERAL.replace_all(&result, "?").to_string();
        
        // Apply custom patterns
        for pattern in &self.custom_patterns {
            result = pattern.replace_all(&result, "?").to_string();
        }
        
        result
    }
    
    /// Smart sanitization - only replace potentially sensitive data
    fn smart_sanitize(&self, query: &str) -> String {
        let mut result = query.to_string();
        let query_lower = query.to_lowercase();
        
        // Check if query references any PII columns
        let has_pii_columns = PII_COLUMNS.iter().any(|col| {
            query_lower.contains(&format!(".{}", col)) ||
            query_lower.contains(&format!(" {} ", col)) ||
            query_lower.contains(&format!("\"{}\"", col)) ||
            query_lower.contains(&format!("`{}`", col))
        });
        
        // Always sanitize obvious PII patterns
        result = EMAIL_PATTERN.replace_all(&result, "?@?.?").to_string();
        result = PHONE_PATTERN.replace_all(&result, "?-?-?").to_string();
        result = SSN_PATTERN.replace_all(&result, "?-?-?").to_string();
        result = CREDIT_CARD_PATTERN.replace_all(&result, "?-?-?-?").to_string();
        
        // If query touches PII columns, be more aggressive
        if has_pii_columns {
            result = self.sanitize_near_pii_columns(&result);
        }
        
        // Sanitize literals in WHERE/SET clauses that might contain PII
        result = self.sanitize_sensitive_clauses(&result);
        
        // Apply custom patterns
        for pattern in &self.custom_patterns {
            result = pattern.replace_all(&result, "?").to_string();
        }
        
        result
    }
    
    /// Sanitize values near PII column references
    fn sanitize_near_pii_columns(&self, query: &str) -> String {
        let mut result = query.to_string();
        
        // For each PII column, find and sanitize values assigned to it
        for col in PII_COLUMNS.iter() {
            // Pattern: column = 'value' or column = value
            let pattern = format!(r"(?i)\b{}\s*=\s*('[^']*'|\S+)", regex::escape(col));
            if let Ok(re) = Regex::new(&pattern) {
                result = re.replace_all(&result, format!("{} = ?", col)).to_string();
            }
            
            // Pattern: VALUES (...) for INSERT
            // This is more complex and would need context-aware parsing
        }
        
        result
    }
    
    /// Sanitize sensitive clauses (WHERE, SET, VALUES)
    fn sanitize_sensitive_clauses(&self, query: &str) -> String {
        let mut result = query.to_string();
        
        // Find WHERE clause and sanitize string literals in it
        if let Some(where_pos) = query.to_lowercase().find(" where ") {
            let before_where = &query[..where_pos + 7];
            let after_where = &query[where_pos + 7..];
            
            // Find the end of WHERE clause (next major keyword or end)
            let end_keywords = vec![" group ", " having ", " order ", " limit ", " union ", ";"];
            let mut end_pos = after_where.len();
            
            for keyword in end_keywords {
                if let Some(pos) = after_where.to_lowercase().find(keyword) {
                    if pos < end_pos {
                        end_pos = pos;
                    }
                }
            }
            
            let where_clause = &after_where[..end_pos];
            let after_clause = &after_where[end_pos..];
            
            // Sanitize the WHERE clause more aggressively
            let sanitized_where = STRING_LITERAL.replace_all(where_clause, "'?'");
            
            result = format!("{}{}{}", before_where, sanitized_where, after_clause);
        }
        
        result
    }
    
    /// Check if a query contains potential PII
    pub fn contains_pii(&self, query: &str) -> bool {
        // Check for PII patterns
        if EMAIL_PATTERN.is_match(query) ||
           PHONE_PATTERN.is_match(query) ||
           SSN_PATTERN.is_match(query) ||
           CREDIT_CARD_PATTERN.is_match(query) {
            return true;
        }
        
        // Check for PII column names
        let query_lower = query.to_lowercase();
        PII_COLUMNS.iter().any(|col| query_lower.contains(col))
    }
    
    /// Generate a warning if PII is detected
    pub fn get_pii_warning(&self, query: &str) -> Option<String> {
        if self.contains_pii(query) {
            Some("Query may contain PII data. Consider reviewing your data handling practices.".to_string())
        } else {
            None
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_full_sanitization() {
        let sanitizer = QuerySanitizer::new(SanitizationMode::Full);
        
        let query = "SELECT * FROM users WHERE email = 'test@example.com' AND id = 123";
        let sanitized = sanitizer.sanitize(query);
        assert_eq!(sanitized, "SELECT * FROM users WHERE email = '?' AND id = ?");
        
        let query2 = "INSERT INTO logs (message, ip) VALUES ('User logged in', '192.168.1.1')";
        let sanitized2 = sanitizer.sanitize(query2);
        assert_eq!(sanitized2, "INSERT INTO logs (message, ip) VALUES ('?', '?')");
    }
    
    #[test]
    fn test_smart_sanitization() {
        let sanitizer = QuerySanitizer::new(SanitizationMode::Smart);
        
        // Should sanitize email
        let query = "SELECT * FROM users WHERE email = 'john@example.com'";
        let sanitized = sanitizer.sanitize(query);
        assert!(sanitized.contains("?@?.?"));
        
        // Should sanitize phone number
        let query2 = "UPDATE contacts SET phone = '555-123-4567' WHERE id = 1";
        let sanitized2 = sanitizer.sanitize(query2);
        assert!(sanitized2.contains("?-?-?"));
        
        // Should not sanitize non-PII
        let query3 = "SELECT * FROM products WHERE price > 100";
        let sanitized3 = sanitizer.sanitize(query3);
        assert_eq!(sanitized3, query3);
    }
    
    #[test]
    fn test_pii_detection() {
        let sanitizer = QuerySanitizer::new(SanitizationMode::Smart);
        
        assert!(sanitizer.contains_pii("SELECT email FROM users"));
        assert!(sanitizer.contains_pii("WHERE phone = '555-1234'"));
        assert!(sanitizer.contains_pii("INSERT INTO table VALUES ('123-45-6789')"));
        assert!(!sanitizer.contains_pii("SELECT id, name FROM products"));
    }
}