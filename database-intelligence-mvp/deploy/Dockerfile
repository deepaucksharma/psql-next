# Database Intelligence Collector - OTEL-First Dockerfile
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
COPY processors/adaptivesampler/go.mod processors/adaptivesampler/go.sum ./processors/adaptivesampler/
COPY processors/circuitbreaker/go.mod processors/circuitbreaker/go.sum ./processors/circuitbreaker/

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the collector
RUN go build -o /bin/database-intelligence-collector ./main.go

# Runtime image
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /bin/database-intelligence-collector /bin/database-intelligence-collector

# Create config directory
RUN mkdir -p /etc/otel

# Expose ports
EXPOSE 8888 8889 13133 4317 4318

# Set entrypoint
ENTRYPOINT ["/bin/database-intelligence-collector"]