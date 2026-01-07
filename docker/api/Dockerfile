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

# Go build stage
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

# Copy frontend dist from frontend builder
COPY --from=frontend-builder /frontend/dist ./internal/web/dist

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o /app/api ./cmd/api

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates wget

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
