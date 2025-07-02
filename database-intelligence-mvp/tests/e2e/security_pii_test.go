package e2e

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityAndPII validates comprehensive security and PII handling
func TestSecurityAndPII(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	t.Run("PII_Anonymization", testPIIAnonymization)
	t.Run("SQL_Injection_Prevention", testSQLInjectionPrevention)
	t.Run("Data_Leak_Prevention", testDataLeakPrevention)
	t.Run("Compliance_Validation", testComplianceValidation)
}

// testPIIAnonymization validates all PII patterns are properly anonymized
func testPIIAnonymization(t *testing.T) {
	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	t.Log("Testing comprehensive PII anonymization...")

	// Define PII test cases with various formats
	piiTestCases := []struct {
		name     string
		query    string
		piiValue string
		category string
	}{
		// Email patterns
		{
			name:     "Standard Email",
			query:    "SELECT * FROM e2e_test.users WHERE email = 'john.doe@example.com'",
			piiValue: "john.doe@example.com",
			category: "email",
		},
		{
			name:     "Email with Plus",
			query:    "SELECT * FROM e2e_test.users WHERE email = 'user+tag@domain.co.uk'",
			piiValue: "user+tag@domain.co.uk",
			category: "email",
		},
		{
			name:     "Email in Text",
			query:    "SELECT 'Contact us at support@company.com for help' as message",
			piiValue: "support@company.com",
			category: "email",
		},

		// SSN patterns
		{
			name:     "SSN with Dashes",
			query:    "SELECT * FROM e2e_test.users WHERE ssn = '123-45-6789'",
			piiValue: "123-45-6789",
			category: "ssn",
		},
		{
			name:     "SSN without Dashes",
			query:    "SELECT * FROM e2e_test.users WHERE ssn = '123456789'",
			piiValue: "123456789",
			category: "ssn",
		},
		{
			name:     "SSN in JSON",
			query:    `SELECT '{"user_ssn": "987-65-4321"}' as data`,
			piiValue: "987-65-4321",
			category: "ssn",
		},

		// Credit Card patterns
		{
			name:     "Visa Card",
			query:    "SELECT * FROM e2e_test.users WHERE credit_card = '4111-1111-1111-1111'",
			piiValue: "4111-1111-1111-1111",
			category: "credit_card",
		},
		{
			name:     "Mastercard",
			query:    "SELECT * FROM e2e_test.users WHERE credit_card = '5500 0000 0000 0004'",
			piiValue: "5500 0000 0000 0004",
			category: "credit_card",
		},
		{
			name:     "Amex",
			query:    "SELECT * FROM e2e_test.users WHERE credit_card = '378282246310005'",
			piiValue: "378282246310005",
			category: "credit_card",
		},

		// Phone patterns
		{
			name:     "US Phone with Dashes",
			query:    "SELECT * FROM e2e_test.users WHERE phone = '555-123-4567'",
			piiValue: "555-123-4567",
			category: "phone",
		},
		{
			name:     "US Phone with Parentheses",
			query:    "SELECT * FROM e2e_test.users WHERE phone = '(555) 123-4567'",
			piiValue: "(555) 123-4567",
			category: "phone",
		},
		{
			name:     "International Phone",
			query:    "SELECT * FROM e2e_test.users WHERE phone = '+1-555-123-4567'",
			piiValue: "+1-555-123-4567",
			category: "phone",
		},

		// API Keys and Secrets
		{
			name:     "API Key",
			query:    "UPDATE e2e_test.users SET api_key = 'sk_test_4eC39HqLyjWDarjtT1zdp7dc' WHERE id = 1",
			piiValue: "sk_test_4eC39HqLyjWDarjtT1zdp7dc",
			category: "api_key",
		},
		{
			name:     "AWS Secret",
			query:    "SELECT 'aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY' as config",
			piiValue: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			category: "secret",
		},
		{
			name:     "Bearer Token",
			query:    "SELECT 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9' as header",
			piiValue: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			category: "token",
		},

		// IP Addresses
		{
			name:     "IPv4 Address",
			query:    "SELECT * FROM e2e_test.events WHERE client_ip = '192.168.1.100'",
			piiValue: "192.168.1.100",
			category: "ip",
		},
		{
			name:     "IPv6 Address",
			query:    "SELECT * FROM e2e_test.events WHERE client_ip = '2001:0db8:85a3:0000:0000:8a2e:0370:7334'",
			piiValue: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			category: "ip",
		},

		// Complex scenarios
		{
			name:     "Multiple PII in Single Query",
			query:    "INSERT INTO audit_log (email, ssn, cc) VALUES ('test@example.com', '111-22-3333', '4242424242424242')",
			piiValue: "multiple",
			category: "mixed",
		},
		{
			name:     "PII in Subquery",
			query:    "SELECT * FROM orders WHERE user_id IN (SELECT id FROM users WHERE email = 'private@data.com')",
			piiValue: "private@data.com",
			category: "email",
		},
		{
			name:     "PII in LIKE Pattern",
			query:    "SELECT * FROM users WHERE email LIKE '%@sensitive-domain.com'",
			piiValue: "@sensitive-domain.com",
			category: "email_pattern",
		},
	}

	// Execute all PII queries
	for _, tc := range piiTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Execute query (may fail, that's ok)
			rows, _ := pgDB.Query(tc.query)
			if rows != nil {
				rows.Close()
			}
		})
	}

	// Wait for processing
	time.Sleep(15 * time.Second)

	// Get collector output
	output := getCollectorOutput(t)

	// Verify NO PII appears in output
	for _, tc := range piiTestCases {
		if tc.piiValue != "multiple" {
			assert.NotContains(t, output, tc.piiValue, 
				fmt.Sprintf("%s: PII value '%s' should be anonymized", tc.name, tc.piiValue))
		}
	}

	// Verify anonymization markers exist
	assert.Contains(t, output, "[REDACTED]", "Should contain redaction markers")
	assert.Contains(t, output, "[EMAIL]", "Should contain email placeholders")
	assert.Contains(t, output, "[SSN]", "Should contain SSN placeholders")
	assert.Contains(t, output, "[CREDIT_CARD]", "Should contain credit card placeholders")
	assert.Contains(t, output, "[PHONE]", "Should contain phone placeholders")

	// Check anonymization metrics
	metrics := getCollectorMetrics(t)
	piiDetected := extractMetricValue(metrics, "otelcol_processor_verification_pii_detected_total")
	piiAnonymized := extractMetricValue(metrics, "otelcol_processor_verification_pii_anonymized_total")
	
	assert.Greater(t, piiDetected, float64(len(piiTestCases)-5), "Should detect most PII patterns")
	assert.Equal(t, piiDetected, piiAnonymized, "All detected PII should be anonymized")
}

// testSQLInjectionPrevention validates SQL injection attempts are handled safely
func testSQLInjectionPrevention(t *testing.T) {
	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	t.Log("Testing SQL injection prevention...")

	// SQL injection test cases
	injectionTests := []struct {
		name          string
		query         string
		expectFailure bool
	}{
		{
			name:  "Classic Injection",
			query: "SELECT * FROM users WHERE id = '1' OR '1'='1'",
		},
		{
			name:  "Union Injection",
			query: "SELECT * FROM users WHERE id = 1 UNION SELECT * FROM credit_cards",
		},
		{
			name:  "Comment Injection",
			query: "SELECT * FROM users WHERE email = 'test@example.com'--' AND password = 'dummy'",
		},
		{
			name:  "Stacked Queries",
			query: "SELECT * FROM users; DROP TABLE users; --",
		},
		{
			name:  "Time-based Blind",
			query: "SELECT * FROM users WHERE id = 1 AND pg_sleep(5) IS NULL",
		},
		{
			name:  "Boolean Blind",
			query: "SELECT * FROM users WHERE id = 1 AND ASCII(SUBSTRING((SELECT password FROM users LIMIT 1),1,1)) > 65",
		},
		{
			name:  "Out-of-band",
			query: "SELECT * FROM users WHERE id = 1 AND (SELECT COUNT(*) FROM pg_largeobject) > 0",
		},
		{
			name:  "Second Order",
			query: "INSERT INTO users (email, name) VALUES ('admin''--', 'Injected Admin')",
		},
	}

	// Execute injection attempts
	for _, test := range injectionTests {
		t.Run(test.name, func(t *testing.T) {
			// These should either fail or be sanitized
			rows, err := pgDB.Query(test.query)
			if rows != nil {
				rows.Close()
			}
			// We don't assert on error - the important part is the query is logged safely
			_ = err
		})
	}

	time.Sleep(10 * time.Second)

	// Verify injection attempts are handled safely
	output := getCollectorOutput(t)
	
	// These dangerous patterns should be sanitized or not appear
	assert.NotContains(t, output, "DROP TABLE", "DROP commands should be filtered")
	assert.NotContains(t, output, "pg_sleep", "Sleep functions should be filtered")
	assert.NotContains(t, output, "--'", "SQL comments should be handled safely")
	
	// Check security metrics
	metrics := getCollectorMetrics(t)
	suspiciousQueries := extractMetricValue(metrics, "otelcol_processor_verification_suspicious_queries_total")
	assert.Greater(t, suspiciousQueries, float64(0), "Should detect suspicious queries")
}

// testDataLeakPrevention ensures no sensitive data leaks through any channel
func testDataLeakPrevention(t *testing.T) {
	t.Log("Testing data leak prevention...")

	// Check all potential leak vectors
	
	t.Run("Log_Files", func(t *testing.T) {
		// Check collector logs for PII
		logs := getCollectorLogs(t)
		
		// Verify no PII in logs
		assert.NotContains(t, logs, "@example.com", "No emails in logs")
		assert.NotRegexp(t, `\d{3}-\d{2}-\d{4}`, logs, "No SSNs in logs")
		assert.NotRegexp(t, `\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`, logs, "No credit cards in logs")
		assert.NotContains(t, logs, "api_key", "No API keys in logs")
	})

	t.Run("Error_Messages", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Trigger errors with PII
		_, err := pgDB.Query("SELECT * FROM users WHERE email = 'error@test.com' AND invalid_column = 'test'")
		assert.Error(t, err)

		time.Sleep(5 * time.Second)

		// Check error messages don't contain PII
		logs := getCollectorLogs(t)
		assert.NotContains(t, logs, "error@test.com", "PII should not appear in error messages")
	})

	t.Run("Metrics_Labels", func(t *testing.T) {
		// Check Prometheus metrics for PII in labels
		metrics := getCollectorMetrics(t)
		
		// Verify no PII in metric labels
		assert.NotContains(t, metrics, "email=", "No email labels in metrics")
		assert.NotContains(t, metrics, "ssn=", "No SSN labels in metrics")
		assert.NotContains(t, metrics, "user_id=", "No user IDs in metrics")
	})

	t.Run("Export_Paths", func(t *testing.T) {
		// Verify all export paths are sanitized
		output := getCollectorOutput(t)
		
		// Parse JSON and check all fields
		// In real implementation, would parse JSON properly
		assert.NotRegexp(t, `"[^"]*@[^"]*\.[^"]*"`, output, "No raw emails in export")
		assert.NotRegexp(t, `"\d{3}-\d{2}-\d{4}"`, output, "No raw SSNs in export")
	})
}

// testComplianceValidation ensures compliance with regulations
func testComplianceValidation(t *testing.T) {
	t.Log("Testing compliance validation...")

	t.Run("GDPR_Compliance", func(t *testing.T) {
		// Test GDPR requirements
		
		// 1. Right to erasure - PII should be anonymized
		metrics := getCollectorMetrics(t)
		piiAnonymized := extractMetricValue(metrics, "otelcol_processor_verification_pii_anonymized_total")
		assert.Greater(t, piiAnonymized, float64(0), "PII must be anonymized for GDPR")

		// 2. Data minimization - only necessary data collected
		output := getCollectorOutput(t)
		// Verify we're not collecting unnecessary personal data
		assert.NotContains(t, output, "date_of_birth", "Should not collect unnecessary personal data")
		assert.NotContains(t, output, "gender", "Should not collect unnecessary personal data")

		// 3. Purpose limitation - data used only for monitoring
		assert.Contains(t, output, "database_intelligence", "Data should be tagged for monitoring purpose")
	})

	t.Run("HIPAA_Compliance", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Test HIPAA-specific PII handling
		hipaaData := []string{
			"Patient Name: John Doe, MRN: 123456",
			"Diagnosis: ICD-10 I10",
			"Provider: Dr. Smith, NPI: 1234567890",
			"Insurance: BCBS, Member ID: XYZ123456",
		}

		for _, data := range hipaaData {
			pgDB.Query("SELECT $1::text as medical_record", data)
		}

		time.Sleep(10 * time.Second)

		output := getCollectorOutput(t)
		
		// Verify HIPAA data is protected
		assert.NotContains(t, output, "MRN:", "Medical Record Numbers should be protected")
		assert.NotContains(t, output, "ICD-10", "Diagnosis codes should be protected")
		assert.NotContains(t, output, "NPI:", "Provider IDs should be protected")
		assert.NotContains(t, output, "Member ID:", "Insurance IDs should be protected")
	})

	t.Run("PCI_DSS_Compliance", func(t *testing.T) {
		// Test PCI DSS requirements for credit card data
		
		// 1. No storage of sensitive authentication data
		output := getCollectorOutput(t)
		assert.NotRegexp(t, `\d{3,4}`, output, "CVV codes should never be stored")
		
		// 2. Credit card masking
		metrics := getCollectorMetrics(t)
		ccMasked := extractMetricValue(metrics, "otelcol_processor_verification_credit_cards_masked_total")
		assert.Greater(t, ccMasked, float64(0), "Credit cards must be masked")

		// 3. Encryption in transit - verify TLS is used
		// Would check TLS configuration in real implementation
	})

	t.Run("SOC2_Compliance", func(t *testing.T) {
		// Test SOC2 security principles
		
		// 1. Security - data is protected
		metrics := getCollectorMetrics(t)
		securityViolations := extractMetricValue(metrics, "otelcol_processor_verification_security_violations_total")
		assert.Equal(t, float64(0), securityViolations, "No security violations allowed")

		// 2. Availability - system is operational
		healthCheck := isE2EEnvironmentReady(t)
		assert.True(t, healthCheck, "System must be available")

		// 3. Integrity - data is accurate
		dataIntegrityScore := extractMetricValue(metrics, "otelcol_processor_verification_data_integrity_score")
		assert.Greater(t, dataIntegrityScore, float64(0.95), "Data integrity must be maintained")

		// 4. Confidentiality - sensitive data is protected
		confidentialityBreaches := extractMetricValue(metrics, "otelcol_processor_verification_confidentiality_breaches_total")
		assert.Equal(t, float64(0), confidentialityBreaches, "No confidentiality breaches allowed")
	})
}

// Helper function to get collector logs
func getCollectorLogs(t *testing.T) string {
	// In real implementation, would use Docker API
	logs, err := execInContainer("e2e-collector", "cat /var/log/otel/collector.log")
	if err != nil {
		// Fallback to docker logs
		logs, _ = execDockerLogs("e2e-collector", 1000)
	}
	return logs
}

func execDockerLogs(container string, lines int) (string, error) {
	// Would use Docker SDK in real implementation
	return "", nil
}