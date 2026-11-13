# Step 1 spike: Stripe test-mode facts (2026-11-02)

Platform: US test account (D8 = C). Test mode only; no response or event showed `livemode: true`
(the CLI banner prints the account display name with "live" in it; every event and API object we saw had `livemode: false`).
Run a script with `./run.sh <script>.py`; it loads the key from the local secrets file, calls the API and pipes everything
through `scrub.py` into `outputs/` (webhook run: `./q05_webhook.sh`). `sp.py` is the minimal client (refuses non-test keys, aborts on livemode true).
"Docs" statements below come from the cited official pages as I know them; I did not re-fetch every page this session, so
treat quoted doc behaviour as [docs, unverified this session] and observed behaviour as [measured, test mode].

History: the first run stopped because the Dashboard platform-profile responsibilities acknowledgement was missing
(error code `account_creation_requirement_collection_unacknowledged`, v1 and v2). Will acknowledged it; everything below ran after that.

## Verdicts

- **Custom works in test mode (D6/D7)**: yes. `controller.requirement_collection=application` (+ `fees.payer=application`, `losses.payments=application`, `stripe_dashboard.type=none`) accounts reach `charges_enabled=true` and `payouts_enabled=true` purely through the API for US (about 55 s after the last submit) and for a TW recipient account (immediately). SG reaches `charges_enabled=true` but `payouts_enabled=false`: it requires `individual.verification.proof_of_liveness`, which has no API parameter (`parameter_unknown`), so SG cannot finish by API; it needs Stripe-hosted/embedded onboarding.
- **Fingerprint (D12)**: consistent. The same test bank account attached to two different connected accounts returns the identical `fingerprint`; a different account number gives a different one. See Q6 for the limits of this evidence.
- **TW**: usable only as a recipient-agreement, transfers-only account (no `card_payments`). Both stripe-collected and application-collected work.

## Q1 Countries under the US platform
Answer [measured]: US, SG, JP accounts with `card_payments`+`transfers` were created (application-collected, v1 controller and v1 `type=custom`). TW with `card_payments` is rejected ("You cannot request the `card_payments` capability for accounts in TW ... see cross-border-payouts"). TW with `transfers` only and `tos_acceptance.service_agreement=recipient` is created (application-collected and stripe-collected). Express (`stripe_dashboard.type=express`) requires `fees.payer=application` and `losses.payments=application`.
Docs: https://docs.stripe.com/connect/cross-border-payouts , https://docs.stripe.com/connect/service-agreement-types
Output: `outputs/q01_countries.txt` (rerun after acknowledgement), `q01b_stripe_collected_probe.txt`, `q01c_v2_probe.txt` (the pre-acknowledgement block, kept as evidence). Note: the first q01b run also hit "Stripe no longer recommends Accounts v1" for card_payments accounts; later runs did not. If it returns, enable Accounts v1 support in the Dashboard or move to `/v2/core/accounts`.

## Q2 Custom KYC via API
Answer [measured]: US individual: create, then one update with `individual` (name, email, phone `0000000000`, dob 1901-01-01, address line1 `address_full_match`, `ssn_last_4=0000`, `id_number=000000000`, `verification.document.front=file_identity_document_success`), `business_profile` (mcc, url), `external_account` (routing `110000000`, account `000123456789`), `tos_acceptance` (date, ip) -> `pending_verification` for about 55 s -> `charges_enabled=true`, `payouts_enabled=true`, `requirements` all empty. `settings.payments.statement_descriptor` appeared in `currently_due` at creation and disappeared once the business profile was set.
SG individual: needs `nationality`, `full_name_aliases` (`[""]`), `id_number=S0000001I`, SG phone in E.164 (`+6581234567`; `+6512345678` rejected), bank `1100-000`/`000123456`; result `charges_enabled=true`, `payouts_enabled=false`, `currently_due=[individual.verification.proof_of_liveness]`, `disabled_reason=requirements.past_due`, `current_deadline` set. Submitting `proof_of_liveness` fails `parameter_unknown`; `additional_verification` document was accepted but did not clear it.
TW recipient individual (application-collected): `external_account` (routing `SCBLTWTP`, account `0001234567`; other SWIFT-like values rejected in test mode), individual fields and `tos_acceptance` -> `charges_enabled=true` (transfers capability) and `payouts_enabled=true` in under 8 s. Before the bank was valid only `external_account` stayed due.
Docs: https://docs.stripe.com/connect/testing , https://docs.stripe.com/connect/required-verification-information , https://docs.stripe.com/connect/handling-api-verification
Output: `outputs/q02_custom_kyc.txt`, `q02b_poll.txt`, `q02c_sg.txt`, `q02d_sg_liveness.txt`, `q02e_tw_recipient.txt`, `q02f_tw_bank.txt`.

## Q3 Hosted and embedded onboarding
Answer [measured]: Account Links `account_onboarding` work for stripe-collected Express and for application-collected accounts (URL path `/setup/e/...` vs `/setup/c/...`); `expires_at` is created+300 s; `return_url` is required (`parameter_missing` without it); `refresh_url` was accepted for http and https; `account_update` links are refused for the Express/stripe-collected account ("Valid types for this account are [account_onboarding]") but accepted for the application-collected one. Account Sessions (`components.account_onboarding.enabled`) return a `client_secret` and `expires_at` about 125 s after creation on both kinds of account. Not clicked through.
Docs [unverified this session]: the link is single-use; `refresh_url` is where the user lands when the link is expired, already used, or opened in the wrong browser (mint a new link there); `return_url` only means the user left the flow, not that onboarding is finished, so read `requirements`/`charges_enabled` from the Account (or the `account.updated` webhook) after return. https://docs.stripe.com/api/account_links/create , https://docs.stripe.com/connect/hosted-onboarding , https://docs.stripe.com/connect/embedded-onboarding , https://docs.stripe.com/api/account_sessions/create
Output: `outputs/q03_onboarding.txt`.

## Q4 requirements -> charges/payouts
Real stages [measured] (US, `outputs/q02_custom_kyc.txt`): created: all fields in `currently_due`=`eventually_due`=`past_due` (no deadline, so everything starts past_due), `disabled_reason=requirements.past_due`, both false. After profile: remaining `external_account`, tos; verifiable fields moved to `pending_verification`. After tos: `currently_due=[]`, `details_submitted=true`, `disabled_reason=requirements.pending_verification`, `card_payments=pending`, `transfers=active`, both enabled flags false. After verification: `pending_verification=[]`, `disabled_reason=null`, both true.
SG: `charges_enabled=true` with `payouts_enabled=false` while `past_due=[proof_of_liveness]`: the two flags do not move together; payouts is stricter. A TW recipient shows `transfers` active and `charges_enabled=true` with no card payments.
Errors [measured] (`outputs/q04c_survey.txt`, `q04d_errors.txt`): address `address_no_match`, `id_number=111111111`, dob 1900-01-01 produced `errors=[invalid_address_city_state_postal_code, invalid_street_address, verification_failed_tax_id_match, verification_failed_keyed_identity ...]`, `currently_due` re-listed the failed address fields, `disabled_reason=listed`-style failure value, both flags false. `dob` 1902 and the `file_identity_document_failure` token did NOT fail (account became active; `q04a`, `q04b`).
Docs: https://docs.stripe.com/connect/handling-api-verification , https://docs.stripe.com/api/accounts/object#account_object-requirements

## Q5 account.updated webhook
Answer [measured] with `stripe listen --events account.updated,capability.updated --forward-connect-to` (CLI 1.55 requires `--events`; without `--forward-connect-to` connected-account events are not forwarded) to `webhook_receiver.py`: header `Stripe-Signature: t=<unix>,v1=<hmac>,v0=<legacy>`; signature = HMAC-SHA256(secret, `t + "." + raw body`), constant-time compare; receiver tolerance 300 s. Every real delivery verified; the same body with one changed field failed ("signature mismatch"); a valid body checked at now+1000 s failed ("timestamp outside tolerance"). Events carry `id` (`evt_`), `account` (the connected account), `created`, `request.id`. Five rapid updates were delivered in `created` order and the Events API (Stripe-Account header) listed the same ids. `stripe events resend <evt> --account <acct>` re-delivered the same event id with a new, valid signature.
Docs [unverified this session]: Stripe does not guarantee delivery order; retries with exponential backoff for up to 3 days in live mode (a few hours in test mode); handle duplicates by event id; return 2xx quickly. https://docs.stripe.com/webhooks , https://docs.stripe.com/webhooks#verify-manually , https://docs.stripe.com/cli/events/resend , https://docs.stripe.com/connect/webhooks
Limit: order was observed on 5 sequential events only; docs, not this run, are the evidence for "no guarantee". Retry behaviour was not exercised (receiver always answered 2xx).
Output: `outputs/q05_webhook.txt`.

## Q6 Bank account fingerprint
Answer [measured]: the same test bank account attached to two different connected accounts gives the same `fingerprint` (also via a `tok_`/`btok_` token path); a different account number gives a different one. Attaching the same bank again to the same account returned 200. Platform key can read `fingerprint` on `external_accounts`. Limit: test numbers only; docs describe fingerprint as identifying a particular bank account, but I did not verify whether live-mode fingerprints are scoped per platform. Re-check before relying on it for D12.
Docs: https://docs.stripe.com/api/external_account_bank_accounts/object#account_bank_account_object-fingerprint
Output: `outputs/q06_fingerprint.txt`.

## Q7 Idempotency on account creation
Answer [measured]: same key + same params returns the same account id with `Idempotent-Replayed: true`; same key + different params returns 400 `idempotency_error` ("Keys for idempotent requests can only be used with the same parameters..."); a different key creates a new account. A 400 with the same key replays the 400. Reusing the key on a different endpoint (update) was NOT blocked in this run (it executed as a new request): do not rely on cross-endpoint key reuse for protection.
Docs [unverified this session]: keys expire after 24 hours (a key reused after that is a new request); up to 255 chars. https://docs.stripe.com/api/idempotent_requests
Output: `outputs/q07_idempotency.txt`.

## Q8 Files API for identity documents
Answer [measured]: `POST https://files.stripe.com/v1/files` multipart, `purpose=identity_document`, png/jpeg/pdf only (text/plain refused); returns a `file_...` id; pass it as `individual[verification][document][front]` (and `back`); the Account/Person then shows `verification.document.{front,back,details,details_code}`. A file uploaded with `purpose=dispute_evidence` is refused there ("valid: identity_document, document_provider_identity_document"). `additional_verification` and `account_requirement` purposes also exist; `identity_document` files cannot get file links. Test tokens `file_identity_document_success` work in place of a real file id.
Docs: https://docs.stripe.com/api/files/create , https://docs.stripe.com/connect/handling-api-verification#upload-a-verification-document
Output: `outputs/q08_files.txt`.

## Q9 Accounts v2 recipient, both collection modes (step 5, measured 2026-11-03)

`Stripe-Version: 2026-04-22.preview`, US, `identity.entity_type=individual`, `configuration.recipient.capabilities.stripe_balance.stripe_transfers.requested=true`, `defaults.responsibilities.fees_collector=losses_collector=application`.

- Collection mode is chosen by `dashboard`: `none` gives `defaults.responsibilities.requirements_collector=application`; `express` gives `stripe` (https://docs.stripe.com/connect/accounts-v2/connected-account-configuration). `fees_collector`/`losses_collector=stripe` is rejected for a recipient-only account (`invalid_fields`: "can only be application").
- Idempotency: same `Idempotency-Key` and body returns 200 with the same account id; same key with a different body returns 409 `idempotency_error`.
- Bank fingerprint: an external bank account attached via `POST /v1/accounts/{id}/external_accounts` is listed with `fingerprint` under the platform key in both modes; same tag as Q6 (fingerprint stable). Not measured: a bank entered by the seller inside Stripe-hosted onboarding (step 5 manual run).
- Rename: `POST /v2/core/accounts/{id}` with `display_name` returns 200 in both modes (D17: Stripe does not block; we do not sync).
- `POST /v2/core/account_links` (`use_case.type=account_onboarding`, `configurations=[recipient]`) returns a URL in both modes; `POST /v1/account_sessions` with `account_onboarding` returns a client secret in both modes.
- Cleanup: `POST /v2/core/accounts/{id}/close` returns 400 `configs_must_match_to_close` without a body naming the configurations; `DELETE /v1/accounts/{id}` deletes test accounts.

Output: `outputs/q09_v2_recipient.txt`.

## Q10 Thin events for v2 accounts (step 5, measured 2026-11-03)

`stripe listen --all-thin` forwarding to `webhook_receiver.py`.

- v2 account events are delivered to the platform's own destination (`context` null): `v2.core.account.created`, `v2.core.account[defaults].updated`, `v2.core.account[identity].updated`, `v2.core.account[configuration.recipient].updated`, `v2.core.account[configuration.recipient].capability_status_updated`, `v2.core.account_person.created`. `related_object = {id: acct_…, type: v2.core.account}`. Payload top-level keys: `context, created, id, livemode, object, reason, related_object, type`.
- With `--events-from @accounts` the connected account's v1 events arrive thin as `v1.account.updated`, `v1.capability.updated`, `v1.person.created` with `context = acct_…`; retrieving one with `GET /v2/core/events/{id}` without `Stripe-Context` returns 404. The adapter subscribes only to the platform's `v2.core.account*` events.
- `GET /v2/core/events/{id}` returns `changes.before/after` (not needed: the sync job re-reads the account).
- Signature: `Stripe-Signature: t=…,v1=…`, HMAC-SHA256 of `{t}.{raw body}`; altered body and a timestamp 1000 s off both fail verification.

Output: `outputs/q10_thin.txt` (`@accounts`), `outputs/q10b_thin_self.txt` (platform).

## Q11–Q12 API version 2026-09-30.preview in test mode (step 5, measured 2026-11-03)

- `2026-09-30.preview` removes `use_case.account_onboarding.configurations` from `POST /v2/core/account_links` (https://docs.stripe.com/changelog/endive/2026-09-30/removes-configurations-parameter-from-v2-core-account-links-api) and adds capability status `rejected` with `rejected_*` codes (https://docs.stripe.com/changelog/endive/2026-09-30/rejected-accounts-v2-capabilities).
- In plain test mode (not a sandbox) the Account Link is refused: "Cannot create a v2 account link … with recipient config capabilities other than stripe_balance.stripe_transfers requested in testmode. Please switch to an account created in a sandbox". `stripe_balance.payouts` is added automatically and cannot be set (`Unknown field`). The same call worked on `2026-04-22.preview` (Q9).
- `defaults.profile.business_url = https://example.com` is rejected as `Invalid URL`.
- Decision: E1 revised (Will 2026-11-03), pin `2026-09-30.preview` and record in a sandbox. Fixtures in `outputs/fixtures/` are from plain test mode and are re-recorded there.

Output: `outputs/q11_record.txt`, `outputs/q12_link_version.txt`, `outputs/fixtures/`.

## Q14 Bank fingerprint after Stripe-hosted onboarding (step 6, measured 2026-11-03)
Answer [measured]: in the sandbox, `GET /v1/accounts/{id}/external_accounts?object=bank_account&limit=100` on `2026-09-30.endive` for the v2 account onboarded through the hosted page in step 5b (`acct_1UXVfaID6WkY4CJw`, bank entered by the seller inside Stripe) returns 200 with one `bank_account` that has a 16-character `fingerprint`, under the platform key. This closes the Q9 gap ("a bank entered by the seller inside Stripe-hosted onboarding"). Limit: one account, Stripe's test bank; live-mode scope stays deferred (plan scope).
Output: `outputs/fixtures/external_accounts_bank.json` (also the adapter's contract fixture), script `q14_external_accounts.py` (run with the sandbox env like `q11_run.sh`).

## Q15 Embedded onboarding for a v2 account with connect-js 3.4 (step 7b, measured 2026-11-03)
Answer [measured]: in the sandbox, a v2 account created with the adapter's body (`dashboard=none`, recipient `stripe_transfers`, `2026-09-30.preview`; `acct_1UXdGKRRzdP06vGA`) accepts a v1 `POST /v1/account_sessions` on `2026-09-30.endive` (200, client secret). A local page loading `@stripe/connect-js@3.4.6` initialises `account-onboarding` without a load error and shows the intro screen. With the default session, clicking the start button spins indefinitely with no console error and no overlay iframe: the component waits on the Stripe user-authentication popup, which the doc says is on by default and always a popup (https://docs.stripe.com/connect/get-started-connect-embedded-components?platform=web, "User authentication in Connect embedded components"). With `components[account_onboarding][features][disable_stripe_user_authentication]=true` the form renders directly (`onStepChange`: `networked_onboarding_with_link` then `representative_details`, "Your details"). Decision D25. The website sets no `Cross-Origin-Opener-Policy` header (a value other than `unsafe-none` would break that popup). Limit: the onboarding was not completed in this spike; completion is 7b's manual acceptance. No script or output was kept (the page held a client secret and was deleted); the account was left in the sandbox.

## Q16 stripe-go and the GA API version; closing a v2 account (step 9b, measured 2026-11-13)
Answer [read from source, stripe-go v87.1.0]: `github.com/stripe/stripe-go/v87` v87.1.0 (published 2026-10-29, pins `2026-10-28.endive`; only new transitive dependency `github.com/shopspring/decimal` v1.4.0) has `V2CoreAccounts.Create/Retrieve/Close`, `V2CoreAccountLinks.Create`, `V1AccountSessions` and `Client.ParseEventNotification`; a per-request version override is possible through `Params.Headers` (`stripe.go` sets the default with `Add`, then applies `params.Headers` with `Set`). The capability status `rejected` is typed in the stable SDK.
Answer [measured]: in the sandbox on `2026-10-28.endive`, for `dashboard=express` and `dashboard=none`: create v2 account 200, Account Link 200, retrieve with `include=configuration.recipient&include=requirements` 200, v1 `account_sessions` 200, v1 `external_accounts` list 200. `POST /v2/core/accounts/{id}/close` without `applied_configurations` is 400 `configs_must_match_to_close`; with `["recipient"]` it is 200 and `closed=true`. Closing again is 403 `forbidden` (not idempotent); retrieving the closed account still returns 200 with `closed=true` and no recipient capabilities; an Account Link for it is 403. Closing an unknown id is 400 `invalid_fields`. Limit: accounts that never onboarded; closing an account with a balance or completed onboarding was not measured.
Output: `outputs/q16_ga_version_close.txt`, script `q16_ga_version_close.py` (run with `q16_run.sh`, sandbox env). Both accounts were closed by the script.

## Cleanup
All connected accounts were deleted with `DELETE /v1/accounts/{id}` (`cleanup.py`, `outputs/cleanup_final.txt`: 0 remaining). Webhook listener and receiver were stopped by `q05_webhook.sh`.

## Files
`scrub.py` (masks secrets, ids other than `acct_`/`evt_`, emails, PII keys, DOB, signatures, URL tokens), `sp.py`, `run.sh`, `cleanup.py`, `webhook_receiver.py`, `q05_webhook.sh`, `q05_drive.py`, `q01*` to `q08*` scripts, `outputs/*.txt`. Test personal values are Stripe's public fake test data.
