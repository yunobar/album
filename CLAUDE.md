# Agents

## Verification

After making code changes, verify the implementation using:

```bash
make lint
make build-all
make test-local
```

Do not use `go build` or `go test` directly.

## Project Structure

```text
cmd/
  http/       → HTTP server entrypoint
  job/        → One-off jobs (migrations, asset sync)
internal/
  appconstant/   → Application-wide constants and enums
  core/          → Framework/infra layer (config, logger, otel, services like cache/mail)
  domain/
    client/      → External API clients (interfaces + implementations that call third-party endpoints)
    dto/         → Request/response data transfer objects
    entity/      → Domain entities (DB models via go-crud)
    mapper/      → Entity ↔ DTO conversion functions
    repository/  → Data access layer (repository interfaces + implementations)
    service/     → Business logic (service interfaces + implementations)
  adapters/
    http/        → HTTP server, routes, handlers, middlewares
    db/          → Database setup (postgres)
    job/         → Job implementations
  provider/      → Dependency injection / wiring
```

## Conventions

### Architecture

- Clean Architecture: domain layer has no dependency on adapters or framework.
- All wiring happens in `internal/provider/` — constructors use plain dependency injection (no DI container).

### Naming

- Service implementations: `<name>ServiceImpl` struct, `New<Name>Service` constructor.
- External API clients: anything that makes requests to a third-party endpoint (HTTP, etc.) is named `<Name>Client` / `<name>_client.go` and lives in `internal/domain/client/`, never `internal/domain/service/`. `<name>Client` struct, `New<Name>Client` constructor. Name the client after the provider (`TMDBClient`, `TurnstileClient`), not the domain concept it serves — services in `service/` consume the client interface for their business logic.
- Handlers: `<Name>Handler` struct with `Handle<Action>()` methods returning `gin.HandlerFunc`.
- DTOs: `<Action>Request` / `<Action>Response` in the `dto` package.
- Mappers: standalone functions in `mapper/`, named `<Entity>To<DTO>` or `<DTO>To<Entity>`.

### Error Handling

- Use `github.com/itsLeonB/ungerr` for typed errors (`UnauthorizedError`, `NotFoundError`, `Unknown`, `Wrap`).
- Never return raw `errors.New()` from domain services — always use `ungerr`.

### Testing

- Tests use `testify/assert` and `testify/require`.
- Test files use same package (e.g., `package middlewares`) with `_test.go` file suffix.
- Use table-driven tests for stateless input→output functions (utilities, mappers).
- Use `github.com/vektra/mockery` for generating mocks.
- DB-touching packages run sequentially (`-p 1`); the rest run in parallel. See Makefile `test` target.

#### Test categories

| Layer | Package | What's real | What's mocked |
|-------|---------|-------------|---------------|
| Repository | `internal/domain/repository/` | Test DB | — |
| Client | `internal/domain/client/` | Request/response mapping logic | External endpoint (`httptest.Server`, or mockery for consumers) |
| Service | `internal/domain/service/` | Service logic | Repos, clients, core services (mockery) |
| Mapper | `internal/domain/mapper/` | Stateless in/out | — |
| Core | `internal/core/*/` | Core service logic | External deps (mockery) |
| Handler | `internal/adapters/http/handler/` | Gin test context | Services (mockery) |
| Feature | `internal/tests/` | Test DB, repos, services, router | Auth (fake MW), external HTTP/mail/LLM |

#### Integration Tests (DB)

Integration tests run against a real Postgres database (`album_test`). Config is in `.env.test` with `TEST_DB_*` vars.

**Shared helpers** live in `internal/testhelpers/testhelpers.go`:

- `SetupTestDB(envPath string) (*gorm.DB, func(), error)` — loads env, connects, runs migrations. Returns a cleanup func to close the connection.
- `RequireTestDB(t, db)` — call at the top of any DB test; skips with `t.Skip` if DB is nil (setup failed).
- `TruncateAll(t, db)` — truncates all tables (except `goose_db_version`) between tests for isolation.

**TestMain pattern** (one per package with DB tests):

```go
var testDB *gorm.DB

func TestMain(m *testing.M) {
    db, cleanup, err := testhelpers.SetupTestDB("../../../.env.test")
    if err != nil {
        fmt.Fprintf(os.Stderr, "skipping DB tests: %v\n", err)
    } else {
        testDB = db
        defer cleanup()
    }
    m.Run()
}
```

**Test function pattern**:

```go
func TestSomethingRepository_Method(t *testing.T) {
    testhelpers.RequireTestDB(t, testDB)

    t.Run("case name", func(t *testing.T) {
        testhelpers.TruncateAll(t, testDB)
        // ... test logic with DB assertions ...
    })
}
```

Key points:
- `TestMain` runs once per package. If DB is unavailable, tests skip gracefully instead of failing.
- Each subtest calls `TruncateAll` to start with a clean slate — tests don't depend on execution order.
- Adjust the relative path to `.env.test` based on package depth from project root.

#### Feature Tests (`internal/tests/`)

Feature tests exercise the full request→response flow with real DB, real services, and real routing — only external boundaries are faked.

**Setup** (`setup_test.go`):
- Wires real repos and services against the test DB.
- Uses a fake auth middleware that injects a test user (bypasses JWT validation).
- Registers routes with inline thin handlers to avoid importing the full handler dependency tree.

**Test pattern**:

```go
func TestSomeEndpoint(t *testing.T) {
    testhelpers.RequireTestDB(t, testDB)

    t.Run("case name", func(t *testing.T) {
        testhelpers.TruncateAll(t, testDB)

        // Seed data
        testDB.Create(&entity.Something{...})

        // Make request
        w := httptest.NewRecorder()
        req, _ := http.NewRequest(http.MethodGet, "/api/v1/something", nil)
        testRouter.ServeHTTP(w, req)

        // Assert response
        assert.Equal(t, http.StatusOK, w.Code)
        // ... unmarshal and verify JSON body ...
    })
}
```

Key points:
- Use `assert.ElementsMatch` for list responses where DB ordering is not guaranteed.
- Seed test data directly via `testDB.Create(...)` — no fixtures or factories needed.
- Mock only what crosses a network boundary (HTTP clients, mail, LLM, NATS).

### Dependencies (key libraries)

- HTTP framework: `gin-gonic/gin`
- ORM: `github.com/itsLeonB/go-crud` (wraps GORM with `BaseEntity`, `Transactor`)
- Auth: `github.com/itsLeonB/go-authkit`
- Validation: gin's `binding` tags on DTOs
- Decimal: `github.com/shopspring/decimal`
- UUID: `github.com/google/uuid`
- Config: `github.com/kelseyhightower/envconfig` (via `split_words` / `default` struct tags)
- Observability: OpenTelemetry (`internal/core/otel`)

### Configuration

- All config is loaded from environment variables (struct tags in `internal/core/config/`).
- `.env` is auto-loaded via `github.com/joho/godotenv/autoload`.
- See `.env.example` for required variables.
