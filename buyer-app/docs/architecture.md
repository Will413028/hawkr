# Hawkr Buyer App Architecture

## Direction

Buyer features use unidirectional data flow:

```text
View -> Riverpod controller -> Repository -> Stateless service -> API/storage
View <- immutable typed state <- Repository result
```

- Presentation owns layout, routing, animation, scroll controllers, focus, and
  short-lived form state.
- Controllers own screen state, async commands, request ordering, optimistic
  updates, and error presentation state.
- Repositories are the feature source of truth. They parse service responses,
  reconcile sources, and expose typed immutable models.
- Stateless services are the only feature classes that open HTTP or local
  storage clients. They contain transport/storage mechanics, not business
  state.
- Domain/use-case classes are added only when logic combines multiple
  repositories or is reused by multiple controllers.

Views must not import Dio, obtain `dioProvider`, import
`shared_preferences`, or call `SharedPreferences.getInstance()`.

## Enforced ratchet

Run:

```sh
dart run tool/architecture_check.dart
```

The same check runs in the Flutter test suite through
`test/architecture/presentation_io_guardrail_test.dart`.

`tool/presentation_io_policy.json` is a debt ratchet, not an extension point.
Each legacy exception names an owner, rationale, and objective removal
condition. New presentation I/O is rejected. Remove an exception in the same
change that migrates its file; do not add exceptions for new work.

## Migration batches

| Batch | Scope | State |
| --- | --- | --- |
| Foundation | Architecture guide and presentation I/O ratchet | Implemented |
| C1 | Wallet coupon list remote data | Implemented |
| B1 | Order receipt-photo local fallback and detail photo section | Implemented |
| A2 | Search and Marketplace products/posts | Follow-up |
| C2 | Merchant screen and merchant tab I/O | Follow-up |
| B2 | Remaining order-detail sections and chat thread | Follow-up |
| C3 | Subscription, comments, and remaining wallet sections | Follow-up |

The tracking issue remains open until the ratchet reaches zero and the large
screens are delivered as separately reviewed, behavior-preserving section
extractions.
