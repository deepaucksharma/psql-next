.PHONY: help build test run clean docker-up docker-down setup release install

# Variables
BINARY_NAME := postgres-unified-collector
DOCKER_IMAGE := postgres-unified-collector
HELM_RELEASE := postgres-collector
NAMESPACE := postgres-monitoring

# Default target
help:
	@echo "PostgreSQL Unified Collector - Make Targets"
	@echo ""
	@echo "Setup & Build:"
	@echo "  make setup        - Initial setup (copy config example)"
	@echo "  make build        - Build debug binary"
	@echo "  make release      - Build optimized release binary"
	@echo "  make test         - Run all tests"
	@echo "  make clean        - Clean build artifacts"
	@echo ""
	@echo "Running:"
	@echo "  make run          - Run collector with config.toml"
	@echo "  make run-nri      - Run in NRI mode"
	@echo "  make run-otel     - Run in OTLP mode"
	@echo "  make run-hybrid   - Run in hybrid mode (both outputs)"
	@echo ""
	@echo "Docker & Kubernetes:"
	@echo "  make docker       - Build Docker image"
	@echo "  make docker-up    - Start Docker Compose stack"
	@echo "  make docker-down  - Stop Docker Compose stack"
	@echo "  make helm-deploy  - Deploy with Helm chart"
	@echo "  make k8s-deploy   - Deploy with raw Kubernetes manifests"
	@echo ""
	@echo "Development:"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Run clippy linter"
	@echo "  make doc          - Generate and open documentation"

# Setup
setup:
	@if [ ! -f config.toml ]; then \
		cp config.example.toml config.toml; \
		echo "Created config.toml - please update with your PostgreSQL connection details"; \
	fi
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env file - please add your New Relic license key"; \
	fi
	@cargo --version || (echo "Please install Rust from https://rustup.rs" && exit 1)

# Build
build:
	cargo build --all-features

release:
	cargo build --release --all-features

# Test
test:
	cargo test --all-features

test-integration:
	cargo test --all-features --test '*' -- --nocapture

# Run
run: check-config
	./target/debug/$(BINARY_NAME) --config config.toml

run-nri: check-config build
	./target/debug/$(BINARY_NAME) --config config.toml --mode nri

run-otel: check-config build
	./target/debug/$(BINARY_NAME) --config config.toml --mode otel

run-hybrid: check-config build
	./target/debug/$(BINARY_NAME) --config config.toml --mode hybrid

# Docker
docker:
	docker build -t $(DOCKER_IMAGE):latest .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down -v

docker-logs:
	docker-compose logs -f

# Kubernetes
k8s-deploy:
	kubectl apply -f deployments/kubernetes/

helm-deploy:
	helm install $(HELM_RELEASE) ./charts/postgres-collector \
		--namespace $(NAMESPACE) --create-namespace

helm-upgrade:
	helm upgrade $(HELM_RELEASE) ./charts/postgres-collector \
		--namespace $(NAMESPACE)

# Install
install: release
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp target/release/$(BINARY_NAME) /usr/local/bin/

# Utilities
clean:
	cargo clean
	rm -rf target/ coverage/ *.log tmp/ temp/

check-config:
	@if [ ! -f config.toml ]; then \
		echo "Error: config.toml not found. Run 'make setup' first."; \
		exit 1; \
	fi

# Development
fmt:
	cargo fmt

lint:
	cargo clippy -- -D warnings

doc:
	cargo doc --no-deps --open