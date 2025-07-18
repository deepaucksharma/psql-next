package e2e

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleE2EFlow tests basic end-to-end flow with data generation
func TestSimpleE2EFlow(t *testing.T) {
	// Skip if no credentials
	if os.Getenv("NEW_RELIC_API_KEY") == "" {
		t.Skip("NEW_RELIC_API_KEY not set")
	}

	t.Run("MySQL_Setup_And_Data_Generation", func(t *testing.T) {
		// Connect to MySQL
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
			getEnvOrDefault("MYSQL_USER", "root"),
			getEnvOrDefault("MYSQL_PASSWORD", "rootpassword"),
			getEnvOrDefault("MYSQL_HOST", "localhost"),
			getEnvOrDefault("MYSQL_PORT", "3306"),
			getEnvOrDefault("MYSQL_DATABASE", "production"))
		
		// Create data generator
		dg, err := NewDataGenerator(dsn)
		require.NoError(t, err)
		defer dg.Stop()
		
		// Setup test schema
		t.Log("Setting up test schema...")
		err = dg.SetupTestSchema()
		require.NoError(t, err)
		
		// Populate base data
		t.Log("Populating base data...")
		err = dg.PopulateBaseData()
		require.NoError(t, err)
		
		// Generate mixed workload for 30 seconds
		t.Log("Generating mixed workload...")
		dg.GenerateMixedWorkload(30*time.Second, 10)
		
		// Wait for workload to complete
		time.Sleep(35 * time.Second)
		dg.Stop()
		
		// Get metrics
		metrics := dg.GetMetrics()
		t.Logf("Workload metrics:")
		t.Logf("  Queries executed: %d", metrics["queries_executed"])
		t.Logf("  Locks generated: %d", metrics["locks_generated"]) 
		t.Logf("  Slow queries: %d", metrics["slow_queries"])
		
		assert.Greater(t, metrics["queries_executed"], int64(100), "Should have executed many queries")
	})
	
	t.Run("Verify_MySQL_Metrics", func(t *testing.T) {
		db := connectMySQL(t)
		defer db.Close()
		
		// Check Performance Schema has data
		var waitCount int
		err := db.QueryRow(`
			SELECT COUNT(*) 
			FROM performance_schema.events_waits_history_long 
			WHERE EVENT_NAME NOT LIKE 'idle%'
		`).Scan(&waitCount)
		require.NoError(t, err)
		
		t.Logf("Wait events recorded: %d", waitCount)
		assert.Greater(t, waitCount, 0, "Should have wait events")
		
		// Check statement digests
		var digestCount int
		err = db.QueryRow(`
			SELECT COUNT(DISTINCT DIGEST) 
			FROM performance_schema.events_statements_summary_by_digest
			WHERE DIGEST IS NOT NULL
		`).Scan(&digestCount)
		require.NoError(t, err)
		
		t.Logf("Unique query digests: %d", digestCount)
		assert.Greater(t, digestCount, 5, "Should have multiple query patterns")
	})
	
	t.Run("Query_NewRelic_For_Data", func(t *testing.T) {
		nrClient := NewNewRelicClient(t)
		
		// Wait a bit for data to reach New Relic
		t.Log("Waiting 30s for data to reach New Relic...")
		time.Sleep(30 * time.Second)
		
		// Query for any MySQL metrics
		nrql := `SELECT count(*) as metric_count 
		         FROM Metric 
		         WHERE metricName LIKE 'mysql%' 
		         SINCE 5 minutes ago`
		
		results, err := nrClient.QueryNRQL(nrql)
		require.NoError(t, err)
		
		if len(results) > 0 {
			count := results[0]["metric_count"].(float64)
			t.Logf("MySQL metrics in New Relic: %.0f", count)
			
			if count > 0 {
				// Query specific wait metrics
				waitNrql := `SELECT count(*) as wait_count 
				             FROM Metric 
				             WHERE metricName = 'mysql.query.wait_profile' 
				             SINCE 5 minutes ago`
				
				waitResults, _ := nrClient.QueryNRQL(waitNrql)
				if len(waitResults) > 0 {
					waitCount := waitResults[0]["wait_count"].(float64)
					t.Logf("Wait profile metrics: %.0f", waitCount)
				}
			}
		}
	})
}

