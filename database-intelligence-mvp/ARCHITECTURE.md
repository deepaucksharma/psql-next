# Technical Architecture

## Overview

This document provides a deep dive into the technical architecture of the Database Intelligence MVP. The project follows a safety-first, iterative approach, beginning with a robust, production-ready collector that uses standard OpenTelemetry components and evolving toward a more advanced solution with custom-built processors.

## v1.0.0: Production Architecture

The current production release (v1.0.0) provides a highly available and scalable solution for collecting database metadata.

### Core Design Principles

- **Safety and Stability**: The collector is built on standard, battle-tested OpenTelemetry components. There is no custom Go code in the critical data path for the production release, which minimizes risk and ensures stability.
- **High Availability**: The architecture is designed for horizontal scalability. It uses a leader election mechanism in Kubernetes to ensure that only one collector instance is actively querying databases at a time, preventing data duplication and unnecessary load.
- **Metadata, Not Plans**: To ensure safety and avoid performance degradation on production databases, the collector **does not** collect full query execution plans. Instead, it gathers query metadata and performance metrics from `pg_stat_statements` and Performance Schema.

### Production Data Flow

```
┌─────────────────┐      ┌─────────────────┐
│   PostgreSQL    │      │      MySQL      │
│  (Read Replica) │      │ (Read Replica)  │
└────────┬────────┘      └────────┬────────┘
         │                        │
         │      SQL Queries       │
         ▼                        ▼
┌──────────────────────────────────────────┐
│     OpenTelemetry Collector (HA)         │
│  - Leader election for active collection │
│  - Multiple replicas for availability    │
├──────────────────────────────────────────┤
│ ┌──────────────────────────────────────┐ │
│ │      Standard `sqlquery` Receiver    │ │
│ │   - Safety timeouts (e.g., 3000ms)   │ │
│ │   - Connection pooling (e.g., 2 max) │ │
│ └──────────────────────────────────────┘ │
│                    │                     │
│                    ▼                     │
│ ┌──────────────────────────────────────┐ │
│ │          `memory_limiter`            │ │
│ │   - Prevents Out-of-Memory errors    │ │
│ └──────────────────────────────────────┘ │
│                    │                     │
│                    ▼                     │
│ ┌──────────────────────────────────────┐ │
│ │      `transform/sanitize_pii`        │ │
│ │   - PII sanitization with regex      │ │
│ └──────────────────────────────────────┘ │
│                    │                     │
│                    ▼                     │
│ ┌──────────────────────────────────────┐ │
│ │      `probabilistic_sampler`         │ │
│ │   - Reduces data volume (e.g., 25%)  │ │
│ └──────────────────────────────────────┘ │
│                    │                     │
│                    ▼                     │
│ ┌──────────────────────────────────────┐ │
│ │              `batch`                 │ │
│ │   - Optimizes network efficiency     │ │
│ └──────────────────────────────────────┘ │
│                    │                     │
│                    ▼                     │
│ ┌──────────────────────────────────────┐ │
│ │          `otlp` Exporter             │ │
│ │   - Securely sends data to New Relic │ │
│ └──────────────────────────────────────┘ │
└──────────────────────────────────────────┘
                 │
                 ▼
          ┌──────────────┐
          │  New Relic   │
          └──────────────┘
```

### State Management

In the v1.0.0 HA architecture, state management is simplified:

- **Stateless Processors**: The processors in the pipeline (sampler, batch, etc.) are stateless. This allows for safe horizontal scaling, as any collector instance can process any data.
- **Leader Election**: The `leader_election` extension for Kubernetes ensures that only one collector instance is the "leader" at any given time. The leader is responsible for executing the `sqlquery` receiver, which prevents multiple collectors from querying the same database simultaneously.
- **No `file_storage` for State**: Unlike the earlier MVP, the HA architecture does not rely on `file_storage` for processor state, which was the primary cause of the single-instance constraint.

### Deployment

The recommended deployment pattern for the production architecture is the Kubernetes `Deployment` with multiple replicas, as defined in `deploy/k8s/ha-deployment.yaml`. This provides both high availability and scalability.

```yaml
# Recommended HA Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db-intelligence-collector
spec:
  replicas: 3 # Multiple replicas for HA
  ...
```

## Future Vision: Advanced Capabilities

The project includes experimental, custom-built Go components that represent the future vision for Database Intelligence. These components are **not enabled** in the v1.0.0 production configuration but are under active development.

### Custom Components (Not in Production Pipeline)

- **`planattributeextractor`**: A processor to parse detailed attributes from JSON execution plans.
- **`adaptivesampler`**: An intelligent, stateful sampler that can make more sophisticated sampling decisions based on query characteristics.
- **`circuitbreaker`**: A processor to automatically halt data collection from a database that is unhealthy or under duress.
- **`postgresqlquery` receiver**: An advanced receiver with built-in support for Active Session History (ASH) sampling and deeper PostgreSQL integration.

### Visionary Data Flow

This is the target architecture that will be enabled as the custom components are integrated and production-hardened.

```
Database → Advanced Receiver → Memory Limiter → PII Sanitizer → Plan Attribute Extractor → 
                                                                         ↓
                                                                   Entity Synthesis
                                                                         ↓
                                                                   Circuit Breaker
                                                                         ↓
                                                                 Adaptive Sampler
                                                                         ↓
                                                                      Batch → New Relic
                                                                         ↑
                                     └────────────── Redis (External State Store) ───────────────────┘
```

## Security Architecture

The project follows a defense-in-depth security model:

1.  **Network**: The collector is designed to connect to read-replica endpoints only. Kubernetes `NetworkPolicy` objects are provided to restrict traffic to and from the collector.
2.  **Authentication**: Database connections should be made with dedicated, read-only users.
3.  **Authorization**: The principle of least privilege is applied. The monitoring user should only have the minimum necessary permissions to query performance views.
4.  **Data**: PII sanitization is a standard component in the pipeline, redacting sensitive information like emails, SSNs, and credit card numbers from query text.
5.  **Transport**: All data is sent to New Relic over a secure TLS connection.
