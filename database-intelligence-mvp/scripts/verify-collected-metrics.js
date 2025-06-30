#!/usr/bin/env node

/**
 * Verify Collected Metrics Script
 * Checks what metrics are actually being collected in New Relic
 * Run this before creating the dashboard to ensure queries will work
 */

require('dotenv').config();
const https = require('https');

// Configuration
const NEW_RELIC_USER_KEY = process.env.NEW_RELIC_USER_KEY;
const NEW_RELIC_ACCOUNT_ID = process.env.NEW_RELIC_ACCOUNT_ID || '3510613';

if (!NEW_RELIC_USER_KEY) {
  console.error('Error: NEW_RELIC_USER_KEY not found in environment variables');
  process.exit(1);
}

// NRQL queries to verify different metric types
const verificationQueries = [
  {
    name: "PostgreSQL Metrics",
    query: `SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgresql.%' SINCE 1 hour ago LIMIT MAX`
  },
  {
    name: "MySQL Metrics", 
    query: `SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'mysql.%' SINCE 1 hour ago LIMIT MAX`
  },
  {
    name: "Database Query Logs",
    query: `FROM Log SELECT count(*) WHERE query_id IS NOT NULL AND collector.name = 'otelcol' FACET database_name SINCE 1 hour ago`
  },
  {
    name: "Available Dimensions",
    query: `SELECT uniques(dimensions) FROM Metric WHERE metricName LIKE 'postgresql.%' OR metricName LIKE 'mysql.%' SINCE 1 hour ago LIMIT 1`
  },
  {
    name: "Sample PostgreSQL Data",
    query: `SELECT latest(postgresql.backends), latest(postgresql.commits), latest(postgresql.database.disk_usage) FROM Metric WHERE metricName LIKE 'postgresql.%' SINCE 1 hour ago`
  },
  {
    name: "Sample MySQL Data",
    query: `SELECT latest(mysql.threads), latest(mysql.uptime), latest(mysql.buffer_pool.limit) FROM Metric WHERE metricName LIKE 'mysql.%' SINCE 1 hour ago`
  },
  {
    name: "Query Log Sample",
    query: `FROM Log SELECT query_id, query_text, avg_duration_ms, execution_count WHERE query_id IS NOT NULL LIMIT 5`
  }
];

// GraphQL query for NRQL
const nrqlQuery = `
query($accountId: Int!, $nrql: Nrql!) {
  actor {
    account(id: $accountId) {
      nrql(query: $nrql) {
        results
      }
    }
  }
}
`;

// Function to make GraphQL request
function makeGraphQLRequest(query, variables) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify({
      query: query,
      variables: variables
    });

    const options = {
      hostname: 'api.newrelic.com',
      path: '/graphql',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'API-Key': NEW_RELIC_USER_KEY,
        'Content-Length': data.length
      }
    };

    const req = https.request(options, (res) => {
      let responseData = '';

      res.on('data', (chunk) => {
        responseData += chunk;
      });

      res.on('end', () => {
        try {
          const parsed = JSON.parse(responseData);
          if (parsed.errors) {
            reject(new Error(JSON.stringify(parsed.errors)));
          } else {
            resolve(parsed);
          }
        } catch (e) {
          reject(e);
        }
      });
    });

    req.on('error', (error) => {
      reject(error);
    });

    req.write(data);
    req.end();
  });
}

// Function to run a single verification query
async function runVerificationQuery(queryInfo) {
  try {
    const result = await makeGraphQLRequest(nrqlQuery, {
      accountId: parseInt(NEW_RELIC_ACCOUNT_ID),
      nrql: queryInfo.query
    });

    console.log(`\n=== ${queryInfo.name} ===`);
    
    if (result.data && result.data.actor && result.data.actor.account && result.data.actor.account.nrql) {
      const results = result.data.actor.account.nrql.results;
      
      if (results && results.length > 0) {
        // Pretty print results
        console.log(JSON.stringify(results, null, 2));
        return { name: queryInfo.name, success: true, count: results.length };
      } else {
        console.log('No data found');
        return { name: queryInfo.name, success: false, count: 0 };
      }
    }
  } catch (error) {
    console.error(`Error running query: ${error.message}`);
    return { name: queryInfo.name, success: false, error: error.message };
  }
}

// Main function
async function verifyMetrics() {
  console.log('Verifying Database Intelligence Metrics Collection...');
  console.log(`Account ID: ${NEW_RELIC_ACCOUNT_ID}`);
  console.log(`Checking data from the last hour...\n`);

  const results = [];
  
  for (const query of verificationQueries) {
    const result = await runVerificationQuery(query);
    results.push(result);
    
    // Add a small delay between queries to avoid rate limiting
    await new Promise(resolve => setTimeout(resolve, 500));
  }

  // Summary
  console.log('\n=== VERIFICATION SUMMARY ===');
  console.log('Query Results:');
  results.forEach(result => {
    const status = result.success ? '✅' : '❌';
    const detail = result.error ? ` (${result.error})` : result.count > 0 ? ` (${result.count} results)` : '';
    console.log(`${status} ${result.name}${detail}`);
  });

  // Recommendations
  console.log('\n=== RECOMMENDATIONS ===');
  
  const hasPostgresMetrics = results.find(r => r.name === 'PostgreSQL Metrics' && r.success);
  const hasMySQLMetrics = results.find(r => r.name === 'MySQL Metrics' && r.success);
  const hasQueryLogs = results.find(r => r.name === 'Database Query Logs' && r.success);

  if (!hasPostgresMetrics && !hasMySQLMetrics && !hasQueryLogs) {
    console.log('❌ No database metrics found. Please ensure:');
    console.log('   1. The OTEL collector is running');
    console.log('   2. Database receivers are properly configured');
    console.log('   3. The collector can connect to your databases');
    console.log('   4. Data is being exported to New Relic');
  } else {
    console.log('✅ Found database metrics! You can proceed with dashboard creation.');
    
    if (!hasPostgresMetrics) {
      console.log('⚠️  No PostgreSQL metrics found - PostgreSQL widgets may show no data');
    }
    if (!hasMySQLMetrics) {
      console.log('⚠️  No MySQL metrics found - MySQL widgets may show no data');
    }
    if (!hasQueryLogs) {
      console.log('⚠️  No query logs found - Query analysis widgets may show no data');
    }
  }

  console.log('\nTo create the dashboard, run: node scripts/create-database-dashboard.js');
}

// Run the script
verifyMetrics();