# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /server ./cmd/server

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /server .

# Create directory for SQLite and WhatsApp session (if needed)
RUN mkdir -p /app/data /app/whatsapp-session

# Expose port (Railway/Render sets PORT env var)
EXPOSE 8000

# Railway uses PORT env var, start server
CMD ["./server"]
