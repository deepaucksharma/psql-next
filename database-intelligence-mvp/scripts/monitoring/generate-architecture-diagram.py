#!/usr/bin/env python3
"""
Generate architecture diagrams for Database Intelligence MVP
Shows both Standard and Experimental mode architectures
"""

import os
import sys

def create_mermaid_diagrams():
    """Create Mermaid diagram definitions for both architectures"""
    
    standard_diagram = """
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
"""

    experimental_diagram = """
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
"""

    comparison_diagram = """
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
"""

    return {
        "standard": standard_diagram,
        "experimental": experimental_diagram,
        "comparison": comparison_diagram
    }

def create_html_page(diagrams):
    """Create an HTML page with all diagrams"""
    
    html_content = """<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Database Intelligence MVP - Architecture Diagrams</title>
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
    <script>mermaid.initialize({startOnLoad:true, theme:'default'});</script>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #f5f5f5;
        }
        h1, h2 {
            color: #2c3e50;
        }
        .diagram-container {
            background: white;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .mermaid {
            text-align: center;
            margin: 20px 0;
        }
        .description {
            margin: 20px 0;
            padding: 15px;
            background: #e8f4f8;
            border-left: 4px solid #3498db;
            border-radius: 4px;
        }
        .comparison-table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
        }
        .comparison-table th, .comparison-table td {
            border: 1px solid #ddd;
            padding: 12px;
            text-align: left;
        }
        .comparison-table th {
            background: #3498db;
            color: white;
        }
        .comparison-table tr:nth-child(even) {
            background: #f9f9f9;
        }
        .buttons {
            margin: 20px 0;
            text-align: center;
        }
        .button {
            display: inline-block;
            padding: 10px 20px;
            margin: 0 10px;
            background: #3498db;
            color: white;
            text-decoration: none;
            border-radius: 5px;
            transition: background 0.3s;
        }
        .button:hover {
            background: #2980b9;
        }
        .button.experimental {
            background: #9b59b6;
        }
        .button.experimental:hover {
            background: #8e44ad;
        }
    </style>
</head>
<body>
    <h1>Database Intelligence MVP - Architecture Diagrams</h1>
    
    <div class="diagram-container">
        <h2>Decision Flow</h2>
        <div class="description">
            Use this flowchart to determine which deployment mode best fits your needs.
        </div>
        <div class="mermaid">
""" + diagrams["comparison"] + """
        </div>
    </div>
    
    <div class="diagram-container">
        <h2>Standard Mode Architecture</h2>
        <div class="description">
            <strong>Production-ready deployment using proven OpenTelemetry components.</strong>
            <ul>
                <li>High availability with 3 replicas and leader election</li>
                <li>Low resource usage (512MB RAM)</li>
                <li>5-minute collection intervals</li>
                <li>25% probabilistic sampling</li>
            </ul>
        </div>
        <div class="mermaid">
""" + diagrams["standard"] + """
        </div>
        <div class="buttons">
            <a href="#" class="button" onclick="alert('Run: ./quickstart.sh all')">Deploy Standard Mode</a>
        </div>
    </div>
    
    <div class="diagram-container">
        <h2>Experimental Mode Architecture</h2>
        <div class="description">
            <strong>Advanced monitoring with custom Go components.</strong>
            <ul>
                <li>Active Session History with 1-second sampling</li>
                <li>Adaptive sampling based on query cost and errors</li>
                <li>Circuit breaker for automatic database protection</li>
                <li>Multi-database federation support</li>
                <li>Single instance deployment (stateful components)</li>
            </ul>
        </div>
        <div class="mermaid">
""" + diagrams["experimental"] + """
        </div>
        <div class="buttons">
            <a href="#" class="button experimental" onclick="alert('Run: ./quickstart.sh --experimental all')">Deploy Experimental Mode</a>
        </div>
    </div>
    
    <div class="diagram-container">
        <h2>Feature Comparison</h2>
        <table class="comparison-table">
            <tr>
                <th>Feature</th>
                <th>Standard Mode</th>
                <th>Experimental Mode</th>
            </tr>
            <tr>
                <td>Build Required</td>
                <td>❌ No</td>
                <td>✅ Yes (automated)</td>
            </tr>
            <tr>
                <td>High Availability</td>
                <td>✅ 3 replicas</td>
                <td>⚠️ Single instance</td>
            </tr>
            <tr>
                <td>Memory Usage</td>
                <td>256-512MB</td>
                <td>1-2GB</td>
            </tr>
            <tr>
                <td>ASH Sampling</td>
                <td>❌</td>
                <td>✅ 1-second intervals</td>
            </tr>
            <tr>
                <td>Adaptive Sampling</td>
                <td>❌ Fixed 25%</td>
                <td>✅ Dynamic based on query cost</td>
            </tr>
            <tr>
                <td>Circuit Breaker</td>
                <td>❌</td>
                <td>✅ Automatic protection</td>
            </tr>
            <tr>
                <td>Multi-Database</td>
                <td>⚠️ Manual configuration</td>
                <td>✅ Native federation</td>
            </tr>
            <tr>
                <td>Production Ready</td>
                <td>✅ Yes</td>
                <td>⚠️ Beta quality</td>
            </tr>
        </table>
    </div>
    
    <div class="diagram-container">
        <h2>Next Steps</h2>
        <div class="buttons">
            <a href="https://github.com/newrelic/database-intelligence-mvp" class="button">View on GitHub</a>
            <a href="GETTING-STARTED.md" class="button">Getting Started Guide</a>
            <a href="DEPLOYMENT-OPTIONS.md" class="button">Detailed Comparison</a>
        </div>
    </div>
</body>
</html>"""
    
    return html_content

def create_markdown_diagrams(diagrams):
    """Create markdown file with diagram definitions"""
    
    markdown_content = """# Architecture Diagrams

This document contains the architecture diagrams for Database Intelligence MVP in Mermaid format.

## Decision Flow

```mermaid
""" + diagrams["comparison"] + """
```

## Standard Mode Architecture

```mermaid
""" + diagrams["standard"] + """
```

## Experimental Mode Architecture

```mermaid
""" + diagrams["experimental"] + """
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
"""
    
    return markdown_content

def main():
    """Generate architecture diagram files"""
    
    print("Generating architecture diagrams...")
    
    # Create diagrams
    diagrams = create_mermaid_diagrams()
    
    # Create docs directory if it doesn't exist
    docs_dir = os.path.join(os.path.dirname(os.path.dirname(__file__)), 'docs')
    os.makedirs(docs_dir, exist_ok=True)
    
    # Generate HTML file
    html_content = create_html_page(diagrams)
    html_path = os.path.join(docs_dir, 'architecture-diagrams.html')
    with open(html_path, 'w') as f:
        f.write(html_content)
    print(f"✓ Created HTML diagrams: {html_path}")
    
    # Generate Markdown file
    markdown_content = create_markdown_diagrams(diagrams)
    markdown_path = os.path.join(os.path.dirname(os.path.dirname(__file__)), 'ARCHITECTURE-DIAGRAMS.md')
    with open(markdown_path, 'w') as f:
        f.write(markdown_content)
    print(f"✓ Created Markdown diagrams: {markdown_path}")
    
    print("\nView diagrams:")
    print(f"  - HTML: open {html_path}")
    print(f"  - Markdown: {markdown_path}")

if __name__ == "__main__":
    main()