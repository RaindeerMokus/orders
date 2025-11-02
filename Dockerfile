# Simple multi-stage Dockerfile for a Go project
# Builds a static binary in a builder stage and produces a minimal runtime image.

FROM golang:1.25-alpine AS builder
WORKDIR /src

# Add necessary packages
RUN apk add --no-cache git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/main ./cmd/orders/main.go

# Final minimal image
FROM alpine:3.18
COPY --from=builder /app/main /app/main

# Install migrate CLI for database migrations
RUN apk add --no-cache curl tar
RUN curl -L -o migrate.tar.gz https://github.com/golang-migrate/migrate/releases/download/v4.15.2/migrate.linux-amd64.tar.gz
RUN tar -xzf migrate.tar.gz  
RUN mv migrate /usr/local/bin/migrate 
RUN chmod +x /usr/local/bin/migrate  
RUN rm migrate.tar.gz


# Set environment variables default (can be overridden at runtime)
ENV PORT=8080
ENV DATABASE_URL=postgres://postgres:postgres@db:5432/ordersdb?sslmode=disable

# Expose port
EXPOSE 8080

# ENTRYPOINT ["/app/main"]