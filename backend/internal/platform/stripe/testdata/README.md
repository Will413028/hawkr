# testdata

`create_*`, `retrieve_*` and `account_session_platform` are copies of
`docs/plans/2026-10-29-payment-onboarding-stripe/spike/outputs/fixtures/*.json` (`{status, body}`), recorded in plain test
mode on `2026-04-22`-era behavior and masked (`<MASKED>` values). Re-record in a sandbox on `Stripe-Version: 2026-09-30.preview`
(plan step 5b, E1 revision) and copy them over.

The Account Link success response is synthetic (`client_test.go`, `TestAccountLinkRequestHasNoConfigurationsField`): the
sandbox recording does not exist yet. Re-record in sandbox. The recorded `account_link_*.json` fixtures are 400 responses from
plain test mode and are not used.

`account_session_platform` is a v1 response. v1 calls send the GA version of the same release (`APIVersionV1`,
`2026-09-30.endive`); the preview version is for v2 calls only. This is verified only when the fixture is re-recorded in the
sandbox on that version (the recorded copy predates it).
