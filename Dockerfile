# PostgreSQL Unified Collector - Optimized Multi-Stage Dockerfile

# Build stage - Using latest stable Rust
FROM rust:1.82-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    musl-dev \
    openssl-dev \
    pkgconfig \
    protobuf-dev

# Create app directory
WORKDIR /build

# Copy only dependency files first for better caching
COPY Cargo.toml Cargo.lock ./
COPY crates/core/Cargo.toml ./crates/core/
COPY crates/extensions/Cargo.toml ./crates/extensions/
COPY crates/query-engine/Cargo.toml ./crates/query-engine/
COPY crates/nri-adapter/Cargo.toml ./crates/nri-adapter/
COPY crates/otel-adapter/Cargo.toml ./crates/otel-adapter/

# Create dummy source files to build dependencies
RUN mkdir -p src crates/core/src crates/extensions/src \
    crates/query-engine/src crates/nri-adapter/src crates/otel-adapter/src && \
    echo "fn main() {}" > src/main.rs && \
    echo "pub fn dummy() {}" > crates/core/src/lib.rs && \
    echo "pub fn dummy() {}" > crates/extensions/src/lib.rs && \
    echo "pub fn dummy() {}" > crates/query-engine/src/lib.rs && \
    echo "pub fn dummy() {}" > crates/nri-adapter/src/lib.rs && \
    echo "pub fn dummy() {}" > crates/otel-adapter/src/lib.rs

# Build dependencies only
RUN cargo build --release --no-default-features --features "nri otel" || true
RUN rm -rf src crates/*/src

# Copy actual source code
COPY . .

# Build the final binary
RUN cargo build --release --bin postgres-unified-collector

# Runtime stage - Minimal Alpine image
FROM alpine:3.18 AS runtime

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    libgcc \
    libstdc++ \
    openssl \
    tini

# Create non-root user
RUN addgroup -g 1000 collector && \
    adduser -D -u 1000 -G collector collector

# Copy binary from builder
COPY --from=builder /build/target/release/postgres-unified-collector /usr/local/bin/postgres-collector

# Create necessary directories
RUN mkdir -p /app/configs /app/logs && \
    chown -R collector:collector /app

# Switch to non-root user
USER collector
WORKDIR /app

# Expose ports (can be overridden by service configs)
EXPOSE 8080 8081 9090 9091

# Health check with dynamic port support
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${HEALTH_PORT:-8080}/health || exit 1

# Use tini for proper signal handling
ENTRYPOINT ["/sbin/tini", "--"]

# Default command
CMD ["postgres-collector", "-c", "/app/config.toml"]

# Development stage - Includes debugging tools
FROM runtime AS development

USER root

# Install development tools
RUN apk add --no-cache \
    bash \
    curl \
    jq \
    postgresql-client \
    vim \
    htop \
    net-tools

# Install Rust toolchain for debugging
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable

# Copy source code for debugging
COPY --from=builder /build /app/src

USER collector

# Override entrypoint for development
ENTRYPOINT ["/bin/bash"]