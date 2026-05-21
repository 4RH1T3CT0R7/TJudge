# Dependencies caching stage
FROM golang:1.26-alpine AS deps

WORKDIR /deps
COPY go.mod go.sum ./
RUN go mod download

# Build stage
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

# Build the binary with cache mount for faster rebuilds
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o /app/worker ./cmd/worker

# Final stage
FROM alpine:3.20

# DOCKER_GID должен совпадать с GID группы docker на ХОСТЕ (часто 999 на Ubuntu,
# 0 при Docker Desktop на macOS). Передавайте через --build-arg DOCKER_GID=$(getent group docker | cut -d: -f3).
ARG DOCKER_GID=999

# Install Docker CLI for running tjudge-cli in containers
RUN apk --no-cache add docker-cli ca-certificates

# Create non-root user and docker group (aligned with host GID).
# SECURITY (P0.8): раньше worker запускался от root для доступа к docker.sock;
# теперь appuser член docker-группы с matching GID. См. docker-compose:
#   user: "${WORKER_UID:-1000}:${DOCKER_GID:-999}".
#
# UR bug_021: DOCKER_GID=0 (macOS Docker Desktop часто использует root group)
# конфликтует с уже существующей root-группой alpine. В Alpine 3.20 GID 999
# занят группой `ping`. Логика: если группа с нужным GID уже существует —
# переиспользуем её; иначе создаём docker-группу.
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    existing_group=$(awk -F: -v gid="${DOCKER_GID}" '$3==gid {print $1; exit}' /etc/group) && \
    if [ -n "$existing_group" ]; then \
        adduser appuser "$existing_group"; \
    else \
        addgroup -g ${DOCKER_GID} docker && \
        adduser appuser docker; \
    fi

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/worker .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Run the binary
CMD ["./worker"]
