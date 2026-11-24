# testdata

Every fixture is a `{status, body}` sandbox recording taken on `Stripe-Version: 2026-10-28.endive` (the version
pinned by stripe-go v87.1.0, `stripe.APIVersion`) with secrets and identifiers masked (`<MASKED>` values). One version
covers all calls: `create_*`, `retrieve_*` and `account_link_platform` are v2 calls; `account_session_platform` is a v1
call; `external_accounts_bank` is a v1 list response (`GET /v1/accounts/{id}/external_accounts?object=bank_account`).
Tests read expected values (ids, statuses, expiry timestamps) from the fixture itself instead of hard-coding them.

The two-step creation (D55) is replayed from four recordings: `create_shell_platform` and `create_shell_provider` are the
first step (`POST /v2/core/accounts` with only `metadata.store_id` and `metadata.hawkr_namespace`, idempotent by key), `update_platform` and
`update_provider` the second (`POST /v2/core/accounts/{id}` with identity, recipient configuration, defaults, dashboard and
`contact_email`). The `platform` pair is dashboard `none`, the `provider` pair dashboard `express`; a pair shares one account id.

Step 9g fixtures (recorded against the Stripe sandbox): `close_configs_must_match` is closing a bare account with `["recipient"]` (400) and
`close_bare_account` the same account closed with `applied_configurations: []`. A bare account for retrieve replays reuses `create_shell_provider`.

B2 step 1 fixtures (v1 payment calls on the same pinned version, recorded against the Stripe sandbox, US platform, US v2
recipient account): `payment_intent_confirmed` (USD PaymentIntent with `transfer_group`, confirmed with a test card),
`transfer_source_transaction`, `transfer_reversal_partial`, `refund_after_transfer`, `transfer_reversal_exceeds_remaining` (400),
`transfer_exceeds_refunded_source` (400: transfer above the charge amount left after a refund),
`transfer_currency_mismatch` (400: TWD transfer from a charge that settled in USD), `transfer_platform_insufficient_funds`
(400: platform total balance below the transfer, even with `source_transaction`), `transfer_reversal_recipient_drained`
(200: reversal succeeds after the recipient paid out, leaving a negative recipient balance), `event_destination_snapshot`
(v2 event destination with `event_payload: snapshot`) and `event_destination_snapshot_rejects_thin` (400).

B2 step 3d: `payment_intent_confirmed_expanded` is derived from `payment_intent_confirmed` (not a recording): `latest_charge` replaced by the expanded charge object (`id`, `created`) as the API reference describes, for the adapter paid-at mapping.
