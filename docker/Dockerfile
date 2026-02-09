# Build stage
FROM golang:1.25.7-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

# Set working directory
WORKDIR /app

# Bypass Go proxy for rshade modules - fetch directly from GitHub
ENV GOPRIVATE=github.com/rshade/*

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application directly (avoid make dependency on external tools)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p bin && \
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "docker") && \
    COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown") && \
    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    CGO_ENABLED=0 go build \
        -ldflags "-X github.com/rshade/finfocus/pkg/version.version=${VERSION} -X github.com/rshade/finfocus/pkg/version.gitCommit=${COMMIT} -X github.com/rshade/finfocus/pkg/version.buildDate=${BUILD_DATE}" \
        -o bin/finfocus ./cmd/finfocus

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl

# Create non-root user
RUN addgroup -g 1001 -S finfocus && \
    adduser -u 1001 -S finfocus -G finfocus

# Create directories for finfocus data
RUN mkdir -p /home/finfocus/.finfocus/plugins && \
    mkdir -p /home/finfocus/.finfocus/specs && \
    chown -R finfocus:finfocus /home/finfocus/.finfocus

# Copy binary from builder stage
COPY --from=builder /app/bin/finfocus /usr/local/bin/finfocus

# Make binary executable
RUN chmod +x /usr/local/bin/finfocus

# Switch to non-root user
USER finfocus

# Set working directory
WORKDIR /home/finfocus

# Set environment variables
ENV HOME=/home/finfocus
ENV PATH=/usr/local/bin:$PATH

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD finfocus --help || exit 1

# Default command
ENTRYPOINT ["finfocus"]
CMD ["--help"]