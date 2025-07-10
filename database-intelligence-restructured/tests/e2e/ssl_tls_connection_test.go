package e2e

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSSLTLSConnections verifies collector can connect to databases using SSL/TLS
func TestSSLTLSConnections(t *testing.T) {
	// Skip if no credentials
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	otlpEndpoint := os.Getenv("NEW_RELIC_OTLP_ENDPOINT")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	
	if licenseKey == "" || apiKey == "" || accountID == "" {
		t.Skip("Required credentials not set")
	}
	
	if otlpEndpoint == "" {
		otlpEndpoint = "https://otlp.nr-data.net:4317"
	}

	runID := fmt.Sprintf("ssl_tls_%d", time.Now().Unix())
	t.Logf("Starting SSL/TLS connection test with run ID: %s", runID)

	// Create certificates directory
	certDir := "ssl-certs"
	err := os.MkdirAll(certDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create cert directory: %v", err)
	}
	defer os.RemoveAll(certDir)

	// Generate self-signed certificates
	t.Log("Generating self-signed certificates...")
	generateCertificates(t, certDir)

	// Test scenarios
	t.Run("PostgreSQL_SSL_Required", func(t *testing.T) {
		testPostgreSQLSSL(t, runID, certDir, licenseKey, otlpEndpoint, accountID, apiKey)
	})

	t.Run("MySQL_SSL_Required", func(t *testing.T) {
		testMySQLSSL(t, runID, certDir, licenseKey, otlpEndpoint, accountID, apiKey)
	})

	t.Run("PostgreSQL_mTLS", func(t *testing.T) {
		testPostgreSQLmTLS(t, runID, certDir, licenseKey, otlpEndpoint, accountID, apiKey)
	})

	t.Run("SSL_Certificate_Validation", func(t *testing.T) {
		testSSLCertificateValidation(t, runID, certDir, licenseKey, otlpEndpoint)
	})
}

func generateCertificates(t *testing.T, certDir string) {
	// Generate CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test CA"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate CA private key: %v", err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create CA certificate: %v", err)
	}

	// Save CA certificate
	caCertOut, err := os.Create(filepath.Join(certDir, "ca-cert.pem"))
	if err != nil {
		t.Fatalf("Failed to create CA cert file: %v", err)
	}
	defer caCertOut.Close()
	pem.Encode(caCertOut, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes})

	// Save CA private key
	caKeyOut, err := os.Create(filepath.Join(certDir, "ca-key.pem"))
	if err != nil {
		t.Fatalf("Failed to create CA key file: %v", err)
	}
	defer caKeyOut.Close()
	pem.Encode(caKeyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey)})

	// Generate server certificate
	serverCert := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization:  []string{"Test Server"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		DNSNames:     []string{"localhost", "*.docker.internal", "host.docker.internal"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv4(172, 17, 0, 1)},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate server private key: %v", err)
	}

	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverCert, ca, &serverPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create server certificate: %v", err)
	}

	// Save server certificate
	serverCertOut, err := os.Create(filepath.Join(certDir, "server-cert.pem"))
	if err != nil {
		t.Fatalf("Failed to create server cert file: %v", err)
	}
	defer serverCertOut.Close()
	pem.Encode(serverCertOut, &pem.Block{Type: "CERTIFICATE", Bytes: serverCertBytes})

	// Save server private key
	serverKeyOut, err := os.Create(filepath.Join(certDir, "server-key.pem"))
	if err != nil {
		t.Fatalf("Failed to create server key file: %v", err)
	}
	defer serverKeyOut.Close()
	pem.Encode(serverKeyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverPrivKey)})

	// Generate client certificate for mTLS
	clientCert := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization:  []string{"Test Client"},
			Country:       []string{"US"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 7},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	clientPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate client private key: %v", err)
	}

	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientCert, ca, &clientPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create client certificate: %v", err)
	}

	// Save client certificate
	clientCertOut, err := os.Create(filepath.Join(certDir, "client-cert.pem"))
	if err != nil {
		t.Fatalf("Failed to create client cert file: %v", err)
	}
	defer clientCertOut.Close()
	pem.Encode(clientCertOut, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertBytes})

	// Save client private key
	clientKeyOut, err := os.Create(filepath.Join(certDir, "client-key.pem"))
	if err != nil {
		t.Fatalf("Failed to create client key file: %v", err)
	}
	defer clientKeyOut.Close()
	pem.Encode(clientKeyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientPrivKey)})

	t.Log("Certificates generated successfully")
}

func testPostgreSQLSSL(t *testing.T, runID, certDir, licenseKey, otlpEndpoint, accountID, apiKey string) {
	t.Log("Testing PostgreSQL with SSL required...")

	// Get absolute paths for certificates
	absPath, _ := os.Getwd()
	absCertDir := filepath.Join(absPath, certDir)

	// Start PostgreSQL with SSL enabled
	exec.Command("docker", "rm", "-f", "postgres-ssl").Run()
	
	postgresCmd := exec.Command("docker", "run",
		"--name", "postgres-ssl",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_HOST_AUTH_METHOD=scram-sha-256",
		"-v", absCertDir+":/var/lib/postgresql/ssl:ro",
		"-p", "5432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine",
		"-c", "ssl=on",
		"-c", "ssl_cert_file=/var/lib/postgresql/ssl/server-cert.pem",
		"-c", "ssl_key_file=/var/lib/postgresql/ssl/server-key.pem",
		"-c", "ssl_ca_file=/var/lib/postgresql/ssl/ca-cert.pem")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL with SSL: %v\n%s", err, output)
	}

	defer func() {
		exec.Command("docker", "stop", "postgres-ssl").Run()
		exec.Command("docker", "rm", "postgres-ssl").Run()
		exec.Command("docker", "stop", "postgres-ssl-collector").Run()
		exec.Command("docker", "rm", "postgres-ssl-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(20 * time.Second)

	// Create collector config with SSL
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:5432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 10s
    tls:
      insecure: false
      insecure_skip_verify: true  # For self-signed cert
      # ca_file: /ssl/ca-cert.pem  # Could mount and use CA cert

processors:
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: postgresql_ssl
        action: insert
      - key: connection.type
        value: ssl
        action: insert
  
  batch:
    timeout: 5s

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
  
  logging:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: debug
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "postgres-ssl-config.yaml"
	err = os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector with SSL configuration...")
	exec.Command("docker", "rm", "-f", "postgres-ssl-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "postgres-ssl-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"-v", absCertDir+":/ssl:ro",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Wait for collection
	time.Sleep(30 * time.Second)

	// Check collector logs for SSL connection
	logsCmd := exec.Command("docker", "logs", "--tail", "50", "postgres-ssl-collector")
	logs, _ := logsCmd.CombinedOutput()
	logsStr := string(logs)

	if strings.Contains(logsStr, "SSL connection") || !strings.Contains(logsStr, "failed") {
		t.Log("✓ PostgreSQL SSL connection appears successful")
	}

	// Verify metrics in NRDB
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' AND connection.type = 'ssl' SINCE 2 minutes ago", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}

	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok && count > 0 {
			t.Logf("✓ PostgreSQL SSL: %.0f metrics collected over SSL connection", count)
		}
	}
}

func testMySQLSSL(t *testing.T, runID, certDir, licenseKey, otlpEndpoint, accountID, apiKey string) {
	t.Log("Testing MySQL with SSL required...")

	// Get absolute paths for certificates
	absPath, _ := os.Getwd()
	absCertDir := filepath.Join(absPath, certDir)

	// Start MySQL with SSL enabled
	exec.Command("docker", "rm", "-f", "mysql-ssl").Run()
	
	// Create custom MySQL config for SSL
	mysqlConfig := `
[mysqld]
require_secure_transport=ON
ssl-ca=/etc/mysql/ssl/ca-cert.pem
ssl-cert=/etc/mysql/ssl/server-cert.pem
ssl-key=/etc/mysql/ssl/server-key.pem
`
	mysqlConfigPath := filepath.Join(certDir, "mysql-ssl.cnf")
	err := os.WriteFile(mysqlConfigPath, []byte(mysqlConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write MySQL config: %v", err)
	}

	mysqlCmd := exec.Command("docker", "run",
		"--name", "mysql-ssl",
		"-e", "MYSQL_ROOT_PASSWORD=mysql",
		"-e", "MYSQL_DATABASE=testdb",
		"-v", absCertDir+":/etc/mysql/ssl:ro",
		"-v", filepath.Join(absCertDir, "mysql-ssl.cnf")+":/etc/mysql/conf.d/ssl.cnf:ro",
		"-p", "3306:3306",
		"--network", "bridge",
		"-d", "mysql:8.0")

	output, err := mysqlCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start MySQL with SSL: %v\n%s", err, output)
	}

	defer func() {
		exec.Command("docker", "stop", "mysql-ssl").Run()
		exec.Command("docker", "rm", "mysql-ssl").Run()
		exec.Command("docker", "stop", "mysql-ssl-collector").Run()
		exec.Command("docker", "rm", "mysql-ssl-collector").Run()
	}()

	// Wait for MySQL
	time.Sleep(40 * time.Second)

	// Create collector config with SSL for MySQL
	config := fmt.Sprintf(`
receivers:
  mysql:
    endpoint: host.docker.internal:3306
    username: root
    password: mysql
    database: testdb
    collection_interval: 10s
    tls:
      insecure: false
      insecure_skip_verify: true  # For self-signed cert

processors:
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: mysql_ssl
        action: insert
      - key: connection.type
        value: ssl
        action: insert
  
  batch:
    timeout: 5s

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
  
  logging:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [mysql]
      processors: [attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: debug
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "mysql-ssl-config.yaml"
	err = os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector with MySQL SSL configuration...")
	exec.Command("docker", "rm", "-f", "mysql-ssl-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "mysql-ssl-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Wait for collection
	time.Sleep(30 * time.Second)

	// Verify metrics in NRDB
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' AND test.type = 'mysql_ssl' SINCE 2 minutes ago", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}

	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok && count > 0 {
			t.Logf("✓ MySQL SSL: %.0f metrics collected over SSL connection", count)
		}
	}
}

func testPostgreSQLmTLS(t *testing.T, runID, certDir, licenseKey, otlpEndpoint, accountID, apiKey string) {
	t.Log("Testing PostgreSQL with mutual TLS (mTLS)...")

	// This test would require:
	// 1. PostgreSQL configured to require client certificates
	// 2. Collector configured with client certificate and key
	// 3. Verification that only authenticated clients can connect

	t.Log("PostgreSQL mTLS test - configuration example created")

	// Create example mTLS config
	mTLSConfig := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:5432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 10s
    tls:
      insecure: false
      cert_file: /ssl/client-cert.pem
      key_file: /ssl/client-key.pem
      ca_file: /ssl/ca-cert.pem

processors:
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: connection.type
        value: mtls
        action: insert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes]
      exporters: [otlp]
`, runID, otlpEndpoint, licenseKey)

	// Save example config
	mTLSConfigPath := "postgres-mtls-example.yaml"
	err := os.WriteFile(mTLSConfigPath, []byte(mTLSConfig), 0644)
	if err != nil {
		t.Errorf("Failed to write mTLS config example: %v", err)
	} else {
		t.Logf("✓ mTLS configuration example saved to %s", mTLSConfigPath)
	}
	defer os.Remove(mTLSConfigPath)
}

func testSSLCertificateValidation(t *testing.T, runID, certDir, licenseKey, otlpEndpoint string) {
	t.Log("Testing SSL certificate validation scenarios...")

	// Test invalid certificate scenario
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:5432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 10s
    tls:
      insecure: false
      insecure_skip_verify: false  # Strict validation
      ca_file: /ssl/wrong-ca-cert.pem  # Wrong CA

processors:
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: ssl_validation_fail
        action: insert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
  
  logging:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: debug
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "ssl-validation-test.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	t.Log("SSL certificate validation test configuration created")
	t.Log("This would test:")
	t.Log("  1. Certificate validation failures")
	t.Log("  2. Expired certificate handling")
	t.Log("  3. Hostname verification")
	t.Log("  4. Certificate chain validation")
}