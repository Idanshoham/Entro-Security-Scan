FROM golang:1.24.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod tidy

COPY . .

RUN go build -o server

FROM debian:bookworm-slim

WORKDIR /root/
COPY --from=builder /app/server .

RUN apt update && apt install -y libc6

EXPOSE 8080

CMD ["/root/server"]