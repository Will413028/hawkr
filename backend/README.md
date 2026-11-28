# Hawkr backend

The Hawkr API server: a Go modular monolith (`cmd/hawkr`) serving the Connect API defined in [`api/`](../api/) to the website, admin console, and mobile apps. Services that have not moved into the monolith yet run in `services/` behind the REST gateway in `gateway/`; the monolith forwards their routes.

## Architecture

| Path | Role |
|---|---|
| `cmd/hawkr` | The single deployed binary and the only place modules are wired together (`serve`, `worker`, `migrate`, operational subcommands) |
| `internal/modules/<name>` | Business modules; each owns its PostgreSQL schema, exposes its API from the root package, and keeps the implementation in its own `internal/` |
| `internal/httpapi` | Connect handlers: transport, authorization declarations, and read composition for screens |
| `internal/platform` | Shared infrastructure (database, events, jobs, errors, UI text) that depends on no module |
| `internal/arch` | Tests that enforce the module dependency graph |
| `gateway/`, `services/` | Services not yet moved into the monolith, and their REST gateway |

Module specs live in [`docs/modules/`](docs/modules/); the allowed dependencies between modules are defined in `internal/arch/modules.go`. The rules every change must follow are in [`AGENTS.md`](AGENTS.md).

Storage and runtime: PostgreSQL, River for background jobs, S3-compatible object storage for media.

## Prerequisites

- Go (version in [`.go-version`](.go-version))
- Docker, for local infrastructure, code generators, and integration tests
- libvips (`brew install vips` or `apt-get install libvips-dev`); on macOS also `export PKG_CONFIG_PATH="$(brew --prefix libarchive)/lib/pkgconfig"`

## Run locally

```bash
make infra-up              # PostgreSQL and supporting services
make migrate
make serve                 # HTTP on :8080, /healthz and /readyz
make worker                # background jobs
make dev-seed              # reset test data
make dev-token args='-account 2001'          # customer session token
make dev-token args='-admin -user dev_ops'   # admin token
```

`go run ./cmd/hawkr` lists every subcommand.

## Test and check

```bash
GOWORK=off go test -race ./...                     # unit tests, including dependency checks
GOWORK=off go test -race -tags integration ./...   # integration tests (Docker)
make lint-monolith
```

CI runs the same checks from [`.github/workflows/backend-*.yml`](../.github/workflows/).

## Code generation

Generated code is never edited by hand.

| After changing | Run |
|---|---|
| `api/**/*.proto` | `make connect` |
| a module's `db/schema.sql`, `objects.sql`, or `queries/` | `make sqlc` |
| a module's `db/schema.sql` (migration) | `make atlas-diff module=<name> name=<migration>` |
| the UI text catalog or a module's `text/*.json` | `make uitext` |

New modules start from `make new-module`.

## Documentation

- [`AGENTS.md`](AGENTS.md): monolith structure and rules
- [`docs/modules/`](docs/modules/): per-module specifications
- [`docs/plans/`](docs/plans/): migration plans and progress
- [`../docs/adr/`](../docs/adr/): architecture decision records
