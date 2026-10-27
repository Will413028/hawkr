# Shared Components

Cross-domain components that **carry product semantics** but are reused across
multiple features. Examples:

- `BrandLogo` – Hawkr wordmark / logomark
- `RoleAwareNavbar` – top nav with login + role-aware right rail
- `LoginRequiredModal` – auth-prompt modal
- `AppIcon` – the only static icon entrypoint (semantic name to Material
  Symbols, drawn by `@hawkr/icons`)
- `Pagination` – list pagination
- `ProductCard` – product card preview (used in market, store page, search)

## What does NOT belong here

- **Pure UI primitives** – `Button`, `Card`, `Input`, `Modal` etc. live in
  `components/ui/` because they have no product semantics.
- **Feature-only components** – if only one feature uses it, it belongs in
  `features/<feature>/components/`.

## Migration

Files currently in `components/<X>.tsx` get moved here once we confirm they
are used by ≥ 2 features. Single-feature components migrate into the feature
module instead.
