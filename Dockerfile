# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./cmd/server/main.go

# Final stage
FROM alpine:latest

RUN apk add --no-cache sqlite-libs

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/server .
# Copy static assets and templates
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# Create data directory
RUN mkdir -p /app/data

# Expose the port the app runs on
EXPOSE 8080

# Command to run the application
CMD ["./server"]
