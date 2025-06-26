# Build stage
FROM rust:1.75-alpine AS builder

RUN apk add --no-cache musl-dev openssl-dev

WORKDIR /app
COPY . .

# Build all features
RUN cargo build --release --features all

# Runtime stage
FROM alpine:3.18

RUN apk add --no-cache ca-certificates libgcc

# Create non-root user
RUN addgroup -g 1000 collector && \
    adduser -D -u 1000 -G collector collector

# Copy binaries
COPY --from=builder /app/target/release/postgres-unified-collector /usr/local/bin/
COPY --from=builder /app/target/release/nri-postgresql /usr/local/bin/
COPY --from=builder /app/target/release/postgres-otel-collector /usr/local/bin/

# Copy default configs
COPY configs/collector-config.toml /etc/postgres-collector/
COPY configs/otel-config.toml /etc/postgres-collector/

USER collector

# Default to unified collector
ENTRYPOINT ["postgres-unified-collector"]
CMD ["--config", "/etc/postgres-collector/collector-config.toml"]