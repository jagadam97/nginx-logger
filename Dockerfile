FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /nginx-logger

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /nginx-logger ./
COPY .env ./
CMD ["./nginx-logger"]
