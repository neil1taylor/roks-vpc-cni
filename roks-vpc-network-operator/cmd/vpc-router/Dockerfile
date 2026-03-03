# Stage 1: Build Go binary
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /workspace

# Copy go module files first for cache
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the vpc-router binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /vpc-router ./cmd/vpc-router/

# Stage 2: Runtime image (~50MB)
FROM alpine:3.20

RUN apk add --no-cache \
    nftables \
    dnsmasq \
    iproute2 \
    dhclient \
    && rm -rf /var/cache/apk/*

COPY --from=builder /vpc-router /vpc-router

ENTRYPOINT ["/vpc-router"]
