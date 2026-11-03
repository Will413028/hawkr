# testdata

Every fixture is a `{status, body}` sandbox recording taken on `Stripe-Version: 2026-09-30.preview` with
secrets and identifiers masked (`<MASKED>` values). `create_*`, `retrieve_*` and `account_link_platform` are v2 calls.
`account_session_platform` is a v1 response recorded with the GA version of the same release (`APIVersionV1`,
`2026-09-30.endive`); the preview version is for v2 calls only.
`external_accounts_bank` is a v1 list response (`GET /v1/accounts/{id}/external_accounts?object=bank_account`) on `APIVersionV1`.
