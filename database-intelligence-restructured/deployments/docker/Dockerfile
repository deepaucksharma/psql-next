# Multi-stage Dockerfile for Database Intelligence Collector
# Supports minimal, standard, and enterprise profiles via BUILD_PROFILE arg

ARG BUILD_PROFILE=standard
ARG GO_VERSION=1.22

# Stage 1: Builder
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /build

# Copy workspace files first
COPY go.work go.work.sum ./

# Copy module files for better caching
COPY components/go.mod components/go.sum ./components/
COPY internal/go.mod internal/go.sum ./internal/
COPY distributions/unified/go.mod distributions/unified/go.sum ./distributions/unified/

# Copy source code
COPY . .

# Set build profile
ARG BUILD_PROFILE
ENV BUILD_PROFILE=${BUILD_PROFILE}

# Build the collector with the specified profile
WORKDIR /build/distributions/unified
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.Version=2.0.0 -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /database-intelligence-collector .

# Stage 2: Runtime base
FROM alpine:3.19 AS runtime-base

RUN apk add --no-cache ca-certificates curl

# Create non-root user
RUN addgroup -g 1001 -S otel && \
    adduser -u 1001 -S otel -G otel

# Stage 3: Minimal runtime
FROM runtime-base AS runtime-minimal
LABEL profile="minimal"
COPY --from=builder /database-intelligence-collector /usr/local/bin/database-intelligence-collector
USER otel
EXPOSE 4317 4318 8888 13133

# Stage 4: Standard runtime
FROM runtime-base AS runtime-standard
LABEL profile="standard"
# Add additional tools for standard profile
RUN apk add --no-cache jq vim
COPY --from=builder /database-intelligence-collector /usr/local/bin/database-intelligence-collector
USER otel
EXPOSE 4317 4318 8888 8889 13133 14250 14268

# Stage 5: Enterprise runtime
FROM runtime-base AS runtime-enterprise
LABEL profile="enterprise"
# Add monitoring and debugging tools for enterprise
RUN apk add --no-cache jq vim htop iotop sysstat strace
COPY --from=builder /database-intelligence-collector /usr/local/bin/database-intelligence-collector
USER otel
EXPOSE 4317 4318 8888 8889 13133 14250 14268 6060

# Final stage: Select runtime based on BUILD_PROFILE
FROM runtime-${BUILD_PROFILE} AS final

# Set profile environment variable
ARG BUILD_PROFILE
ENV DB_INTEL_PROFILE=${BUILD_PROFILE}

# Default configuration path
ENV CONFIG_PATH=/etc/otel-collector/config.yaml

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:13133/health || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/database-intelligence-collector"]
CMD ["--profile=${DB_INTEL_PROFILE}", "--config=${CONFIG_PATH}"]