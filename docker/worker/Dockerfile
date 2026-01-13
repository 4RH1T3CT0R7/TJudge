# Dependencies caching stage
FROM golang:1.24-alpine AS deps

WORKDIR /deps
COPY go.mod go.sum ./
RUN go mod download

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy cached dependencies
COPY --from=deps /go/pkg /go/pkg

# Copy go mod files
COPY go.mod go.sum ./

# Copy source code
COPY . .

# Build the binary with cache mount for faster rebuilds
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o /app/worker ./cmd/worker

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
