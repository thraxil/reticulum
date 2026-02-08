# Stage 1: Builder
FROM golang:1.25 AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary, important for scratch/alpine base images
# -o /app/reticulum specifies output path
RUN CGO_ENABLED=0 go build -o /app/reticulum .

# Stage 2: Final image
# Use a minimal base image, e.g., alpine or scratch
FROM alpine:latest

# Install imagemagick for image resizing functionality
# This will be removed if the resize library is replaced with a pure Go one
RUN apk add --no-cache imagemagick

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/reticulum .

# Expose the port the application listens on
EXPOSE 8080

# Command to run the application
CMD ["./reticulum"]
