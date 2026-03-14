# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go modules manifests and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary (CGO_ENABLED=0 ensures a static binary that runs anywhere)
RUN CGO_ENABLED=0 GOOS=linux go build -o api-gateway .

# Stage 2: Create the tiny production image
FROM alpine:latest

WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/api-gateway .

# Expose the API port
EXPOSE 8080

# Command to run the executable
CMD ["./api-gateway"]