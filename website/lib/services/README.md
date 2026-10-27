# Services Layer

`lib/services/` is the stable, typed domain API surface. Components and hooks
should import from here, not from individual `lib/<domain>/repo.ts` files.

## Goal

```txt
component → hook → service → apiClient
```

Services own:

- Endpoint paths (e.g. `/app/products?view=manage`)
- Request body shape (camelCase ↔ snake_case adapters)
- Response normalisation (`raw → typed`)
- Idempotency keys / retry policy / cache invalidation hooks

Services do **not** own:

- React state (that's hooks' job)
- UI rendering (components' job)

## Migration

Today, repository code lives at `lib/<domain>/repo.ts` (e.g.
`lib/products/seller-repo.ts`, `lib/seller/shipment.ts`). They will be
gradually unified under `lib/services/<domain>.ts` with a thin, stable
public surface.

```txt
lib/services/
  products.ts      # was lib/products/{repo,seller-repo}.ts
  sellers.ts       # was lib/seller/repo.ts
  orders.ts        # was lib/orders/repo.ts
  shipments.ts     # was lib/seller/shipment.ts
  certifications.ts
  auth.ts          # was lib/auth/index.ts
```

Repos may still exist underneath as the implementation; services re-export
them with a curated public API.
