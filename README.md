# OpenGate

[![Tests](https://github.com/ksysoev/opengate/actions/workflows/tests.yml/badge.svg)](https://github.com/ksysoev/opengate/actions/workflows/tests.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ksysoev/opengate)](https://goreportcard.com/report/github.com/ksysoev/opengate)
[![Go Reference](https://pkg.go.dev/badge/github.com/ksysoev/opengate.svg)](https://pkg.go.dev/github.com/ksysoev/opengate)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

OpenGate is a production-ready API gateway that uses OpenAPI specifications for automatic routing and request forwarding. Configure your API routes declaratively using standard OpenAPI 3.x JSON files, and OpenGate handles the proxying to your backend services.

## Features

- **OpenAPI-driven routing** - Define routes using standard OpenAPI 3.x specifications
- **Dynamic request forwarding** - Automatically proxy requests to backend services based on OpenAPI configuration
- **Path parameter extraction** - Full support for path parameters like `/users/{id}`
- **Request ID tracking** - Built-in middleware for request correlation
- **Structured logging** - Using Go's standard `log/slog` package
- **Clean architecture** - Dependency injection, interface-driven design, comprehensive testing
- **Production-ready** - Graceful shutdown, context propagation, error handling

## Quick Start

### Installation

#### Using Go

```sh
go install github.com/ksysoev/opengate/cmd/opengate@latest
```

#### Building from Source

```sh
git clone https://github.com/ksysoev/opengate.git
cd opengate
go build -o opengate ./cmd/opengate
```

### Configuration

Create a configuration file `config.yml`:

```yaml
api:
  listen: :8080

gateway:
  spec_path: gateway.json
```

Create an OpenAPI specification file `gateway.json`:

```json
{
  "openapi": "3.1.0",
  "info": {
    "version": "1.0.0",
    "title": "My API Gateway"
  },
  "paths": {
    "/api/users": {
      "get": {
        "summary": "Get all users",
        "operationId": "get-users",
        "x-opengate": {
          "handler": {
            "options": {
              "baseUrl": "http://localhost:3000"
            }
          }
        }
      }
    },
    "/api/users/{id}": {
      "get": {
        "summary": "Get user by ID",
        "operationId": "get-user",
        "x-opengate": {
          "handler": {
            "options": {
              "baseUrl": "http://localhost:3000"
            }
          }
        }
      }
    }
  }
}
```

### Running

```sh
opengate --config=config.yml
```

Or with custom logging:

```sh
opengate --config=config.yml --log-level=debug --log-text=true
```

## OpenAPI Configuration

OpenGate uses the `x-opengate` extension in your OpenAPI specification to configure routing.

### Extension Format

```json
{
  "paths": {
    "/your/path": {
      "get": {
        "operationId": "unique-operation-id",
        "x-opengate": {
          "handler": {
            "options": {
              "baseUrl": "http://your-backend-service:8080"
            }
          }
        }
      }
    }
  }
}
```

### Supported HTTP Methods

- GET
- POST
- PUT
- DELETE
- PATCH

### Path Parameters

OpenGate fully supports OpenAPI path parameters:

```json
{
  "paths": {
    "/users/{userId}/posts/{postId}": {
      "get": {
        "parameters": [
          {"name": "userId", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "postId", "in": "path", "required": true, "schema": {"type": "string"}}
        ],
        "x-opengate": {
          "handler": {
            "options": {
              "baseUrl": "http://backend:3000"
            }
          }
        }
      }
    }
  }
}
```

A request to `/users/123/posts/456` will be proxied to `http://backend:3000/users/123/posts/456`.

## Architecture

OpenGate follows clean architecture principles with clear separation of concerns:

```
cmd/opengate/          - Entry point
pkg/
  ├── api/             - HTTP server layer
  ├── core/            - Business logic
  │   ├── router/      - Dynamic routing engine
  │   └── proxy/       - HTTP request forwarding
  ├── spec/            - OpenAPI specification parsing
  └── cmd/             - CLI and configuration
```

### Key Components

- **Spec Parser** - Parses OpenAPI 3.x JSON specifications
- **Router** - Matches incoming requests to routes with regex-based path matching
- **Proxy Handler** - Forwards requests to backend services with proper headers
- **Middleware** - Request ID tracking, logging, and more

## Development

### Prerequisites

- Go 1.24.4 or later
- golangci-lint
- mockery (for generating mocks)

### Building

```sh
make build
```

### Testing

```sh
make test      # Run tests with race detector
make lint      # Run linters
make mocks     # Generate mocks
```

### Before Committing

```sh
make lint && make test
```

## Configuration Reference

### Environment Variables

Configuration can be overridden with environment variables:

```sh
export API_LISTEN=:9090
export GATEWAY_SPEC_PATH=/path/to/spec.json
export LOG_LEVEL=debug
```

### Command-Line Flags

```
--config          Path to configuration file
--log-level       Log level (debug, info, warn, error)
--log-text        Use text logging instead of JSON
```

## Examples

See the `runtime/` directory for example configurations:

- `runtime/config.yml` - Example configuration file
- `runtime/gateway.json` - Example OpenAPI specification with multiple routes

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass (`make test`)
2. Code is properly formatted (`make fmt`)
3. Linting passes (`make lint`)
4. New code includes tests

## License

OpenGate is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
