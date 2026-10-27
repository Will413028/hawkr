# A3 seller — domain, data, mechanisms inventory (read-only)

Source: `backend/services/seller-service` at HEAD `bdf13a034`.
Scope: domain / data / mechanisms. RPC-by-RPC, gateway routes and client migration are the other agent's; they appear here only where needed for the D8 "wired or dead" call.

## 0. Size

| What | LOC | Command |
|---|---|---|
| hand-written Go, non-test (incl. 738 lines of `zzz_enumer_*` generated enums) | 23,219 | `find services/seller-service -name '*.go' ! -name '*_test.go' ! -path '*/gen/*' \| xargs cat \| wc -l` |
| sqlc generated (`repository/postgres/gen`) | 8,234 | `find ... -path '*/gen/*' -name '*.go' \| xargs cat \| wc -l` |
| tests | 33,747 | `find ... -name '*_test.go' \| xargs cat \| wc -l` |
| gRPC RPCs in `api/proto/seller/seller.proto` | 98 | `rg -c '^\s*rpc ' backend/api/proto/seller/seller.proto` |
| tables | 25 | `rg -c '^CREATE TABLE' db/schema/schema.sql` |
| migrations | 72 `.sql` + `atlas.sum` | `ls db/migrations/*.sql \| wc -l` |

Hand-written LOC by subdomain (rough, classified by file name; `core_or_shared` includes the 1,344-line `grpc/handler.go` that spans every subdomain, `seller.go`, repo, config, DI):

```
7597 core_or_shared   3538 governance   3213 payuni   2795 kyc_certification
2237 prospect         1178 deletion     803 suspension 575 membership
 535 vendor_portal     312 analytics     231 recommended 205 address
```

One-off binaries not to migrate: `cmd/backfill-seller-deleted`, `cmd/backfillstorecode` (backfills for data that never went live). `cmd/grpc-healthcheck` disappears with gRPC.

## 1. Subdomains and D8 wiring

Wiring evidence: inventory `docs/plans/2026-09-24-go-modular-monolith/rpc-calls.csv` (callee=seller-service) cross-checked against current gateway routes (`gateway/internal/router/routes_*.go`) and client code (`rg` over `website`, `admin`, `seller-app`, `buyer-app`, excluding `node_modules`, `gen`, tests).

| Subdomain | Tables | Reached by | Verdict |
|---|---|---|---|
| Store profile / stint (`sellers`, apply, profile, logo/cover, SEO, food-business reg., onboarding, public storefront, city lists, principals/store-ID resolution, BatchGetSellerMeta) | `sellers` | all 4 clients; 9 services call `BatchGetSellerMeta`/`ResolvePrincipals` | wired, core |
| KYC certification (8-step draft, submit, admin approve/reject, reviews, GCIS lookup, address suggest/validate via TGOS) | `certification_applications`, `certification_reviews` | website (`lib/seller/certification.ts`), seller-app, admin | wired. Old draft API `SaveCertificationDraft`/`SubmitCertification` has gateway client wrappers but no handler calls them → dead |
| PayUni sub-merchant (bind at approve, status poll, refresh, merchant-profile PATCH, webhook result) | `payuni_registrations`, `payuni_status_history`, `payuni_merchant_profile_updates`, `payuni_merchant_profile_outbox` | website, seller-app (`/app/sellers/me/payuni/*`), payuni-webhook-service via MQ `seller.review.result` | wired. `RegisterPayuni`, `RetryPayuniRegistration`, `GetSellerPayuniConfig` have 0 callers → dead |
| Merchant-verification notifications | `merchant_verification_outbox` | → notification-service (`merchant.verification.*`) | wired |
| Admin manual suspension | `seller_suspensions` | admin | wired |
| Governance: violation reports, food-safety reports (public, no login), violations/penalties, appeals, admin notify, FDA export | `seller_violation_reports`, `food_safety_reports`, `seller_violations`, `seller_notification_log` | admin (reports, violations), website + seller-app (appeals, my violations), website public food-safety form, public violations on storefront | wired. But the 5 `seller.governance.*` events go to exchange `hawkr.governance`, which `events.csv` marks "exchange not provisioned", no consumer → dead side effect |
| Membership plans (free/plus/pro, admin assign) | `seller_membership_plans`, `seller_membership_subscriptions` | admin only (`admin/src/app/admin/seller-membership-plans`) | half-wired: the only effect is `seller.membership_changed` → ai-service `seller_ranking_facts` (ranking, moving to A11). `monthly_order_cap` is enforced nowhere (`rg -il monthly_order_cap services gateway` outside seller hits only ai-service); all prices are 0; payment's `payment.membership.seller.*` consumer "not implemented" (events.csv). D8 candidate |
| Seller prospects (BD CRM for admin + procurement candidate search) | `seller_prospects`, `seller_prospect_categories`, `seller_prospect_invites` | admin (19 files), ai-service `SearchSellerProspects` | wired, except invite attribution: `CreateSellerRequest.prospect_invite_token` is never sent (gateway `ApplySeller` has no such field; 0 client hits) → `attributeProspect`, `ConsumeInvite`, `FindUsableInviteByToken` dead |
| Vendor portal (invite-link sessions for non-member vendors answering procurement RFQs) | `vendor_portal_identities`, `vendor_portal_invites`, `vendor_portal_sessions` | website (8 files), notification-service `EnsureVendorPortalInvite` | wired, but it is procurement (A9 agent) functionality stored in seller |
| Analytics: store visitors | `seller_store_visitors` | written fire-and-forget from gateway `GetSellerInfo`; read route `GET /app/sellers/me/store_visitors` has 0 client hits (`rg -il 'store.?visitor\|storeVisitor'` over 4 clients = 0) | write-only → D8 dead |
| Analytics: product views | `seller_product_view_log` | anonymous `POST /public/products/:id/view` + MQ `product.viewed`; read by seller dashboard `pageViewCount` (seller-app, website) | wired |
| Account/stint deletion + PII cleanup | `seller_deleted_outbox` (generalised lifecycle outbox), `seller_deletion_audit` | `DELETE /app/sellers/me`, admin account delete | wired |
| Recommended sellers ranking | none (reads order/product/buyer) | website public, marketplace | excluded (A11 discovery) |
| Capability / suspension gate | none in seller; lives in gateway `authz` + Redis key written by seller | 153 capability-declared routes across 14 gateway route files | wired (see §3.4) |
| Stint assistant history | none in seller (ai-service ADK session keyed by `store_id`) | — | nothing to migrate in seller |

Zero-caller RPCs (verified now, not only snapshot): `GetSellerByMemberId`, `GetSellerPayuniConfig`, `RegisterPayuni`, `RetryPayuniRegistration`, `ValidateSeller` (0 callers anywhere), plus `SaveCertificationDraft`, `SubmitCertification` (gateway wrapper only). Command: `rg -l "\.$m\(" gateway services internal --glob '!**/seller-service/**' --glob '!*.pb.go' ...`.

## 2. Data

### 2.1 Tables by subdomain (25)

- Store/stint (1): `sellers`
- KYC (2): `certification_applications`, `certification_reviews`
- PayUni (4): `payuni_registrations`, `payuni_status_history`, `payuni_merchant_profile_updates`, `payuni_merchant_profile_outbox`
- Notification outbox (1): `merchant_verification_outbox`
- Lifecycle / deletion (2): `seller_deleted_outbox`, `seller_deletion_audit`
- Suspension (1): `seller_suspensions`
- Governance (4): `seller_violation_reports`, `food_safety_reports`, `seller_violations`, `seller_notification_log`
- Membership (2): `seller_membership_plans` (seed rows 2001–2003 in schema), `seller_membership_subscriptions`
- Prospects (3): `seller_prospects`, `seller_prospect_categories`, `seller_prospect_invites`
- Vendor portal (3): `vendor_portal_identities`, `vendor_portal_invites`, `vendor_portal_sessions`
- Analytics (2): `seller_store_visitors`, `seller_product_view_log`

No data migration needed (never live), so the 72 migrations collapse into one baseline, as content/identity did.

### 2.2 ID schemes

- Every PK except two: `bigint` from `pkg/tsid` (`fx.Provide(tsid.NewGenerator)` in `usecase/di.go:114`), including `sellers.store_id` (surrogate per stint).
- `certification_reviews.id` is `bigserial` (the only DB-generated one).
- `seller_membership_plans.id` is hard-coded seed (2001/2002/3), with the natural key `plan_code` as the FK target.
- `sellers.store_code`: public display code `YY*10^8 + crypto/rand(10^8)`, UNIQUE, with a 5-attempt collision retry in `CreateSeller` (`domain/store_code.go`). PayUni MerID = `"HAWK" + store_code` (`domain.FormatMerID`). It is the external ID PayUni sees, so a new stint needs a new store_code.
- `certification_applications.application_no` = `CERT-<unix>-<...>` string, UNIQUE.
- `account_id` (identity principal) stored as bigint in `sellers`, `certification_applications`, `payuni_*`, outboxes, audit. Plan note: A2/A3 IDs still bigint because Java consumers store them as BIGINT; transitional rule `bigint GENERATED ALWAYS AS IDENTITY` for PKs, then uuid once the last Java consumer is rewritten. **For seller that rule has a catch**: `store_id` is minted per stint and is not IDENTITY-shaped today (TSID), and `store_code` is the external ID. ADR `2026-09-25-row-identity-by-data-kind.md` should decide store_id (IDENTITY vs UUIDv7), store_code (keep, it is external), and log/outbox tables (IDENTITY or dropped when River replaces the outboxes).
- Mixed keys: `seller_violations.seller_id`, `seller_violation_reports.target_seller_id`, `food_safety_reports.target_seller_id`, `seller_notification_log.seller_id`, `seller_product_view_log.seller_id`, `seller_suspensions.seller_id` all hold **store_id** under the old name `seller_id`. `seller_suspensions` is keyed by store but looked up via `HasActiveByAccountID`. `payuni_status_history` and `certification_applications` are keyed by account_id (business-key rule in backend AGENTS.md says business tables use store_id).

### 2.3 Extensions, functions

No `CREATE EXTENSION` (`rg -c "CREATE EXTENSION" db` = 0). Built-ins used: `sha256`, `hashtextextended` (advisory lock key), `date_trunc`. Monolith needs nothing extra.

### 2.4 Encryption (ADR 2026-07-20)

- `certification_applications.id_card_no`, `.bank_account`: AES-256-GCM via `pkg/crypto` (`ENCRYPTION_MASTER_KEY`), stored as Base64 in `varchar(255)`; `step_data` JSONB encrypts keys `idNumber`, `id_card_no`, `idCardNo`, `bankAccount`, `bank_account` (`repository/postgres/mapper.go:49`).
- Plaintext by decision: `real_name` (admin ILIKE), `birth_date`.
- **Leak that defeats the ADR**: `payuni_registrations.raw_request` (JSONB) stores `json.Marshal(BindSellerRequest)`, which includes `id_card_no` in plaintext (`usecase/certification.go:572-610`, `payuni_client.go:33` tag `json:"id_card_no"`). Also plaintext PayUni contact PII (`contact_name`, `contact_mobile`, `mer_addr`, `mer_email`).
- Identity module already has its own `seal` (AES-GCM) for job payloads. The monolith needs one crypto owner in platform rather than a second copy.

### 2.5 Soft delete / lifecycle

- `sellers`: `status IN (active, suspended, suspended_governance, closed)`, `deleted_at` (CHECK: only with closed), `is_current` + partial unique `uq_sellers_account_current`, CHECK active ⇒ is_current. `pii_cleared_at` after daily cron (19:05 UTC). Clears name/phone/email/logo, **not** address, birthday, gender, `certification_applications` PII, or `payuni_registrations.raw_request`.
- `seller_prospects.status` ARCHIVED (+ `merged_into_prospect_id`); vendor portal `REVOKED` / `revoked_at`.
- `food_safety_reports` phone/email "anonymised" after 90 days as unsalted `sha256(phone)`. Taiwan mobile numbers are about 10^8, so an unsalted hash is reversible by enumeration.
- Everything else hard rows (append-only logs).

### 2.6 Optimistic locking / version columns

| Column | Enforced when | Conflict code |
|---|---|---|
| `sellers.version` (bigint, CHECK ≥ 0), bumped by ~10 UPDATEs in `sellers.sql` | only when a caller passes `expectedVersion`; `-1`/nil skips (`expected_version < 0 OR version = ...`). Only the delegated path (`delegation.ParseExpectedVersion`) sends it: `UpdateSeller`, `UpdateLogo`, `UpdateCover`. Seller self-edits are last-write-wins | `bizcode.DelegationResourceVersionConflict` 9266 (`domain.ErrVersionConflict`) |
| `payuni_registrations.merchant_profile_version` | always on merchant-profile PATCH (If-Match) + `Idempotency-Key` with payload hash in `payuni_merchant_profile_updates` + `pg_advisory_xact_lock` | 9234 `PAYUNI_MERCHANT_PROFILE_VERSION_CONFLICT`, 9235 key reused, 9236 not active |
| `certification_applications.version` (int) | server-internal read-modify-write guard in `UpdateApplication` | generic `errors.NewConflict` (no bizcode) |
| `seller_prospects.media_version` | admin media PUT | 409 (generic) |
| prospect text fields | none (last-write-wins, `WHERE id AND status='ACTIVE'`) | — |

Four different optimistic-lock shapes. In the monolith this is a single convention question: AIP-154 `etag` in Connect, `ABORTED` + a `ErrorReason`.

## 3. Mechanisms (carry-over audit candidates)

### 3.1 Packages (go.mod direct)

`pgx/v5`, `looplab/fsm` (certification FSM, `domain/certification_fsm.go`), `redis/go-redis/v9`, `prometheus/client_golang` (outbox metrics, fail-open counters), `otel sdk/metric`, `uber/fx` (DI), `zap`, `golang.org/x/time` (PayUni rate limiter `client/payuni/ratelimit.go`), `grpc`. Enum codegen `enumer` (`zzz_enumer_*` for sellerType, payuniStatus, payuniMerStatus, reviewResult, reviewAction, certificationStatus).

### 3.2 Shared `pkg/` (import counts, `rg -o '"github.com/Will413028/hawkr/go-pkg/[^"]+"' --glob '!*_test.go'`)

database 29, logger 27, errors 26, outbox 13, mq 11, proto/seller 9, grpc interceptor 7, media 7, grpcclient 7, errors/bizcode 7, revalidate 6, twaddr 5, timeutil 4, syncutil 4, rolestate 4, pgxutil 4, crypto 3, server 2, redis 2, phone 2, geocode 2, config 2, codemeta 2, urlsafety 1, tsid 1, rolestate/rolestatepb 1, proto/{social,review,product,order,media,buyer,account} 1 each, payuni 1, pagination 1, otel 1, delegation 1.

Those that need a monolith owner decision: `twaddr` (city/district/postal code), `phone` (normalise), `geocode` (TGOS client, also used by buyer), `payuni` (shared with payment), `crypto`, `urlsafety`, `outbox` (Scheduler/DailyScheduler, replaced by River), `rolestate` (goes away when role removal is in-process), `delegation` (expectedVersion / capability for ADM writes → delegation module), `revalidate` (platform already has it as a River job).

### 3.3 Redis (4 uses)

1. `seller:suspended:<accountId>` marker written by `SyncRedis` after commit (`usecase/seller_status_recompute.go:140`); read by the gateway capability guard.
2. GCIS business-registration cache (`infrastructure/business_registration_cache.go`).
3. TGOS geocode cache (`infrastructure/address_geocode_cache.go`).
4. Public violations cache with fail-open metric (`infrastructure/violation_public_cache.go`).

Plus the gateway's own Redis `store:current:{accountId}` (60 s TTL, invalidated by `seller.onboarded`/`seller.deleted` over RabbitMQ), and cart's `seller:meta:{storeId}` (evicted by `seller.profile.changed`). In-process, (1) and the gateway resolver cache can become a DB read. Identity already reads account status from the DB on every request (ADR 2026-09-18 capability header). (2)–(4) are genuine external-call or hot-read caches.

### 3.4 Capability check (ADR 2026-09-18-seller-capability-authorization)

- Lives in the gateway, not seller: `gateway/internal/authz/{capability,policy}.go` has 14 `Cap*` constants. `router/seller_capability.go` `requireSeller(cap)` / `sellerHandle` registers method+path→cap and prepends the guard. A boot assertion (`seller_route_assert.go`) makes an undeclared seller-area route panic. 153 declared routes across 14 route files (`rg -c 'sellerHandle(Authed)?\(|sellerViewGuard\('`, minus 3 definitions). Also `sellerViewGuard` (`?view=seller`) and `injectSellerSuspension` (header `X-Seller-Suspended` for social comments).
- Suspension lookup: Redis `EXISTS`-style lookup, 500 ms. On a miss it goes back to seller-service `GetSellerByAccountID` (900 ms) and caches the result for 60 s. Any error → **fail-open**. The fail-closed backstops (ADR D4) are in Java: product ACTIVE transition and checkout validate. Their implementation was not verified here. product/order `SellerSuspendedConsumer` exist.
- Deny → `bizcode.SellerSuspendedOperationBlocked` **9240**, message "賣家帳號停權中，無法執行此操作", HTTP 409 via `response.BizDeny`. Plan A3: replace with `permission_denied` + `ErrorInfo.reason SELLER_SUSPENDED` (ADR `2026-09-25-api-error-model.md` D10). `rg SELLER_SUSPENDED api backend/internal` finds no ErrorReason yet (only `OrderCancelReasonSellerSuspended` in platform codemeta).
- Premises that change in the monolith: D3 ("per-request gRPC to seller-service adds a hot-path RPC and a SPOF") no longer holds. It becomes an in-process indexed read, the same cost identity already pays per request. D4's fail-open existed only because Redis sat in front, so with a DB read the answer is fail-closed or propagate the error. D1 "declaration = mounting, compile-time mandatory" is a good idea to keep: per-Connect-method declaration in `internal/httpapi/rpc`, with the boot/test assertion over all registered procedures. D7 tier: membership has no effect today.

### 3.5 Governance enforcement (ADR 2026-09-18-seller-governance-schema-and-enforcement)

- DB CHECKs: penalty field mutex (6 penalty types), source FK mutex, `status IN (enforced, revoked)`, "effective now" computed (`status='enforced' AND (expires_at IS NULL OR expires_at > now())`).
- Dedup: partial unique per reporter/seller/category/UTC calendar day; food safety per phone + seller + ISO week (`COALESCE(target_seller_id,0)`).
- `pg_advisory_xact_lock(hashtextextended('violation_create', seller))` to serialise the "90-day medium ≥3 → high" escalation.
- `recomputeSellerStatus` single derivation point (`Recompute`), priority closed > suspended > suspended_governance > active. Called from 6 places, then `SyncRedis` + website revalidate after commit.
- Cron (`app/cron`, in-process, per replica, via `pkg/outbox` schedulers): seller PII cleanup daily 19:05 UTC; PayUni status history cleanup daily 19:40 UTC; food-safety phone anonymise daily; seller status restore every 10 min (expired full suspensions → `RestoreSellerStatusActive`).
- Workers (`internal/worker`): PayUni sync poll every 15 min (`SyncPendingStatuses`); 3 DB outboxes with claim lease 3 min, backoff 1 min→1 h, 30-day retention monitors: `seller_deleted_outbox` (lifecycle: deleted/onboarded/suspended/unsuspended/membership_changed + account role sync), `merchant_verification_outbox`, `payuni_merchant_profile_outbox` (calls PayUni `UpdateMerchant`, already D5-shaped).
- Rate limit for the public food-safety form is in the gateway (10 min/5, 1 h/10 per IP).

### 3.6 Events (RabbitMQ; from `events.csv`, verified in code)

Produced: `seller.deleted` (gateway, im, notification, pricebook, product, social), `seller.onboarded` (gateway), `seller.suspended` / `seller.unsuspended` (coupon, im, notification, order, product), `seller.profile.changed` (cart; **direct best-effort publish**, not outbox), `seller.membership_changed` (ai), `merchant.verification.{approved,forwarded,pending_info,rejected,under_review}` (notification), `seller.governance.*` ×5 (no exchange, no consumer).
Consumed: `account.deleted` (release logo/cover media refs), `product.viewed` (view log), `review.created` / `review.hidden` (overwrite `sellers.rating`), `seller.review.result` from payuni-webhook-service (PayUni agent review result).
Planned in inventory but not implemented: `payment.membership.seller.{success,failed}`.

### 3.7 External providers

PayUni (bind_merchant, get status, update_merchant; `client/payuni`, rate-limited). GCIS company registry (5 s timeout). TGOS address via `pkg/geocode` (via proxy relay). Website on-demand revalidate. Media service (Java) for Finalize/Link/Release. No Uber and no R2 in seller-service (`rg -il uber` hits are false positives). D5 status: `registerPayuni` is called **synchronously inside the admin Approve request** (not in a tx, but not intent+job either). `RefreshPayuniStatus` and the 15-min sync call PayUni in-request/in-loop. Only the merchant-profile update follows D5.

### 3.8 Upward calls (dependency table allows seller → identity, media only)

- order: `GetTopSellersByRevenue` (ranking → A11); `GetActiveMerchantOrderCount` is a **stub returning 0** (`client/order/client.go:30`) → port from ordering (ADR lists it).
- product: `ListStoreIDsWithPublicProducts` (ranking), `GetProductsByIDs` (product-view → resolve seller; the product module should emit it with store_id).
- review: `GetSellerReviewStats` in `GetSeller`/`GetSellerStats` handler → http composition.
- buyer: `GetSubscriberCount` (stats → http), `BatchGetSubscriberCounts` (ranking), `HasActiveBuyer` (rolestate for role removal → disappears with in-process identity).
- social: `HasPublishedPost` → port (ADR lists it).
- ADR 2026-09-24 table says "seller's upward calls to order/product/review are all ranking". That is inaccurate: review stats, product-view resolution and buyer subscriber count are not ranking.

### 3.9 CodeMeta domains and error codes used

CodeMeta (`使用服務` contains seller): `PayuniMerchantVerificationStatus`, `PayuniMemberVerificationStatus`, `PayuniPaymentChannelStatus`, `SellerSuspensionReason` (9601/9602/9603, stored as Value in `seller_suspensions.reason_code`), `PayuniErrorRemedy`, `PayuniSellerFacingError` (`codemeta.PayuniSellerFacingErrorLabel` ×3), `MembershipEventType`, `MembershipBillingErrorCode`, `OrderEventType`, `SellerEventType`. Governance enums (category, severity, penalty_type, appeal_status, report status), seller status, cert status and prospect enums are Go types + CHECK only, not CodeMeta.

bizcodes (`rg -o 'bizcode\.\w+'`): 9204 AccountAlreadyInDeletion, 9205 SellerHasActiveOrders, 9230 SellerNotActive, 9231 FoodBusinessRegNoInvalid, 9232 SellerWebsiteUrlInvalid, 9233 SellerNotFound, 9234/9235/9236 PayuniMerchantProfile*, 9266 DelegationResourceVersionConflict, 9410 SellerInvalidPlanCode, 9700 FeatureAddressLookupDisabled, 9704 FeaturePayuniMerchantDisabled. Gateway-side: 9240 SellerSuspendedOperationBlocked (9241 SellerSuspendedProductUnavailable is product-side). Also `ErrInvalidTransition` DomainCode 1200. Many generic `errors.NewPreconditionFailed/Conflict/InvalidInput` with Chinese text in `Error()`. The handler puts `err.Error()` into a success response's `Message` (~55 sites, per `domain/errors.go` comment), so UX text is part of the wire contract.

### 3.10 Other conventions to audit

- gRPC "Code in body + OK status" envelope (`SellerResponse{Code, Message}`), replaced by Connect errors.
- `fx` DI, replaced by the monolith's explicit composition.
- Per-replica in-process schedulers (`pkg/outbox.Scheduler`), replaced by River periodic jobs.
- `seller_deleted_outbox` is really a generic lifecycle outbox (5 event types). With in-process events and River it goes away.
- `account_roles.source_id` fence (#3189) exists only because role removal is async from seller. Identity's AGENTS.md says to remove it at A3.
- `sellers.rating` (double) projection of review data. Owner question: should review own it and seller drop the column (the http layer composes it)?
- `sellers.birthday`, `gender`, `prize` on the store table: personal attributes on a store row.
- `sellers.email`, `contact_phone` copied from account at create and never re-synced (replicas.csv).

## 4. Apply to become seller (`POST /app/sellers`)

Order today:

1. Monolith → gatewayproxy → gateway `ApplySeller` (`gateway/internal/handler/seller.go`). JWTAuth only, no role guard. Binds `{storeName, description, contactPhone, city, district, addressDetail, prize}`. No validation.
2. gRPC `CreateSeller` → seller-service `grpc/handler.go:168`:
   a. `usecase.CreateSeller` (`usecase/seller.go:213`): `accountClient.GetAccountInfo` (now identity via `internal/legacy/accountservice`; error only logged) fills email/phone. A random store name is used if blank. Then tx: `GetByAccountIDForUpdate`. If a non-closed seller exists → Conflict "您已經是賣家". If closed → `RetireCurrentStint` + new store_id. `sellerRepo.Create`, then outbox `seller.onboarded`. Retries on store_code collision. **Commit.**
   b. After commit: `accountClient.BindSellerRole(account, store_id)` synchronously (error only logged; the outbox row converges it). **SELLER role granted, roles_epoch bumped.**
   c. Prospect attribution if a token was given (never, see §1).
   d. Back in the handler: **only if** `description`, `city` or `prize` is non-empty, call `UpdateProfile`. It is there that `twaddr.CanonicalCity` validates the city (`seller.go:339-345`) → `InvalidInput "invalid Taiwan city"` → returned as `SellerResponse{Code}` → gateway 400. **Store row, stint, outbox and SELLER role are already committed.** A retry now hits "您已經是賣家" (Conflict), so the user can only fix it through `PUT /app/sellers/me`.
3. Gateway invalidates the `storeid` resolver cache; the client refreshes the token (roles_epoch).

Additional defects in the same flow:

- `district`/`addressDetail` are silently dropped when description, city and prize are all empty (the guard does not test them).
- When the second step runs with an empty `contactPhone`, `UpdateProfile` writes `NormalizeToLocal("")` = "", wiping the phone that step (a) defaulted from the account. `UpdateProfile` has PUT semantics: empty description, city or prize also clear the field.
- District is never validated (`LookupPostalCodeByCityDistrict` just returns "").
- The DB `chk_sellers_city` CHECK uses Chinese names (`台北市` …), so city-code canonicalisation is mandatory before insert.

Monolith target (per plan): validate the full input (city/district via `twaddr`, phone, name) first; one transaction for create stint + grant role (in-process identity call) + profile fields; outbox/role fence not needed.

## 5. Related ADRs: what each decides, and whether its premises still hold

| ADR | Decides | Premise in a pre-launch monolith |
|---|---|---|
| 2026-06-29-identity-defer-store-entity | No `store`/`store_staff` entity; one account = at most one current store, expressed by the seller aggregate with stint surrogate | Holds (still one-person-one-store, no multi-store/transfer requirement). The "blast radius confined to seller-service" argument is gone, but so is the cost of re-keying (no data). Re-evaluate cheaply now if multi-store/staff is ever likely; otherwise keep |
| 2026-07-11-identity-store-stint-symmetry | Re-onboarding inserts a new stint row (`is_current`, new store_id + new store_code) rather than rewriting in place; buyers mirror it | The PayUni reason holds: MerID `HAWK{store_code}` is a one-time external binding, so a new stint needs a new store_code. "Resolver in gateway, 8 downstream services key by surrogate" becomes in-process. The buyer symmetry is buyer's call (A3 buyer half) |
| 2026-07-20-seller-kyc-pii-encryption-scope | Encrypt only `id_card_no` (+ `bank_account`); `real_name`/`birth_date` plaintext for admin ILIKE; Base64 in varchar; step_data sensitive keys | Constraint "don't split the table" and "Go backfill with downtime" no longer apply (no data, fresh schema). A clean redesign (separate identity/PII table or one `pii_ciphertext` blob + blind index for admin name search) is now cheap. Also broken in practice by the plaintext `raw_request` (§2.4). **Needs a decision** |
| 2026-08-19-identity-current-store-id-rpc | Gateway `storeid.Resolver` uses a dedicated `GetCurrentStoreID` returning gRPC NotFound rather than parsing UX business codes | The gateway/gRPC premise disappears. The principle (a narrow typed API for "current store of account", never reuse UX errors) carries over as a module API `CurrentStoreID(ctx, accountID) (StoreID, bool, error)`. Supersede or annotate |
| 2026-09-11-uber-commerce-model-seller-direct | Uber orders are seller-direct: buyer pays into the seller's PayUni sub-merchant, platform bills the seller a logistics fee | Touches seller only via the premise that every seller is its own PayUni sub-merchant (MerID from store_code). Still holds. No seller-module change; relevant to fulfillment/payment/finance |
| 2026-09-18-seller-capability-authorization | Coarse capabilities declared at mount (compile-time), Redis `seller:suspended` marker written by seller, gateway fail-open + two fail-closed backstops, 9240 | D3/D4 premises (separate gateway process, avoid per-request RPC, Redis blip) do not hold in-process. D1/D2/D9 (declaration = mounting, coarse groups, seller vs admin registries) still sound. 9240 → `SELLER_SUSPENDED` (D10). Plan already asks for re-evaluation |
| 2026-09-18-seller-governance-schema-and-enforcement | Governance tables separate from social reports; penalty columns + CHECK mutex; status two-valued with computed "effective"; partial-unique dedup; IP rate limit; advisory lock; single `recomputeSellerStatus` | Holds. Premises are schema-level, not deployment-level. "Tables in seller-service" → schema `seller` (or a separate governance submodule). Notification of governance events is broken (no exchange). Decision 8's "admin UI polling" assumes an admin who opens the UI regularly |
| 2026-09-20-seller-stint-assistant-history | New stint = new AI assistant conversation space (keyed by store_id); no cross-stint history | Holds; depends only on stint model + ai-service. No seller-module work. Keep, and relevant again at A9 |

Also relevant: `2026-09-25-api-error-model.md` D10 (SELLER_SUSPENDED), `2026-09-25-codemeta-retirement.md`, `2026-09-25-row-identity-by-data-kind.md`, and `2026-09-24-go-modular-monolith.md` (dependency table, ports, the inaccurate "all upward calls are ranking" line).

## 6. Suspected defects and surprising behaviour

1. **Apply flow commits store + role before validating the city** (known; §4), plus phone wipe, dropped district/addressDetail, and no district validation.
2. **Plaintext national ID in `payuni_registrations.raw_request`**, written on every bind attempt, including failures. `payuni_status_history` copies only `raw_response`. This defeats ADR 2026-07-20.
3. **Governance full suspension never emits `seller.suspended`/`seller.unsuspended`**. `violation.go` only runs `Recompute` + `SyncRedis`, and the restore cron likewise. So products stay listed, pending-payment orders are not cancelled and coupons stay active for `suspended_governance` sellers. Only the gateway 9240 guard applies. Only admin manual suspension (`seller_suspension.go:119,210`) writes the lifecycle outbox.
4. **Suspended sellers cannot close their store, not even by admin**. `DeleteSellerAccount` requires `IsActive()` and returns 9204 "帳號已在刪除流程中", which is misleading. This contradicts CAP_ACCOUNT_CLOSE being whitelisted for suspended sellers (ADR) and blocks admin account deletion of a suspended seller.
5. **Active-order check on seller deletion is a stub returning 0** (`client/order/client.go:30`). 9205 is unreachable, and the admin path forces `OverrideActiveOrders=true` anyway.
6. **Media reference leak**: logo/cover are linked with `ref_id = store_id` (`seller.go:828`), but the `account.deleted` consumer releases `ref_id = account_id`, so the release never matches.
7. **PII cleanup is partial**: it clears name/phone/email/logo only. Address, birthday, gender, KYC rows (real_name, birth_date, encrypted ID/bank), PayUni contact fields and raw_request are kept indefinitely.
8. **Unsalted sha256 "anonymisation"** of reporter phone/email in food-safety reports is reversible.
9. `Approve`: `GetByAccountIDForUpdate` runs outside any transaction (the lock is meaningless). The PayUni bind is a synchronous external call in the admin request. The food-business-reg projection is outside the tx and only logged on failure. Concurrent approvals rely on the AGENT04017 self-heal.
10. `seller.profile.changed` is a direct best-effort publish (no outbox), so cart's `seller:meta` cache can stay stale.
11. The five `seller.governance.*` events are published to an exchange that is not provisioned, with no consumer. The `seller_notification_log` "sent" audit therefore records notifications nobody received.
12. `seller_store_visitors` is write-only (no client reads the visitors route). Each public storefront view by a logged-in buyer fires a goroutine RPC.
13. Membership: plans are assignable, but nothing enforces `monthly_order_cap`, prices are 0, and billing is not implemented. The only effect is ranking facts.
14. Prospect invite attribution on apply is dead code (the token is never sent).
15. Seller self-edits have no optimistic lock; only delegated writes send `expectedVersion`. Four different lock/conflict conventions (§2.6).
16. Store-scoped tables use the old column name `seller_id` for store_id, while `seller_suspensions` is keyed by store but queried by account. `certification_applications` and `payuni_status_history` are keyed by account_id rather than store_id, so a re-onboarded stint sees the previous stint's certification history via account.
17. The gateway capability check fails open on any Redis or seller-service error, including the 900 ms fallback RPC timeout.

## 7. Design questions for Will

1. D8 scope: drop `seller_store_visitors` + visitors RPCs, the 7 zero-caller RPCs, prospect-invite attribution, governance MQ events (or wire them to notification at A8)? Membership: migrate as-is (admin + ranking signal only), or drop until cap/billing are real?
2. Ownership: should prospects + vendor portal (procurement/BD) live in seller, or move to agent/procurement (A9) or a separate module? Should governance be a submodule of seller or its own module?
3. KYC PII: redesign now (separate PII table / whole-record ciphertext + blind index for admin name search, drop plaintext `raw_request`), or keep the 2026-07-20 scope?
4. Suspension gate: in-process DB read (fail-closed) instead of the Redis marker + fail-open. Keep per-Connect-method capability declaration with a startup assertion?
5. Governance suspension should emit the same suspended/unsuspended event as admin suspension. Confirm that is intended (fix #3).
6. Suspended seller self-close / admin delete: allowed? (#4)
7. IDs: store_id as IDENTITY bigint (transitional rule) or UUIDv7? Keep store_code as the external PayUni-facing code?
8. Optimistic locking: one etag convention for seller profile (all writers), certification and PayUni profile?
9. PayUni bind at approve → intent row + River job (D5), making approve asynchronous (`forwarded` set by the job)?
