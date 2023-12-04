# Use an official Go runtime as a parent image
FROM golang:1.21-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY ./HadesAPI/go.mod ./HadesAPI/go.sum ./HadesAPI/
COPY ./shared/go.mod ./shared/go.sum ./shared/
RUN cd HadesAPI && go mod download

# Copy the Go application source code into the container
COPY ./HadesAPI ./HadesAPI
COPY ./shared ./shared

# Build the Go application
WORKDIR /app/HadesAPI
RUN go build -o hadesCI-api .

# Start a new stage for the minimal runtime container
FROM alpine:latest

RUN apk update && apk add --no-cache ca-certificates libc6-compat

# Set the working directory inside the minimal runtime container
WORKDIR /app

# Copy the built binary from the builder container into the minimal runtime container
COPY --from=builder /app/HadesAPI/hadesCI-api . 

# Ensure the binary is executable
RUN chmod +x /app/hadesCI-api

# Run your Go application
CMD ["/app/hadesCI-api"]
