# Hawkr

An e-commerce platform for buyers, sellers, and platform operators: a Go backend, two Next.js web apps, and two Flutter apps sharing one Protobuf API contract.

## Architecture

```
buyer-app / seller-app (Flutter) ─┐
website (Next.js, buyers+sellers) ─┼─ Connect (api/) ─▶ backend (Go modular monolith) ─▶ PostgreSQL
admin (Next.js, operators) ───────┘
```

- **API contract:** `api/` holds the Protobuf + [Connect](https://connectrpc.com) definitions. Every client uses code generated from it (`packages/hawkr-api-ts`, `packages/hawkr_api`); nobody hand-writes API calls.
- **Backend:** a Go modular monolith with enforced dependency directions (`backend/internal/modules/`). Services not yet moved into it run behind a REST gateway in `backend/services/` and `backend/gateway/`.
- **Shared packages:** UI text catalog, icons, Taiwan address codes, and native sign-in live in `packages/`, generated from a single source where possible.

## Repository layout

| Path | What it is |
|---|---|
| [`backend/`](backend/) | Go modular monolith |
| [`website/`](website/) | Next.js site for buyers and sellers |
| [`admin/`](admin/) | Next.js operator console |
| [`buyer-app/`](buyer-app/) | Flutter buyer app |
| [`seller-app/`](seller-app/) | Flutter seller app |
| [`api/`](api/) | Protobuf + Connect contract |
| [`packages/`](packages/) | Shared TypeScript and Dart packages |
| [`docs/`](docs/) | Architecture decision records and cross-project conventions |
| [`mutation/`](mutation/) | Mutation-testing manifests run in CI |

## Getting started

Tool versions are pinned in the repository: Go in `backend/.go-version`, Node in `.nvmrc`, pnpm in `package.json` (`packageManager`), Flutter in each app's `.fvmrc`, buf in `api/.buf-version`.

```bash
pnpm install        # website and admin share one pnpm workspace
```

Each project's README has its own setup and run commands:
[backend](backend/README.md) · [website](website/README.md) · [admin](admin/README.md) · [buyer-app](buyer-app/README.md) · [seller-app](seller-app/README.md)

## Development

- **Checks:** CI lives in [`.github/workflows/`](.github/workflows/); run the same checks locally before pushing. Each project's README lists them.
- **Changing the API:** edit `api/`, then regenerate every client in the same change (`make connect` in `backend/`, `pnpm --filter @hawkr/api gen:api`, `../../api/generate.sh buf.gen.yaml` in `packages/hawkr_api`).
- **Conventions:** [`AGENTS.md`](AGENTS.md) holds the cross-project rules; decisions are recorded in [`docs/adr/`](docs/adr/).
- **Branches:** work lands on `dev` through pull requests; `main` only receives `dev` via merge commits.

## Security

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## Third-party assets

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
