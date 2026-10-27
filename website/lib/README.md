# Lib Layering

`lib/` is the application infrastructure layer. Feature modules can import
from it, but `lib/` must not import from `app/`, `features/`, or `components/`.

## Layers

```txt
lib/
  api/          # backend base URL, typed public API loaders, BFF client helpers
  auth/         # Better Auth server setup, client context, redirect/storage helpers
  cart/         # cart orchestrator, repo, guest/saved helpers, cart types
  config/       # environment and deployment flavor
  content/      # static/mock editorial and market content
  hooks/        # shared client hooks that are not owned by one feature yet
  media/        # CDN URL helpers, upload helpers, public asset manifests
  server/       # server-only auth/session helpers
  services/     # use-case facades; prefer these for cross-feature workflows
  theme/        # site-wide theme provider and boot script
  utils/        # tiny framework-neutral utilities
  <domain>/     # domain repositories and pure helpers
```

## Import Rules

- Route files and feature code may import public facades like `@/lib/api`,
  `@/lib/cart`, `@/lib/services/orders`, or specific domain helpers.
- New backend calls should prefer `lib/api/client.ts` or a domain repo instead
  of ad-hoc `fetch`.
- Root-level facade files should live in their domain folder as `index.ts`.
  For example, `@/lib/api` resolves to `lib/api/index.ts`.
- `lib/` root is navigation only. New files must be created inside an owned
  subfolder.
- Domain repos should be data access only. UI state, screen composition, and
  React components belong in `features/` or `components/`.
- `lib/hooks` is transitional. When a hook becomes feature-specific, move it to
  `features/<domain>/hooks`.
- `lib/` must not own runtime assets. Imported assets belong in `assets/`, and
  URL-served static files belong in `public/`. Public URL manifests belong in
  `lib/media/public-assets.ts`.
