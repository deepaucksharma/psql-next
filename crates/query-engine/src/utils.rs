use regex::Regex;
use lazy_static::lazy_static;

/// Match OHI anonymization exactly
pub fn anonymize_query_text(query: &str) -> String {
    // OHI regex: `'[^']*'|\d+|".*?"`
    lazy_static! {
        static ref RE: Regex = Regex::new(r#"'[^']*'|\d+|".*?""#).unwrap();
    }
    RE.replace_all(query, "?").to_string()
}

pub fn anonymize_and_normalize(query: &str) -> String {
    // Match OHI normalization steps exactly
    let mut result = query.to_string();
    
    // Replace numbers
    let re_numbers = Regex::new(r"\d+").unwrap();
    result = re_numbers.replace_all(&result, "?").to_string();
    
    // Replace single quotes
    let re_single = Regex::new(r"'[^']*'").unwrap();
    result = re_single.replace_all(&result, "?").to_string();
    
    // Replace double quotes
    let re_double = Regex::new(r#""[^"]*""#).unwrap();
    result = re_double.replace_all(&result, "?").to_string();
    
    // Remove dollar signs
    result = result.replace('$', "");
    
    // Convert to lowercase
    result = result.to_lowercase();
    
    // Remove semicolons
    result = result.replace(';', "");
    
    // Trim and normalize spaces
    result = result.trim().to_string();
    result = result.split_whitespace().collect::<Vec<_>>().join(" ");
    
    result
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_anonymize_query_text() {
        let test_cases = vec![
            (
                "SELECT * FROM users WHERE id = 1 AND name = 'John'",
                "SELECT * FROM users WHERE id = ? AND name = ?",
            ),
            (
                "SELECT * FROM employees WHERE id = 10 OR name <> 'John Doe'",
                "SELECT * FROM employees WHERE id = ? OR name <> ?",
            ),
            (
                r#"UPDATE table SET col = "value" WHERE id = 123"#,
                "UPDATE table SET col = ? WHERE id = ?",
            ),
        ];
        
        for (input, expected) in test_cases {
            assert_eq!(anonymize_query_text(input), expected);
        }
    }
    
    #[test]
    fn test_anonymize_and_normalize() {
        let query = "SELECT * FROM users WHERE id = 123 AND name = 'John';";
        let normalized = anonymize_and_normalize(query);
        assert_eq!(normalized, "select * from users where id = ? and name = ?");
    }
}