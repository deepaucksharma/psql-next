# Contributing to PostgreSQL Unified Collector

Thank you for your interest in contributing to the PostgreSQL Unified Collector project!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR-USERNAME/postgres-unified-collector`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Set up development environment: `make setup`

## Development Setup

### Prerequisites
- Rust 1.70+ (install from https://rustup.rs)
- PostgreSQL 12+ with pg_stat_statements enabled
- Docker and Docker Compose (for testing)
- Make

### Initial Setup
```bash
make setup          # Copy config templates
make build          # Build debug binary
make test           # Run tests
```

## Project Structure

```
├── src/                    # Main application source
│   ├── bin/               # Binary entry points
│   ├── collection_engine.rs # Core collection logic
│   ├── config.rs          # Configuration structures
│   └── adapters.rs        # Output adapters (NRI/OTLP)
├── crates/                # Workspace crates
│   ├── core/             # Core types and traits
│   ├── nri-adapter/      # New Relic Infrastructure adapter
│   ├── otel-adapter/     # OpenTelemetry adapter
│   ├── query-engine/     # PostgreSQL query execution
│   └── extensions/       # PostgreSQL extension management
├── scripts/              # Utility scripts
├── deployments/          # Kubernetes manifests
├── charts/               # Helm chart
└── examples/             # Example configurations
```

## Making Changes

### Code Style
- Run `make fmt` before committing
- Run `make lint` to check for issues
- Follow Rust naming conventions

### Testing
- Add tests for new functionality
- Run `make test` to run all tests
- Run `make test-integration` for integration tests
- Test with both NRI and OTLP modes

### Commit Messages
Follow conventional commits format:
- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `test:` Test additions/changes
- `refactor:` Code refactoring
- `chore:` Maintenance tasks

Example: `feat: add support for pg_stat_monitor extension`

## Submitting Changes

1. Ensure all tests pass: `make test`
2. Update documentation if needed
3. Push to your fork
4. Create a Pull Request with:
   - Clear description of changes
   - Any related issue numbers
   - Test results/screenshots if applicable

## Testing Locally

### Run with Docker Compose
```bash
make docker-up      # Start PostgreSQL
make run-hybrid     # Run collector in dual mode
make docker-logs    # View logs
```

### Test Different Modes
```bash
make run-nri        # NRI mode only
make run-otel       # OTLP mode only
make run-hybrid     # Both outputs
```

## Release Process

1. Update version in `Cargo.toml` files
2. Update `CHANGELOG.md`
3. Create git tag: `git tag v1.0.0`
4. Push tag: `git push origin v1.0.0`

## Getting Help

- Check existing issues
- Join discussions in issues/PRs
- Read the documentation in `docs/`

## Code of Conduct

- Be respectful and inclusive
- Welcome newcomers
- Focus on constructive feedback
- Assume good intentions