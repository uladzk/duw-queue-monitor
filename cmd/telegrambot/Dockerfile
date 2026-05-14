FROM golang:1.25.3-alpine3.22 AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -o bin/telegrambot ./cmd/telegrambot

FROM alpine:3.22 AS final

WORKDIR /app

COPY --from=build /app/bin/telegrambot .

CMD ["./telegrambot"]