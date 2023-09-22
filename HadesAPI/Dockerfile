# Use an official Go runtime as a parent image
FROM golang:1.21-alpine AS builder

ENV BINARY=hadesCI-API

# Set the working directory inside the container
WORKDIR /app

# Copy the Go application source code into the container
COPY . .

# Build the Go application
RUN go build -o ${BINARY} .

# Start a new stage for the minimal runtime container
FROM alpine:latest

RUN apk update && apk add ca-certificates libc6-compat

# Set the working directory inside the minimal runtime container
WORKDIR /app

# Copy the built binary from the builder container into the minimal runtime container
COPY --from=builder /app/${BINARY} . 

# Ensure the binary is executable
RUN chmod +x /app/${BINARY}

# Run your Go application
CMD ["/app/${BINARY}"]
