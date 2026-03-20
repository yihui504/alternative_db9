FROM golang:1.23-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod ./
RUN go mod download && go mod verify

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl postgresql-client

WORKDIR /root/

COPY --from=builder /app/api .

EXPOSE 8080

CMD ["./api"]
