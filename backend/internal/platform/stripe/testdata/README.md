# testdata

Every fixture is a `{status, body}` sandbox recording taken on `Stripe-Version: 2026-10-28.endive` (the version
pinned by stripe-go v87.1.0, `stripe.APIVersion`) with secrets and identifiers masked (`<MASKED>` values). One version
covers all calls: `create_*`, `retrieve_*` and `account_link_platform` are v2 calls; `account_session_platform` is a v1
call; `external_accounts_bank` is a v1 list response (`GET /v1/accounts/{id}/external_accounts?object=bank_account`).
Tests read expected values (ids, statuses, expiry timestamps) from the fixture itself instead of hard-coding them.
