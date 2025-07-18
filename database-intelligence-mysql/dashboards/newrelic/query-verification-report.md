# NRQL Query Verification Report

**Generated:** Fri Jul 18 19:50:55 IST 2025  
**Account ID:** 3630072

## Summary

- **Total Queries Tested:** 67
- **Successful Queries:** 5 (7.5%)
- **Failed Queries:** 46 (68.7%)
- **Queries with No Data:** 16 (23.9%)

## Query Results

### Successful Queries
- **System:Data Availability Test**: Success (1 results)
- **System:Data Availability Test**: Success (1 results)
- **System:Data Availability Test**: Success (1 results)
- **MySQL Wait-Based Performance Analysis - Enhanced:Lock Wait Timeline**: Success (120 results)
- **MySQL Wait-Based Performance Analysis - Enhanced:Deadlock Detection**: Success (1 results)
- **MySQL Wait-Based Performance Analysis - Enhanced:Lock Wait Severity Heatmap**: Success (1 results)
- **MySQL Wait-Based Performance Analysis - Enhanced:Saturation Metrics**: Success (1 results)
- **MySQL Wait-Based Performance Analysis - Enhanced:P1 Advisory Summary**: Success (1 results)

### Queries with Warnings
- **MySQL Wait-Based Performance Analysis - Enhanced:Real-Time Wait Analysis**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Category Distribution**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Severity Analysis**: No data (Messages: The behavior of count(*) has changed and will now count the metric's 'count' value for Summary, Distribution, Gauge, and Timeslice metrics. More info here[https://docs.newrelic.com/whats-new/2025/03/whats-new-03-07-change-to-dimensional].)
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Impact by Schema**: No data (Messages: The behavior of count(*) has changed and will now count the metric's 'count' value for Summary, Distribution, Gauge, and Timeslice metrics. More info here[https://docs.newrelic.com/whats-new/2025/03/whats-new-03-07-change-to-dimensional].)
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Distribution Heatmap**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Real-Time Blocking Sessions**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Lock Type Analysis**: No data (Messages: The behavior of count(*) has changed and will now count the metric's 'count' value for Summary, Distribution, Gauge, and Timeslice metrics. More info here[https://docs.newrelic.com/whats-new/2025/03/whats-new-03-07-change-to-dimensional].)
- **MySQL Wait-Based Performance Analysis - Enhanced:Missing Index Impact Analysis**: No data (Messages: The behavior of count(*) has changed and will now count the metric's 'count' value for Summary, Distribution, Gauge, and Timeslice metrics. More info here[https://docs.newrelic.com/whats-new/2025/03/whats-new-03-07-change-to-dimensional].)
- **MySQL Wait-Based Performance Analysis - Enhanced:Query Optimization Opportunities**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Time vs System Resources Correlation**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Resource Bottleneck Detection**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Performance Anomaly Detection**: No data (Messages: The behavior of count(*) has changed and will now count the metric's 'count' value for Summary, Distribution, Gauge, and Timeslice metrics. More info here[https://docs.newrelic.com/whats-new/2025/03/whats-new-03-07-change-to-dimensional].)
- **MySQL Wait-Based Performance Analysis - Enhanced:Service Health Score Trend**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Capacity Exhaustion Forecast**: No data returned
- **MySQL Wait-Based Performance Analysis - Enhanced:Query Pattern Evolution**: No data (Messages: The behavior of count(*) has changed and will now count the metric's 'count' value for Summary, Distribution, Gauge, and Timeslice metrics. More info here[https://docs.newrelic.com/whats-new/2025/03/whats-new-03-07-change-to-dimensional].)
- **MySQL Wait-Based Performance Analysis - Enhanced:Predicted Resource Bottlenecks**: No data returned

### Failed Queries
- **MySQL Wait-Based Performance Analysis - Enhanced:Performance Advisory Dashboard**: NRQL Syntax Error: Error at line 1 position 315, perhaps unbalanced (
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Time Anomaly Detection**: Unknown function baseline()
- **MySQL Wait-Based Performance Analysis - Enhanced:Anomaly Alert Status**: NRQL Syntax Error: Error at line 1 position 17, unexpected 'filter'
- **MySQL Wait-Based Performance Analysis - Enhanced:Top Queries by Total Wait Time**: NRQL Syntax Error: Error at line 1 position 393, perhaps unbalanced (
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Time Trends for Top Queries**: NRQL Syntax Error: Error at line 1 position 172, perhaps unbalanced (
- **MySQL Wait-Based Performance Analysis - Enhanced:Query Performance Regression**: Unknown function baseline()
- **MySQL Wait-Based Performance Analysis - Enhanced:Blocking Chain Analysis**: NRQL Syntax Error: Error at line 1 position 66, unexpected 'latest(blocking_thread) as 'Root Blocker', count(DISTINCT waiting_thread'
- **MySQL Wait-Based Performance Analysis - Enhanced:Composite Advisory Analysis**: NRQL Syntax Error: Error at line 1 position 381, perhaps unbalanced (
- **MySQL Wait-Based Performance Analysis - Enhanced:Query Plan Analysis**: NRQL Syntax Error: Error at line 1 position 466, perhaps unbalanced (
- **MySQL Wait-Based Performance Analysis - Enhanced:Index Effectiveness Score**: NRQL Syntax Error: Error at line 1 position 47, unexpected '(100 - ((sum(mysql.query.wait_profile) filter'
- **MySQL Wait-Based Performance Analysis - Enhanced:I/O Saturation Analysis**: NRQL Syntax Error: Error at line 1 position 38, unexpected 'filter'
- **MySQL Wait-Based Performance Analysis - Enhanced:Lock Wait Storm Detection**: NRQL Syntax Error: Error at line 1 position 54, unexpected 'filter'
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Correlation Matrix**: Unknown function corr()
- **MySQL Wait-Based Performance Analysis - Enhanced:SLI Impacting Queries**: NRQL Syntax Error: Error at line 1 position 341, perhaps unbalanced (
- **MySQL Wait-Based Performance Analysis - Enhanced:Service Impact Timeline**: NRQL Syntax Error: Error at line 1 position 58, unexpected 'filter'
- **MySQL Wait-Based Performance Analysis - Enhanced:Business Impact Summary**: NRQL Syntax Error: Error at line 1 position 80, unexpected 'filter'
- **MySQL Wait-Based Performance Analysis - Enhanced:Wait Time Prediction (Next 4 Hours)**: NRQL Syntax Error: Error at line 1 position 183, unexpected 'from'
- **MySQL Wait-Based Performance Analysis - Enhanced:Performance Trend Analysis**: Unknown function movingAverage()
- **MySQL Query Detail Analysis - Enhanced:Query Execution Overview**: NRQL Syntax Error: Error at line 1 position 259, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Query Wait Profile Over Time**: NRQL Syntax Error: Error at line 1 position 230, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Wait Type Breakdown**: NRQL Syntax Error: Error at line 1 position 86, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Execution Statistics**: NRQL Syntax Error: Error at line 1 position 363, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Active Advisories & Recommendations**: NRQL Syntax Error: Error at line 1 position 320, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Execution Time Trend Analysis**: NRQL Syntax Error: Error at line 1 position 270, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Execution Count & Pattern**: NRQL Syntax Error: Error at line 1 position 66, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Performance Regression Detection**: NRQL Syntax Error: Error at line 1 position 290, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Query Plan Evolution**: NRQL Syntax Error: Error at line 1 position 299, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Resource Usage Trend**: NRQL Syntax Error: Error at line 1 position 159, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Wait Category Distribution Over Time**: NRQL Syntax Error: Error at line 1 position 90, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Wait Severity Heatmap**: NRQL Syntax Error: Error at line 1 position 75, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Wait Event Details**: NRQL Syntax Error: Error at line 1 position 223, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Lock Wait Analysis**: NRQL Syntax Error: Error at line 1 position 265, unexpected '%'
- **MySQL Query Detail Analysis - Enhanced:Wait Time Correlation with Load**: NRQL Syntax Error: Error at line 1 position 127, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Current Execution Plan**: NRQL Syntax Error: Error at line 1 position 353, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Index Usage Analysis**: NRQL Syntax Error: Error at line 1 position 67, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Access Type Distribution**: NRQL Syntax Error: Error at line 1 position 61, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Query Optimization Opportunities**: NRQL Syntax Error: Error at line 1 position 398, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Performance by Instance**: NRQL Syntax Error: Error at line 1 position 285, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Instance Performance Comparison**: NRQL Syntax Error: Error at line 1 position 92, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Resource Usage by Instance**: NRQL Syntax Error: Error at line 1 position 154, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Cache Performance by Instance**: NRQL Syntax Error: Error at line 1 position 238, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Service Impact Summary**: NRQL Syntax Error: Error at line 1 position 185, unexpected 'filter'
- **MySQL Query Detail Analysis - Enhanced:Downstream Service Impact**: NRQL Syntax Error: Error at line 1 position 256, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:User Experience Impact**: NRQL Syntax Error: Error at line 1 position 63, unexpected 'filter'
- **MySQL Query Detail Analysis - Enhanced:Cost Impact Analysis**: NRQL Syntax Error: Error at line 1 position 150, unexpected 'sample_hash'
- **MySQL Query Detail Analysis - Enhanced:Optimization ROI**: NRQL Syntax Error: Error at line 1 position 312, unexpected 'sample_hash'

## Recommendations

### For Queries with No Data:
1. Ensure the OpenTelemetry collector is sending data with the expected attributes
2. Verify that the metric names and attribute names match your collector configuration
3. Check if data exists for the specified time range
4. Confirm that the instrumentation.provider attribute is set to 'opentelemetry'

### For Failed Queries:
1. Check the NRQL syntax for errors
2. Verify that all referenced attributes exist in your data
3. Ensure function calls are properly formatted
4. Check for any deprecated NRQL features
