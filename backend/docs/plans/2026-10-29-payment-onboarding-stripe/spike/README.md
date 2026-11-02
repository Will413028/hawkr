# Step 1 spike: Stripe test-mode facts (status: STOPPED at Q1, 2026-11-02)

Platform: US test account (D8 = C). Test mode only; no response showed `livemode: true`.
Run a script with `./run.sh <script>.py`; it loads the key from the local secrets file, calls the API
and pipes everything through `scrub.py` into `outputs/`. `sp.py` is the minimal client (key never printed).

## Stop reason (needs Will)

Every account creation with `controller.requirement_collection = application` (and legacy `type=custom`)
is rejected by Stripe with HTTP 400, code `account_creation_requirement_collection_unacknowledged`:
"Please review the responsibilities of collecting requirements for connected accounts at
https://dashboard.stripe.com/settings/connect/platform-profile". Same result on Accounts v1 and Accounts v2
(`POST /v2/core/accounts`), for US, TW, SG, JP. This is a dashboard setting only the account owner can
change: **Connect > Settings > Platform profile > acknowledge "responsibilities of collecting requirements
for connected accounts"**. Per the task rules the spike stopped here and did
not work around it; Q2 to Q8 were NOT run (they all need connected accounts, and Q2 needs this setting).
After Will acknowledges it, rerun `./run.sh q01_countries.py` and continue.

Also observed once (first run of `q01b`, later run succeeded, cause unknown, possibly a dashboard change in
between): "Stripe no longer recommends Accounts v1 for new Connect integrations ... enable Accounts v1 support in the
Dashboard (settings/developers/api-policies/feat_accounts_v1_support)". If v1 creation starts failing again, either
enable that setting or move the plan's `controller` params to Accounts v2 (`/v2/core/accounts`; `responsibilities.requirements_collector`).

## Q1: connected-account countries under the US platform (partial)

| Case | Result | Output |
|---|---|---|
| application-collected (any country, v1 controller, v1 custom, v2) | blocked by the platform-profile setting above | `outputs/q01_countries.txt`, `outputs/q01c_v2_probe.txt` |
| stripe-collected, Express dashboard, `fees.payer=application`, `losses.payments=application`, US / SG / JP with `card_payments`+`transfers` | created (200) | `outputs/q01b_stripe_collected_probe.txt` |
| same, TW with `card_payments` | 400: "You cannot request the `card_payments` capability for accounts in TW ... see cross-border-payouts" | same |
| same, TW and SG with `transfers` only and `tos_acceptance.service_agreement=recipient` | created (200); TW account has `capabilities.transfers: inactive` | same |
| Express with `fees.payer=account` | 400: "When stripe_dashboard[type]=express, your platform must collect fees and be liable for negative balances..." | `outputs/q01_countries.txt` |

Answer so far: **TW is allowed only as a recipient-agreement account (transfers only, no `card_payments`)**, created under a
stripe-collected Express controller. Whether a TW account can be application-collected (Custom) is untested (blocked).
Docs: https://docs.stripe.com/connect/cross-border-payouts , https://stripe.com/global ,
https://docs.stripe.com/connect/service-agreement-types , https://docs.stripe.com/connect/accounts-v2/account-creation

## Q2 to Q8: not run (blocked)

Q2 Custom KYC via API, Q3 Account Links / Account Sessions, Q4 requirements mapping, Q5 webhook + `stripe events resend`,
Q6 fingerprint across accounts, Q7 idempotency, Q8 Files API: no output. (Q3 to Q8 might be answerable with
stripe-collected Express accounts, which can be created today, but the brief says stop, so they were not started.)

## Verdicts

- D6 / D7 (does Custom work in test mode?): **unknown**. Not disproved; blocked by one dashboard acknowledgement.
- D12 (is the bank-account fingerprint consistent across accounts?): **unknown** (Q6 not run).

## Cleanup

All connected accounts created by the spike were deleted via `DELETE /v1/accounts/{id}` (`q01b` deletes its own);
`GET /v1/accounts` returned 0 accounts afterwards.

## Files

`scrub.py` (masks secrets, ids other than `acct_`/`evt_`, emails, PII keys, DOB, signatures), `sp.py`, `run.sh`,
`q01_countries.py`, `q01b_stripe_collected_probe.py`, `q01c_v2_probe.py`, `outputs/*.txt`.
