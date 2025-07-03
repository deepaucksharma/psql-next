package e2e

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "github.com/lib/pq"
	_ "github.com/go-sql-driver/mysql"
)

type DatabaseToNRDBVerificationSuite struct {
	suite.Suite
	pgDB         *sql.DB
	mysqlDB      *sql.DB
	collector    *TestCollector
	nrdbClient   *NRDBClient
	testEnv      *TestEnvironment
	checksums    map[string]string
	checksumLock sync.RWMutex
}

func TestDatabaseToNRDBVerification(t *testing.T) {
	suite.Run(t, new(DatabaseToNRDBVerificationSuite))
}

func (s *DatabaseToNRDBVerificationSuite) SetupSuite() {
	s.testEnv = NewTestEnvironment()
	s.checksums = make(map[string]string)
	
	// Setup PostgreSQL connection
	pgDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		s.testEnv.PostgresHost, s.testEnv.PostgresPort,
		s.testEnv.PostgresUser, s.testEnv.PostgresPassword,
		s.testEnv.PostgresDB)
	
	var err error
	s.pgDB, err = sql.Open("postgres", pgDSN)
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.pgDB.Ping())
	
	// Setup MySQL connection if available
	if s.testEnv.MySQLEnabled {
		mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			s.testEnv.MySQLUser, s.testEnv.MySQLPassword,
			s.testEnv.MySQLHost, s.testEnv.MySQLPort,
			s.testEnv.MySQLDB)
		
		s.mysqlDB, err = sql.Open("mysql", mysqlDSN)
		require.NoError(s.T(), err)
		require.NoError(s.T(), s.mysqlDB.Ping())
	}
	
	// Setup collector
	s.collector = NewTestCollector(s.testEnv)
	require.NoError(s.T(), s.collector.Start(s.getCollectorConfig()))
	
	// Setup NRDB client
	s.nrdbClient = NewNRDBClient(s.testEnv.NewRelicAccountID, s.testEnv.NewRelicAPIKey)
	
	// Setup test schema
	s.setupTestSchema()
}

func (s *DatabaseToNRDBVerificationSuite) TearDownSuite() {
	if s.pgDB != nil {
		s.pgDB.Close()
	}
	if s.mysqlDB != nil {
		s.mysqlDB.Close()
	}
	s.collector.Stop()
	s.testEnv.Cleanup()
}

// Test: Checksum-Based Data Integrity Verification
func (s *DatabaseToNRDBVerificationSuite) TestDataIntegrityWithChecksums() {
	ctx := context.Background()
	
	// Test different metric types
	metricTests := []struct {
		name         string
		dbType       string
		sourceQuery  string
		metricName   string
		nrqlQuery    string
		tolerance    float64
	}{
		{
			name:        "pg_rows_fetched",
			dbType:      "postgresql",
			sourceQuery: "SELECT SUM(n_tup_fetched) FROM pg_stat_user_tables",
			metricName:  "postgresql.rows_fetched",
			nrqlQuery:   "SELECT sum(postgresql.rows_fetched) FROM Metric WHERE db.system = 'postgresql' SINCE 5 minutes ago",
			tolerance:   0.001, // 0.1% tolerance
		},
		{
			name:        "pg_blocks_read",
			dbType:      "postgresql",
			sourceQuery: "SELECT SUM(blks_read) FROM pg_stat_database WHERE datname = current_database()",
			metricName:  "postgresql.blocks_read",
			nrqlQuery:   "SELECT sum(postgresql.blocks_read) FROM Metric WHERE db.system = 'postgresql' SINCE 5 minutes ago",
			tolerance:   0.001,
		},
		{
			name:        "pg_transactions",
			dbType:      "postgresql",
			sourceQuery: "SELECT SUM(xact_commit + xact_rollback) FROM pg_stat_database WHERE datname = current_database()",
			metricName:  "postgresql.transactions.count",
			nrqlQuery:   "SELECT sum(postgresql.transactions.count) FROM Metric WHERE db.system = 'postgresql' SINCE 5 minutes ago",
			tolerance:   0,
		},
	}
	
	for _, test := range metricTests {
		s.Run(test.name, func() {
			// Get baseline value from database
			var baselineValue float64
			err := s.pgDB.QueryRow(test.sourceQuery).Scan(&baselineValue)
			require.NoError(s.T(), err)
			
			// Calculate checksum
			checksum := s.calculateChecksum(test.metricName, baselineValue, time.Now())
			s.storeChecksum(test.metricName, checksum)
			
			// Wait for metric collection and export
			time.Sleep(65 * time.Second) // Wait for collection interval + processing
			
			// Get new value from database
			var currentValue float64
			err = s.pgDB.QueryRow(test.sourceQuery).Scan(&currentValue)
			require.NoError(s.T(), err)
			
			// Query NRDB
			result, err := s.nrdbClient.Query(ctx, test.nrqlQuery)
			require.NoError(s.T(), err)
			
			// Extract value from NRDB result
			nrdbValue := s.extractValueFromNRQL(result)
			
			// Calculate expected value (difference between current and baseline)
			expectedValue := currentValue - baselineValue
			
			// Verify data integrity
			if test.tolerance == 0 {
				assert.Equal(s.T(), expectedValue, nrdbValue,
					"NRDB value should exactly match database value")
			} else {
				deviation := math.Abs(nrdbValue-expectedValue) / expectedValue
				assert.Less(s.T(), deviation, test.tolerance,
					"NRDB value should be within %.2f%% of database value (expected: %.2f, got: %.2f)",
					test.tolerance*100, expectedValue, nrdbValue)
			}
			
			// Verify checksum in NRDB
			checksumQuery := fmt.Sprintf(
				"SELECT latest(checksum) FROM Metric WHERE metricName = '%s' SINCE 5 minutes ago",
				test.metricName)
			checksumResult, err := s.nrdbClient.Query(ctx, checksumQuery)
			require.NoError(s.T(), err)
			
			nrdbChecksum := s.extractStringFromNRQL(checksumResult)
			assert.Equal(s.T(), checksum, nrdbChecksum,
				"Checksum should be preserved through pipeline")
		})
	}
}

// Test: Timestamp Accuracy and Timezone Handling
func (s *DatabaseToNRDBVerificationSuite) TestTimestampAccuracy() {
	ctx := context.Background()
	
	testCases := []struct {
		name        string
		timezone    string
		testTime    time.Time
		description string
	}{
		{
			name:        "utc_time",
			timezone:    "UTC",
			testTime:    time.Now().UTC(),
			description: "UTC timestamp",
		},
		{
			name:        "est_time",
			timezone:    "America/New_York",
			testTime:    time.Now(),
			description: "Eastern time",
		},
		{
			name:        "pst_time",
			timezone:    "America/Los_Angeles",
			testTime:    time.Now(),
			description: "Pacific time",
		},
		{
			name:        "dst_transition",
			timezone:    "America/New_York",
			testTime:    time.Date(2024, 3, 10, 2, 30, 0, 0, time.UTC), // DST transition
			description: "Daylight saving transition",
		},
	}
	
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Create marker event with precise timestamp
			markerID := fmt.Sprintf("marker_%s_%d", tc.name, tc.testTime.UnixNano())
			
			// Insert marker into database
			_, err := s.pgDB.Exec(`
				INSERT INTO test_markers (marker_id, created_at, description)
				VALUES ($1, $2, $3)
			`, markerID, tc.testTime, tc.description)
			require.NoError(s.T(), err)
			
			// Wait for collection
			time.Sleep(65 * time.Second)
			
			// Query NRDB for marker
			nrqlQuery := fmt.Sprintf(`
				SELECT latest(timestamp) FROM Metric 
				WHERE marker_id = '%s' 
				SINCE 5 minutes ago
			`, markerID)
			
			result, err := s.nrdbClient.Query(ctx, nrqlQuery)
			require.NoError(s.T(), err)
			
			// Extract timestamp from NRDB
			nrdbTimestamp := s.extractTimestampFromNRQL(result)
			
			// Verify timestamp accuracy (within 1 second)
			timeDiff := math.Abs(float64(nrdbTimestamp.Sub(tc.testTime)))
			assert.Less(s.T(), timeDiff, float64(time.Second),
				"NRDB timestamp should be within 1 second of source timestamp")
			
			// Verify timezone handling
			assert.Equal(s.T(), tc.testTime.UTC(), nrdbTimestamp.UTC(),
				"Timestamps should match when converted to UTC")
		})
	}
}

// Test: Attribute Preservation and Special Characters
func (s *DatabaseToNRDBVerificationSuite) TestAttributePreservation() {
	ctx := context.Background()
	
	// Test various attribute scenarios
	attributeTests := []struct {
		name       string
		attributes map[string]interface{}
	}{
		{
			name: "basic_attributes",
			attributes: map[string]interface{}{
				"db.name":     "test_db",
				"db.system":   "postgresql",
				"db.version":  "15.2",
				"custom.tag":  "test_value",
			},
		},
		{
			name: "numeric_attributes",
			attributes: map[string]interface{}{
				"connection.count":  int64(42),
				"query.duration":    float64(123.456),
				"cache.hit_ratio":   float64(0.95),
				"port":             int64(5432),
			},
		},
		{
			name: "special_characters",
			attributes: map[string]interface{}{
				"query.text":     "SELECT * FROM users WHERE email = 'test@example.com'",
				"error.message":  "Connection refused: host=\"localhost\" port=5432",
				"unicode.text":   "Hello 世界 🌍 مرحبا мир",
				"json.data":      `{"key": "value", "nested": {"array": [1,2,3]}}`,
			},
		},
		{
			name: "long_values",
			attributes: map[string]interface{}{
				"long.string":    strings.Repeat("a", 1000),
				"very.long":      strings.Repeat("b", 4096),
				"query.plan":     generateLongJSON(2000),
			},
		},
		{
			name: "high_cardinality",
			attributes: map[string]interface{}{
				"request.id":     generateUUID(),
				"trace.id":       generateTraceID(),
				"span.id":        generateSpanID(),
				"correlation.id": fmt.Sprintf("corr_%d", time.Now().UnixNano()),
			},
		},
	}
	
	for _, test := range attributeTests {
		s.Run(test.name, func() {
			// Create test event with attributes
			eventID := fmt.Sprintf("attr_test_%s_%d", test.name, time.Now().UnixNano())
			
			// Send metric with attributes
			err := s.collector.SendMetricWithAttributes("test.attributes", 1.0, 
				mergeAttributes(test.attributes, map[string]interface{}{
					"event.id": eventID,
				}))
			require.NoError(s.T(), err)
			
			// Wait for processing
			time.Sleep(65 * time.Second)
			
			// Query NRDB for all attributes
			nrqlQuery := fmt.Sprintf(`
				SELECT * FROM Metric 
				WHERE event.id = '%s' 
				SINCE 5 minutes ago
				LIMIT 1
			`, eventID)
			
			result, err := s.nrdbClient.Query(ctx, nrqlQuery)
			require.NoError(s.T(), err)
			
			// Extract attributes from result
			nrdbAttributes := s.extractAttributesFromNRQL(result)
			
			// Verify each attribute
			for key, expectedValue := range test.attributes {
				nrdbValue, exists := nrdbAttributes[key]
				assert.True(s.T(), exists, "Attribute %s should exist in NRDB", key)
				
				// Handle different types and truncation
				switch v := expectedValue.(type) {
				case string:
					if len(v) > 4000 {
						// NRDB truncates at 4000 chars
						assert.Equal(s.T(), v[:4000], nrdbValue,
							"Long string should be truncated at 4000 characters")
					} else {
						assert.Equal(s.T(), v, nrdbValue,
							"String attribute should match exactly")
					}
				case int64, float64:
					// Numeric values might be converted
					assert.Equal(s.T(), fmt.Sprintf("%v", v), fmt.Sprintf("%v", nrdbValue),
						"Numeric attribute should match")
				}
			}
		})
	}
}

// Test: Extreme Values and Edge Cases
func (s *DatabaseToNRDBVerificationSuite) TestExtremeValues() {
	ctx := context.Background()
	
	extremeTests := []struct {
		name          string
		metricName    string
		value         interface{}
		expectedNRDB  string
		description   string
	}{
		{
			name:         "max_int64",
			metricName:   "test.max_int",
			value:        int64(math.MaxInt64),
			expectedNRDB: "9223372036854775807",
			description:  "Maximum int64 value",
		},
		{
			name:         "min_int64",
			metricName:   "test.min_int",
			value:        int64(math.MinInt64),
			expectedNRDB: "-9223372036854775808",
			description:  "Minimum int64 value",
		},
		{
			name:         "very_small_float",
			metricName:   "test.small_float",
			value:        float64(1e-308),
			expectedNRDB: "1e-308",
			description:  "Very small float",
		},
		{
			name:         "very_large_float",
			metricName:   "test.large_float",
			value:        float64(1e308),
			expectedNRDB: "1e+308",
			description:  "Very large float",
		},
		{
			name:         "high_precision_float",
			metricName:   "test.precision",
			value:        float64(math.Pi),
			expectedNRDB: "3.141592653589793",
			description:  "High precision float",
		},
		{
			name:         "zero_value",
			metricName:   "test.zero",
			value:        float64(0),
			expectedNRDB: "0",
			description:  "Zero value",
		},
		{
			name:         "negative_zero",
			metricName:   "test.neg_zero",
			value:        float64(-0.0),
			expectedNRDB: "0",
			description:  "Negative zero",
		},
		{
			name:         "infinity",
			metricName:   "test.infinity",
			value:        math.Inf(1),
			expectedNRDB: "null", // NRDB typically stores infinity as null
			description:  "Positive infinity",
		},
		{
			name:         "nan",
			metricName:   "test.nan",
			value:        math.NaN(),
			expectedNRDB: "null", // NRDB stores NaN as null
			description:  "Not a number",
		},
	}
	
	for _, test := range extremeTests {
		s.Run(test.name, func() {
			// Send metric with extreme value
			eventID := fmt.Sprintf("extreme_%s_%d", test.name, time.Now().UnixNano())
			
			err := s.collector.SendMetricWithAttributes(test.metricName, test.value,
				map[string]interface{}{
					"event.id":    eventID,
					"description": test.description,
				})
			require.NoError(s.T(), err)
			
			// Wait for processing
			time.Sleep(65 * time.Second)
			
			// Query NRDB
			nrqlQuery := fmt.Sprintf(`
				SELECT latest(%s) as value FROM Metric 
				WHERE event.id = '%s' 
				SINCE 5 minutes ago
			`, test.metricName, eventID)
			
			result, err := s.nrdbClient.Query(ctx, nrqlQuery)
			require.NoError(s.T(), err)
			
			// Verify value handling
			nrdbValue := s.extractValueStringFromNRQL(result)
			
			if test.expectedNRDB == "null" {
				assert.True(s.T(), nrdbValue == "null" || nrdbValue == "",
					"Infinity/NaN should be stored as null in NRDB")
			} else {
				// For numeric comparison, parse both values
				if strings.Contains(test.expectedNRDB, "e") {
					// Scientific notation - compare as floats
					expected, _ := parseFloat(test.expectedNRDB)
					actual, _ := parseFloat(nrdbValue)
					assert.InEpsilon(s.T(), expected, actual, 1e-10,
						"Scientific notation values should match")
				} else {
					assert.Equal(s.T(), test.expectedNRDB, nrdbValue,
						"Extreme value should be preserved accurately")
				}
			}
		})
	}
}

// Test: Null and Empty Value Handling
func (s *DatabaseToNRDBVerificationSuite) TestNullAndEmptyValues() {
	ctx := context.Background()
	
	// Create test table with nullable columns
	_, err := s.pgDB.Exec(`
		CREATE TABLE IF NOT EXISTS null_test (
			id SERIAL PRIMARY KEY,
			nullable_text TEXT,
			nullable_int INTEGER,
			nullable_float DOUBLE PRECISION,
			nullable_json JSONB,
			empty_string TEXT NOT NULL DEFAULT '',
			zero_int INTEGER NOT NULL DEFAULT 0
		)
	`)
	require.NoError(s.T(), err)
	
	// Insert test data
	testData := []struct {
		name         string
		nullableText sql.NullString
		nullableInt  sql.NullInt64
		nullableFloat sql.NullFloat64
		emptyString  string
		zeroInt      int
	}{
		{
			name:         "all_nulls",
			nullableText: sql.NullString{Valid: false},
			nullableInt:  sql.NullInt64{Valid: false},
			nullableFloat: sql.NullFloat64{Valid: false},
			emptyString:  "",
			zeroInt:      0,
		},
		{
			name:         "mixed_nulls",
			nullableText: sql.NullString{String: "test", Valid: true},
			nullableInt:  sql.NullInt64{Valid: false},
			nullableFloat: sql.NullFloat64{Float64: 0.0, Valid: true},
			emptyString:  "",
			zeroInt:      0,
		},
		{
			name:         "no_nulls",
			nullableText: sql.NullString{String: "value", Valid: true},
			nullableInt:  sql.NullInt64{Int64: 42, Valid: true},
			nullableFloat: sql.NullFloat64{Float64: 3.14, Valid: true},
			emptyString:  "not empty",
			zeroInt:      100,
		},
	}
	
	for _, td := range testData {
		s.Run(td.name, func() {
			// Insert row
			var rowID int
			err := s.pgDB.QueryRow(`
				INSERT INTO null_test (nullable_text, nullable_int, nullable_float, empty_string, zero_int)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id
			`, td.nullableText, td.nullableInt, td.nullableFloat, td.emptyString, td.zeroInt).Scan(&rowID)
			require.NoError(s.T(), err)
			
			// Create metric from row
			eventID := fmt.Sprintf("null_test_%s_%d", td.name, rowID)
			
			attributes := map[string]interface{}{
				"event.id": eventID,
				"row.id":   rowID,
			}
			
			// Add non-null values
			if td.nullableText.Valid {
				attributes["nullable.text"] = td.nullableText.String
			}
			if td.nullableInt.Valid {
				attributes["nullable.int"] = td.nullableInt.Int64
			}
			if td.nullableFloat.Valid {
				attributes["nullable.float"] = td.nullableFloat.Float64
			}
			attributes["empty.string"] = td.emptyString
			attributes["zero.int"] = td.zeroInt
			
			err = s.collector.SendMetricWithAttributes("test.nulls", 1.0, attributes)
			require.NoError(s.T(), err)
			
			// Wait for processing
			time.Sleep(65 * time.Second)
			
			// Query NRDB
			nrqlQuery := fmt.Sprintf(`
				SELECT * FROM Metric 
				WHERE event.id = '%s' 
				SINCE 5 minutes ago
				LIMIT 1
			`, eventID)
			
			result, err := s.nrdbClient.Query(ctx, nrqlQuery)
			require.NoError(s.T(), err)
			
			nrdbAttrs := s.extractAttributesFromNRQL(result)
			
			// Verify null handling
			if !td.nullableText.Valid {
				_, exists := nrdbAttrs["nullable.text"]
				assert.False(s.T(), exists, "NULL text should not exist in NRDB")
			}
			
			if !td.nullableInt.Valid {
				_, exists := nrdbAttrs["nullable.int"]
				assert.False(s.T(), exists, "NULL int should not exist in NRDB")
			}
			
			// Verify empty string vs null distinction
			emptyVal, exists := nrdbAttrs["empty.string"]
			assert.True(s.T(), exists, "Empty string should exist in NRDB")
			assert.Equal(s.T(), td.emptyString, emptyVal, "Empty string should be preserved")
			
			// Verify zero value preservation
			zeroVal, exists := nrdbAttrs["zero.int"]
			assert.True(s.T(), exists, "Zero value should exist in NRDB")
			assert.Equal(s.T(), fmt.Sprintf("%d", td.zeroInt), fmt.Sprintf("%v", zeroVal),
				"Zero value should be preserved")
		})
	}
}

// Test: Special SQL Types
func (s *DatabaseToNRDBVerificationSuite) TestSpecialSQLTypes() {
	ctx := context.Background()
	
	// Create table with special types
	_, err := s.pgDB.Exec(`
		CREATE TABLE IF NOT EXISTS special_types (
			id SERIAL PRIMARY KEY,
			uuid_col UUID,
			json_col JSON,
			jsonb_col JSONB,
			array_col INTEGER[],
			interval_col INTERVAL,
			inet_col INET,
			cidr_col CIDR,
			macaddr_col MACADDR,
			bytea_col BYTEA,
			bit_col BIT(8),
			tsvector_col TSVECTOR,
			point_col POINT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	require.NoError(s.T(), err)
	
	// Test data
	testData := []struct {
		name     string
		sqlType  string
		value    interface{}
		expected string
	}{
		{
			name:     "uuid",
			sqlType:  "uuid_col",
			value:    "550e8400-e29b-41d4-a716-446655440000",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "json",
			sqlType:  "json_col",
			value:    `{"key": "value", "nested": {"data": 123}}`,
			expected: `{"key": "value", "nested": {"data": 123}}`,
		},
		{
			name:     "array",
			sqlType:  "array_col",
			value:    "{1,2,3,4,5}",
			expected: "[1,2,3,4,5]",
		},
		{
			name:     "interval",
			sqlType:  "interval_col",
			value:    "1 year 2 months 3 days 04:05:06",
			expected: "1 year 2 mons 3 days 04:05:06",
		},
		{
			name:     "inet",
			sqlType:  "inet_col",
			value:    "192.168.1.100/32",
			expected: "192.168.1.100",
		},
		{
			name:     "cidr",
			sqlType:  "cidr_col",
			value:    "192.168.0.0/24",
			expected: "192.168.0.0/24",
		},
		{
			name:     "macaddr",
			sqlType:  "macaddr_col",
			value:    "08:00:2b:01:02:03",
			expected: "08:00:2b:01:02:03",
		},
		{
			name:     "bytea",
			sqlType:  "bytea_col",
			value:    []byte{0x00, 0xFF, 0x10, 0x20},
			expected: "00ff1020", // Hex representation
		},
		{
			name:     "point",
			sqlType:  "point_col",
			value:    "(1.5,2.5)",
			expected: "(1.5,2.5)",
		},
	}
	
	for _, td := range testData {
		s.Run(td.name, func() {
			// Insert test data
			query := fmt.Sprintf(`
				INSERT INTO special_types (%s)
				VALUES ($1)
				RETURNING id, %s::text as value_text
			`, td.sqlType, td.sqlType)
			
			var rowID int
			var valueText string
			err := s.pgDB.QueryRow(query, td.value).Scan(&rowID, &valueText)
			require.NoError(s.T(), err)
			
			// Send as metric
			eventID := fmt.Sprintf("special_type_%s_%d", td.name, rowID)
			
			err = s.collector.SendMetricWithAttributes("test.special_types", 1.0,
				map[string]interface{}{
					"event.id":       eventID,
					"type.name":      td.name,
					"type.value":     valueText,
					"type.original":  fmt.Sprintf("%v", td.value),
				})
			require.NoError(s.T(), err)
			
			// Wait for processing
			time.Sleep(65 * time.Second)
			
			// Query NRDB
			nrqlQuery := fmt.Sprintf(`
				SELECT latest(type.value) as value FROM Metric 
				WHERE event.id = '%s' 
				SINCE 5 minutes ago
			`, eventID)
			
			result, err := s.nrdbClient.Query(ctx, nrqlQuery)
			require.NoError(s.T(), err)
			
			// Verify type handling
			nrdbValue := s.extractStringFromNRQL(result)
			
			// Some types might have slight format differences
			if td.name == "array" {
				// PostgreSQL uses {}, NRDB/JSON uses []
				assert.Contains(s.T(), nrdbValue, "1", "Array should contain elements")
				assert.Contains(s.T(), nrdbValue, "5", "Array should contain all elements")
			} else if td.name == "bytea" {
				// Verify hex representation
				assert.True(s.T(), strings.HasPrefix(nrdbValue, "\\x") || nrdbValue == td.expected,
					"Bytea should be in hex format")
			} else {
				assert.Equal(s.T(), td.expected, nrdbValue,
					"Special type %s should be preserved", td.name)
			}
		})
	}
}

// Test: Query Plan Data Accuracy
func (s *DatabaseToNRDBVerificationSuite) TestPlanDataAccuracy() {
	ctx := context.Background()
	
	// Enable auto_explain
	_, err := s.pgDB.Exec(`
		LOAD 'auto_explain';
		SET auto_explain.log_min_duration = 0;
		SET auto_explain.log_analyze = true;
		SET auto_explain.log_format = 'json';
	`)
	require.NoError(s.T(), err)
	
	// Create test table with index
	_, err = s.pgDB.Exec(`
		CREATE TABLE IF NOT EXISTS plan_test (
			id SERIAL PRIMARY KEY,
			indexed_col INTEGER,
			non_indexed_col INTEGER,
			data TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_indexed_col ON plan_test(indexed_col);
	`)
	require.NoError(s.T(), err)
	
	// Insert test data
	for i := 0; i < 1000; i++ {
		_, err = s.pgDB.Exec(`
			INSERT INTO plan_test (indexed_col, non_indexed_col, data)
			VALUES ($1, $2, $3)
		`, i, i, fmt.Sprintf("data_%d", i))
		require.NoError(s.T(), err)
	}
	
	// Analyze table for accurate statistics
	_, err = s.pgDB.Exec("ANALYZE plan_test")
	require.NoError(s.T(), err)
	
	// Test queries with known plan characteristics
	planTests := []struct {
		name             string
		query            string
		expectedPlanType string
		expectedCostLess float64
		expectedRows     int
	}{
		{
			name:             "index_scan",
			query:            "SELECT * FROM plan_test WHERE indexed_col = 42",
			expectedPlanType: "Index Scan",
			expectedCostLess: 10.0,
			expectedRows:     1,
		},
		{
			name:             "seq_scan",
			query:            "SELECT * FROM plan_test WHERE non_indexed_col = 42",
			expectedPlanType: "Seq Scan",
			expectedCostLess: 100.0,
			expectedRows:     1,
		},
		{
			name:             "nested_loop",
			query:            "SELECT * FROM plan_test p1 JOIN plan_test p2 ON p1.id = p2.indexed_col WHERE p1.id < 10",
			expectedPlanType: "Nested Loop",
			expectedCostLess: 1000.0,
			expectedRows:     10,
		},
	}
	
	for _, test := range planTests {
		s.Run(test.name, func() {
			// Execute query with plan tracking
			planID := fmt.Sprintf("plan_%s_%d", test.name, time.Now().UnixNano())
			
			// Add comment to track query
			commentedQuery := fmt.Sprintf("/* plan_id: %s */ %s", planID, test.query)
			
			// Execute query
			rows, err := s.pgDB.Query(commentedQuery)
			require.NoError(s.T(), err)
			defer rows.Close()
			
			// Consume results
			count := 0
			for rows.Next() {
				count++
			}
			
			// Wait for plan to be collected and exported
			time.Sleep(65 * time.Second)
			
			// Query NRDB for plan data
			nrqlQuery := fmt.Sprintf(`
				SELECT 
					latest(plan.type) as planType,
					latest(plan.total_cost) as totalCost,
					latest(plan.rows) as planRows,
					latest(plan.hash) as planHash
				FROM Log 
				WHERE plan_id = '%s' 
				SINCE 5 minutes ago
			`, planID)
			
			result, err := s.nrdbClient.Query(ctx, nrqlQuery)
			require.NoError(s.T(), err)
			
			// Extract plan details
			planData := s.extractPlanDataFromNRQL(result)
			
			// Verify plan type
			assert.Contains(s.T(), planData.PlanType, test.expectedPlanType,
				"Plan should use expected operation type")
			
			// Verify cost estimation
			assert.Less(s.T(), planData.TotalCost, test.expectedCostLess,
				"Plan cost should be reasonable for operation type")
			
			// Verify row estimation
			assert.InDelta(s.T(), test.expectedRows, planData.Rows, float64(test.expectedRows)*0.2,
				"Row estimation should be close to actual")
			
			// Verify plan hash exists and is consistent
			assert.NotEmpty(s.T(), planData.Hash, "Plan should have a hash")
			assert.Len(s.T(), planData.Hash, 64, "Plan hash should be SHA-256")
			
			// Verify anonymization was applied
			nrqlQueryFull := fmt.Sprintf(`
				SELECT latest(db.statement) as statement FROM Log 
				WHERE plan_id = '%s' 
				SINCE 5 minutes ago
			`, planID)
			
			resultFull, err := s.nrdbClient.Query(ctx, nrqlQueryFull)
			require.NoError(s.T(), err)
			
			statement := s.extractStringFromNRQL(resultFull)
			assert.NotContains(s.T(), statement, "42",
				"Literal values should be anonymized in exported data")
		})
	}
}

// Test: Plan Change Detection
func (s *DatabaseToNRDBVerificationSuite) TestPlanChangeDetection() {
	ctx := context.Background()
	
	// Create table for plan change testing
	_, err := s.pgDB.Exec(`
		CREATE TABLE IF NOT EXISTS plan_change_test (
			id SERIAL PRIMARY KEY,
			col1 INTEGER,
			col2 INTEGER,
			data TEXT
		)
	`)
	require.NoError(s.T(), err)
	
	// Insert data
	for i := 0; i < 10000; i++ {
		_, err = s.pgDB.Exec(`
			INSERT INTO plan_change_test (col1, col2, data)
			VALUES ($1, $2, $3)
		`, i%100, i%200, fmt.Sprintf("data_%d", i))
		require.NoError(s.T(), err)
	}
	
	// Analyze for statistics
	_, err = s.pgDB.Exec("ANALYZE plan_change_test")
	require.NoError(s.T(), err)
	
	// Test query
	testQuery := "SELECT * FROM plan_change_test WHERE col1 = 50"
	queryID := fmt.Sprintf("plan_change_%d", time.Now().UnixNano())
	
	// Phase 1: Execute without index
	var phase1Hash string
	{
		commentedQuery := fmt.Sprintf("/* query_id: %s, phase: 1 */ %s", queryID, testQuery)
		rows, err := s.pgDB.Query(commentedQuery)
		require.NoError(s.T(), err)
		rows.Close()
		
		// Wait for export
		time.Sleep(65 * time.Second)
		
		// Get plan hash
		result, err := s.nrdbClient.Query(ctx, fmt.Sprintf(`
			SELECT latest(plan.hash) as hash FROM Log 
			WHERE query_id = '%s' AND phase = '1' 
			SINCE 5 minutes ago
		`, queryID))
		require.NoError(s.T(), err)
		
		phase1Hash = s.extractStringFromNRQL(result)
		assert.NotEmpty(s.T(), phase1Hash, "Should have plan hash for phase 1")
	}
	
	// Phase 2: Create index
	_, err = s.pgDB.Exec("CREATE INDEX idx_plan_change_col1 ON plan_change_test(col1)")
	require.NoError(s.T(), err)
	
	// Phase 3: Execute with index
	var phase3Hash string
	{
		commentedQuery := fmt.Sprintf("/* query_id: %s, phase: 3 */ %s", queryID, testQuery)
		rows, err := s.pgDB.Query(commentedQuery)
		require.NoError(s.T(), err)
		rows.Close()
		
		// Wait for export
		time.Sleep(65 * time.Second)
		
		// Get plan hash and change detection
		result, err := s.nrdbClient.Query(ctx, fmt.Sprintf(`
			SELECT 
				latest(plan.hash) as hash,
				latest(plan.change_detected) as changeDetected,
				latest(plan.previous_hash) as previousHash
			FROM Log 
			WHERE query_id = '%s' AND phase = '3' 
			SINCE 5 minutes ago
		`, queryID))
		require.NoError(s.T(), err)
		
		planData := s.extractPlanChangeDataFromNRQL(result)
		phase3Hash = planData.Hash
		
		// Verify plan changed
		assert.NotEqual(s.T(), phase1Hash, phase3Hash,
			"Plan hash should change after index creation")
		assert.True(s.T(), planData.ChangeDetected,
			"Plan change should be detected")
		assert.Equal(s.T(), phase1Hash, planData.PreviousHash,
			"Previous hash should match phase 1")
	}
	
	// Phase 4: Drop index (regression)
	_, err = s.pgDB.Exec("DROP INDEX idx_plan_change_col1")
	require.NoError(s.T(), err)
	
	// Phase 5: Execute without index again
	{
		commentedQuery := fmt.Sprintf("/* query_id: %s, phase: 5 */ %s", queryID, testQuery)
		rows, err := s.pgDB.Query(commentedQuery)
		require.NoError(s.T(), err)
		rows.Close()
		
		// Wait for export
		time.Sleep(65 * time.Second)
		
		// Check for regression detection
		result, err := s.nrdbClient.Query(ctx, fmt.Sprintf(`
			SELECT 
				latest(plan.hash) as hash,
				latest(plan.regression_detected) as regressionDetected,
				latest(plan.performance_impact) as performanceImpact
			FROM Log 
			WHERE query_id = '%s' AND phase = '5' 
			SINCE 10 minutes ago
		`, queryID))
		require.NoError(s.T(), err)
		
		regressionData := s.extractRegressionDataFromNRQL(result)
		
		// Verify regression detected
		assert.Equal(s.T(), phase1Hash, regressionData.Hash,
			"Plan should revert to original")
		assert.True(s.T(), regressionData.RegressionDetected,
			"Regression should be detected")
		assert.Greater(s.T(), regressionData.PerformanceImpact, 0.0,
			"Performance impact should be calculated")
	}
}

// Helper methods

func (s *DatabaseToNRDBVerificationSuite) getCollectorConfig() string {
	return `
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    databases:
      - ${POSTGRES_DB}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 60s
    tls:
      insecure: true

  sqlquery/postgresql:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable"
    queries:
      - sql: |
          SELECT 
            marker_id,
            created_at,
            description
          FROM test_markers
          WHERE created_at > NOW() - INTERVAL '5 minutes'
        metrics:
          - metric_name: test.marker
            value_column: "1"
            attribute_columns: ["marker_id", "created_at", "description"]

processors:
  planattributeextractor:
    enabled: true
  
  verification:
    enabled: true
    checksum_attributes: ["metricName", "value", "timestamp"]
  
  attributes/checksum:
    actions:
      - key: checksum
        action: insert
        from_attribute: _checksum

exporters:
  otlp/newrelic:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    
  debug:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 100

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [verification, attributes/checksum]
      exporters: [otlp/newrelic, debug]
    
    logs:
      receivers: [sqlquery/postgresql]
      processors: [planattributeextractor]
      exporters: [otlp/newrelic]
`
}

func (s *DatabaseToNRDBVerificationSuite) setupTestSchema() {
	// Create test tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS test_markers (
			marker_id VARCHAR(255) PRIMARY KEY,
			created_at TIMESTAMP WITH TIME ZONE,
			description TEXT
		)`,
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
		`CREATE EXTENSION IF NOT EXISTS "pg_stat_statements"`,
	}
	
	for _, query := range queries {
		_, err := s.pgDB.Exec(query)
		if err != nil {
			s.T().Logf("Warning: Failed to execute setup query: %v", err)
		}
	}
}

func (s *DatabaseToNRDBVerificationSuite) calculateChecksum(metricName string, value float64, timestamp time.Time) string {
	data := fmt.Sprintf("%s:%.6f:%d", metricName, value, timestamp.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *DatabaseToNRDBVerificationSuite) storeChecksum(metricName, checksum string) {
	s.checksumLock.Lock()
	defer s.checksumLock.Unlock()
	s.checksums[metricName] = checksum
}

func (s *DatabaseToNRDBVerificationSuite) extractValueFromNRQL(result *NRQLResult) float64 {
	// Implementation depends on NRQL result structure
	// This is a simplified version
	if result != nil && len(result.Results) > 0 {
		if val, ok := result.Results[0]["value"].(float64); ok {
			return val
		}
	}
	return 0
}

func (s *DatabaseToNRDBVerificationSuite) extractStringFromNRQL(result *NRQLResult) string {
	if result != nil && len(result.Results) > 0 {
		if val, ok := result.Results[0]["value"].(string); ok {
			return val
		}
	}
	return ""
}

func (s *DatabaseToNRDBVerificationSuite) extractTimestampFromNRQL(result *NRQLResult) time.Time {
	if result != nil && len(result.Results) > 0 {
		if val, ok := result.Results[0]["timestamp"].(int64); ok {
			return time.Unix(0, val*1e6) // Convert milliseconds to nanoseconds
		}
	}
	return time.Time{}
}

func (s *DatabaseToNRDBVerificationSuite) extractAttributesFromNRQL(result *NRQLResult) map[string]interface{} {
	if result != nil && len(result.Results) > 0 {
		return result.Results[0]
	}
	return make(map[string]interface{})
}

func (s *DatabaseToNRDBVerificationSuite) extractValueStringFromNRQL(result *NRQLResult) string {
	if result != nil && len(result.Results) > 0 {
		if val := result.Results[0]["value"]; val != nil {
			return fmt.Sprintf("%v", val)
		}
	}
	return "null"
}

func (s *DatabaseToNRDBVerificationSuite) extractPlanDataFromNRQL(result *NRQLResult) PlanData {
	data := PlanData{}
	if result != nil && len(result.Results) > 0 {
		r := result.Results[0]
		if v, ok := r["planType"].(string); ok {
			data.PlanType = v
		}
		if v, ok := r["totalCost"].(float64); ok {
			data.TotalCost = v
		}
		if v, ok := r["planRows"].(float64); ok {
			data.Rows = int(v)
		}
		if v, ok := r["planHash"].(string); ok {
			data.Hash = v
		}
	}
	return data
}

func (s *DatabaseToNRDBVerificationSuite) extractPlanChangeDataFromNRQL(result *NRQLResult) PlanChangeData {
	data := PlanChangeData{}
	if result != nil && len(result.Results) > 0 {
		r := result.Results[0]
		if v, ok := r["hash"].(string); ok {
			data.Hash = v
		}
		if v, ok := r["changeDetected"].(bool); ok {
			data.ChangeDetected = v
		}
		if v, ok := r["previousHash"].(string); ok {
			data.PreviousHash = v
		}
	}
	return data
}

func (s *DatabaseToNRDBVerificationSuite) extractRegressionDataFromNRQL(result *NRQLResult) RegressionData {
	data := RegressionData{}
	if result != nil && len(result.Results) > 0 {
		r := result.Results[0]
		if v, ok := r["hash"].(string); ok {
			data.Hash = v
		}
		if v, ok := r["regressionDetected"].(bool); ok {
			data.RegressionDetected = v
		}
		if v, ok := r["performanceImpact"].(float64); ok {
			data.PerformanceImpact = v
		}
	}
	return data
}

// Helper functions

func generateUUID() string {
	// Simple UUID v4 generation
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func generateTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSpanID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateLongJSON(size int) string {
	data := make(map[string]interface{})
	for i := 0; i < size/50; i++ {
		data[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	jsonBytes, _ := json.Marshal(data)
	return string(jsonBytes)
}

func mergeAttributes(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func parseFloat(s string) (float64, error) {
	return fmt.Sscanf(s, "%e", new(float64))
}

// Data structures

type PlanData struct {
	PlanType  string
	TotalCost float64
	Rows      int
	Hash      string
}

type PlanChangeData struct {
	Hash           string
	ChangeDetected bool
	PreviousHash   string
}

type RegressionData struct {
	Hash               string
	RegressionDetected bool
	PerformanceImpact  float64
}