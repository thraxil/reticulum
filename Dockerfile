# Builder stage
FROM golang:1.25-bookworm AS builder

# Install libvips and libwebp dependencies
RUN apt-get update && apt-get install -y \
    libvips-dev \
    libwebp-dev \
    pkg-config \
    gcc \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# CGO_ENABLED=1 is required for bimg and nativewebp
RUN CGO_ENABLED=1 GOOS=linux go build -o reticulum .

# Final stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    libvips \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from the builder
COPY --from=builder /app/reticulum /usr/local/bin/reticulum

# Expose the default port
EXPOSE 8080

# Command to run
CMD ["reticulum"]