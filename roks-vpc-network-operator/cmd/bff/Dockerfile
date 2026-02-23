# Multi-stage build
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy entire project (needed for replace directive in cmd/bff/go.mod)
COPY . .

# Build from the BFF module directory
WORKDIR /build/cmd/bff
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /build/bff ./main.go

# Final stage
FROM gcr.io/distroless/static-debian12

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bff .

# Expose port
EXPOSE 8443

# Run the BFF service
ENTRYPOINT ["/app/bff"]
