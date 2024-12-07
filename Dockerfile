FROM golang:alpine
WORKDIR /app

COPY go.mod go.sum ./
COPY .env

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /nginx-logger

CMD ["/nginx-logger"]
