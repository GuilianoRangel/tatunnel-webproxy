# Build stage
FROM golang:1.22.4-alpine AS builder
WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o tatunnel-server ./cmd/server

# Build cross-platform clients
RUN mkdir -p /app/public/downloads
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/public/downloads/tatunnel-linux-amd64 ./cmd/client
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /app/public/downloads/tatunnel-windows-amd64.exe ./cmd/client
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /app/public/downloads/tatunnel-darwin-amd64 ./cmd/client
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /app/public/downloads/tatunnel-darwin-arm64 ./cmd/client

# Run stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/tatunnel-server .
COPY --from=builder /app/public ./public

EXPOSE 8080
CMD ["./tatunnel-server"]
