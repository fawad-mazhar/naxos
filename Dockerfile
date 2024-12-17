# Dockerfile
FROM golang:1.23-alpine

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the API server
RUN go build -o /app/api ./cmd/api

# Build the orchestrator
RUN go build -o /app/orchestrator ./cmd/orchestrator