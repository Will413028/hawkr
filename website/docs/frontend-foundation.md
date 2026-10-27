# Frontend Foundation

This document defines the first-pass ownership rules for the shared frontend
state, form, URL, i18n, theme, monitoring, and testing packages.

Adoption order and migration targets are tracked in
`docs/planning/FRONTEND_FOUNDATION_ROADMAP.md`.

## State Ownership

- Server state belongs in TanStack Query.
- Client-only shared UI state belongs in Zustand when local React state becomes
  awkward to pass through the tree.
- Session truth must come from the backend session endpoint.
- Zustand stores must not copy API responses, roles, permissions, payment
  results, order records, or session truth from TanStack Query.
- URL state belongs in `nuqs` when it affects shareable filters, sorting,
  pagination, tab selection, or login page display mode.
- Form state belongs in React Hook Form. Validation schemas belong in Zod and
  should be reused by Server Actions where possible.

## TanStack Query + Zustand

- Use TanStack Query for API fetching, caching, revalidation, optimistic
  updates, mutation status, and server-derived loading/error states.
- Use Zustand for global UI controls that are not shareable URLs: app overlays,
  drawers, command palette state, temporary panel state, selected local tool
  mode, or cross-layout UI toggles.
- Keep entity identifiers in Zustand only when they point at data owned by
  TanStack Query. For example, store `selectedOrderId`, then read order data
  through a query key.
- Do not hydrate Zustand from query results unless the user is editing a draft
  form or temporary UI state that can intentionally diverge from the backend.
- Mutations should update the backend through TanStack Query or Server Actions,
  then invalidate the smallest relevant query key. Zustand may close overlays or
  reset UI after the mutation succeeds.

## Current Foundation

- `components/shared/AppProviders.tsx`
  - `QueryClientProvider`
  - `NextIntlClientProvider`
  - `next-themes`
  - `NuqsAdapter`
  - `sonner` Toaster
  - React Query Devtools in development
- `lib/query/keys.ts`
  - shared query key factory entrypoint
- `lib/query/client.ts`
  - shared QueryClient defaults
- `lib/state/app-ui-store.ts`
  - first Zustand entrypoint for client-only shared UI state
- `lib/actions/safe-action.ts`
  - shared `next-safe-action` client
- `lib/forms/action-result.ts`
  - common action result shape for forms
- `lib/forms/zod-errors.ts`
  - Zod error mapping for form/action field errors
- `lib/feedback/action-toast.ts`
  - sonner helper for action result feedback
- `lib/search-params/parsers.ts`
  - shared `nuqs` parsers for common URL state
- `lib/intl/config.ts` and `i18n/request.ts`
  - default locale/messages and next-intl request config
- `messages/zh-TW.json`
  - initial message namespaces
- `tests/mocks/*`
  - MSW handler/server/browser scaffold; Vitest starts the Node server through
    `tests/setup-msw.ts`
- `instrumentation*.ts` and `sentry.*.config.ts`
  - Sentry setup gated by `NEXT_PUBLIC_SENTRY_DSN`

## Migration Rules

1. New API-backed client hooks should use TanStack Query.
2. Mutation success should invalidate the smallest relevant query key.
3. New forms should use Zod schemas and React Hook Form.
4. Server Actions should use `actionClient` and return a predictable result
   shape.
5. Toast feedback should go through `sonner` for new mutation/action flows.
6. Search params that users can share or refresh should use `nuqs`.
7. New user-facing strings should use message namespaces when the touched
   surface is already being migrated.
8. Sentry events must not include cookies, tokens, authorization headers, CSRF
   tokens, personal data, or payment-sensitive data.

## Not In This Slice

- No full auth/session migration to TanStack Query yet.
- Locale routing (`app/[locale]`, next-intl `as-needed`) landed in I-web-0; see `AGENTS.md` "多語系路由".
- No replacement of the existing theme toggle context yet.
- No global conversion of existing forms or list pages yet.
- No Uppy/Tus, virtual list, WebSocket wrapper, or Comlink dependency until a
  concrete performance or reliability issue requires it.
