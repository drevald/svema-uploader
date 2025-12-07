# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies for Fyne
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o svema-uploader .

# Final stage - minimal runtime image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates libstdc++ libgcc

# Copy binary from builder
COPY --from=builder /app/svema-uploader .

# Run the application
CMD ["./svema-uploader"]
