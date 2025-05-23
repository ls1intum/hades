# Use an official Go runtime as a parent image
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY ./HadesScheduler/go.mod ./HadesScheduler/go.sum ./HadesScheduler/
COPY ./shared/go.mod ./shared/go.sum ./shared/
RUN cd HadesScheduler && go mod download

# Copy the Go application source code into the container
COPY ./HadesScheduler ./HadesScheduler
COPY ./shared ./shared

# Build the Go application
WORKDIR /app/HadesScheduler
RUN CGO_ENABLED=0 go build -o hadesCI-scheduler .

# Start a new stage for the minimal runtime container
FROM gcr.io/distroless/static-debian12

# Set the working directory inside the minimal runtime container
WORKDIR /app

# Copy the built binary from the builder container into the minimal runtime container
COPY --from=builder /app/HadesScheduler/hadesCI-scheduler . 

# Run your Go application
CMD ["/app/hadesCI-scheduler"]
