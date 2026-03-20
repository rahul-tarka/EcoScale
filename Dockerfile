# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /ecoscale ./cmd/ecoscale

# Runtime stage
FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /

COPY --from=builder /ecoscale /ecoscale
COPY --from=builder /app/ui /ui

# Run as non-root
RUN adduser -D -g '' ecoscale
USER ecoscale

EXPOSE 8080

ENTRYPOINT ["/ecoscale"]
