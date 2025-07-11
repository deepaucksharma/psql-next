package planattributeextractor

import (
	"regexp"
	"strings"
)

// queryAnonymizer handles query text sanitization to remove sensitive data
type queryAnonymizer struct {
	// Compiled regex patterns for performance
	numericPattern      *regexp.Regexp
	stringPattern       *regexp.Regexp
	hexPattern          *regexp.Regexp
	uuidPattern         *regexp.Regexp
	emailPattern        *regexp.Regexp
	ipPattern           *regexp.Regexp
	datePattern         *regexp.Regexp
	boolPattern         *regexp.Regexp
	inClausePattern     *regexp.Regexp
	betweenPattern      *regexp.Regexp
	casePattern         *regexp.Regexp
}

// newQueryAnonymizer creates a new query anonymizer with pre-compiled patterns
func newQueryAnonymizer() *queryAnonymizer {
	return &queryAnonymizer{
		// Numeric literals (including decimals, scientific notation, and negative numbers)
		numericPattern: regexp.MustCompile(`-?\b\d+\.?\d*([eE][+-]?\d+)?\b`),
		
		// String literals (single and double quotes, handling escaped quotes)
		stringPattern: regexp.MustCompile(`'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"`),
		
		// Hex literals (0x prefix)
		hexPattern: regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`),
		
		// UUID pattern
		uuidPattern: regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`),
		
		// Email pattern (basic)
		emailPattern: regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		
		// IP address pattern
		ipPattern: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		
		// Date patterns (various formats)
		datePattern: regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}(?:[T\s]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)?\b`),
		
		// Boolean literals
		boolPattern: regexp.MustCompile(`\b(?i)(true|false)\b`),
		
		// IN clause values
		inClausePattern: regexp.MustCompile(`(?i)\bIN\s*\([^)]+\)`),
		
		// BETWEEN values
		betweenPattern: regexp.MustCompile(`(?i)\bBETWEEN\s+('[^']*'|\d+)\s+AND\s+('[^']*'|\d+)`),
		
		// CASE statements (can contain sensitive data)
		casePattern: regexp.MustCompile(`(?i)\bCASE\s+WHEN\s+[^END]+END\b`),
	}
}

// AnonymizeQuery removes sensitive data from SQL query text
// This follows the same approach as the legacy New Relic integration
func (a *queryAnonymizer) AnonymizeQuery(query string) string {
	if query == "" {
		return ""
	}
	
	// Preserve original for fallback
	anonymized := query
	
	// Order matters: do string literals first to avoid replacing within strings
	// 1. Replace string literals
	anonymized = a.stringPattern.ReplaceAllString(anonymized, "?")
	
	// 2. Replace special patterns that might contain sensitive data
	anonymized = a.replaceINClause(anonymized)
	anonymized = a.replaceBETWEEN(anonymized)
	anonymized = a.replaceCASE(anonymized)
	
	// 3. Replace other literals
	anonymized = a.emailPattern.ReplaceAllString(anonymized, "?")
	anonymized = a.ipPattern.ReplaceAllString(anonymized, "?")
	anonymized = a.uuidPattern.ReplaceAllString(anonymized, "?")
	anonymized = a.hexPattern.ReplaceAllString(anonymized, "?")
	anonymized = a.datePattern.ReplaceAllString(anonymized, "?")
	
	// 4. Replace numeric literals (do this after other patterns to avoid breaking them)
	anonymized = a.numericPattern.ReplaceAllString(anonymized, "?")
	
	// 5. Replace boolean literals
	anonymized = a.boolPattern.ReplaceAllString(anonymized, "?")
	
	// 6. Normalize whitespace
	anonymized = normalizeWhitespace(anonymized)
	
	// 7. Remove trailing semicolons
	anonymized = strings.TrimRight(anonymized, "; \t\n")
	
	return anonymized
}

// GenerateFingerprint creates a normalized query fingerprint for deduplication
// This is used for query pattern identification
func (a *queryAnonymizer) GenerateFingerprint(query string) string {
	// First anonymize
	fingerprint := a.AnonymizeQuery(query)
	
	// Additional normalization for fingerprinting
	// Convert to lowercase for case-insensitive matching
	fingerprint = strings.ToLower(fingerprint)
	
	// Remove comments
	fingerprint = removeComments(fingerprint)
	
	// Collapse multiple ? into single ?
	fingerprint = collapsePlaceholders(fingerprint)
	
	// Remove database/schema prefixes (e.g., mydb.mytable -> mytable)
	fingerprint = removeDatabasePrefixes(fingerprint)
	
	return fingerprint
}

// replaceINClause handles IN clause anonymization
func (a *queryAnonymizer) replaceINClause(query string) string {
	return a.inClausePattern.ReplaceAllStringFunc(query, func(match string) string {
		// Keep the IN keyword, replace contents with (?)
		if strings.HasPrefix(strings.ToUpper(match), "IN") {
			return "IN (?)"
		}
		return match
	})
}

// replaceBETWEEN handles BETWEEN clause anonymization
func (a *queryAnonymizer) replaceBETWEEN(query string) string {
	return a.betweenPattern.ReplaceAllString(query, "BETWEEN ? AND ?")
}

// replaceCASE handles CASE statement anonymization
func (a *queryAnonymizer) replaceCASE(query string) string {
	// First replace the CASE pattern, then handle remaining values
	result := a.casePattern.ReplaceAllStringFunc(query, func(match string) string {
		// Count WHEN clauses
		whenCount := strings.Count(strings.ToUpper(match), "WHEN")
		
		// Build replacement with correct number of WHEN clauses
		var replacement strings.Builder
		replacement.WriteString("CASE")
		for i := 0; i < whenCount; i++ {
			replacement.WriteString(" WHEN ? THEN ?")
		}
		replacement.WriteString(" ELSE ? END")
		return replacement.String()
	})
	return result
}

// normalizeWhitespace collapses multiple whitespace characters
func normalizeWhitespace(s string) string {
	// Replace multiple spaces, tabs, newlines with single space
	wsPattern := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(wsPattern.ReplaceAllString(s, " "))
}

// removeComments removes SQL comments
func removeComments(s string) string {
	// Remove -- comments
	lineCommentPattern := regexp.MustCompile(`--[^\n]*`)
	s = lineCommentPattern.ReplaceAllString(s, "")
	
	// Remove /* */ comments
	blockCommentPattern := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	s = blockCommentPattern.ReplaceAllString(s, "")
	
	return s
}

// collapsePlaceholders reduces multiple ? to single ?
func collapsePlaceholders(s string) string {
	// Replace (?, ?, ?) with (?)
	placeholderPattern := regexp.MustCompile(`\(\s*\?(?:\s*,\s*\?)*\s*\)`)
	s = placeholderPattern.ReplaceAllString(s, "(?)")
	
	// Replace ?, ?, ? with ?
	multiPlaceholderPattern := regexp.MustCompile(`\?(?:\s*,\s*\?)+`)
	s = multiPlaceholderPattern.ReplaceAllString(s, "?")
	
	return s
}

// removeDatabasePrefixes removes database/schema qualifiers
func removeDatabasePrefixes(s string) string {
	// Pattern for database.table or schema.table
	prefixPattern := regexp.MustCompile(`\b\w+\.(\w+)\b`)
	return prefixPattern.ReplaceAllString(s, "$1")
}