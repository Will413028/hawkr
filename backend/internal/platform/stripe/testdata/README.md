# testdata

Every fixture is a `{status, body}` sandbox recording taken on `Stripe-Version: 2026-10-28.endive` (the version
pinned by stripe-go v87.1.0, `stripe.APIVersion`) with secrets and identifiers masked (`<MASKED>` values). One version
covers all calls: `create_*`, `retrieve_*` and `account_link_platform` are v2 calls; `account_session_platform` is a v1
call; `external_accounts_bank` is a v1 list response (`GET /v1/accounts/{id}/external_accounts?object=bank_account`).
Tests read expected values (ids, statuses, expiry timestamps) from the fixture itself instead of hard-coding them.

The two-step creation (D55) is replayed from four recordings: `create_shell_platform` and `create_shell_provider` are the
first step (`POST /v2/core/accounts` with only `metadata.store_id`, idempotent by key), `update_platform` and
`update_provider` the second (`POST /v2/core/accounts/{id}` with identity, recipient configuration, defaults, dashboard and
`contact_email`). The `platform` pair is dashboard `none`, the `provider` pair dashboard `express`; a pair shares one account id.

Step 9g fixtures (recorded by `spike/q25_record_list_close.py`): `list_accounts_page1` and `list_accounts_page2` are the two pages of
`GET /v2/core/accounts` over the sandbox's open accounts (recorded with `limit=5` so they fall on two pages; the adapter asks for `limit=20`),
with bare first-step accounts and configured ones; `close_configs_must_match` is closing a bare account with `["recipient"]` (400) and
`close_bare_account` the same account closed with `applied_configurations: []`. A bare account for retrieve replays reuses `create_shell_provider`.
