# Technical Architecture

## Overview

This document details the technical architecture of the Database Intelligence MVP, focusing on a safety-first, iterative approach using OpenTelemetry components.

## Standard Mode: Production Architecture

This mode provides a highly available and scalable solution for collecting database metadata.

### Core Design Principles

*   **Safety and Stability**: Built on standard, battle-tested OpenTelemetry components with no custom Go code in the critical data path.
*   **High Availability**: Designed for horizontal scalability using Kubernetes leader election to prevent data duplication.
*   **Metadata, Not Plans**: Collects query metadata and performance metrics from `pg_stat_statements` and Performance Schema, avoiding full query execution plans for safety.

### Production Data Flow

```mermaid
graph TB
    subgraph "Databases"
        PG[(PostgreSQL<br/>Read Replica)]
        MySQL[(MySQL<br/>Read Replica)]
    end
    
    subgraph "Standard Mode Collector"
        subgraph "Receivers"
            SQLQuery[SQL Query Receiver<br/>- 5 min intervals<br/>- pg_stat_statements]
        end
        
        subgraph "Processors"
            MemLimit[Memory Limiter<br/>512MB cap]
            Transform[Transform/PII<br/>Sanitization]
            Sampler[Probabilistic<br/>Sampler 25%]
            Batch[Batch<br/>Processor]
        end
        
        subgraph "Exporters"
            OTLP[OTLP Exporter<br/>gRPC + compression]
        end
        
        SQLQuery --> MemLimit
        MemLimit --> Transform
        Transform --> Sampler
        Sampler --> Batch
        Batch --> OTLP
    end
    
    subgraph "High Availability"
        Leader[Leader Election<br/>3 replicas]
    end
    
    PG -.-> SQLQuery
    MySQL -.-> SQLQuery
    Leader -.-> SQLQuery
    OTLP --> NR[New Relic]
    
    classDef database fill:#f9f,stroke:#333,stroke-width:2px
    classDef processor fill:#bbf,stroke:#333,stroke-width:2px
    classDef exporter fill:#bfb,stroke:#333,stroke-width:2px
    classDef ha fill:#fbb,stroke:#333,stroke-width:2px
    
    class PG,MySQL database
    class MemLimit,Transform,Sampler,Batch processor
    class OTLP exporter
    class Leader ha
```

### State Management

*   **Stateless Processors**: Allows safe horizontal scaling.
*   **Leader Election**: Ensures only one collector instance actively queries databases.
*   **No `file_storage` for State**: Avoids single-instance constraints.

### Deployment

Recommended deployment is Kubernetes `Deployment` with multiple replicas (`deploy/k8s/ha-deployment.yaml`).

## Experimental Mode: Advanced Capabilities

This mode includes custom-built Go components for advanced monitoring.

### Custom Components

*   **`planattributeextractor`**: Parses detailed attributes from JSON execution plans.
*   **`adaptivesampler`**: Intelligent, stateful sampler for sophisticated sampling decisions.
*   **`circuitbreaker`**: Automatically halts data collection from unhealthy databases.
*   **`postgresqlquery` receiver**: Advanced receiver with ASH sampling and deeper PostgreSQL integration.

### Experimental Data Flow

```mermaid
graph TB
    subgraph "Databases"
        PG1[(Primary<br/>PostgreSQL)]
        PG2[(Analytics<br/>PostgreSQL)]
        MySQL[(MySQL<br/>Read Replica)]
    end
    
    subgraph "Experimental Mode Collector"
        subgraph "Custom Receivers"
            PGQuery[PostgreSQL Query<br/>Receiver<br/>- Multi-DB support<br/>- Cloud detection]
            ASH[ASH Sampler<br/>- 1 sec intervals<br/>- Ring buffer]
            SQLQuery[SQL Query<br/>MySQL fallback]
        end
        
        subgraph "Advanced Processors"
            MemLimit[Memory Limiter<br/>2GB cap]
            Circuit[Circuit Breaker<br/>- Failure detection<br/>- Auto-protection]
            Transform[Transform/PII<br/>Sanitization]
            Adaptive[Adaptive Sampler<br/>- Cost-aware<br/>- Error-aware]
            Plan[Plan Extractor<br/>- Regression detection<br/>- Cost analysis]
            Verify[Verification<br/>- Data quality<br/>- Validation]
            Batch[Batch<br/>Processor]
        end
        
        subgraph "Enhanced Exporters"
            OTLP[OTLP Exporter<br/>- Entity synthesis<br/>- Rich metadata]
            Debug[Debug/Logging<br/>Development mode]
        end
        
        PGQuery --> ASH
        ASH --> MemLimit
        SQLQuery --> MemLimit
        MemLimit --> Circuit
        Circuit --> Transform
        Transform --> Plan
        Plan --> Adaptive
        Adaptive --> Verify
        Verify --> Batch
        Batch --> OTLP
        Batch --> Debug
    end
    
    subgraph "State Management"
        State[Memory State<br/>Single instance]
        Redis[(Redis<br/>Future: Multi-instance)]
    end
    
    PG1 -.-> PGQuery
    PG2 -.-> PGQuery
    MySQL -.-> SQLQuery
    Adaptive -.-> State
    State -.-> Redis
    OTLP --> NR[New Relic]
    
    classDef database fill:#f9f,stroke:#333,stroke-width:2px
    classDef receiver fill:#ff9,stroke:#333,stroke-width:2px
    classDef processor fill:#bbf,stroke:#333,stroke-width:2px
    classDef exporter fill:#bfb,stroke:#333,stroke-width:2px
    classDef state fill:#fbb,stroke:#333,stroke-width:2px
    
    class PG1,PG2,MySQL database
    class PGQuery,ASH,SQLQuery receiver
    class MemLimit,Circuit,Transform,Adaptive,Plan,Verify,Batch processor
    class OTLP,Debug exporter
    class State,Redis state
```

## Security Architecture

The project employs a defense-in-depth security model:

1.  **Network**: Connects to read-replica endpoints only; uses Kubernetes `NetworkPolicy` to restrict traffic.
2.  **Authentication**: Uses dedicated, read-only database users.
3.  **Authorization**: Applies the principle of least privilege.
4.  **Data**: PII sanitization redacts sensitive information from query text.
5.  **Transport**: All data is sent to New Relic over a secure TLS connection.