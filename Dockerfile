FROM golang:1.24.4-alpine AS builder

ARG VERSION=${VERSION}

WORKDIR /app

COPY . .
RUN go mod download

RUN CGO_ENABLED=0 go build -o opengate -ldflags "-X main.version=$VERSION -X main.name=opengate" ./cmd/opengate/main.go

FROM scratch

COPY --from=builder /app/opengate .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/opengate"]
