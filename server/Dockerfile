FROM golang:1.26.5 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o filetransfer-server ./server

FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /app/filetransfer-server .

CMD ["./filetransfer-server"]
