package planattributeextractor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryAnonymizer_AnonymizeQuery(t *testing.T) {
	anonymizer := newQueryAnonymizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple numeric values",
			input:    "SELECT * FROM users WHERE id = 123",
			expected: "SELECT * FROM users WHERE id = ?",
		},
		{
			name:     "string literals",
			input:    "SELECT * FROM users WHERE name = 'John Doe'",
			expected: "SELECT * FROM users WHERE name = ?",
		},
		{
			name:     "multiple values",
			input:    "SELECT * FROM orders WHERE user_id = 456 AND status = 'active'",
			expected: "SELECT * FROM orders WHERE user_id = ? AND status = ?",
		},
		{
			name:     "IN clause",
			input:    "SELECT * FROM products WHERE id IN (1, 2, 3, 4, 5)",
			expected: "SELECT * FROM products WHERE id IN (?)",
		},
		{
			name:     "BETWEEN clause",
			input:    "SELECT * FROM orders WHERE created_at BETWEEN '2024-01-01' AND '2024-12-31'",
			expected: "SELECT * FROM orders WHERE created_at BETWEEN ? AND ?",
		},
		{
			name:     "email addresses",
			input:    "SELECT * FROM users WHERE email = 'user@example.com'",
			expected: "SELECT * FROM users WHERE email = ?",
		},
		{
			name:     "UUID values",
			input:    "SELECT * FROM accounts WHERE id = '550e8400-e29b-41d4-a716-446655440000'",
			expected: "SELECT * FROM accounts WHERE id = ?",
		},
		{
			name:     "IP addresses",
			input:    "SELECT * FROM logs WHERE ip = '192.168.1.100'",
			expected: "SELECT * FROM logs WHERE ip = ?",
		},
		{
			name:     "hex values",
			input:    "SELECT * FROM data WHERE hash = 0xDEADBEEF",
			expected: "SELECT * FROM data WHERE hash = ?",
		},
		{
			name:     "dates and timestamps",
			input:    "SELECT * FROM events WHERE timestamp = '2024-06-28T10:30:00Z'",
			expected: "SELECT * FROM events WHERE timestamp = ?",
		},
		{
			name:     "boolean values",
			input:    "SELECT * FROM settings WHERE enabled = true AND deleted = false",
			expected: "SELECT * FROM settings WHERE enabled = ? AND deleted = ?",
		},
		{
			name:     "complex CASE statement",
			input:    "SELECT CASE WHEN price > 100 THEN 'expensive' WHEN price > 50 THEN 'moderate' ELSE 'cheap' END",
			expected: "SELECT CASE WHEN price > ? THEN ? WHEN price > ? THEN ? ELSE ? END",
		},
		{
			name:     "empty query",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace normalization",
			input:    "SELECT  *   FROM   users\n\nWHERE   id =   123",
			expected: "SELECT * FROM users WHERE id = ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := anonymizer.AnonymizeQuery(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryAnonymizer_GenerateFingerprint(t *testing.T) {
	anonymizer := newQueryAnonymizer()

	tests := []struct {
		name        string
		queries     []string
		sameHash    bool
	}{
		{
			name: "same query pattern",
			queries: []string{
				"SELECT * FROM users WHERE id = 123",
				"SELECT * FROM users WHERE id = 456",
				"SELECT * FROM users WHERE id = 789",
			},
			sameHash: true,
		},
		{
			name: "different query patterns",
			queries: []string{
				"SELECT * FROM users WHERE id = 123",
				"SELECT * FROM orders WHERE user_id = 123",
				"SELECT * FROM products WHERE price > 100",
			},
			sameHash: false,
		},
		{
			name: "case insensitive",
			queries: []string{
				"SELECT * FROM users WHERE id = 123",
				"select * from users where id = 456",
				"SeLeCt * FrOm UsErS WhErE iD = 789",
			},
			sameHash: true,
		},
		{
			name: "with database prefixes",
			queries: []string{
				"SELECT * FROM db1.users WHERE id = 123",
				"SELECT * FROM db2.users WHERE id = 456",
				"SELECT * FROM users WHERE id = 789",
			},
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fingerprints := make([]string, len(tt.queries))
			for i, query := range tt.queries {
				fingerprints[i] = anonymizer.GenerateFingerprint(query)
			}

			if tt.sameHash {
				// All fingerprints should be the same
				for i := 1; i < len(fingerprints); i++ {
					assert.Equal(t, fingerprints[0], fingerprints[i], 
						"Expected same fingerprint for queries: %s and %s", 
						tt.queries[0], tt.queries[i])
				}
			} else {
				// All fingerprints should be different
				for i := 0; i < len(fingerprints); i++ {
					for j := i + 1; j < len(fingerprints); j++ {
						assert.NotEqual(t, fingerprints[i], fingerprints[j],
							"Expected different fingerprints for queries: %s and %s",
							tt.queries[i], tt.queries[j])
					}
				}
			}
		})
	}
}

func TestQueryAnonymizer_SensitiveData(t *testing.T) {
	anonymizer := newQueryAnonymizer()

	// Test that sensitive data is properly anonymized
	sensitiveQueries := []string{
		"INSERT INTO users (ssn) VALUES ('123-45-6789')",
		"SELECT * FROM accounts WHERE credit_card = '4111-1111-1111-1111'",
		"UPDATE users SET email = 'john.doe@example.com' WHERE id = 123",
		"SELECT * FROM logs WHERE ip_address = '192.168.1.100'",
		"INSERT INTO data (uuid) VALUES ('550e8400-e29b-41d4-a716-446655440000')",
	}

	for _, query := range sensitiveQueries {
		anonymized := anonymizer.AnonymizeQuery(query)
		
		// Ensure no sensitive patterns remain
		assert.NotContains(t, anonymized, "123-45-6789", "SSN should be anonymized")
		assert.NotContains(t, anonymized, "4111-1111-1111-1111", "Credit card should be anonymized")
		assert.NotContains(t, anonymized, "john.doe@example.com", "Email should be anonymized")
		assert.NotContains(t, anonymized, "192.168.1.100", "IP address should be anonymized")
		assert.NotContains(t, anonymized, "550e8400-e29b-41d4-a716-446655440000", "UUID should be anonymized")
		
		// Ensure structure is preserved
		assert.Contains(t, anonymized, "?", "Query should contain placeholder")
	}
}

func TestQueryAnonymizer_EdgeCases(t *testing.T) {
	anonymizer := newQueryAnonymizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "nested strings",
			input:    `SELECT * FROM users WHERE data = '{"name": "John", "age": 30}'`,
			expected: "SELECT * FROM users WHERE data = ?",
		},
		{
			name:     "escaped quotes",
			input:    `SELECT * FROM users WHERE name = 'O\'Brien'`,
			expected: "SELECT * FROM users WHERE name = ?",
		},
		{
			name:     "scientific notation",
			input:    "SELECT * FROM data WHERE value = 1.23e-4",
			expected: "SELECT * FROM data WHERE value = ?",
		},
		{
			name:     "negative numbers",
			input:    "SELECT * FROM accounts WHERE balance > -100.50",
			expected: "SELECT * FROM accounts WHERE balance > ?",
		},
		{
			name:     "multiple IN clauses",
			input:    "SELECT * FROM orders WHERE status IN ('pending', 'active') AND user_id IN (1, 2, 3)",
			expected: "SELECT * FROM orders WHERE status IN (?) AND user_id IN (?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := anonymizer.AnonymizeQuery(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkQueryAnonymizer_AnonymizeQuery(b *testing.B) {
	anonymizer := newQueryAnonymizer()
	query := "SELECT * FROM users WHERE id = 123 AND email = 'user@example.com' AND created_at BETWEEN '2024-01-01' AND '2024-12-31'"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = anonymizer.AnonymizeQuery(query)
	}
}

func BenchmarkQueryAnonymizer_GenerateFingerprint(b *testing.B) {
	anonymizer := newQueryAnonymizer()
	query := "SELECT * FROM users WHERE id = 123 AND email = 'user@example.com' AND created_at BETWEEN '2024-01-01' AND '2024-12-31'"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = anonymizer.GenerateFingerprint(query)
	}
}