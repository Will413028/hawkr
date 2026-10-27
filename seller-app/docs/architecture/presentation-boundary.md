# Seller app presentation boundary

Issue #99 is a tracking issue. Its architecture work must ship as small,
independently reviewable batches; it is not permission to rewrite routes,
scroll behavior, visual design, or backend contracts in one change.

## Required dependency flow

Protected API and persistent-storage reads follow one direction:

```text
Widget / section
  -> Riverpod controller command
  <- typed immutable UI state
  -> repository (single source of truth / orchestration)
  -> stateless remote or local service
  -> central Dio or storage SDK
```

Views may own ephemeral UI state such as focus, animation, text input, selected
tab, and scroll controllers. Views may render state, dispatch commands, and
perform navigation after a controller returns a typed route payload. They must
not obtain Dio, invoke `apiGet`/`apiPost` helpers, or open
`SharedPreferences`.

Repositories own data mutation and aggregation. Services only translate one
external transport or storage contract and do not retain mutable feature
state. Controller-visible models and lists are immutable.

## Enforced ratchet

Run:

```sh
fvm dart run tool/presentation_io_guard.dart
```

The guard scans every non-generated Dart file under a `presentation/`
directory. It blocks new direct Dio imports/client access, low-level
`apiGet`/`apiPost` family calls, raw Dio/HttpClient construction, and direct
SharedPreferences access.

The current policy has zero exceptions. If a time-bounded exception is ever
required, `tool/presentation_io_policy.json` requires:

- an owner;
- a concrete rationale;
- a removal condition;
- an exact occurrence budget.

The budget is an exact ratchet: adding a violation fails, and removing one
also requires reducing or deleting its policy entry. The architecture policy
test runs as part of the normal Flutter test suite, so the rule is CI-gating
without a separate optional workflow step.

## First independently deliverable batch

This batch establishes two reference paths:

1. Subscriber list: stateless subscriber remote service -> subscriber
   repository -> Riverpod async controller -> immutable list view data.
   Metadata fan-out is bounded to four concurrent requests. The view retains
   the existing `/customer/detail` route and delegates payload resolution to a
   controller command.
2. Chat attachment catalogs: stateless remote service -> typed picker
   repository -> Riverpod command state. The order/product picker coordinator
   is extracted from `chat_thread_screen.dart`; message payload types and
   modal behavior remain unchanged.

## Current #99 checkpoint

The presentation I/O inventory is now zero:

- Batch A protected certification previews use the protected-media
  service/repository/controller path. KYC compression remains owned by #64.
- Batch B product image hydration uses the same typed protected-media command,
  and the product-listing chat repository now lives under `product/data`.
- Batch C attachment catalog loading is extracted behind typed picker state.
  Further visual section splits do not require presentation-layer I/O.
- Batch D shop readiness and the bounded seller-products cache use immutable
  shop-tab controller state.
- Page seller stats, image-editor history and shipping-settings commands now
  flow through service/repository/controller boundaries. The stateless TGOS
  service lives under `settings/data`.

Large-screen section extraction can continue as small reviewable changes, but
it must preserve this zero-exception boundary.

Each batch must begin with behavior/characterization coverage, keep route and
scroll contracts stable, run the architecture guard, focused tests, full
tests, analyzer, and custom lint, and report any device/runtime evidence that
is still missing.
