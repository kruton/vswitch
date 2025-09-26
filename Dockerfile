# Multi-stage build for vswitch
# Stage 1: Build the application
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod file first for better caching
COPY go.mod ./

# Download dependencies (if any)
RUN go mod download

# Copy source code
COPY . .

ARG VERSION=dev

RUN VERSION=${VERSION} make build

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create non-root user for security
RUN addgroup -g 1001 -S vswitch && \
    adduser -u 1001 -S vswitch -G vswitch

# Create directory for PID files
RUN mkdir -p /var/run/vswitch && \
    chown vswitch:vswitch /var/run/vswitch

# Copy the binary from builder stage
COPY --from=builder /app/bin/vswitch /usr/local/bin/vswitch

# Switch to non-root user
USER vswitch

# Expose default ports (these can be overridden)
EXPOSE 9999 9998

# Set default command to run the switch with default ports
CMD ["vswitch", "-ports", "9999,9998"]

# Labels for image metadata
LABEL maintainer="Virtual Switch for QEMU" \
      description="A virtual Ethernet switch for QEMU VM networking"
