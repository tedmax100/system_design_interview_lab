# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /exchange-service ./cmd/server

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install wget for healthcheck
RUN apk add --no-cache ca-certificates wget

# Copy binary from builder
COPY --from=builder /exchange-service .

# Expose ports
EXPOSE 8080 9090

# Run the service
CMD ["./exchange-service"]
