# Frontend build stage
FROM node:20-alpine AS frontend-builder

WORKDIR /build/dashboard

# Copy dashboard package files
COPY dashboard/package*.json ./

# Install dependencies
RUN npm ci

# Copy dashboard source
COPY dashboard/ ./

# Build frontend
RUN npm run build

# Backend build stage
FROM golang:1.25-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /build

# Copy go mod files (both, for deterministic reproducible builds)
COPY go.mod go.sum ./

# Download modules against the committed go.sum (no `go mod tidy`: it can drift
# versions and needs network beyond the module cache)
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Copy built frontend into embed location
COPY --from=frontend-builder /build/dashboard/dist ./internal/dashboard/dist

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o extractor ./cmd/extractor

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    exiftool \
    tzdata

# Create non-root user
RUN addgroup -g 1000 extractor && \
    adduser -D -u 1000 -G extractor extractor

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /build/extractor /app/extractor

# Create directories for volumes
RUN mkdir -p /config /photoprism/originals /photoprism/_daily && \
    chown -R extractor:extractor /app /config /photoprism

USER extractor

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/extractor"]
CMD ["-config", "/config/config.yaml"]
