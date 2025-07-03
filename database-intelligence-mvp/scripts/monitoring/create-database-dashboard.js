#!/usr/bin/env node

/**
 * Database Intelligence Dashboard Creator
 * Creates a New Relic dashboard for database metrics collected via OpenTelemetry
 * 
 * Based on the MySQL OHI dashboard structure but adapted for OTEL metrics
 */

require('dotenv').config();
const fs = require('fs');
const https = require('https');

// Configuration
const NEW_RELIC_USER_KEY = process.env.NEW_RELIC_USER_KEY;
const NEW_RELIC_ACCOUNT_ID = process.env.NEW_RELIC_ACCOUNT_ID || '3510613';

if (!NEW_RELIC_USER_KEY) {
  console.error('Error: NEW_RELIC_USER_KEY not found in environment variables');
  process.exit(1);
}

// Dashboard definition adapted for OTEL metrics
const dashboardDefinition = {
  name: "Database Intelligence - OTEL Metrics",
  description: "Database performance monitoring using OpenTelemetry collector metrics",
  permissions: "PUBLIC_READ_WRITE",
  pages: [
    {
      name: "Bird's-Eye View",
      description: "Overview of database performance metrics",
      widgets: [
        {
          title: "Databases Overview",
          layout: {
            column: 1,
            row: 1,
            width: 2,
            height: 3
          },
          visualization: {
            id: "viz.bar"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT uniqueCount(dimensions.database_name) as 'Database Count' FROM Metric WHERE metricName LIKE 'postgresql.%' OR metricName LIKE 'mysql.%' FACET dimensions.database_name`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "Query Execution Time (PostgreSQL)",
          layout: {
            column: 3,
            row: 1,
            width: 3,
            height: 3
          },
          visualization: {
            id: "viz.line"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            legend: {
              enabled: true
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `FROM Log SELECT average(numeric(avg_duration_ms)) as 'Avg Query Time (ms)' WHERE query_id IS NOT NULL AND collector.name = 'otelcol' TIMESERIES FACET database_name`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            },
            yAxisLeft: {
              zero: true
            }
          }
        },
        {
          title: "Active Connections",
          layout: {
            column: 6,
            row: 1,
            width: 3,
            height: 3
          },
          visualization: {
            id: "viz.line"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            legend: {
              enabled: true
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT latest(postgresql.backends) as 'PostgreSQL Connections', latest(mysql.threads) as 'MySQL Threads' FROM Metric WHERE metricName IN ('postgresql.backends', 'mysql.threads') TIMESERIES`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            },
            yAxisLeft: {
              zero: true
            }
          }
        },
        {
          title: "Database Disk Usage",
          layout: {
            column: 9,
            row: 1,
            width: 3,
            height: 3
          },
          visualization: {
            id: "viz.bar"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT latest(postgresql.database.disk_usage) / 1048576 as 'Disk Usage (MB)' FROM Metric WHERE metricName = 'postgresql.database.disk_usage' FACET dimensions.database_name`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "Slowest Queries (from logs)",
          layout: {
            column: 1,
            row: 4,
            width: 12,
            height: 4
          },
          visualization: {
            id: "viz.table"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `FROM Log SELECT latest(query_text) as 'Query', latest(avg_duration_ms) as 'Avg Time (ms)', latest(execution_count) as 'Execution Count', latest(total_duration_ms) as 'Total Time (ms)' WHERE query_id IS NOT NULL AND collector.name = 'otelcol' FACET query_id LIMIT 20`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "PostgreSQL Commits vs Rollbacks",
          layout: {
            column: 1,
            row: 8,
            width: 6,
            height: 3
          },
          visualization: {
            id: "viz.line"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            legend: {
              enabled: true
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT rate(sum(postgresql.commits), 1 minute) as 'Commits/min', rate(sum(postgresql.rollbacks), 1 minute) as 'Rollbacks/min' FROM Metric WHERE metricName IN ('postgresql.commits', 'postgresql.rollbacks') TIMESERIES`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            },
            yAxisLeft: {
              zero: true
            }
          }
        },
        {
          title: "InnoDB Buffer Pool Usage",
          layout: {
            column: 7,
            row: 8,
            width: 6,
            height: 3
          },
          visualization: {
            id: "viz.line"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            legend: {
              enabled: true
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT latest(mysql.buffer_pool.data) / 1048576 as 'Buffer Pool Data (MB)', latest(mysql.buffer_pool.limit) / 1048576 as 'Buffer Pool Limit (MB)' FROM Metric WHERE metricName LIKE 'mysql.buffer_pool.%' TIMESERIES`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            },
            yAxisLeft: {
              zero: true
            }
          }
        },
        {
          title: "Database Operations Overview",
          layout: {
            column: 1,
            row: 11,
            width: 12,
            height: 3
          },
          visualization: {
            id: "viz.table"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT 
                  rate(sum(postgresql.blocks_read), 1 minute) as 'PG Blocks Read/min',
                  rate(sum(mysql.operations), 1 minute) as 'MySQL Operations/min',
                  latest(mysql.uptime) / 3600 as 'MySQL Uptime (hours)',
                  latest(postgresql.backends) as 'PG Active Connections'
                FROM Metric 
                WHERE metricName IN ('postgresql.blocks_read', 'mysql.operations', 'mysql.uptime', 'postgresql.backends')
                SINCE 1 hour ago`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        }
      ]
    },
    {
      name: "Query Performance",
      description: "Detailed query performance analysis",
      widgets: [
        {
          title: "Query Log Analysis",
          layout: {
            column: 1,
            row: 1,
            width: 12,
            height: 4
          },
          visualization: {
            id: "viz.table"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `FROM Log 
                SELECT 
                  latest(query_id) as 'Query ID',
                  latest(query_text) as 'Query Text',
                  latest(avg_duration_ms) as 'Avg Duration (ms)',
                  latest(execution_count) as 'Executions',
                  latest(total_duration_ms) as 'Total Time (ms)',
                  latest(plan_metadata) as 'Plan Info'
                WHERE query_id IS NOT NULL 
                  AND collector.name = 'otelcol'
                FACET query_id, database_name
                LIMIT 50`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "Query Execution Trends",
          layout: {
            column: 1,
            row: 5,
            width: 6,
            height: 3
          },
          visualization: {
            id: "viz.line"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            legend: {
              enabled: true
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `FROM Log SELECT count(*) as 'Query Count' WHERE query_id IS NOT NULL AND collector.name = 'otelcol' TIMESERIES FACET database_name`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "Average Query Duration by Database",
          layout: {
            column: 7,
            row: 5,
            width: 6,
            height: 3
          },
          visualization: {
            id: "viz.bar"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `FROM Log SELECT average(numeric(avg_duration_ms)) as 'Avg Duration (ms)' WHERE query_id IS NOT NULL AND collector.name = 'otelcol' FACET database_name`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        }
      ]
    },
    {
      name: "Database Resources",
      description: "Resource utilization and performance metrics",
      widgets: [
        {
          title: "PostgreSQL Table I/O",
          layout: {
            column: 1,
            row: 1,
            width: 6,
            height: 3
          },
          visualization: {
            id: "viz.stacked-bar"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            legend: {
              enabled: true
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT sum(postgresql.blocks_read) as 'Blocks Read' FROM Metric WHERE metricName = 'postgresql.blocks_read' FACET dimensions.source TIMESERIES`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "MySQL Handler Operations",
          layout: {
            column: 7,
            row: 1,
            width: 6,
            height: 3
          },
          visualization: {
            id: "viz.line"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            legend: {
              enabled: true
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT rate(sum(mysql.handlers), 1 minute) as 'Handler Ops/min' FROM Metric WHERE metricName = 'mysql.handlers' FACET dimensions.kind TIMESERIES`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "PostgreSQL Background Writer",
          layout: {
            column: 1,
            row: 4,
            width: 6,
            height: 3
          },
          visualization: {
            id: "viz.line"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            legend: {
              enabled: true
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT 
                  rate(sum(postgresql.bgwriter.buffers.allocated), 1 minute) as 'Buffers Allocated/min',
                  rate(sum(postgresql.bgwriter.buffers.writes), 1 minute) as 'Buffer Writes/min'
                FROM Metric 
                WHERE metricName LIKE 'postgresql.bgwriter.%' 
                TIMESERIES`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "MySQL Temporary Resources",
          layout: {
            column: 7,
            row: 4,
            width: 6,
            height: 3
          },
          visualization: {
            id: "viz.bar"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT latest(mysql.tmp_resources) as 'Temp Resources' FROM Metric WHERE metricName = 'mysql.tmp_resources' FACET dimensions.kind`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        },
        {
          title: "Connection and Thread Status",
          layout: {
            column: 1,
            row: 7,
            width: 12,
            height: 3
          },
          visualization: {
            id: "viz.table"
          },
          rawConfiguration: {
            facet: {
              showOtherSeries: false
            },
            nrqlQueries: [
              {
                accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
                query: `SELECT 
                  latest(postgresql.backends) as 'PG Connections',
                  latest(mysql.threads) as 'MySQL Threads',
                  latest(mysql.opened_resources) as 'MySQL Opened Resources'
                FROM Metric 
                WHERE metricName IN ('postgresql.backends', 'mysql.threads', 'mysql.opened_resources')
                FACET dimensions.database_name`
              }
            ],
            platformOptions: {
              ignoreTimeRange: false
            }
          }
        }
      ]
    }
  ],
  variables: []
};

// NerdGraph mutation for creating dashboard
const createDashboardMutation = `
mutation CreateDashboard($accountId: Int!, $dashboard: DashboardInput!) {
  dashboardCreate(accountId: $accountId, dashboard: $dashboard) {
    entityResult {
      guid
      name
      accountId
      createdAt
      updatedAt
      permissions
      pages {
        guid
        name
        widgets {
          id
          title
        }
      }
    }
    errors {
      description
      type
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

// Main function
async function createDashboard() {
  console.log('Creating Database Intelligence Dashboard...');
  console.log(`Account ID: ${NEW_RELIC_ACCOUNT_ID}`);

  try {
    // Create the dashboard
    const result = await makeGraphQLRequest(createDashboardMutation, {
      accountId: parseInt(NEW_RELIC_ACCOUNT_ID),
      dashboard: dashboardDefinition
    });

    if (result.data && result.data.dashboardCreate && result.data.dashboardCreate.entityResult) {
      const dashboard = result.data.dashboardCreate.entityResult;
      console.log('\n✅ Dashboard created successfully!');
      console.log(`Dashboard Name: ${dashboard.name}`);
      console.log(`Dashboard GUID: ${dashboard.guid}`);
      console.log(`Created At: ${dashboard.createdAt}`);
      console.log(`\nView your dashboard at: https://one.newrelic.com/dashboards/${dashboard.guid}`);
      
      // Save dashboard info
      const dashboardInfo = {
        guid: dashboard.guid,
        name: dashboard.name,
        accountId: dashboard.accountId,
        createdAt: dashboard.createdAt,
        url: `https://one.newrelic.com/dashboards/${dashboard.guid}`
      };
      
      fs.writeFileSync('dashboard-info.json', JSON.stringify(dashboardInfo, null, 2));
      console.log('\nDashboard info saved to dashboard-info.json');
    } else if (result.data && result.data.dashboardCreate && result.data.dashboardCreate.errors) {
      console.error('❌ Dashboard creation failed:');
      result.data.dashboardCreate.errors.forEach(error => {
        console.error(`  - ${error.type}: ${error.description}`);
      });
    }
  } catch (error) {
    console.error('❌ Error creating dashboard:', error.message);
    if (error.message.includes('Unauthorized')) {
      console.error('\nPlease ensure your NEW_RELIC_USER_KEY is valid and has dashboard creation permissions.');
    }
  }
}

// Run the script
createDashboard();