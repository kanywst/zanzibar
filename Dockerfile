FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src/ ./src/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o zanzibar-server ./src

# Use a minimal alpine image for the final stage
FROM alpine:3.17

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/zanzibar-server .

# Expose the port the server listens on
EXPOSE 8080

# Run the server
CMD ["./zanzibar-server"]
