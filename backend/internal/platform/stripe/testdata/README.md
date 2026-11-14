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

Step 9g fixtures `list_accounts_page1`, `list_accounts_page2`, `close_bare_account` and `close_configs_must_match` are SYNTHETIC
(marked `"synthetic": true`): shaped from the spike Q24-Q26 outputs (`GET /v2/core/accounts` with `limit=20` and `next_page_url`;
closing a bare account with `applied_configurations: []`; closed account with `metadata: null`; 400 `configs_must_match_to_close`),
not recorded. They must be re-recorded in the sandbox by the orchestrator. A bare (first step only) account for retrieve
replays reuse `create_shell_provider`.
