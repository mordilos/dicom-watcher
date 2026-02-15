# Use the official Golang image as the base image
FROM golang:1.25 AS builder

# Set the working directory in the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download the dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o dicom-watcher ./cmd/dicom-watcher

# Use a minimal base image for the final stage
FROM alpine:latest

# Set the working directory in the container
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/dicom-watcher .

# Copy the configuration file
COPY config.yaml .

# Expose the port if needed (e.g., for health checks or metrics)
# EXPOSE 8080

# Run the application
CMD ["./dicom-watcher"]
