# AGENTS.md

**Project:** OpenGate - OpenAPI-powered API Gateway  
**Architecture:** Clean Architecture with Dependency Injection  
**Language:** Go 1.24.4  
**Last Updated:** 2026-01-29

---

## Quick Start

### Essential Commands

```bash
# Build
go build -o opengate ./cmd/opengate

# Run
./opengate --config=runtime/config.yml

# Development workflow
make test      # Run tests with race detector
make lint      # Run golangci-lint (must pass)
make mocks     # Generate mocks after interface changes
make fields    # Fix struct field alignment
make fmt       # Format code

# Before every commit
make lint && make test
```

### Pre-Commit Checklist

**IMPORTANT: Run these checks before committing. All must pass:**

1. `make lint` - Fix ALL linting issues until clean
2. `make test` - All tests must pass
3. `make fields` - Run if structs were modified
4. `make mocks` - Run if interfaces changed
5. `make fmt` - Format all code
6. Write tests for new code

### Pre-Merge Requirements

- CI pipeline passes (GitHub Actions)
- Code coverage maintained or improved
- All review comments addressed

---

## Project Overview

OpenGate is a production-ready API gateway demonstrating clean architecture with:
- Dependency injection (no global state)
- Interface-driven design
- Context propagation
- Structured logging (slog)
- Graceful shutdown
- Comprehensive testing

**Architecture Layers:**
- `cmd/opengate/` - Entry point (minimal, delegates to pkg/cmd)
- `pkg/cmd/` - CLI, config, dependency wiring
- `pkg/api/` - HTTP layer (handlers, middleware, routing)
- `pkg/core/` - Business logic (framework-independent)
- `pkg/repo/` - Data access layer
- `pkg/prov/` - External service providers

---

## Key Principles

### SOLID in Go

1. **Single Responsibility** - Each package has one clear purpose
2. **Open-Closed** - Use interfaces for extension points
3. **Liskov Substitution** - Mocks substitute real implementations
4. **Interface Segregation** - Small, focused interfaces
5. **Dependency Inversion** - High-level modules define interfaces, low-level implement them

### Go Best Practices

- Clear is better than clever
- Accept interfaces, return structs
- Wrap errors with `%w` at every layer
- Pass context to all operations
- Don't panic - return errors explicitly
- Define interfaces at the consumer (not provider)

---

## Code Patterns

### 1. Dependency Injection

Dependencies are created bottom-up and injected via constructors in `pkg/cmd/server.go`:

```go
// Create dependencies
rdb := redis.NewClient(&redis.Options{...})
someAPI := someapi.New(someapi.Config{...})
userRepo := user.New(rdb)

// Wire together
svc := core.New(userRepo, someAPI)
apiSvc, err := api.New(cfg.API, svc)
```

**Key:** No global state, clear dependency graph, enables testing with mocks.

### 2. Interface Definition

Interfaces are defined by **consumers**, not providers:

```go
// pkg/api/api.go - API layer defines what it needs
type Service interface {
    CheckHealth(ctx context.Context) error
}

// pkg/core/svc.go - Core defines what it needs
type userRepo interface {
    CheckHealth(ctx context.Context) error
}
```

**Benefit:** Prevents circular dependencies, enables duck typing.

### 3. Constructors

All components use `New()` constructors:

```go
// Simple
func New(users userRepo, someAPI someAPIProv) *Service {
    return &Service{users: users, someAPI: someAPI}
}

// With validation
func New(cfg Config, svc Service) (*API, error) {
    if cfg.Listen == "" {
        return nil, fmt.Errorf("listen address must be specified")
    }
    return &API{svc: svc, config: cfg}, nil
}
```

### 4. Error Handling

**Always wrap errors with `%w` and add context:**

```go
if err := res.Err(); err != nil {
    return fmt.Errorf("fail to check health for user repo: %w", err)
}
```

**Log at the boundary (HTTP handlers), not in business logic:**

```go
// In handler - log and handle
func (a *API) healthCheck(w http.ResponseWriter, r *http.Request) {
    if err := a.svc.CheckHealth(r.Context()); err != nil {
        slog.ErrorContext(r.Context(), "Health check failed", "error", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
}

// In service - wrap and return (don't log)
func (s *Service) CheckHealth(ctx context.Context) error {
    if err := s.users.CheckHealth(ctx); err != nil {
        return fmt.Errorf("user repo health check failed: %w", err)
    }
    return nil
}
```

### 5. Context Propagation

Pass context through all operations:

```go
func (s *Service) CheckHealth(ctx context.Context) error {
    eg, ctx := errgroup.WithContext(ctx)
    
    eg.Go(func() error { return s.someAPI.CheckHealth(ctx) })
    eg.Go(func() error { return s.users.CheckHealth(ctx) })
    
    return eg.Wait()
}
```

---

## Testing

### Philosophy

1. Use testify for all assertions
2. Table-driven tests for multiple scenarios
3. Mock external dependencies
4. Test behavior, not implementation

### Test Structure

```go
func TestService_CheckHealth(t *testing.T) {
    tests := []struct {
        name       string
        setupMocks func(t *testing.T, users *MockuserRepo, someAPI *MocksomeAPIProv)
        wantErr    bool
    }{
        {
            name: "Success",
            setupMocks: func(t *testing.T, users *MockuserRepo, someAPI *MocksomeAPIProv) {
                t.Helper()
                users.EXPECT().CheckHealth(mock.Anything).Return(nil)
                someAPI.EXPECT().CheckHealth(mock.Anything).Return(nil)
            },
            wantErr: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            users := NewMockuserRepo(t)
            someAPI := NewMocksomeAPIProv(t)
            tt.setupMocks(t, users, someAPI)
            
            svc := New(users, someAPI)
            err := svc.CheckHealth(context.Background())
            
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Running Tests

```bash
make test                    # Run all tests with race detector
go test -v ./...             # Verbose output
go test -cover ./...         # With coverage
go test -run TestName ./pkg  # Specific test
```

---

## Adding New Features

### Adding a New HTTP Endpoint

1. Add handler in `pkg/api/handlers.go`:
   ```go
   func (a *API) getUserHandler(w http.ResponseWriter, r *http.Request) {
       userID := r.PathValue("id")
       user, err := a.svc.GetUser(r.Context(), userID)
       if err != nil {
           slog.ErrorContext(r.Context(), "Failed to get user", "error", err, "user_id", userID)
           http.Error(w, "Internal Server Error", http.StatusInternalServerError)
           return
       }
       w.Header().Set("Content-Type", "application/json")
       json.NewEncoder(w).Encode(user)
   }
   ```

2. Add method to `api.Service` interface in `pkg/api/api.go`:
   ```go
   type Service interface {
       GetUser(ctx context.Context, userID string) (*User, error)
   }
   ```

3. Register route in `pkg/api/mux.go`:
   ```go
   mux.Handle("GET /users/{id}", middleware.Use(a.getUserHandler, withReqID))
   ```

4. Implement in `pkg/core/svc.go`
5. Add interface method to `core.userRepo`
6. Implement in `pkg/repo/user/user_repo.go`
7. **Run `make mocks`** to regenerate mocks
8. **Write tests for all layers**

### Adding a New Repository

1. Create package: `pkg/repo/product/product_repo.go`
2. Define DAO interface and implement repository:
   ```go
   type productDAO interface {
       Get(ctx context.Context, key string) *redis.StringCmd
   }
   
   type ProductRepo struct { dao productDAO }
   
   func New(dao productDAO) *ProductRepo {
       return &ProductRepo{dao: dao}
   }
   ```

3. Add interface to `pkg/core/svc.go`:
   ```go
   type productRepo interface {
       GetByID(ctx context.Context, productID string) (*Product, error)
   }
   ```

4. Update `.mockery.yaml` with new interface
5. Wire in `pkg/cmd/server.go`:
   ```go
   productRepo := product.New(rdb)
   svc := core.New(userRepo, productRepo, someAPI)
   ```

6. **Run `make mocks`**
7. **Write tests**

### Adding a New Provider

1. Create package: `pkg/prov/payment/client.go`
2. Implement client with `Config` struct and `New()` constructor
3. Add interface to `pkg/core/svc.go`
4. Update `pkg/cmd/config.go` with new config struct
5. Update `.mockery.yaml`
6. Wire in `pkg/cmd/server.go`
7. Update `runtime/config.yml` with configuration
8. **Run `make mocks`**
9. **Write tests**

### Adding Middleware

1. Create in `pkg/api/middleware/`:
   ```go
   type keyUserID struct{}
   
   func NewAuth(authService AuthService) func(http.Handler) http.Handler {
       return func(next http.Handler) http.Handler {
           return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
               token := r.Header.Get("Authorization")
               userID, err := authService.ValidateToken(r.Context(), token)
               if err != nil {
                   http.Error(w, "Unauthorized", http.StatusUnauthorized)
                   return
               }
               ctx := context.WithValue(r.Context(), keyUserID{}, userID)
               next.ServeHTTP(w, r.WithContext(ctx))
           })
       }
   }
   ```

2. Apply in `pkg/api/mux.go`:
   ```go
   withAuth := middleware.NewAuth(a.authSvc)
   mux.Handle("GET /users/{id}", middleware.Use(a.getUserHandler, withReqID, withAuth))
   ```

---

## Configuration

### Viper Configuration Hierarchy

1. Default values (in flag definitions) - lowest priority
2. Config file (`runtime/config.yml`)
3. Environment variables - highest priority

### Environment Variable Format

Format: `SECTION_KEY=value`

```bash
# Override API listen address
export API_LISTEN=:9090

# Override Redis address
export REDIS_ADDR=localhost:6379

# Nested keys use underscores
export PROVIDER_SOME_API_BASE_URL=http://other.com
```

### Adding New Configuration

1. Add struct in `pkg/cmd/config.go`:
   ```go
   type appConfig struct {
       NewFeature NewFeatureConfig
   }
   
   type NewFeatureConfig struct {
       Setting1 string `mapstructure:"setting1"`
   }
   ```

2. Update `runtime/config.yml`:
   ```yaml
   new_feature:
     setting1: value1
   ```

3. Use in `pkg/cmd/server.go`

---

## Logging

### Use slog for Structured Logging

```go
// Info level
slog.Info("Server starting", "addr", cfg.Listen)

// With context (includes request ID from middleware)
slog.ErrorContext(ctx, "Health check failed", "error", err)

// Include relevant context
slog.Error("Failed to get user", "error", err, "user_id", userID)
```

### Log Levels

```bash
# Set via flag
./opengate --log-level=debug

# Or environment variable
export LOG_LEVEL=debug
```

**Never log sensitive information (passwords, tokens, etc.)**

---

## Development Tools

### Required Tools

```bash
go version  # 1.24.4 or later
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/vektra/mockery/v2@latest
go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest
```

### Mock Generation

**Configuration:** `.mockery.yaml`

Mocks are:
- Auto-generated (never edit manually)
- Excluded from production builds (`//go:build !compile`)
- Committed to repository

**Generate:** `make mocks` or `mockery`

### Linting

**Configuration:** `.golangci.yml` (30+ linters enabled)

**Critical linters:**
- `bodyclose` - HTTP response bodies closed
- `errcheck` - Unchecked errors
- `govet` - Includes fieldalignment
- `staticcheck` - Static analysis
- `gosec` - Security checks

**Run:** `make lint` - **Must pass before committing**

### Field Alignment

Optimizes struct memory layout. Run `make fields` before committing if linter complains about struct alignment.

---

## Naming Conventions

### Files

- Source: `snake_case.go` (e.g., `user_repo.go`)
- Tests: `*_test.go` (e.g., `user_repo_test.go`)
- Mocks: `*_mock.go` (auto-generated)

### Interfaces

- No "I" prefix: `Service`, not `IService`
- Private interfaces: lowercase (e.g., `userRepo`, `someAPIProv`)
- Public interfaces: exported (e.g., `Service`)

### Error Messages

- Lowercase, no punctuation: `"failed to connect to redis"`
- Add context: `"failed to get user: %w"`
- Consistent prefixes: `"failed to..."`, `"invalid..."`, `"missing..."`

### Context Keys

Always use empty struct types:
```go
type keyReqID struct{}
```

Never use strings or primitives as context keys.

---

## CI/CD

### GitHub Actions Workflows

**1. tests.yml** - Runs on push to main and PRs:
- Checkout, setup Go 1.24.4
- Run golangci-lint
- Build application
- Run tests with race detector
- Generate coverage, upload to Codecov

**2. docker.yml** - Runs on push to main and tags:
- Build multi-arch image (amd64, arm64)
- Push to ghcr.io
- Tag with version or commit SHA

### Docker

```bash
# Build
docker build -t opengate:latest .

# Run
docker run -p 8080:8080 -v $(pwd)/runtime:/runtime opengate:latest --config=/runtime/config.yml
```

---

## Troubleshooting

### Common Issues

**1. Linter errors about field alignment**
```bash
make fields  # Fix automatically
```

**2. Tests fail with "unexpected method call"**
- Check mock expectations match actual calls
- Verify `EXPECT()` covers all calls
- Use `mock.Anything` for flexible matching

**3. Cannot find mock files**
```bash
make mocks  # Regenerate
```

**4. Configuration not loading**
- Verify config file path
- Check YAML syntax
- Run with `--log-level=debug` to see config loading

---

## Best Practices Summary

### DO

✅ Use dependency injection (no global state)  
✅ Define interfaces at consumer (not provider)  
✅ Wrap errors with `%w`  
✅ Pass context to all operations  
✅ Use table-driven tests  
✅ Run `make lint && make test` before every commit  
✅ Log at boundary (handlers), not in business logic  
✅ Use structured logging (slog)  
✅ Write tests for new code  

### DON'T

❌ Use global state  
❌ Ignore errors  
❌ Use `panic` for normal errors  
❌ Log and return errors (choose one)  
❌ Manually write mocks  
❌ Use string context keys  
❌ Commit without running lint and test  
❌ Modify generated mock files  

---

## Quick Reference

### HTTP Status Codes

- `200 OK` - Success
- `400 Bad Request` - Invalid input
- `401 Unauthorized` - Authentication required
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

### Common Patterns

**Middleware order** - Apply in reverse (last listed runs first)  
**Context values** - Use typed struct keys, provide getter functions  
**Concurrent operations** - Use `errgroup.WithContext(ctx)`  

---

## Additional Resources

- [Go Proverbs](https://go-proverbs.github.io/)
- [Effective Go](https://go.dev/doc/effective_go)
- [testify Documentation](https://pkg.go.dev/github.com/stretchr/testify)
- [mockery Documentation](https://vektra.github.io/mockery/)
