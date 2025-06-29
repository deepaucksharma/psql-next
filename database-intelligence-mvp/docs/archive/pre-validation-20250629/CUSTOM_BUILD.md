# Building Custom Components

This guide provides instructions for building a custom OpenTelemetry Collector that includes the Database Intelligence MVP's custom components. This is necessary for utilizing advanced features available in Experimental Mode.

## Prerequisites

*   Go 1.21+ installed.
*   `builder` tool installed: `go install go.opentelemetry.io/collector/cmd/builder@latest`.

## Build Process

To build your custom collector, you will use the `builder` tool with a configuration file (`otelcol-builder.yaml`) that specifies which components to include.

```bash
cat > otelcol-builder.yaml <<EOF
dist:
  name: otelcol-custom
  description: Custom OTel Collector with Database Intelligence components
  output_path: ./dist
receivers:
  - gomod: github.com/database-intelligence-mvp/receivers/postgresqlquery v0.0.0
    path: ./receivers/postgresqlquery
processors:
  - gomod: github.com/database-intelligence-mvp/processors/adaptivesampler v0.0.0
    path: ./processors/adaptivesampler
  - gomod: github.com/database-intelligence-mvp/processors/circuitbreaker v0.0.0
    path: ./processors/circuitbreaker
exporters:
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.96.0
EOF
builder --config=otelcol-builder.yaml
```

After running the `builder` command, your custom collector binary will be located in the `./dist` directory (e.g., `./dist/otelcol-custom`).

## Using the Custom Collector

Once built, you can use this custom collector binary with your desired configuration files, especially those designed for Experimental Mode features.

```bash
./dist/otelcol-custom --config=/path/to/your/experimental-config.yaml
```

**Note**: For production deployments, it is recommended to use pre-built Docker images if available, as they are typically hardened and optimized.