# opengate

[![Tests](https://github.com/ksysoev/opengate/actions/workflows/tests.yml/badge.svg)](https://github.com/ksysoev/opengate/actions/workflows/tests.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ksysoev/opengate)](https://goreportcard.com/report/github.com/ksysoev/opengate)
[![Go Reference](https://pkg.go.dev/badge/github.com/ksysoev/opengate.svg)](https://pkg.go.dev/github.com/ksysoev/opengate)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

OpenAPI-powered API gateway with automatic routing, request validation, and spec-driven configuration.

## Installation

## Building from Source

```sh
RUN CGO_ENABLED=0 go build -o opengate -ldflags "-X main.version=dev -X main.name=opengate" ./cmd/opengate/main.go
```

### Using Go

If you have Go installed, you can install opengate directly:

```sh
go install github.com/ksysoev/opengate/cmd/opengate@latest
```


## Using

```sh
opengate --log-level=debug --log-text=true --config=runtime/config.yml
```

## License

opengate is licensed under the MIT License. See the LICENSE file for more details.
