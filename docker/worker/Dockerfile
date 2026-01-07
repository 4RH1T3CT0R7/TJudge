# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o /app/worker ./cmd/worker

# Final stage
FROM alpine:latest

# Install Docker CLI for running tjudge-cli in containers
RUN apk --no-cache add docker-cli ca-certificates

# Create non-root user and docker group
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    (addgroup docker || true) && \
    adduser appuser docker

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/worker .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Run the binary
CMD ["./worker"]
