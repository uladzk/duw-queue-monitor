FROM golang:1.25.3-alpine3.22 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=-s -o bin/queuestatsreports ./cmd/queuestatsreports

FROM alpine:3.22 AS final

WORKDIR /app

COPY --from=build /app/bin/queuestatsreports .

ENTRYPOINT ["./queuestatsreports"]
