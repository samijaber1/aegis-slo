# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies (CGO required for sqlite3)
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary
RUN CGO_ENABLED=1 go build -o aegis-server ./cmd/aegis-server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/aegis-server /app/

# Copy schemas and fixtures
COPY schemas /app/schemas
COPY fixtures /app/fixtures

# Create data directory
RUN mkdir -p /data

# Expose HTTP port
EXPOSE 8080

# Run as non-root user
RUN adduser -D -u 1000 aegis
RUN chown -R aegis:aegis /app /data
USER aegis

# Default command
CMD ["/app/aegis-server"]
