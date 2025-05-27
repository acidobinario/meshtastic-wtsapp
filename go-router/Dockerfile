# Stage 1: Build
FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -o go-router .

# Stage 2: Run
FROM alpine:latest

# Needed to run the Go binary and for certificates (if your app calls HTTPS)
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy the compiled binary from builder stage
COPY --from=builder /app/go-router .

EXPOSE 8080

# Run the Go app
CMD ["./go-router"]
