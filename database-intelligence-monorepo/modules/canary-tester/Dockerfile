FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache gcc musl-dev mysql-client

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the canary tester
RUN go build -o canary-tester cmd/canary-tester/main.go

FROM alpine:latest

# WARNING: DO NOT ADD HEALTHCHECK instructions here!
# Health checks have been intentionally removed from production images.
# Use shared/validation/health-check-all.sh for validation purposes.

# Install mysql client for health checks
RUN apk add --no-cache mysql-client ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/canary-tester .

# Copy query files
COPY src/canary-queries.sql /app/queries/

# Expose ports for OpenTelemetry functionality only
EXPOSE 8090
# WARNING: DO NOT expose port 13133 (health check port)!

CMD ["./canary-tester"]