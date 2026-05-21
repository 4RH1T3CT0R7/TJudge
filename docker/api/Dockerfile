# Frontend build stage
FROM node:20-alpine AS frontend-builder

WORKDIR /frontend

# Copy frontend files
COPY web/package*.json ./

# Install dependencies
RUN npm ci

# Copy source
COPY web/ ./

# Build frontend
RUN npm run build

# Dependencies caching stage
FROM golang:1.26-alpine AS deps

WORKDIR /deps
COPY go.mod go.sum ./
RUN go mod download

# Go build stage
FROM golang:1.26-alpine AS builder

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

# Copy frontend dist from frontend builder
# Vite outDir is '../internal/web/dist' relative to /frontend/, resolving to /internal/web/dist
COPY --from=frontend-builder /internal/web/dist ./internal/web/dist

# Build the binary with cache mount for faster rebuilds
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o /app/api ./cmd/api

# Final stage
FROM alpine:3.20

# Install ca-certificates, compilers and interpreters for program validation/compilation
RUN apk --no-cache add ca-certificates wget gcc g++ musl-dev python3 go openjdk17-jdk rust

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Create programs directory
RUN mkdir -p /data/programs && chown -R appuser:appuser /data

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/api .

# Copy migrations (if needed)
COPY --from=builder /build/migrations ./migrations

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q -O - http://localhost:8080/health || exit 1

# Run the binary
CMD ["./api"]
