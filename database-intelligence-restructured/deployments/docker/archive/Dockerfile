# Database Intelligence Collector - OTEL-First Dockerfile
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy the enterprise distribution files specifically
COPY distributions/enterprise/ ./

# Download dependencies for enterprise distribution
RUN go mod download

# Copy source code
COPY . .

# Build the collector from enterprise distribution
RUN GOWORK=off go build -o /bin/database-intelligence-collector ./main.go

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