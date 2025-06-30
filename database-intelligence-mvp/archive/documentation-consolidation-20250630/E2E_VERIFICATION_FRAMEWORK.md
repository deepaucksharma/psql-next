
# E2E Verification Framework: Database Intelligence Collector

## 1. Executive Summary

This document outlines a comprehensive, multi-layered End-to-End (E2E) Verification Framework for the Database Intelligence Collector. The objective is to move beyond basic functionality checks and establish a rigorous, data-driven validation process that guarantees production readiness, data accuracy, and deep integration with the New Relic platform.

The framework is built on the principle of **"Trust, but Verify with Data,"** ensuring that every component and data point is validated from its source in the database to its final representation in New Relic dashboards and alerts. It addresses critical OpenTelemetry and New Relic integration nuances, including silent data-drop failures, entity synthesis, cardinality management, and the specific behavior of the project's custom processors.

## 2. Verification Philosophy

Our E2E verification is not a single event but a continuous process divided into five distinct phases. Each phase builds upon the last, creating layers of confidence. We will treat the collector as a "black box" and validate its behavior by observing its inputs (database state) and its outputs (local metrics and New Relic data).

## 3. The Verification Toolkit

To execute this plan, the following tools and techniques will be employed:

*   **Test Environment**: **Testcontainers** for ephemeral, consistent PostgreSQL and MySQL instances.
*   **Configuration**: A "golden" `collector-test.yaml` configuration file used across all tests.
*   **Workload Generation**: `pgbench` (PostgreSQL) and custom Go scripts (MySQL) to generate specific, repeatable query patterns.
*   **Ground Truth Analysis**: Direct `psql` and `mysql` client queries to establish a baseline for data accuracy.
*   **New Relic Validation**: **NRQL** queries for data verification and **NerdGraph** for entity and relationship validation.
*   **Performance Testing**: `k6` or similar tools for load and stress testing.
*   **Automation**: All verification steps will be codified into shell scripts and Go tests within the `/tests` directory.

---

## 🧪 Phase 0: Foundational Test Environment

**Goal**: Establish a pristine, repeatable, and automated test environment.

| Step | Action | Success Criteria |
| :--- | :--- | :--- |
| **1. Automated Provisioning** | Implement Testcontainers to spin up clean PostgreSQL and MySQL instances for each test run. | Tests can be run from a clean state with a single command (`task test:e2e`). |
| **2. Database Seeding** | Create a standard SQL script (`tests/e2e/sql/seed.sql`) to populate databases with a known dataset, including tables designed to trigger specific query patterns (e.g., full table scans, index usage, complex joins). | The database schema and data are identical before every test run, ensuring repeatable results. |
| **3. Golden Configuration** | Define a `collector-test.yaml` that includes a `debug` exporter alongside the `otlp` exporter. This allows local inspection of the exact data being sent to New Relic. | All E2E tests use this single, version-controlled configuration file as their baseline. |

---

## 🧪 Phase 1: Data Pipeline Integrity & New Relic Handshake

**Goal**: Confirm that data is flowing reliably from the collector to New Relic and that the platform is accepting it without silent failures.

| Step | Action | Verification Method & Success Criteria |
| :--- | :--- | :--- |
| **1. The "Silent Failure" Test** | Generate a small, valid workload. Monitor collector logs for successful OTLP exports. | **NRQL Query**: `SELECT count(*) FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' AND message LIKE '%database%' SINCE 30 minutes ago`. <br/> **Success**: Query returns `0`. This is the most critical handshake test. |
| **2. Data Freshness Validation** | After the workload, query New Relic for the latest metric timestamp. | **NRQL Query**: `SELECT latest(timestamp) FROM Metric WHERE collector.name = 'database-intelligence' SINCE 10 minutes ago`. <br/> **Success**: Timestamp is within the last 5 minutes. |
| **3. Collector Health Telemetry** | Verify that the collector's own internal health metrics (`otelcol_*`) are being exported and ingested. | **NRQL Query**: `SELECT average(otelcol_process_memory_rss) FROM Metric SINCE 10 minutes ago`. <br/> **Success**: Query returns a non-zero value. |
| **4. Data Type & Schema Test** | Intentionally send a metric with an incorrect data type (e.g., a string value for a numeric metric). | **NRQL Query**: `SELECT count(*) FROM NrIntegrationError WHERE message LIKE '%Invalid type%' SINCE 10 minutes ago`. <br/> **Success**: Query returns `> 0`, proving that schema validation errors are caught and reported. |

---

## 🧪 Phase 2: Data Accuracy & OHI Parity Validation

**Goal**: Prove that the metrics in New Relic are an accurate representation of the database's state and provide the same level of insight as the legacy OHI.

| Step | Action | Verification Method & Success Criteria |
| :--- | :--- | :--- |
| **1. Ground Truth Comparison** | Run a specific query (e.g., `UPDATE users SET...`) 10 times against the test DB. Query `pg_stat_statements` directly to get the `calls` count. | **NRQL Query**: `SELECT latest(db.query.calls) FROM Metric WHERE db.query.id = '...'`. <br/> **Success**: The `calls` count in New Relic exactly matches the count from `pg_stat_statements`. Repeat for `total_exec_time` and `rows`. |
| **2. OHI Semantic Parity** | Create a mapping of the Top 10 OHI use cases (e.g., "Find slowest queries") to their equivalent NRQL queries. | **Automated Test**: A Go test iterates through the mapping, executes the NRQL query, and asserts that a valid, non-empty result is returned. <br/> **Success**: All 10 use cases can be successfully replicated with NRQL. |
| **3. Dimensional Correctness** | Generate a workload that hits two different databases (`db1`, `db2`) from the same collector. | **NRQL Query**: `SELECT uniqueCount(db.name) FROM Metric WHERE metricName = 'postgresql.connections.active'`. <br/> **Success**: Query returns `2`. Run similar queries for `db.system`, `service.name`, etc. |

---

## 🧪 Phase 3: New Relic Platform Integration Validation

**Goal**: Ensure data is not just present but is fully functional within the New Relic platform (e.g., entity correlation, dashboards).

| Step | Action | Verification Method & Success Criteria |
| :--- | :--- | :--- |
| **1. Entity Synthesis** | Send metrics from a test host for a test database. | **NerdGraph Query**: Search for a `DATABASE` entity with `tags.host.name = 'your-test-host'`. <br/> **Success**: The entity is found via NerdGraph, and navigating to its GUID in the UI shows the correct metrics and relationships to the host entity. |
| **2. Dashboard Validation** | Create a test dashboard from `monitoring/newrelic/dashboards/database-intelligence-overview.json`. | **Manual & Automated UI Check**: All widgets on the dashboard populate with data and show non-zero values. No "No data" messages are visible. |
| **3. Alerting Validation** | Create a test NRQL alert condition (e.g., `SELECT count(*) FROM Metric WHERE collector.name = 'database-intelligence'`) with a threshold of `> 0`. | **Automated Check**: The alert condition should evaluate to `true` and turn green. Then, stop the collector and verify that the condition violates and an incident is opened. |

---

## 🧪 Phase 4: Advanced Processor Validation

**Goal**: Prove that the custom processors behave correctly under controlled, scenario-based tests.

| Step | Action | Verification Method & Success Criteria |
| :--- | :--- | :--- |
| **1. Adaptive Sampler Test** | Generate a workload of 95% fast queries (<10ms) and 5% slow queries (>1s). | **NRQL Query**: `SELECT percentage(count(*), WHERE db.query.mean_duration > 1000) as '% Slow Sampled', percentage(count(*), WHERE db.query.mean_duration < 10) as '% Fast Sampled' FROM Metric`. <br/> **Success**: The sampling rate for slow queries is >90%, while the rate for fast queries is <20%. |
| **2. Circuit Breaker Test** | 1. Run normal workload. 2. Stop the test database container. 3. Wait 1 minute. 4. Restart the database container. | **NRQL Query**: `SELECT latest(circuit_breaker_state) FROM Metric FACET db.name TIMESERIES`. <br/> **Success**: The metric shows the state transitioning from `closed` -> `open` -> `half-open` -> `closed`. |
| **3. Plan Extractor Test** | Run a query known to cause a sequential scan. | **NRQL Query**: `SELECT latest(db.query.plan.has_seq_scan) FROM Metric WHERE db.query.id = '...'`. <br/> **Success**: The query returns `true`. |
| **4. Verification Processor Test** | Intentionally send a log containing a fake email address. | **NRQL Query**: `SELECT count(*) FROM Log WHERE feedback.category = 'pii_detection'`. <br/> **Success**: The query returns `> 0`, indicating the PII was detected and a feedback event was generated. |

---

## 🧪 Phase 5: Production Readiness & Scalability Validation

**Goal**: Ensure the solution is robust, secure, and performs well under pressure.

| Step | Action | Verification Method & Success Criteria |
| :--- | :--- | :--- |
| **1. Cardinality Stress Test** | Generate queries with high-cardinality attributes (e.g., UUIDs in `WHERE` clauses). | **NRQL Query**: `SELECT count(*) FROM NrIntegrationError WHERE message LIKE '%cardinality%'`. <br/> **Success**: The query returns `0`. The collector's normalization and grouping processors successfully control cardinality. |
| **2. Load & Soak Test** | Use `k6` or `pgbench` to run a sustained, high-volume workload for 1-2 hours. | **Monitoring**: Track collector CPU/memory (`otelcol_process_*`), exporter latency (`otelcol_exporter_send_duration_milliseconds`), and queue size (`otelcol_exporter_queue_size`). <br/> **Success**: Collector resources plateau and remain stable. Queue size remains near zero. Database CPU overhead is <5%. |
| **3. HA / Failover Test** | Deploy a 2-replica collector with Redis state management. Kill the leader pod. | **Manual & Automated Check**: The second replica takes over leadership (verified via logs or a status endpoint). There is no data duplication and minimal data loss (<1 minute) during the failover. |

## 6. Conclusion

Executing this comprehensive E2E verification framework will provide unparalleled confidence in the Database Intelligence Collector. It moves beyond simple "is it on?" checks to a deep, data-driven validation of accuracy, performance, reliability, and platform integration, ensuring the final product is truly enterprise-ready.
