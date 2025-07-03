# OHI to OTEL Migration - Gap Analysis Diagram

## Current State vs Required State

```mermaid
graph TB
    subgraph "Current Implementation"
        A[Native OTEL Receivers] --> B[Custom Processors]
        B --> C[OTLP Export to NR]
        
        subgraph "Processors"
            D[AdaptiveSampler]
            E[CircuitBreaker]
            F[PlanExtractor]
            G[Verification]
            H[CostControl]
        end
    end
    
    subgraph "Required for OHI Migration"
        I[OHI + OTEL Receivers] --> J[Translation Layer]
        J --> K[Validation Layer]
        K --> L[Dual Export]
        
        subgraph "Missing Components"
            M[Metric Name Translator]
            N[Entity Correlator]
            O[Real-time Validator]
            P[Rollback Controller]
        end
    end
    
    subgraph "Migration Flow"
        Q[Phase 1: Discovery] -.-> R[Phase 2: Parallel Run]
        R -.-> S[Phase 3: Validation]
        S -.-> T[Phase 4: Cutover]
    end
    
    style M fill:#ff6b6b
    style N fill:#ff6b6b
    style O fill:#ff6b6b
    style P fill:#ff6b6b
```

## Metric Translation Gap

```mermaid
flowchart LR
    subgraph "OHI Metrics"
        A1[mysql.node.net.bytesReceivedPerSecond]
        A2[postgresql.db.connections.active]
        A3[mysql.node.query.questionsPerSecond]
    end
    
    subgraph "Current OTEL Metrics"
        B1[mysql.net.bytes]
        B2[postgresql.connection.count]
        B3[mysql.statement.count]
    end
    
    subgraph "Required Translation"
        C[Metric Transform Processor]
        C --> D[Name Mapping]
        C --> E[Label Extraction]
        C --> F[Unit Conversion]
    end
    
    A1 -.-> C
    A2 -.-> C
    A3 -.-> C
    C -.-> B1
    C -.-> B2
    C -.-> B3
    
    style C fill:#ffd93d
```

## Entity Correlation Gap

```mermaid
flowchart TB
    subgraph "New Relic Entity Model"
        A[Infrastructure Host]
        B[MySQL Instance]
        C[PostgreSQL Instance]
        A --> B
        A --> C
    end
    
    subgraph "Current OTEL Model"
        D[Service: database-intelligence]
        E[MySQL Metrics]
        F[PostgreSQL Metrics]
        D -.-> E
        D -.-> F
    end
    
    subgraph "Required Correlation"
        G[Entity GUID Generator]
        H[Relationship Mapper]
        I[Type Classifier]
    end
    
    style G fill:#ff6b6b
    style H fill:#ff6b6b
    style I fill:#ff6b6b
```

## Validation Framework Gap

```mermaid
sequenceDiagram
    participant OHI as OHI Collector
    participant OTEL as OTEL Collector
    participant VAL as Validator ❌
    participant NR as New Relic
    participant ALERT as Alerting ❌
    
    OHI->>NR: mysql.node.connections (value: 50)
    OTEL->>NR: mysql.connections (value: 48)
    
    Note over VAL: Missing Component:
    Note over VAL: Should compare values
    Note over VAL: and alert on drift
    
    Note over ALERT: Missing Component:
    Note over ALERT: Should trigger alerts
    Note over ALERT: when variance > 5%
```

## Recommended Implementation Priority

```mermaid
gantt
    title OHI Migration Implementation Roadmap
    dateFormat  YYYY-MM-DD
    section P0 - Critical
    Metric Name Translation    :crit, a1, 2024-01-08, 2w
    Entity Correlation         :crit, a2, 2024-01-08, 2w
    Parallel Running Config    :crit, a3, after a1, 1w
    
    section P1 - Important
    Validation Framework       :active, b1, after a3, 2w
    OHI Compatibility Tests    :active, b2, after a3, 2w
    Rollback Procedures        :active, b3, after b1, 1w
    
    section P2 - Enhancement
    Graduated Rollout          :c1, after b3, 2w
    Cost Comparison            :c2, after b3, 1w
    Migration Automation       :c3, after c1, 2w
```

## Risk Heat Map

```mermaid
quadrantChart
    title Migration Risk Assessment
    x-axis Low Impact --> High Impact
    y-axis Low Likelihood --> High Likelihood
    quadrant-1 Monitor
    quadrant-2 Mitigate Urgently
    quadrant-3 Accept
    quadrant-4 Mitigate
    
    "Metric Name Mismatch": [0.9, 0.95]
    "Entity Correlation Loss": [0.8, 0.7]
    "Performance Degradation": [0.4, 0.3]
    "Rollback Failure": [0.9, 0.5]
    "Data Loss": [0.95, 0.2]
    "Cost Overrun": [0.3, 0.6]
```

## Component Dependency Graph

```mermaid
graph TD
    subgraph "Existing Components ✅"
        A[OTEL Receivers]
        B[Custom Processors]
        C[Exporters]
    end
    
    subgraph "Required Components ❌"
        D[Metric Translator]
        E[Entity Correlator]
        F[Validator]
        G[Rollback Controller]
    end
    
    subgraph "Integration Points"
        H[Config Management]
        I[Monitoring]
        J[Alerting]
    end
    
    A --> D
    D --> B
    B --> E
    E --> F
    F --> C
    G --> H
    G --> I
    G --> J
    
    style D fill:#ff6b6b
    style E fill:#ff6b6b
    style F fill:#ff6b6b
    style G fill:#ff6b6b
```