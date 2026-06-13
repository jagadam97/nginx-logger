# Pin the toolchain to the 1.25 line so builds match go.mod (go 1.25.3) and stay reproducible.
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /nginx-logger

FROM alpine:latest
# ca-certificates is required for the HTTPS InfluxDB connection (explicit so it
# doesn't depend on the base image happening to include the bundle).
RUN apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=builder /nginx-logger ./
COPY --from=builder /app/frontend ./frontend
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -q --spider "http://localhost:${API_PORT:-8080}/api/health" || exit 1
CMD ["./nginx-logger"]
