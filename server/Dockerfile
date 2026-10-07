FROM golang:1.26.5 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o silkwrap-server ./server


FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /build/silkwrap-server /usr/local/bin/silkwrap-server

CMD ["silkwrap-server"]