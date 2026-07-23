FROM golang:1.25.3-alpine3.22 AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=-s -o bin/duwdoctorchecker ./cmd/duwdoctorchecker

# Download and convert Certum OV CA certificate
# Required due to issues in validating cert during establishing TLS connection with DUW API: issue in DUW server configuration
RUN apk --no-cache add curl ca-certificates openssl && \
    curl -s http://repository.certum.pl/ovcasha2.cer -o /tmp/certum-ov-ca.cer && \
    openssl x509 -inform DER -in /tmp/certum-ov-ca.cer -out /usr/local/share/ca-certificates/certum-ov-ca.crt && \
    update-ca-certificates && \
    rm /tmp/certum-ov-ca.cer

FROM alpine:3.22 AS final

WORKDIR /app

COPY --from=build /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=build /app/bin/duwdoctorchecker .

CMD ["./duwdoctorchecker"]
