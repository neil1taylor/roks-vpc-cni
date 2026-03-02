FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
COPY api/ api/
COPY pkg/ pkg/
COPY cmd/metrics-exporter/ cmd/metrics-exporter/
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /metrics-exporter ./cmd/metrics-exporter/

FROM scratch
COPY --from=builder /metrics-exporter /metrics-exporter
EXPOSE 9100
ENTRYPOINT ["/metrics-exporter"]
