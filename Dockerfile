# Build stage
FROM golang:1.22.4-alpine AS builder
WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o tatunnel-server ./cmd/server

# Run stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/tatunnel-server .

EXPOSE 8080
CMD ["./tatunnel-server"]
