# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS builder

WORKDIR /src

# Copy Go module files
COPY go.mod .
COPY go.sum .
# Download dependencies
RUN go mod download

# Copy the source code
COPY main.go .

# Build the static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/argo-generator-from-cattle-helm .

# Stage 2: Create the final, minimal image
FROM scratch

# Copy the static binary from the builder stage
COPY --from=builder /app/argo-generator-from-cattle-helm /usr/local/bin/

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/argo-generator-from-cattle-helm"]
