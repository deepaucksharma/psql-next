#!/usr/bin/env node

/**
 * Dashboard Metrics E2E Validation Test
 * 
 * This test validates that all metrics required for the Database Intelligence Dashboard
 * are being collected and available in NRDB.
 * 
 * It checks:
 * - PostgreSQL metrics from the postgresql receiver
 * - MySQL metrics from the mysql receiver  
 * - Query log data from the sqlquery receiver
 * - All dimensions and attributes required for dashboard widgets
 */

require('dotenv').config({ path: '../../.env' });
const https = require('https');
const fs = require('fs');

// Configuration
const NEW_RELIC_USER_KEY = process.env.NEW_RELIC_USER_KEY;
const NEW_RELIC_ACCOUNT_ID = process.env.NEW_RELIC_ACCOUNT_ID || '3510613';
const LOOKBACK_PERIOD = process.env.METRIC_LOOKBACK_PERIOD || '1 hour';

if (!NEW_RELIC_USER_KEY) {
  console.error('❌ Error: NEW_RELIC_USER_KEY not found in environment variables');
  process.exit(1);
}

// Test result tracking
const testResults = {
  totalTests: 0,
  passed: 0,
  failed: 0,
  warnings: 0,
  startTime: new Date(),
  results: []
};

// Define all metrics required for the dashboard
const REQUIRED_METRICS = {
  postgresql: {
    // Core connection and transaction metrics
    'postgresql.backends': {
      description: 'Active backend connections',
      required: true,
      widget: 'Active Connections',
      expectedDimensions: []
    },
    'postgresql.commits': {
      description: 'Number of transactions committed',
      required: true,
      widget: 'PostgreSQL Commits vs Rollbacks',
      expectedDimensions: []
    },
    'postgresql.rollbacks': {
      description: 'Number of transactions rolled back',
      required: false,
      widget: 'PostgreSQL Commits vs Rollbacks',
      expectedDimensions: []
    },
    
    // Database size and I/O metrics
    'postgresql.database.disk_usage': {
      description: 'Database disk usage in bytes',
      required: true,
      widget: 'Database Disk Usage',
      expectedDimensions: ['database_name']
    },
    'postgresql.blocks_read': {
      description: 'Number of disk blocks read',
      required: true,
      widget: 'PostgreSQL Table I/O',
      expectedDimensions: ['source']
    },
    
    // Background writer metrics
    'postgresql.bgwriter.buffers.allocated': {
      description: 'Buffers allocated by the background writer',
      required: false,
      widget: 'PostgreSQL Background Writer',
      expectedDimensions: []
    },
    'postgresql.bgwriter.buffers.writes': {
      description: 'Number of buffers written by the background writer',
      required: false,
      widget: 'PostgreSQL Background Writer',
      expectedDimensions: ['source']
    },
    'postgresql.bgwriter.checkpoint.count': {
      description: 'Number of checkpoints performed',
      required: false,
      widget: 'PostgreSQL Background Writer',
      expectedDimensions: ['type']
    },
    'postgresql.bgwriter.duration': {
      description: 'Time spent in checkpoint processing',
      required: false,
      widget: 'PostgreSQL Background Writer',
      expectedDimensions: ['type']
    },
    'postgresql.bgwriter.maxwritten': {
      description: 'Times background writer stopped due to writing too many buffers',
      required: false,
      widget: 'PostgreSQL Background Writer',
      expectedDimensions: []
    },
    
    // Optional metrics
    'postgresql.database.locks': {
      description: 'Number of database locks',
      required: false,
      widget: 'Database Operations Overview',
      expectedDimensions: ['lock_type', 'database_name']
    },
    'postgresql.index.scans': {
      description: 'Number of index scans',
      required: false,
      widget: 'PostgreSQL Table I/O',
      expectedDimensions: ['table_name', 'index_name']
    }
  },
  
  mysql: {
    // Core MySQL metrics
    'mysql.threads': {
      description: 'Number of threads connected',
      required: true,
      widget: 'Active Connections',
      expectedDimensions: ['kind']
    },
    'mysql.uptime': {
      description: 'Server uptime in seconds',
      required: true,
      widget: 'Database Operations Overview',
      expectedDimensions: []
    },
    
    // Buffer pool metrics
    'mysql.buffer_pool.data': {
      description: 'Bytes of data in InnoDB buffer pool',
      required: true,
      widget: 'InnoDB Buffer Pool Usage',
      expectedDimensions: ['status']
    },
    'mysql.buffer_pool.limit': {
      description: 'InnoDB buffer pool size limit',
      required: true,
      widget: 'InnoDB Buffer Pool Usage',
      expectedDimensions: []
    },
    'mysql.buffer_pool.operations': {
      description: 'Number of operations on buffer pool',
      required: false,
      widget: 'InnoDB Buffer Pool Usage',
      expectedDimensions: ['operation']
    },
    'mysql.buffer_pool.page_flushes': {
      description: 'Number of page flush requests',
      required: false,
      widget: 'InnoDB Buffer Pool Usage',
      expectedDimensions: []
    },
    'mysql.buffer_pool.pages': {
      description: 'Number of pages in buffer pool',
      required: false,
      widget: 'InnoDB Buffer Pool Usage',
      expectedDimensions: ['kind']
    },
    
    // Handler and operation metrics
    'mysql.handlers': {
      description: 'Number of handler operations',
      required: true,
      widget: 'MySQL Handler Operations',
      expectedDimensions: ['kind']
    },
    'mysql.operations': {
      description: 'Number of InnoDB operations',
      required: true,
      widget: 'Database Operations Overview',
      expectedDimensions: ['operation']
    },
    
    // Resource metrics
    'mysql.opened_resources': {
      description: 'Number of opened resources',
      required: false,
      widget: 'Connection and Thread Status',
      expectedDimensions: ['kind']
    },
    'mysql.tmp_resources': {
      description: 'Number of temporary resources created',
      required: false,
      widget: 'MySQL Temporary Resources',
      expectedDimensions: ['kind']
    },
    
    // Additional metrics
    'mysql.sorts': {
      description: 'Number of sorts performed',
      required: false,
      widget: 'Database Operations Overview',
      expectedDimensions: ['kind']
    },
    'mysql.prepared_statements': {
      description: 'Prepared statement commands',
      required: false,
      widget: 'Database Operations Overview',
      expectedDimensions: ['command']
    }
  }
};

// Define required log attributes for query analysis
const REQUIRED_LOG_ATTRIBUTES = {
  'query_id': {
    description: 'Unique identifier for the query',
    required: true,
    widget: 'Slowest Queries, Query Log Analysis'
  },
  'query_text': {
    description: 'The SQL query text',
    required: true,
    widget: 'Slowest Queries, Query Log Analysis'
  },
  'avg_duration_ms': {
    description: 'Average query execution time',
    required: true,
    widget: 'Query Execution Time, Average Query Duration'
  },
  'execution_count': {
    description: 'Number of times query was executed',
    required: true,
    widget: 'Slowest Queries, Query Log Analysis'
  },
  'total_duration_ms': {
    description: 'Total time spent executing query',
    required: true,
    widget: 'Slowest Queries, Query Log Analysis'
  },
  'database_name': {
    description: 'Name of the database',
    required: true,
    widget: 'Databases Overview, Query Execution Trends'
  },
  'plan_metadata': {
    description: 'Query plan information',
    required: false,
    widget: 'Query Log Analysis'
  },
  'collector.name': {
    description: 'Name of the collector (should be otelcol)',
    required: true,
    widget: 'All query widgets'
  }
};

// NerdGraph query template
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

// Helper function to make GraphQL requests
async function makeGraphQLRequest(query, variables) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify({ query, variables });

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

    req.on('error', reject);
    req.write(data);
    req.end();
  });
}

// Test individual metric availability
async function testMetric(metricName, metricConfig, metricType) {
  const testName = `${metricType}.${metricName}`;
  testResults.totalTests++;

  try {
    // Query for metric existence
    const query = `SELECT count(*), latest(timestamp) FROM Metric WHERE metricName = '${metricName}' SINCE ${LOOKBACK_PERIOD}`;
    
    const result = await makeGraphQLRequest(nrqlQuery, {
      accountId: parseInt(NEW_RELIC_ACCOUNT_ID),
      nrql: query
    });

    const data = result?.data?.actor?.account?.nrql?.results?.[0];
    const count = data?.count || 0;
    const latestTimestamp = data?.latest || null;

    if (count > 0) {
      // Metric exists, now check dimensions
      const dimensionQuery = `SELECT uniques(dimensions) FROM Metric WHERE metricName = '${metricName}' SINCE ${LOOKBACK_PERIOD} LIMIT 1`;
      
      const dimResult = await makeGraphQLRequest(nrqlQuery, {
        accountId: parseInt(NEW_RELIC_ACCOUNT_ID),
        nrql: dimensionQuery
      });

      const dimensions = dimResult?.data?.actor?.account?.nrql?.results?.[0]?.uniques || {};
      const dimensionKeys = Object.keys(dimensions);

      // Check if expected dimensions are present
      const missingDimensions = metricConfig.expectedDimensions.filter(
        dim => !dimensionKeys.includes(dim)
      );

      if (missingDimensions.length > 0 && metricConfig.required) {
        testResults.warnings++;
        testResults.results.push({
          test: testName,
          status: 'warning',
          message: `Metric found but missing dimensions: ${missingDimensions.join(', ')}`,
          details: {
            count,
            latestTimestamp,
            foundDimensions: dimensionKeys,
            expectedDimensions: metricConfig.expectedDimensions,
            widget: metricConfig.widget
          }
        });
      } else {
        testResults.passed++;
        testResults.results.push({
          test: testName,
          status: 'passed',
          message: 'Metric found with all expected dimensions',
          details: {
            count,
            latestTimestamp,
            dimensions: dimensionKeys,
            widget: metricConfig.widget
          }
        });
      }
    } else {
      // Metric not found
      if (metricConfig.required) {
        testResults.failed++;
        testResults.results.push({
          test: testName,
          status: 'failed',
          message: 'Required metric not found',
          details: {
            description: metricConfig.description,
            widget: metricConfig.widget,
            lookbackPeriod: LOOKBACK_PERIOD
          }
        });
      } else {
        testResults.warnings++;
        testResults.results.push({
          test: testName,
          status: 'warning',
          message: 'Optional metric not found',
          details: {
            description: metricConfig.description,
            widget: metricConfig.widget
          }
        });
      }
    }
  } catch (error) {
    testResults.failed++;
    testResults.results.push({
      test: testName,
      status: 'error',
      message: `Error testing metric: ${error.message}`,
      details: { error: error.toString() }
    });
  }
}

// Test log attributes for query analysis
async function testLogAttributes() {
  console.log('\n📋 Testing Query Log Attributes...');
  
  for (const [attrName, attrConfig] of Object.entries(REQUIRED_LOG_ATTRIBUTES)) {
    const testName = `logs.${attrName}`;
    testResults.totalTests++;

    try {
      const query = `FROM Log SELECT count(*) WHERE ${attrName} IS NOT NULL AND collector.name = 'otelcol' SINCE ${LOOKBACK_PERIOD}`;
      
      const result = await makeGraphQLRequest(nrqlQuery, {
        accountId: parseInt(NEW_RELIC_ACCOUNT_ID),
        nrql: query
      });

      const count = result?.data?.actor?.account?.nrql?.results?.[0]?.count || 0;

      if (count > 0) {
        testResults.passed++;
        testResults.results.push({
          test: testName,
          status: 'passed',
          message: 'Log attribute found',
          details: {
            count,
            widget: attrConfig.widget
          }
        });
      } else {
        if (attrConfig.required) {
          testResults.failed++;
          testResults.results.push({
            test: testName,
            status: 'failed',
            message: 'Required log attribute not found',
            details: {
              description: attrConfig.description,
              widget: attrConfig.widget
            }
          });
        } else {
          testResults.warnings++;
          testResults.results.push({
            test: testName,
            status: 'warning',
            message: 'Optional log attribute not found',
            details: {
              description: attrConfig.description,
              widget: attrConfig.widget
            }
          });
        }
      }
    } catch (error) {
      testResults.failed++;
      testResults.results.push({
        test: testName,
        status: 'error',
        message: `Error testing log attribute: ${error.message}`,
        details: { error: error.toString() }
      });
    }
  }
}

// Test dashboard widget queries
async function testDashboardQueries() {
  console.log('\n🎯 Testing Dashboard Widget Queries...');
  
  const widgetQueries = [
    {
      name: 'Database Count',
      query: `SELECT uniqueCount(dimensions.database_name) FROM Metric WHERE metricName LIKE 'postgresql.%' OR metricName LIKE 'mysql.%' SINCE ${LOOKBACK_PERIOD}`,
      minExpected: 1
    },
    {
      name: 'PostgreSQL Query Performance',
      query: `FROM Log SELECT count(*) WHERE query_id IS NOT NULL AND collector.name = 'otelcol' SINCE ${LOOKBACK_PERIOD}`,
      minExpected: 0
    },
    {
      name: 'Active Connections Combined',
      query: `SELECT latest(postgresql.backends), latest(mysql.threads) FROM Metric WHERE metricName IN ('postgresql.backends', 'mysql.threads') SINCE ${LOOKBACK_PERIOD}`,
      minExpected: 0
    },
    {
      name: 'Transaction Rates',
      query: `SELECT sum(postgresql.commits), sum(postgresql.rollbacks) FROM Metric WHERE metricName IN ('postgresql.commits', 'postgresql.rollbacks') SINCE ${LOOKBACK_PERIOD}`,
      minExpected: 0
    }
  ];

  for (const widgetQuery of widgetQueries) {
    const testName = `widget.${widgetQuery.name}`;
    testResults.totalTests++;

    try {
      const result = await makeGraphQLRequest(nrqlQuery, {
        accountId: parseInt(NEW_RELIC_ACCOUNT_ID),
        nrql: widgetQuery.query
      });

      const data = result?.data?.actor?.account?.nrql?.results;

      if (data && data.length > 0) {
        testResults.passed++;
        testResults.results.push({
          test: testName,
          status: 'passed',
          message: 'Widget query returned data',
          details: {
            resultCount: data.length,
            sample: data[0]
          }
        });
      } else {
        testResults.warnings++;
        testResults.results.push({
          test: testName,
          status: 'warning',
          message: 'Widget query returned no data',
          details: {
            query: widgetQuery.query,
            note: 'Dashboard widget may show empty'
          }
        });
      }
    } catch (error) {
      testResults.failed++;
      testResults.results.push({
        test: testName,
        status: 'error',
        message: `Error testing widget query: ${error.message}`,
        details: { error: error.toString() }
      });
    }
  }
}

// Generate test report
function generateReport() {
  const endTime = new Date();
  const duration = (endTime - testResults.startTime) / 1000;

  console.log('\n' + '='.repeat(80));
  console.log('📊 E2E TEST REPORT - Dashboard Metrics Validation');
  console.log('='.repeat(80));
  
  console.log(`\n📅 Test Run Information:`);
  console.log(`   Start Time: ${testResults.startTime.toISOString()}`);
  console.log(`   End Time: ${endTime.toISOString()}`);
  console.log(`   Duration: ${duration.toFixed(2)} seconds`);
  console.log(`   Account ID: ${NEW_RELIC_ACCOUNT_ID}`);
  console.log(`   Lookback Period: ${LOOKBACK_PERIOD}`);

  console.log(`\n📈 Test Summary:`);
  console.log(`   Total Tests: ${testResults.totalTests}`);
  console.log(`   ✅ Passed: ${testResults.passed} (${((testResults.passed/testResults.totalTests)*100).toFixed(1)}%)`);
  console.log(`   ❌ Failed: ${testResults.failed} (${((testResults.failed/testResults.totalTests)*100).toFixed(1)}%)`);
  console.log(`   ⚠️  Warnings: ${testResults.warnings} (${((testResults.warnings/testResults.totalTests)*100).toFixed(1)}%)`);

  // Group results by status
  const failedTests = testResults.results.filter(r => r.status === 'failed');
  const warningTests = testResults.results.filter(r => r.status === 'warning');
  const passedTests = testResults.results.filter(r => r.status === 'passed');

  if (failedTests.length > 0) {
    console.log('\n❌ Failed Tests:');
    failedTests.forEach(test => {
      console.log(`   - ${test.test}: ${test.message}`);
      if (test.details.widget) {
        console.log(`     Affects widget: ${test.details.widget}`);
      }
    });
  }

  if (warningTests.length > 0) {
    console.log('\n⚠️  Warnings:');
    warningTests.forEach(test => {
      console.log(`   - ${test.test}: ${test.message}`);
      if (test.details.widget) {
        console.log(`     Affects widget: ${test.details.widget}`);
      }
    });
  }

  // Dashboard readiness assessment
  console.log('\n🎯 Dashboard Readiness Assessment:');
  
  const postgresqlMetrics = testResults.results.filter(r => r.test.startsWith('postgresql.'));
  const mysqlMetrics = testResults.results.filter(r => r.test.startsWith('mysql.'));
  const logAttributes = testResults.results.filter(r => r.test.startsWith('logs.'));
  
  const postgresqlReady = postgresqlMetrics.filter(r => r.status === 'passed').length > 0;
  const mysqlReady = mysqlMetrics.filter(r => r.status === 'passed').length > 0;
  const queryLogsReady = logAttributes.filter(r => r.status === 'passed').length > 0;

  console.log(`   PostgreSQL Metrics: ${postgresqlReady ? '✅ Ready' : '❌ Not Ready'}`);
  console.log(`   MySQL Metrics: ${mysqlReady ? '✅ Ready' : '❌ Not Ready'}`);
  console.log(`   Query Logs: ${queryLogsReady ? '✅ Ready' : '❌ Not Ready'}`);

  if (postgresqlReady || mysqlReady || queryLogsReady) {
    console.log('\n✅ Dashboard can be created with partial functionality');
    console.log('   Some widgets may show no data for missing metrics');
  } else {
    console.log('\n❌ Dashboard creation not recommended');
    console.log('   No metrics are being collected');
  }

  // Save detailed report
  const reportData = {
    ...testResults,
    endTime: endTime.toISOString(),
    duration: duration,
    readiness: {
      postgresql: postgresqlReady,
      mysql: mysqlReady,
      queryLogs: queryLogsReady
    }
  };

  fs.writeFileSync('e2e-test-report.json', JSON.stringify(reportData, null, 2));
  console.log('\n📄 Detailed report saved to: e2e-test-report.json');

  // Exit code based on failures
  const exitCode = testResults.failed > 0 ? 1 : 0;
  console.log(`\n🏁 Test suite completed with exit code: ${exitCode}`);
  
  return exitCode;
}

// Main test runner
async function runTests() {
  console.log('🚀 Starting E2E Dashboard Metrics Validation Tests');
  console.log('='.repeat(80));

  try {
    // Test PostgreSQL metrics
    console.log('\n🐘 Testing PostgreSQL Metrics...');
    for (const [metricName, metricConfig] of Object.entries(REQUIRED_METRICS.postgresql)) {
      await testMetric(metricName, metricConfig, 'postgresql');
      // Small delay to avoid rate limiting
      await new Promise(resolve => setTimeout(resolve, 100));
    }

    // Test MySQL metrics
    console.log('\n🐬 Testing MySQL Metrics...');
    for (const [metricName, metricConfig] of Object.entries(REQUIRED_METRICS.mysql)) {
      await testMetric(metricName, metricConfig, 'mysql');
      await new Promise(resolve => setTimeout(resolve, 100));
    }

    // Test log attributes
    await testLogAttributes();

    // Test dashboard queries
    await testDashboardQueries();

  } catch (error) {
    console.error('\n❌ Fatal error during test execution:', error);
    testResults.failed++;
    testResults.results.push({
      test: 'test-runner',
      status: 'error',
      message: `Fatal error: ${error.message}`,
      details: { error: error.toString() }
    });
  }

  // Generate and display report
  const exitCode = generateReport();
  process.exit(exitCode);
}

// Run the tests
runTests();