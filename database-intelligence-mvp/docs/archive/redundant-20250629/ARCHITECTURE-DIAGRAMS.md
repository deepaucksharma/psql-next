# Architecture Diagrams

This document contains the architecture diagrams for Database Intelligence MVP in Mermaid format.

## Decision Flow

```mermaid

graph LR
    subgraph "Choose Your Mode"
        Start{Start Here}
        Q1{Need ASH<br/>Sampling?}
        Q2{Need Circuit<br/>Breaker?}
        Q3{Multi-DB<br/>Federation?}
        Q4{Can Build<br/>Custom?}
        
        Standard[Standard Mode<br/>✓ Production Ready<br/>✓ No Build<br/>✓ HA Support<br/>✓ Low Resources]
        Experimental[Experimental Mode<br/>✓ Advanced Features<br/>✓ ASH Sampling<br/>✓ Smart Protection<br/>✓ Future Ready]
        
        Start --> Q1
        Q1 -->|No| Q2
        Q1 -->|Yes| Experimental
        Q2 -->|No| Q3
        Q2 -->|Yes| Experimental
        Q3 -->|No| Standard
        Q3 -->|Yes| Q4
        Q4 -->|No| Standard
        Q4 -->|Yes| Experimental
    end
    
    classDef decision fill:#ffd,stroke:#333,stroke-width:2px
    classDef standard fill:#bfb,stroke:#333,stroke-width:3px
    classDef experimental fill:#bbf,stroke:#333,stroke-width:3px
    
    class Q1,Q2,Q3,Q4 decision
    class Standard standard
    class Experimental experimental

```

## Standard Mode Architecture

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

## Experimental Mode Architecture

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

## Rendering These Diagrams

### Option 1: View in GitHub
GitHub automatically renders Mermaid diagrams in markdown files.

### Option 2: Generate HTML
Run the Python script to generate an interactive HTML page:
```bash
python scripts/generate-architecture-diagram.py
open docs/architecture-diagrams.html
```

### Option 3: Use Mermaid CLI
```bash
npm install -g @mermaid-js/mermaid-cli
mmdc -i ARCHITECTURE-DIAGRAMS.md -o architecture.png
```