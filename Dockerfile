# Copyright (C) 2026 Прокофьев Даниил <danieldzen@yandex.ru>
# Лицензировано под GNU Affero General Public License v3.0
# Часть проекта генератора рассадок
FROM golang:1.24.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o seating-server main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/seating-server .

EXPOSE 8080

CMD ["./seating-server"]