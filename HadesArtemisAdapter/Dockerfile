# Use an official Go runtime as a parent image
FROM golang:1.25-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY ./HadesArtemisAdapter/go.mod ./HadesArtemisAdapter/go.sum ./HadesArtemisAdapter/
COPY ./shared/go.mod ./shared/go.sum ./shared/
RUN cd HadesArtemisAdapter && go mod download

# Copy the Go application source code into the container
COPY ./HadesArtemisAdapter ./HadesArtemisAdapter
COPY ./shared ./shared

# Build the Go application
WORKDIR /app/HadesArtemisAdapter
RUN CGO_ENABLED=0 go build -o hades-artemis-adapter .

# Start a new stage for the minimal runtime container
FROM gcr.io/distroless/static-debian12

# Set the working directory inside the minimal runtime container
WORKDIR /app

# Copy the built binary from the builder container into the minimal runtime container
COPY --from=builder /app/HadesArtemisAdapter/hades-artemis-adapter . 

# Run your Go application
CMD ["/app/hades-artemis-adapter"]