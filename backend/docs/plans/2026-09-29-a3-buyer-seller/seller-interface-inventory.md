# A3 seller — interface inventory (read-only)

Snapshot: working tree at `bdf13a034` (2026-09-29). Paths relative to repo root unless noted. All counts are reproducible with the helper scripts kept next to this file in the scratchpad (`callers.py`, `routes.py`, `clients2.py`); commands are quoted where a number is stated.

## 0. Summary

- **RPC count is 98, not ~146.** `rg -c "^SellerService," backend/docs/plans/2026-09-24-go-modular-monolith/rpc-servers.csv` → 98; `rg -c "^\s*rpc " backend/api/proto/seller/seller.proto` → 98; the two name lists diff clean. seller-service implements exactly one gRPC service, `hawkr.seller.SellerService` (the Java copy under `services/java/shared/java-proto/.../seller/seller.proto` also has 98). All 98 `disposition` cells are empty.
- **Live callers, by kind** (current tree; `callers.py` greps `.RPC(` in non-test Go files that import the seller proto / `SellerServiceClient` / `sellerconnect`, and `.rpc(` in Java files importing `SellerServiceGrpc`; one false positive removed: aftersales `getPayuniStatus()` is a message getter):
  - gateway: 82 RPCs (80 reached from a route/middleware + 2 dead wrappers).
  - Java (stay after A3 → need a legacy gRPC server in the monolith): **7 RPCs** — `BatchGetSellerMeta`, `ResolvePrincipals`, `GetSellerBatch`, `ListPublicStoreIDsByCity`, `ListPublicStoresByCity`, `MatchCertificateHolder`, `GetSellerShippingProfile`. Callers: aftersales, cart, coupon, media(Java), order, product, shipping.
  - Unmigrated Go services: **10 RPCs** — social (`GetSeller`, `BatchGetSellerMeta`, `ResolvePrincipals`), im (`GetSeller`, `GetSellerBatch`, `ListStoreIDsByAccount`, `ResolvePrincipals`), notification (`EnsureVendorPortalInvite`, `MarkVendorPortalInviteSecretDelivered`, `ListStoreIDsByAccount`), review (`BatchGetSellerMeta`, `ResolvePrincipals`), invoice (`BatchGetSellerMeta`, `ResolveSellerAccountID`), ai (`BatchGetSellerMeta`, `GetSellerStats`, `SearchSellerProspects`). crm, pricebook: none.
  - Union Java ∪ unmigrated Go = **14 RPCs** (`rg -c "LEGACY-SERVER" rpc_table.md` → 14). Plus the gateway (it keeps routes of every unmigrated module) still needs `GetCurrentStoreID`, `GetSeller` (suspension guard + delegation guard), `GetSellerBatch`, `ResolveSellerAccountID` → a legacy SellerService server in the monolith needs ~15 RPCs.
  - buyer-service (migrates in the same A3): `GetSellerBatch`, `GetSellerByUserId`, `ResolvePrincipals` — these become in-process or move to http composition ("buyer 往上的補資料移到 http").
  - monolith: `internal/legacy/sellerservice` calls `GetSeller` (delegation store state).
  - account-service's `GetSeller` caller is gone with A2.
- **No live caller: 7 RPCs** — `GetSellerByMemberId`, `ValidateSeller`, `RegisterPayuni`, `RetryPayuniRegistration`, `GetSellerPayuniConfig` (no caller anywhere except test mocks), `SaveCertificationDraft`, `SubmitCertification` (only an unused gateway wrapper in `gateway/internal/grpc/seller_client.go:222,234`; the "Certification: Old API (Deprecated)" set). `GetCertificationDraft` of that old set is still read by `GET /app/sellers/me`.
- **Gateway REST: 92 routes reach seller-service** (`routes.py`, 598 routes parsed) — 16 public, 29 app, 47 admin. 88 of them are in `backend/api/openapi/openapi.yaml` (569 ops); the 4 delegated storefront routes are not (they are in gateway's `routesNotInSpec`, A10 territory). About 78 are seller-owned; 14 belong to other modules and only resolve/enrich via seller (list in §4.2).
- **Cross-cutting gateway coupling is larger than the seller routes**: store resolution (`GetCurrentStoreID`, 130 call sites in 28 handler files: `rg -c "resolveStoreID\(|h\.Store\.CurrentStoreID\(" gateway/internal/handler --glob '!*_test.go'`), the suspension guard `requireSeller(cap)` on ~150 capability-registered routes across 14 route files (`rg -c "sellerHandle(Authed)?\(" gateway/internal/router --glob '!*_test.go'` → 152 incl. 2 definitions), `injectSellerSuspension` on 9 social/chat routes, `sellerViewGuard` on 3 order/coupon routes, and the delegation guard `GetStoreStateForGuard` on all 22 delegated routes.
- **Events**: 34 rows in `events.csv` touch seller-service: it produces 16 routing keys (6 seller.* lifecycle/profile, 5 merchant.verification.*, 5 seller.governance.* that are producer-only) and consumes 7 (account.deleted, product.viewed, review.created, review.hidden, seller.review.result, and 2 payment.membership.seller.* that are not implemented).
- **Clients**: website ~34 paths/41 files, admin ~35/19, buyer-app ~11/17, seller-app ~26/25 (heuristic, method-agnostic; see §5). 7 routes have no client caller. 9240 handling: website `lib/api/client.ts` + `SellerCenterShell.tsx`; seller-app `core/networking/dio_provider.dart` + `session_provider.dart`; admin none; buyer-app handles a different code (product-unavailable-due-to-suspension) in cart/checkout.

## 1. RPCs (all under `SellerService`)

One gRPC service only, so grouping below is by the nearest comment header in `seller.proto` (approximate: some headers cover more than their topic, e.g. "PAYUNi 金流整合" also spans the core CRUD block). Columns: Java callers stay after A3; "unmigrated Go" = social/crm/im/notification/pricebook/review/invoice/ai; "buyer (A3)" migrates together. Gateway column lists the routes that reach the RPC (from `routes.json`) plus middleware use.

Call-site evidence (primary client files):
- Java: `services/java/{aftersales,cart,coupon,order/checkout,product,shipping}-service/.../SellerGrpcClient.java`, `services/java/media-service/.../client/SellerGrpcClient.java:69` (ResolvePrincipals). No Java caller was added or removed since the `6c8bc0c15` snapshot (`rpc-calls.csv` matches).
- Go: `services/{social,im,notification,review,invoice,ai,buyer}-service/internal/client/seller/*.go`.
- monolith: `backend/internal/legacy/sellerservice/sellerservice.go:58` (GetSeller via connect-go gRPC, 1200 ms deadline).

| # | RPC | nearest proto header (approx) | Java callers (stay after A3) | unmigrated Go callers | buyer (A3) | monolith | gateway routes | flag |
|---|---|---|---|---|---|---|---|---|
| 1 | GetSeller | (top) |  | im-service, social-service |  | legacy/sellerservice | GET /admin/certifications/:id; GET /admin/sellers/:store_id; POST /app/chat/messages; GET /admin/sellers/:store_id/storefront; PATCH /admin/sellers/:store_id/storefront; POST /admin/sellers/:store_id/storefront/logo; POST /admin/sellers/:store_id/storefront/cover; GET /public/sellers/:id/posts; GET /public/sellers/:id; GET /public/sellers/:id/products; GET /public/sellers/:id/reviews; GET /public/sellers/:id/reviews/stats; GET /app/sellers/me; GET /app/share_url; + gateway suspension guard origin (GetSellerByAccountID, requireSeller on ~150 routes); + delegation guard GetStoreStateForGuard (22 delegated routes) | LEGACY-SERVER(Go) |
| 2 | ResolveSellerAccountID | (top) |  | invoice-service |  |  | PATCH /admin/sellers/:store_id/seo; POST /admin/sellers/:store_id/suspensions; GET /admin/sellers/:store_id/suspensions; POST /admin/sellers/:store_id/suspensions/:id/lift; GET /admin/sellers/:store_id/suspension | LEGACY-SERVER(Go) |
| 3 | GetCurrentStoreID | (top) |  |  |  |  | gateway storeid.Resolver (130 call sites / 28 handler files; x-store-id for every seller route) |  |
| 4 | GetSellerByMemberId | (top) |  |  |  |  |  | NO CALLER |
| 5 | GetSellerByUserId | (top) |  |  | yes |  |  |  |
| 6 | GetSellerBatch | (top) | aftersales-service | im-service | yes |  | GET /public/marketplace; GET /public/sellers/:id/posts; GET /public/posts | LEGACY-SERVER(Java) LEGACY-SERVER(Go) |
| 7 | ValidateSeller | (top) |  |  |  |  |  | NO CALLER |
| 8 | GetSellerStats | (top) |  | ai-service |  |  | GET /public/sellers/:id/stats; GET /public/sellers/:id/rating; GET /app/sellers/me/dashboard | LEGACY-SERVER(Go) |
| 9 | RegisterPayuni | PAYUNi 金流整合 |  |  |  |  |  | NO CALLER |
| 10 | GetPayuniStatus | PAYUNi 金流整合 |  |  |  |  | GET /admin/sellers/:store_id; GET /app/sellers/me/payuni/status; POST /app/sellers/me/payuni/status_syncs |  |
| 11 | RetryPayuniRegistration | PAYUNi 金流整合 |  |  |  |  |  | NO CALLER |
| 12 | RefreshPayuniStatus | PAYUNi 金流整合 |  |  |  |  | POST /app/sellers/me/payuni/status_syncs |  |
| 13 | UpdateMerchantProfile | PAYUNi 金流整合 |  |  |  |  | PATCH /app/sellers/me/payuni/merchant_profile |  |
| 14 | GetSellerPayuniConfig | PAYUNi 金流整合 |  |  |  |  |  | NO CALLER |
| 15 | MatchCertificateHolder | PAYUNi 金流整合 | product-service |  |  |  |  | LEGACY-SERVER(Java) |
| 16 | CreateSeller | PAYUNi 金流整合 |  |  |  |  | POST /app/sellers |  |
| 17 | UpdateSeller | PAYUNi 金流整合 |  |  |  |  | PATCH /admin/sellers/:store_id/storefront; PUT /app/sellers/me |  |
| 18 | UpdateSellerSEO | PAYUNi 金流整合 |  |  |  |  | PATCH /admin/sellers/:store_id/seo |  |
| 19 | UpdateSellerFoodRegistration | PAYUNi 金流整合 |  |  |  |  | PUT /app/sellers/me/food-business-registration |  |
| 20 | CompleteOnboarding | PAYUNi 金流整合 |  |  |  |  | PATCH /app/sellers/me/onboarding |  |
| 21 | UpdateLogo | PAYUNi 金流整合 |  |  |  |  | POST /admin/sellers/:store_id/storefront/logo; PUT /app/sellers/me/logo |  |
| 22 | UpdateCover | PAYUNi 金流整合 |  |  |  |  | POST /admin/sellers/:store_id/storefront/cover; PUT /app/sellers/me/cover |  |
| 23 | GetCertificationDraft | Certification: Old API (Deprecated) |  |  |  |  | GET /app/sellers/me |  |
| 24 | SaveCertificationDraft | Certification: Old API (Deprecated) |  |  |  |  | gateway wrapper only, no handler (dead) | NO CALLER |
| 25 | SubmitCertification | Certification: Old API (Deprecated) |  |  |  |  | gateway wrapper only, no handler (dead) | NO CALLER |
| 26 | CreateCertificationDraft | Certification: New API |  |  |  |  | POST /app/certifications |  |
| 27 | SaveCertificationProgress | Certification: New API |  |  |  |  | PUT /app/certifications/:id/progress |  |
| 28 | CancelCertification | Certification: New API |  |  |  |  | DELETE /app/certifications/:id |  |
| 29 | SubmitCertificationApplication | Certification: New API |  |  |  |  | PATCH /app/certifications/:id |  |
| 30 | GetCertificationApplication | Certification: New API |  |  |  |  | GET /app/certifications/:id; GET /app/certifications/:id/reviews; PUT /app/certifications/:id/progress; PATCH /app/certifications/:id; DELETE /app/certifications/:id |  |
| 31 | LookupBusinessRegistration | Certification: New API |  |  |  |  | GET /app/businessRegistration/:ban |  |
| 32 | SuggestAddress | Certification: New API |  |  |  |  | GET /app/address/suggestions |  |
| 33 | ValidateAddress | Certification: New API |  |  |  |  | GET /app/address/validate |  |
| 34 | ApproveCertification | Certification: New API |  |  |  |  | PATCH /admin/certifications/:id |  |
| 35 | RejectCertification | Certification: New API |  |  |  |  | PATCH /admin/certifications/:id |  |
| 36 | GetCertificationReviews | Certification: New API |  |  |  |  | GET /admin/certifications/:id/audit-logs; GET /app/certifications/:id/reviews |  |
| 37 | ListSellersForAdmin | Admin: Seller Management |  |  |  |  | GET /admin/sellers |  |
| 38 | GetSellerStatsForAdmin | Admin: Seller Management |  |  |  |  | GET /admin/sellers/stats |  |
| 39 | ListCertificationsForAdmin | Admin: Certification Management |  |  |  |  | GET /admin/certifications |  |
| 40 | GetCertificationApplicationForAdmin | Admin: Certification Management |  |  |  |  | GET /admin/certifications/:id; GET /admin/sellers/:store_id |  |
| 41 | BatchGetSellerMeta | Cross-Service: Seller Meta | cart-service, coupon-service, order-service, product-service, shipping-service | ai-service, invoice-service, review-service, social-service |  |  |  | LEGACY-SERVER(Java) LEGACY-SERVER(Go) |
| 42 | ListPublicStoreIDsByCity | Cross-Service: Seller Meta | product-service |  |  |  |  | LEGACY-SERVER(Java) |
| 43 | ListPublicStoresByCity | Cross-Service: Seller Meta | product-service |  |  |  |  | LEGACY-SERVER(Java) |
| 44 | ResolvePrincipals | Cross-Role Guards: Principal Resolution | aftersales-service, cart-service, coupon-service, media-service, order-service, shipping-service | im-service, review-service, social-service | yes |  |  | LEGACY-SERVER(Java) LEGACY-SERVER(Go) |
| 45 | ListStoreIDsByAccount | Cross-Service: Historical Stint Enumeration (internal-use only) |  | im-service, notification-service |  |  |  | LEGACY-SERVER(Go) |
| 46 | GetSellerShippingProfile | Cross-Service: Shipping Profile | shipping-service |  |  |  |  | LEGACY-SERVER(Java) |
| 47 | RecordStoreVisit | Store Visitors |  |  |  |  | GET /public/sellers/:id |  |
| 48 | GetStoreVisitors | Store Visitors |  |  |  |  | GET /app/sellers/me/store_visitors |  |
| 49 | RecordProductView | Product View Stats |  |  |  |  | POST /public/products/:id/view |  |
| 50 | GetProductViewStats | Product View Stats |  |  |  |  | GET /app/sellers/me/dashboard |  |
| 51 | GetRecommendedSellers | Recommended Sellers |  |  |  |  | GET /public/marketplace; GET /public/sellers/recommended | A11 |
| 52 | DeleteSellerAccount | Self-Service: Account Deletion |  |  |  |  | DELETE /admin/accounts/:id; DELETE /app/sellers/me |  |
| 53 | CreateViolationReport | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /app/violation_reports |  |
| 54 | ListMyViolationReports | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | GET /app/violation_reports/me |  |
| 55 | CreateFoodSafetyReport | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /public/food_safety_reports |  |
| 56 | SellerListMyViolations | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | GET /app/sellers/me/violations |  |
| 57 | SellerSubmitAppeal | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /app/sellers/me/violations/:id/appeal |  |
| 58 | SellerGetAppeal | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | GET /app/sellers/me/violations/:id/appeal |  |
| 59 | GetPublicSellerViolations | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | GET /public/sellers/:id/violations/public |  |
| 60 | AdminListViolationReports | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | GET /admin/violation_reports |  |
| 61 | AdminPatchSellerViolationReportStatus | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | PATCH /admin/seller_violation_reports/:id |  |
| 62 | AdminPatchFoodSafetyReportStatus | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | PATCH /admin/food_safety_reports/:id |  |
| 63 | AdminAcknowledgeFoodSafetyReport | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /admin/food_safety_reports/:id/ack |  |
| 64 | AdminGenerateFDAReport | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | GET /admin/food_safety_reports/:id/fda_export |  |
| 65 | AdminMarkFDAReported | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /admin/food_safety_reports/:id/mark_fda_reported |  |
| 66 | AdminNotifySeller | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /admin/food_safety_reports/:id/notify_seller |  |
| 67 | AdminCreateViolation | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /admin/violations |  |
| 68 | AdminGetViolation | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | GET /admin/violations/:id |  |
| 69 | AdminUpdateViolation | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | PATCH /admin/violations/:id |  |
| 70 | AdminRevokeViolation | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /admin/violations/:id/revoke |  |
| 71 | AdminDecideAppeal | B-169 Governance: 商家違規 / 食安通報 / 申訴 (issue #722) |  |  |  |  | POST /admin/violations/:id/appeal/decide |  |
| 72 | SuspendSellerForAdmin | Admin 手動停權（不服勸導，32 §13 / 46 §5） |  |  |  |  | POST /admin/sellers/:store_id/suspensions |  |
| 73 | LiftSellerSuspensionForAdmin | Admin 手動停權（不服勸導，32 §13 / 46 §5） |  |  |  |  | POST /admin/sellers/:store_id/suspensions/:id/lift |  |
| 74 | ListSellerSuspensionsForAdmin | Admin 手動停權（不服勸導，32 §13 / 46 §5） |  |  |  |  | GET /admin/sellers/:store_id/suspensions |  |
| 75 | GetActiveSellerSuspensionForAdmin | Admin 手動停權（不服勸導，32 §13 / 46 §5） |  |  |  |  | GET /admin/sellers/:store_id/suspension |  |
| 76 | AssignSellerMembershipPlan | Admin 賣家會員分級方案（子系統 1） |  |  |  |  | POST /admin/sellers/:store_id/membership |  |
| 77 | GetSellerMembershipPlan | Admin 賣家會員分級方案（子系統 1） |  |  |  |  | GET /admin/sellers/:store_id/membership |  |
| 78 | ListSellerMembershipPlans | Admin 賣家會員分級方案（子系統 1） |  |  |  |  | GET /admin/seller_membership_plans |  |
| 79 | UpdateSellerMembershipPlan | Admin 賣家會員分級方案（子系統 1） |  |  |  |  | PATCH /admin/seller_membership_plans/:plan_code |  |
| 80 | SearchSellerProspects | 站外業者主檔（招商名單） |  | ai-service |  |  |  | LEGACY-SERVER(Go) |
| 81 | CreateSellerProspect | 站外業者主檔（招商名單） |  |  |  |  | POST /admin/seller_prospects; POST /admin/seller_prospects/preview |  |
| 82 | AdminListSellerProspects | 站外業者主檔 — Admin console（階段二） |  |  |  |  | GET /admin/seller_prospects |  |
| 83 | AdminGetSellerProspect | 站外業者主檔 — Admin console（階段二） |  |  |  |  | GET /admin/seller_prospects/:id |  |
| 84 | AdminUpdateSellerProspect | 站外業者主檔 — Admin console（階段二） |  |  |  |  | PUT /admin/seller_prospects/:id |  |
| 85 | AdminArchiveSellerProspect | 站外業者主檔 — Admin console（階段二） |  |  |  |  | DELETE /admin/seller_prospects/:id; POST /admin/seller_prospects/:id/restore |  |
| 86 | AdminListDuplicateProspects | 站外業者主檔 — Admin console（階段二） |  |  |  |  | GET /admin/seller_prospects/duplicates |  |
| 87 | AdminMergeSellerProspects | 站外業者主檔 — Admin console（階段二） |  |  |  |  | POST /admin/seller_prospects/merge |  |
| 88 | AdminConvertSellerProspect | 站外業者主檔 — Admin console（階段二） |  |  |  |  | POST /admin/seller_prospects/:id/convert |  |
| 89 | AdminUpdateSellerProspectMedia | 站外業者主檔 — Admin console（階段二） |  |  |  |  | PUT /admin/seller_prospects/:id/media |  |
| 90 | AdminListProspectTaxIdMatches | 站外業者主檔 — Admin console（階段二） |  |  |  |  | GET /admin/seller_prospects/reconciliation |  |
| 91 | AdminCreateProspectInvite | 站外業者主檔 — Admin console（階段二） |  |  |  |  | POST /admin/seller_prospects/:id/invites |  |
| 92 | AdminRevokeProspectInvite | 站外業者主檔 — Admin console（階段二） |  |  |  |  | DELETE /admin/seller_prospects/:id/invites/:invite_id |  |
| 93 | EnsureVendorPortalInvite | Vendor Portal (#2539 E1) |  | notification-service |  |  |  | LEGACY-SERVER(Go) |
| 94 | MarkVendorPortalInviteSecretDelivered | Vendor Portal (#2539 E1) |  | notification-service |  |  |  | LEGACY-SERVER(Go) |
| 95 | ExchangeVendorPortalInvite | Vendor Portal (#2539 E1) |  |  |  |  | POST /public/vendor_portal/invite_exchanges |  |
| 96 | ValidateVendorPortalSession | Vendor Portal (#2539 E1) |  |  |  |  | middleware.VendorPortalSession on 7 /public/vendor_portal/* routes |  |
| 97 | StepUpVendorPortalSession | Vendor Portal (#2539 E1) |  |  |  |  | POST /public/vendor_portal/step_up/google |  |
| 98 | PreviewVendorPortalInvite | Vendor Portal (#2539 E1) |  |  |  |  | GET /public/vendor_portal/invites/preview |  |


## 2. Events, replicas, id references

### 2.1 `events.csv` rows touching seller-service (34 rows, all `disposition` empty)

Produced by seller-service:

| routing key | exchange | consumer | purpose / class | how produced now |
|---|---|---|---|---|
| seller.deleted | hawkr.seller | gateway (realtime `SellerLifecycleChannel`), im, notification, pricebook, product(Java), social | store resolver cache bust; chat/inbox/market-watch/post cleanup; delist products — STATE/PROJECTION | `seller_deleted_outbox` + worker (`internal/worker/seller_deleted_outbox_worker.go`), also drives account `RemoveRole` |
| seller.onboarded | hawkr.seller | gateway | resolver cache bust — PROJECTION | same outbox; its grant sub-step re-calls account `BindSeller` (level-triggered) |
| seller.suspended | hawkr.seller | coupon(Java), im, notification, order(Java), product(Java) | disable coupons, WS disconnect, notify, cancel unpaid orders, off-shelf — STATE/SIDE_EFFECT | same outbox (`usecase/seller_suspension.go:75`) + Redis marker `SetSellerSuspended` |
| seller.unsuspended | hawkr.seller | coupon, notification, product | restore — STATE | same outbox |
| seller.membership_changed | hawkr.seller | ai-service | ranking facts tier — PROJECTION | same outbox (`usecase/seller_membership.go`) — **ranking input (A11)** |
| seller.profile.changed | hawkr.seller | cart(Java) | evict `seller:meta` cache — PROJECTION | direct best-effort publish (`usecase/seller.go:84`), not outbox |
| merchant.verification.{approved,rejected,pending_info,forwarded,under_review} | hawkr.merchant | notification | push+inbox — SIDE_EFFECT | `merchant_verification_outbox` + worker, `mq/merchant_event_publisher.go` |
| seller.governance.{violation_created,violation_revoked,appeal_decided,notify.assist_investigation,notify.temporary_takedown_self_check} | hawkr.governance | (notification, **exchange not provisioned**) | producer-only — SIDE_EFFECT | direct publish from `usecase/violation.go`, `usecase/food_safety_report.go` |

Consumed by seller-service (`internal/mq/di.go`):

| routing key | producer (now) | purpose | note |
|---|---|---|---|
| account.deleted | **monolith** (`internal/legacy/accountservice/events.go` `OnAccountDeleted` → River job → RabbitMQ) | release logo/cover media refs | after A3 becomes identity's in-process `AccountDeleted` event |
| product.viewed | buyer-service | product view analytics (best-effort) | buyer migrates in A3 too → direct call or drop |
| review.created / review.hidden | review-service | overwrite `sellers.rating` aggregate (flows.md notes a version bug) | review stays until A6; seller must keep a RabbitMQ consumer (legacy bridge) or rating becomes a read of review |
| seller.review.result | payuni-webhook-service (Java) | apply PayUni sub-merchant review result | Java stays → legacy RabbitMQ consumer in monolith needed |
| payment.membership.seller.{success,failed} | payment (Java) | membership charge result | producer-only; **no consumer implemented** (flow C3 dormant) |

`event-pairs.csv` rows (14): seller→notification 8 keys, seller→product 3, seller→coupon 2, seller→gateway 2, seller→im 2, seller→ai 1, seller→cart 1, seller→order 1, seller→pricebook 1, seller→social 1; review→seller 2, account→seller 1, buyer→seller 1, payuni-webhook→seller 1.

After A3 the seller module must still emit seller.deleted/suspended/unsuspended/onboarded/profile.changed/membership_changed onto RabbitMQ for Java (product, coupon, order, cart), unmigrated Go (im, notification, pricebook, social, ai) and the gateway — same bridge shape as `accountservice.OnAccountDeleted` (River job publishing to `hawkr.seller`).

### 2.2 `replicas.csv` rows involving seller (20; `rg`-equivalent: python filter on holder/copied_from containing "seller")

Held **by** seller:
- `sellers.rating` ← review (review.created/hidden events) — PROJECTION.
- `sellers.email, contact_phone` ← account (copied once at CreateSeller, editable, never re-synced).

Copied **from** seller (who holds store data):
- Java: order `orders.mer_id` (+ sender fields) — checkout snapshot; shipping `seller_shipment_settings.mer_id,label_company_name` (onboarding only, no sync); cart Redis `seller:meta:{storeId}` (lazy fill, evicted by seller.profile.changed); product `seller_deleted_at` and `product_suspension_snapshot`/`status` (events); coupon `disabled_by_suspension`; payment membership quote snapshot (dormant); media `asset_reference` reverse index.
- Go: ai `seller_ranking_facts` (membership_changed + order.completed; **ranking**), ai `procurement_candidates.seller_name,logo_key,store_code…` and RFQ invite snapshots; social `cms_comment.user_name/avatar` (overwritten at read by live gRPC), social `reports.target_snapshot`; invoice issuer snapshot (seller tax profile).
- identity (ex-account): delegation session/audit target store snapshot — already dispositioned "snapshot" (A2).
- gateway: Redis `store:current:{accountId}` (60 s TTL; resolver) and `seller:suspended:*` / guard cache keys (`pkg/redis/seller_suspension.go`, written by seller-service `internal/infrastructure/seller_suspension_redis.go`, read by gateway `router/seller_capability.go`).

### 2.3 `idrefs.csv`

37 rows mention seller in holder/owner. Seller **holds** foreign ids in 8 rows: account (account_id on certification_applications, outboxes, payuni_*; admin ids on reviews/suspensions/audit), ai (`vendor_portal_invites.rfq_recipient_id`), buyer (`seller_store_visitors.visitor_buyer_id`), media (certification asset ids, logos), order (`target_order_no` on reports), product (`target_product_id`, `seller_product_view_log.product_id`, `seller_prospect_categories.category_id`), social (`seller_violation_reports.target_post_id`). Everyone else stores `store_id` (and `payuni_mer_id`: aftersales, order, payment, shipping). Details are the domain agent's scope.

## 3. Outgoing calls from seller-service

gRPC (`services/seller-service/internal/client/*`, targets from `internal/config/config.go:203-221`; compose sets only ACCOUNT and MEDIA):

| target | RPC | used by | after A3 |
|---|---|---|---|
| account → **monolith legacy** (`ACCOUNT_SERVICE_ADDR=host.docker.internal:9941`, `internal/legacy/accountservice/server.go`) | `GetAccountInfo` (×2) | CreateSeller (copy email/phone), certification | identity API (allowed dep) |
| same | `BindSeller` | CreateSeller + seller.onboarded outbox re-grant | `identity.GrantRole(SELLER)` in the same tx (plan A3 note) |
| same | `RemoveRole` (with `source_id` fence, OtherRoleState) | delete-seller usecase + outbox worker | same tx; lets identity drop the `source_id` fence (identity AGENTS.md:44) |
| media (Java via `MEDIA_SERVICE_ADDR=media-service:9937`) | `FinalizeImages`, `LinkImages`, `ReleaseReferences` | seller logo/cover, certification docs, violation reports, prospect media, account.deleted consumer | monolith `media` module (backed by `internal/legacy/mediaservice` until B1) |
| buyer | `GetSubscriberDashboardStats` (`GetSubscriberCount`) | gRPC `GetSellerStats` handler (followers) | **D2 violation**: seller may not depend on buyer → compose in http or port |
| buyer | `GetBuyerProfile` (`HasActiveBuyer`) | role removal OtherRoleState | identity knows roles → ask identity instead |
| buyer | `BatchGetSubscriberCounts` | recommended ranking | A11 |
| order (Java) | `GetTopSellersByRevenue` | recommended ranking | A11 |
| order | `GetActiveMerchantOrderCount` | delete-seller guard | **stub: returns 0 without calling** (`client/order/client.go:30`) |
| product (Java) | `ListStoreIDsWithPublicProductsInternal` | recommended ranking | A11 |
| product | `GetProductsByIDs` | `usecase/product_view.go` (view stats) | D2 violation (seller ↛ catalog) |
| review | `GetSellerOrderReviewStats` | gRPC `GetSellerStats` | D2 violation (seller ↛ review) |
| social | `GetPostList` (`HasPublishedPost`) | certification submit | D2 violation; plan A7 "實作 seller 認證申請需要的 port" |

External (`external-systems.csv`): GCIS company registry (`client/gcis`, `data.gcis.nat.gov.tw`), TGOS address validation via a Cloud Run egress proxy (compose default `TGOS_BASE_URL` 指向舊公司 Cloud Run 的 egress proxy — likely old-company infra, D9 check), PayUni agent API (`client/payuni`: BindSeller, UpdateMerchant, GetSellerStatus; D5 intent-first applies), website on-demand revalidate. compose also defaults `MEDIA_BASE_URL` 指向舊公司的 test-media 網域 (old-company domain, D9 check).

Seller-service also writes the Redis suspension marker that the gateway reads (shared Redis).

## 4. Gateway REST

### 4.1 Route table (92 routes that reach seller-service)

Produced by `routes.py` (parses `gateway/internal/router/routes*.go`, joins multi-line registrations, resolves group prefixes and constants; follows handler → `h.Seller.X` → `SellerClient` wrapper → RPC, transitively through helpers). `guard`: sellerHandle = `requireSeller(cap)`; delegated = `delegatedHandle` (runs `GetStoreStateForGuard`); inject = `injectSellerSuspension`. openapi match: `(method, path with {} params)` against 569 ops of `backend/api/openapi/openapi.yaml`.

Not in the table because they are guards/middleware rather than handler calls: `GetCurrentStoreID` (every seller route through `storeid.Resolver`), `ValidateVendorPortalSession` (7 routes under `/public/vendor_portal/`, incl. 6 AI-procurement RFQ routes that are all in openapi), and suspension-guard `GetSeller`.

| method path | gateway handler | guard | seller RPCs | in openapi |
|---|---|---|---|---|
| GET /app/address/suggestions | routes_address.go:AddressSuggestions |  | SuggestAddress | yes |
| GET /app/address/validate | routes_address.go:AddressValidate |  | ValidateAddress | yes |
| GET /admin/certifications/:id | routes_admin.go:GetAdminCertificationDetail |  | GetCertificationApplicationForAdmin, GetSeller | yes |
| GET /admin/certifications/:id/audit-logs | routes_admin.go:GetAdminCertificationReviews |  | GetCertificationReviews | yes |
| PATCH /admin/certifications/:id | routes_admin.go:ReviewCertification |  | ApproveCertification, RejectCertification | yes |
| DELETE /admin/accounts/:id | routes_admin.go:DeleteAccount |  | DeleteSellerAccount | yes |
| GET /admin/sellers | routes_admin.go:ListAdminSellers |  | ListSellersForAdmin | yes |
| GET /admin/sellers/stats | routes_admin.go:GetAdminSellerStats |  | GetSellerStatsForAdmin | yes |
| GET /admin/sellers/:store_id | routes_admin.go:GetAdminSellerProfile |  | GetCertificationApplicationForAdmin, GetPayuniStatus, GetSeller | yes |
| PATCH /admin/sellers/:store_id/seo | routes_admin.go:UpdateSellerSEO |  | ResolveSellerAccountID, UpdateSellerSEO | yes |
| POST /admin/sellers/:store_id/membership | routes_admin.go:AssignSellerMembershipPlan |  | AssignSellerMembershipPlan | yes |
| GET /admin/sellers/:store_id/membership | routes_admin.go:GetSellerMembershipPlan |  | GetSellerMembershipPlan | yes |
| GET /admin/seller_membership_plans | routes_admin.go:ListSellerMembershipPlans |  | ListSellerMembershipPlans | yes |
| PATCH /admin/seller_membership_plans/:plan_code | routes_admin.go:UpdateSellerMembershipPlan |  | UpdateSellerMembershipPlan | yes |
| GET /admin/certifications | routes_admin.go:ListAdminCertifications |  | ListCertificationsForAdmin | yes |
| POST /app/certifications | routes_admin.go:CreateCertificationDraft |  | CreateCertificationDraft | yes |
| GET /app/certifications/:id | routes_admin.go:GetCertificationApplication |  | GetCertificationApplication | yes |
| GET /app/certifications/:id/reviews | routes_admin.go:GetMyCertificationReviews |  | GetCertificationApplication, GetCertificationReviews | yes |
| PUT /app/certifications/:id/progress | routes_admin.go:SaveCertificationProgress |  | GetCertificationApplication, SaveCertificationProgress | yes |
| PATCH /app/certifications/:id | routes_admin.go:SubmitCertificationApplication |  | GetCertificationApplication, SubmitCertificationApplication | yes |
| DELETE /app/certifications/:id | routes_admin.go:CancelCertification |  | CancelCertification, GetCertificationApplication | yes |
| GET /admin/seller_prospects | routes_admin_seller_prospect.go:ListAdminSellerProspects |  | AdminListSellerProspects | yes |
| POST /admin/seller_prospects | routes_admin_seller_prospect.go:CreateAdminSellerProspect |  | CreateSellerProspect | yes |
| POST /admin/seller_prospects/preview | routes_admin_seller_prospect.go:PreviewAdminSellerProspect |  | CreateSellerProspect | yes |
| GET /admin/seller_prospects/duplicates | routes_admin_seller_prospect.go:ListAdminProspectDuplicates |  | AdminListDuplicateProspects | yes |
| GET /admin/seller_prospects/reconciliation | routes_admin_seller_prospect.go:ListAdminProspectTaxIDMatches |  | AdminListProspectTaxIdMatches | yes |
| POST /admin/seller_prospects/merge | routes_admin_seller_prospect.go:MergeAdminSellerProspects |  | AdminMergeSellerProspects | yes |
| GET /admin/seller_prospects/:id | routes_admin_seller_prospect.go:GetAdminSellerProspect |  | AdminGetSellerProspect | yes |
| PUT /admin/seller_prospects/:id | routes_admin_seller_prospect.go:UpdateAdminSellerProspect |  | AdminUpdateSellerProspect | yes |
| DELETE /admin/seller_prospects/:id | routes_admin_seller_prospect.go:ArchiveAdminSellerProspect |  | AdminArchiveSellerProspect | yes |
| POST /admin/seller_prospects/:id/restore | routes_admin_seller_prospect.go:RestoreAdminSellerProspect |  | AdminArchiveSellerProspect | yes |
| POST /admin/seller_prospects/:id/convert | routes_admin_seller_prospect.go:ConvertAdminSellerProspect |  | AdminConvertSellerProspect | yes |
| PUT /admin/seller_prospects/:id/media | routes_admin_seller_prospect.go:UpdateAdminProspectMedia |  | AdminUpdateSellerProspectMedia | yes |
| POST /admin/seller_prospects/:id/invites | routes_admin_seller_prospect.go:CreateAdminProspectInvite |  | AdminCreateProspectInvite | yes |
| DELETE /admin/seller_prospects/:id/invites/:invite_id | routes_admin_seller_prospect.go:RevokeAdminProspectInvite |  | AdminRevokeProspectInvite | yes |
| GET /app/businessRegistration/:ban | routes_business_registration.go:BusinessRegistrationLookup |  | LookupBusinessRegistration | yes |
| POST /app/chat/messages | routes_chat.go:SendChatMessage | inject | GetSeller | yes |
| GET /admin/sellers/:store_id/storefront | routes_delegated.go:GetDelegatedStorefront | delegated | GetSeller | NO |
| PATCH /admin/sellers/:store_id/storefront | routes_delegated.go:UpdateDelegatedStorefront | delegated | GetSeller, UpdateSeller | NO |
| POST /admin/sellers/:store_id/storefront/logo | routes_delegated.go:UpdateDelegatedStorefrontLogo | delegated | GetSeller, UpdateLogo | NO |
| POST /admin/sellers/:store_id/storefront/cover | routes_delegated.go:UpdateDelegatedStorefrontCover | delegated | GetSeller, UpdateCover | NO |
| POST /public/food_safety_reports | routes_governance.go:SubmitFoodSafetyReport |  | CreateFoodSafetyReport | yes |
| GET /public/sellers/:id/violations/public | routes_governance.go:GetPublicSellerViolations |  | GetPublicSellerViolations | yes |
| POST /app/violation_reports | routes_governance.go:SubmitViolationReport |  | CreateViolationReport | yes |
| GET /app/violation_reports/me | routes_governance.go:ListMyViolationReports |  | ListMyViolationReports | yes |
| GET /app/sellers/me/violations | routes_governance.go:SellerListMyViolations |  | SellerListMyViolations | yes |
| POST /app/sellers/me/violations/:id/appeal | routes_governance.go:SellerSubmitAppeal |  | SellerSubmitAppeal | yes |
| GET /app/sellers/me/violations/:id/appeal | routes_governance.go:SellerGetAppeal |  | SellerGetAppeal | yes |
| GET /admin/violation_reports | routes_governance.go:AdminListViolationReports |  | AdminListViolationReports | yes |
| PATCH /admin/seller_violation_reports/:id | routes_governance.go:AdminPatchSellerViolationReportStatus |  | AdminPatchSellerViolationReportStatus | yes |
| PATCH /admin/food_safety_reports/:id | routes_governance.go:AdminPatchFoodSafetyReportStatus |  | AdminPatchFoodSafetyReportStatus | yes |
| POST /admin/food_safety_reports/:id/ack | routes_governance.go:AdminAcknowledgeFoodSafetyReport |  | AdminAcknowledgeFoodSafetyReport | yes |
| GET /admin/food_safety_reports/:id/fda_export | routes_governance.go:AdminGenerateFDAReport |  | AdminGenerateFDAReport | yes |
| POST /admin/food_safety_reports/:id/mark_fda_reported | routes_governance.go:AdminMarkFDAReported |  | AdminMarkFDAReported | yes |
| POST /admin/food_safety_reports/:id/notify_seller | routes_governance.go:AdminNotifySeller |  | AdminNotifySeller | yes |
| POST /admin/violations | routes_governance.go:AdminCreateViolation |  | AdminCreateViolation | yes |
| GET /admin/violations/:id | routes_governance.go:AdminGetViolation |  | AdminGetViolation | yes |
| PATCH /admin/violations/:id | routes_governance.go:AdminUpdateViolation |  | AdminUpdateViolation | yes |
| POST /admin/violations/:id/revoke | routes_governance.go:AdminRevokeViolation |  | AdminRevokeViolation | yes |
| POST /admin/violations/:id/appeal/decide | routes_governance.go:AdminDecideAppeal |  | AdminDecideAppeal | yes |
| POST /admin/sellers/:store_id/suspensions | routes_governance.go:SuspendSellerForAdmin |  | ResolveSellerAccountID, SuspendSellerForAdmin | yes |
| GET /admin/sellers/:store_id/suspensions | routes_governance.go:ListSellerSuspensionsForAdmin |  | ListSellerSuspensionsForAdmin, ResolveSellerAccountID | yes |
| POST /admin/sellers/:store_id/suspensions/:id/lift | routes_governance.go:LiftSellerSuspensionForAdmin |  | LiftSellerSuspensionForAdmin, ResolveSellerAccountID | yes |
| GET /admin/sellers/:store_id/suspension | routes_governance.go:GetActiveSellerSuspensionForAdmin |  | GetActiveSellerSuspensionForAdmin, ResolveSellerAccountID | yes |
| GET /public/marketplace | routes_marketplace.go:GetMarketplace |  | GetRecommendedSellers, GetSellerBatch | yes |
| GET /public/sellers/:id/posts | routes_post.go:GetSellerPosts |  | GetSeller, GetSellerBatch | yes |
| GET /public/posts | routes_post.go:GetLatestPosts |  | GetSellerBatch | yes |
| POST /public/products/:id/view | routes_product.go:RecordProductView |  | RecordProductView | yes |
| GET /public/sellers/recommended | routes_seller.go:GetRecommendedSellers |  | GetRecommendedSellers | yes |
| GET /public/sellers/:id | routes_seller.go:GetSellerInfo |  | GetSeller, RecordStoreVisit | yes |
| GET /public/sellers/:id/stats | routes_seller.go:GetSellerStats |  | GetSellerStats | yes |
| GET /public/sellers/:id/rating | routes_seller.go:GetSellerRating |  | GetSellerStats | yes |
| GET /public/sellers/:id/products | routes_seller.go:GetSellerProducts |  | GetSeller | yes |
| GET /public/sellers/:id/reviews | routes_seller.go:GetPublicSellerReviews |  | GetSeller | yes |
| GET /public/sellers/:id/reviews/stats | routes_seller.go:GetPublicSellerReviewStats |  | GetSeller | yes |
| POST /app/sellers | routes_seller.go:ApplySeller |  | CreateSeller | yes |
| GET /app/sellers/me | routes_seller.go:GetMySellerInfo |  | GetCertificationDraft, GetSeller | yes |
| PUT /app/sellers/me | routes_seller.go:UpdateMySeller | sellerHandle | UpdateSeller | yes |
| PUT /app/sellers/me/food-business-registration | routes_seller.go:UpdateMyFoodBusinessRegistration | sellerHandle | UpdateSellerFoodRegistration | yes |
| DELETE /app/sellers/me | routes_seller.go:DeleteSellerAccount |  | DeleteSellerAccount | yes |
| PUT /app/sellers/me/logo | routes_seller.go:UpdateMyLogo | sellerHandle | UpdateLogo | yes |
| PUT /app/sellers/me/cover | routes_seller.go:UpdateMyCover | sellerHandle | UpdateCover | yes |
| PATCH /app/sellers/me/onboarding | routes_seller.go:CompleteOnboarding | sellerHandle | CompleteOnboarding | yes |
| GET /app/sellers/me/payuni/status | routes_seller.go:GetPayuniStatus |  | GetPayuniStatus | yes |
| POST /app/sellers/me/payuni/status_syncs | routes_seller.go:RefreshPayuniStatus |  | GetPayuniStatus, RefreshPayuniStatus | yes |
| PATCH /app/sellers/me/payuni/merchant_profile | routes_seller.go:PatchPayuniMerchantProfile | sellerHandle | UpdateMerchantProfile | yes |
| GET /app/sellers/me/dashboard | routes_seller.go:GetSellerDashboard | sellerHandle | GetProductViewStats, GetSellerStats | yes |
| GET /app/sellers/me/store_visitors | routes_seller.go:GetStoreVisitors | sellerHandle | GetStoreVisitors | yes |
| GET /app/share_url | routes_share.go:GetShareURL |  | GetSeller | yes |
| GET /public/vendor_portal/invites/preview | routes_vendor_portal.go:VendorPortalInvitePreview |  | PreviewVendorPortalInvite | yes |
| POST /public/vendor_portal/invite_exchanges | routes_vendor_portal.go:VendorPortalInviteExchange |  | ExchangeVendorPortalInvite | yes |
| POST /public/vendor_portal/step_up/google | routes_vendor_portal.go:VendorPortalStepUpGoogle |  | StepUpVendorPortalSession | yes |


### 4.2 Routes that belong to other modules and only resolve/enrich via seller (14)

`POST /app/chat/messages` (im; GetSeller), `GET /public/sellers/:id/posts`, `GET /public/posts` (social; GetSeller/GetSellerBatch), `GET /app/share_url` (GetSeller), `GET /public/marketplace` and `GET /public/sellers/recommended` (ranking, A11), `GET /public/sellers/:id/products` (catalog), `GET /public/sellers/:id/reviews`, `/reviews/stats` (review), `DELETE /admin/accounts/:id` (identity orchestration), 4 delegated storefront routes (A10 orchestration over seller data). The remaining ~78 are seller-owned.

### 4.3 Gateway-side orchestration

- **Apply to become seller** `POST /app/sellers` → `handler/seller.go:729 ApplySeller`: one `CreateSeller` RPC (seller-service creates stint + seller.onboarded outbox, then grants SELLER synchronously via account `BindSeller`; the outbox re-grants on failure), then best-effort `h.Store.Invalidate` of the resolver cache. No input validation in the gateway beyond JSON binding; plan A3 records that city validation happens after the store and role are written (400 with partial state) — fix by validating first and granting in the same tx.
- **Seller self-delete** `DELETE /app/sellers/me` → `handler/seller_delete.go:24`: `DeleteSellerAccount(IsAdmin=false)` + resolver invalidate.
- **Admin delete account** `DELETE /admin/accounts/:id` → `handler/admin_account.go:34`: `Seller.DeleteSellerAccount(IsAdmin, OverrideActiveOrders=true)` (9233 ⇒ absent) → `Buyer.DeleteBuyerAccount` (NotFound ⇒ absent) → if both absent, `Account.SoftDeleteAccount` (monolith legacy). Order matters (seller first so buyer's cascade sees ABSENT). With buyer+seller in the monolith this becomes one transaction; the admin route lives where identity's admin API lives.
- **Vendor portal Google step-up** `POST /public/vendor_portal/step_up/google` → `handler/vendor_portal_step_up.go:48`: `Account.VerifyGoogleIdToken` (monolith legacy accountservice → `identity.VerifyGoogle`) then `Seller.StepUpVendorPortalSession(token, verifiedEmail)`. A2 plan line 27: "A3 隨 seller 搬". Vendor-portal session middleware `middleware/vendor_portal_session.go:32` calls `ValidateVendorPortalSession` for 7 routes whose handlers are ai-service (A9).
- **Seller binding** `PUT /app/buyers/me/seller_binding` → `handler/buyer.go:276`: calls **account** `BindSeller(accountID)` only — no seller-service call, no store id (grants SELLER role without a stint). Client usage: website only — `lib/seller/repo.ts:391 bindSellerRole()` called from `features/seller/onboarding/SellerOnboardingClient.tsx:205` *before* `POST /app/sellers` when the account has no seller role (so today a buyer gets the SELLER role before any store exists, and CreateSeller then grants again with the stint as source). In openapi (`openapi.yaml:16658`). With create-store + grant in one transaction this pre-grant step becomes redundant.
- **Seller capability/suspension guard (9240)** `router/seller_capability.go`: `requireSeller(cap)` checks JWT role, skips when `authz.GrantedWhenSuspended(cap)`, else Redis marker (`pkgredis.LookupSellerSuspension`, 500 ms) → miss/error: origin `GetSellerByAccountID` (= `GetSeller`, 900 ms) → cache 60 s → still fail-open on RPC failure; deny = `response.BizDeny(9240)` (HTTP 409). `injectSellerSuspension` adds `X-Seller-Suspended` + gRPC metadata for im/social (`pkg/middleware/grpc/interceptor/seller_suspension.go`, `services/im-service`, `services/social-service`). `sellerViewGuard` on order/coupon list+detail. Capability table: `gateway/internal/authz/policy.go`; ADR `docs/adr/2026-09-18-seller-capability-authorization.md`.
- **Delegation guard** `router/delegated_capability.go:256` `GetStoreStateForGuard` on every delegated route (22), fail-closed, uncached.
- **Store resolver** `gateway/internal/storeid/resolver.go:55,70`: `GetCurrentStoreID` with Redis `store:current:{accountId}`; invalidated by the realtime `SellerLifecycleChannel` (seller.deleted/onboarded) and by ApplySeller/DeleteSellerAccount.
- **Fire-and-forget copy writes** (flows E2): `GetSellerInfo` spawns `RecordStoreVisit`; `POST /public/products/:id/view` → `RecordProductView`.

## 5. Clients

Heuristic (`clients2.py`): for each of the 92 routes, match the path (params = interpolation `${x}`/`$x`/`{x}`/digits) in non-test, non-generated sources of `website/{app,lib,components,features,middleware.ts}`, `admin/src`, `buyer-app/lib`, `seller-app/lib`; string constants used as bases (`const BASE = "/api/v1/admin/seller_prospects"`) are substituted; a namespace-less match (`/sellers/${id}/stats`) counts only when the file shows a namespace hint (e.g. `.public`, `authedFetch`). It is **method-agnostic** (`/app/sellers/me` counts for GET/PUT/DELETE) and **includes comments**, so file counts are upper bounds.

Totals: website 34 paths / 41 files; admin 35 / 19; buyer-app 11 / 17; seller-app 26 / 25.

Routes with **no client caller** (7): `POST|GET /admin/sellers/:store_id/membership`, `POST /public/food_safety_reports`, `GET /public/sellers/:id/violations/public`, `POST /app/violation_reports`, `GET /app/violation_reports/me`, `GET /app/sellers/me/store_visitors`.

Per-route client hits:

| route | client hits (files) |
|---|---|
| GET /app/address/suggestions | website:1 ; buyer-app:1 ; seller-app:1 |
| GET /app/address/validate | website:2 ; buyer-app:1 ; seller-app:1 |
| GET /admin/certifications/:id | admin:1 |
| GET /admin/certifications/:id/audit-logs | admin:1 |
| PATCH /admin/certifications/:id | admin:1 |
| DELETE /admin/accounts/:id | admin:2 |
| GET /admin/sellers | admin:5 |
| GET /admin/sellers/stats | admin:3 |
| GET /admin/sellers/:store_id | admin:2 |
| PATCH /admin/sellers/:store_id/seo | admin:1 |
| POST /admin/sellers/:store_id/membership | - |
| GET /admin/sellers/:store_id/membership | - |
| GET /admin/seller_membership_plans | admin:1 |
| PATCH /admin/seller_membership_plans/:plan_code | admin:1 |
| GET /admin/certifications | admin:1 |
| POST /app/certifications | website:3 ; seller-app:1 |
| GET /app/certifications/:id | website:1 ; seller-app:1 |
| GET /app/certifications/:id/reviews | website:1 ; seller-app:1 |
| PUT /app/certifications/:id/progress | website:1 ; seller-app:1 |
| PATCH /app/certifications/:id | website:1 ; seller-app:1 |
| DELETE /app/certifications/:id | website:1 ; seller-app:1 |
| GET /admin/seller_prospects | admin:2 |
| POST /admin/seller_prospects | admin:2 |
| POST /admin/seller_prospects/preview | admin:1 |
| GET /admin/seller_prospects/duplicates | admin:1 |
| GET /admin/seller_prospects/reconciliation | admin:1 |
| POST /admin/seller_prospects/merge | admin:1 |
| GET /admin/seller_prospects/:id | admin:1 |
| PUT /admin/seller_prospects/:id | admin:1 |
| DELETE /admin/seller_prospects/:id | admin:1 |
| POST /admin/seller_prospects/:id/restore | admin:1 |
| POST /admin/seller_prospects/:id/convert | admin:1 |
| PUT /admin/seller_prospects/:id/media | admin:1 |
| POST /admin/seller_prospects/:id/invites | admin:1 |
| DELETE /admin/seller_prospects/:id/invites/:invite_id | admin:1 |
| GET /app/businessRegistration/:ban | website:1 ; seller-app:1 |
| POST /app/chat/messages | website:1 ; buyer-app:1 ; seller-app:1 |
| GET /admin/sellers/:store_id/storefront | website:1 |
| PATCH /admin/sellers/:store_id/storefront | website:1 |
| POST /admin/sellers/:store_id/storefront/logo | website:1 |
| POST /admin/sellers/:store_id/storefront/cover | website:1 |
| POST /public/food_safety_reports | - |
| GET /public/sellers/:id/violations/public | - |
| POST /app/violation_reports | - |
| GET /app/violation_reports/me | - |
| GET /app/sellers/me/violations | website:2 ; seller-app:3 |
| POST /app/sellers/me/violations/:id/appeal | website:1 ; seller-app:1 |
| GET /app/sellers/me/violations/:id/appeal | website:1 ; seller-app:1 |
| GET /admin/violation_reports | admin:2 |
| PATCH /admin/seller_violation_reports/:id | admin:1 |
| PATCH /admin/food_safety_reports/:id | admin:1 |
| POST /admin/food_safety_reports/:id/ack | admin:1 |
| GET /admin/food_safety_reports/:id/fda_export | admin:1 |
| POST /admin/food_safety_reports/:id/mark_fda_reported | admin:1 |
| POST /admin/food_safety_reports/:id/notify_seller | admin:1 |
| POST /admin/violations | admin:4 |
| GET /admin/violations/:id | admin:2 |
| PATCH /admin/violations/:id | admin:2 |
| POST /admin/violations/:id/revoke | admin:1 |
| POST /admin/violations/:id/appeal/decide | admin:1 |
| POST /admin/sellers/:store_id/suspensions | admin:2 |
| GET /admin/sellers/:store_id/suspensions | admin:2 |
| POST /admin/sellers/:store_id/suspensions/:id/lift | admin:1 |
| GET /admin/sellers/:store_id/suspension | admin:1 |
| GET /public/marketplace | website:2 ; buyer-app:4 ; seller-app:1 |
| GET /public/sellers/:id/posts | website:1 ; buyer-app:2 ; seller-app:2 |
| GET /public/posts | website:6 ; buyer-app:1 ; seller-app:2 |
| POST /public/products/:id/view | website:1 |
| GET /public/sellers/recommended | website:3 |
| GET /public/sellers/:id | website:8 ; buyer-app:6 ; seller-app:2 |
| GET /public/sellers/:id/stats | website:3 ; buyer-app:2 ; seller-app:2 |
| GET /public/sellers/:id/rating | buyer-app:3 ; seller-app:2 |
| GET /public/sellers/:id/products | website:3 ; buyer-app:5 ; seller-app:1 |
| GET /public/sellers/:id/reviews | website:2 |
| GET /public/sellers/:id/reviews/stats | website:2 |
| POST /app/sellers | website:1 ; seller-app:2 |
| GET /app/sellers/me | website:10 ; seller-app:12 |
| PUT /app/sellers/me | website:10 ; seller-app:12 |
| PUT /app/sellers/me/food-business-registration | website:1 |
| DELETE /app/sellers/me | website:10 ; seller-app:12 |
| PUT /app/sellers/me/logo | seller-app:1 |
| PUT /app/sellers/me/cover | seller-app:1 |
| PATCH /app/sellers/me/onboarding | website:1 ; seller-app:1 |
| GET /app/sellers/me/payuni/status | website:3 ; seller-app:4 |
| POST /app/sellers/me/payuni/status_syncs | website:1 ; seller-app:1 |
| PATCH /app/sellers/me/payuni/merchant_profile | website:1 |
| GET /app/sellers/me/dashboard | website:2 ; seller-app:2 |
| GET /app/sellers/me/store_visitors | - |
| GET /app/share_url | buyer-app:1 ; seller-app:2 |
| GET /public/vendor_portal/invites/preview | website:1 |
| POST /public/vendor_portal/invite_exchanges | website:1 |
| POST /public/vendor_portal/step_up/google | website:1 |

Per client, files and the endpoints they reference:

#### website — 41 files, 34 distinct paths (method-agnostic)

- `website/app/(delegated)/adm/api/storefront/[storeId]/cover/route.ts`: POST /admin/sellers/:store_id/storefront/cover
- `website/app/(delegated)/adm/api/storefront/[storeId]/logo/route.ts`: POST /admin/sellers/:store_id/storefront/logo
- `website/app/(delegated)/adm/api/storefront/[storeId]/route.ts`: GET /admin/sellers/:store_id/storefront, PATCH /admin/sellers/:store_id/storefront
- `website/components/shared/Link.tsx`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/components/shared/MegaMenuPanel.tsx`: GET /public/posts
- `website/features/account/screens/messages/ThreadClient.tsx`: GET /public/sellers/:id
- `website/features/delegated-operations/products/DelegatedProductCreatePage.tsx`: POST /app/certifications
- `website/features/feed/components/FeedClientShell.tsx`: GET /public/posts
- `website/features/feed/components/InfiniteFeed.tsx`: GET /public/posts
- `website/features/feed/components/commentPermissions.ts`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/features/market/components/MarketSellerCard.tsx`: GET /public/sellers/:id
- `website/features/market/products/MarketProductsListPage.tsx`: GET /public/sellers/recommended
- `website/features/product/ProductViewTracker.tsx`: POST /public/products/:id/view
- `website/features/product/components/ProductSellerIntro.tsx`: GET /public/sellers/:id, GET /public/sellers/:id/stats
- `website/features/seller-center/dashboard/DashboardPage.tsx`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/features/seller-center/posts/productPickerAdapter.ts`: GET /public/sellers/:id/products
- `website/features/seller-center/profile/SellerProfileClient.tsx`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/features/seller/components/SellerChatButton.tsx`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/features/seller/components/seller-chat-guard.ts`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/lib/addresses/repo.ts`: GET /app/address/validate
- `website/lib/api/cached.ts`: GET /public/marketplace, GET /public/posts, GET /public/sellers/:id, GET /public/sellers/:id/products, GET /public/sellers/:id/reviews, GET /public/sellers/:id/reviews/stats, GET /public/sellers/:id/stats, GET /public/sellers/recommended
- `website/lib/api/index.ts`: GET /public/marketplace, GET /public/sellers/:id, GET /public/sellers/:id/products, GET /public/sellers/:id/reviews, GET /public/sellers/:id/reviews/stats, GET /public/sellers/:id/stats, GET /public/sellers/recommended
- `website/lib/api/posts.ts`: GET /public/posts, GET /public/sellers/:id/posts
- `website/lib/chat/chatIdentity.ts`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/lib/chat/repo.ts`: POST /app/chat/messages
- `website/lib/hooks/usePayuniStatus.ts`: GET /app/sellers/me/payuni/status
- `website/lib/hooks/useSellerDashboard.ts`: GET /app/sellers/me/dashboard
- `website/lib/hooks/useSellerSelf.ts`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/lib/hooks/useSellerViolations.ts`: GET /app/sellers/me/violations
- `website/lib/products/seller-repo.ts`: POST /app/certifications
- `website/lib/query/keys.ts`: GET /app/sellers/me/payuni/status, GET /app/sellers/me/violations
- `website/lib/seller/canonical-redirect.ts`: GET /public/sellers/:id
- `website/lib/seller/cert-gate.ts`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `website/lib/seller/certification.ts`: DELETE /app/certifications/:id, GET /app/businessRegistration/:ban, GET /app/certifications/:id, GET /app/certifications/:id/reviews, PATCH /app/certifications/:id, POST /app/certifications, PUT /app/certifications/:id/progress
- `website/lib/seller/repo.ts`: DELETE /app/sellers/me, GET /app/sellers/me, GET /app/sellers/me/dashboard, GET /app/sellers/me/payuni/status, PATCH /app/sellers/me/onboarding, PATCH /app/sellers/me/payuni/merchant_profile, POST /app/sellers, POST /app/sellers/me/payuni/status_syncs, PUT /app/sellers/me, PUT /app/sellers/me/food-business-registration
- `website/lib/seller/shipping.ts`: GET /app/address/suggestions, GET /app/address/validate
- `website/lib/seller/violations.ts`: GET /app/sellers/me/violations/:id/appeal, POST /app/sellers/me/violations/:id/appeal
- `website/lib/seo/metadata.ts`: GET /public/sellers/:id
- `website/lib/seo/urls.ts`: GET /public/posts
- `website/lib/state/chat-drawer-store.ts`: GET /public/sellers/:id
- `website/lib/vendor-portal/repo.ts`: GET /public/vendor_portal/invites/preview, POST /public/vendor_portal/invite_exchanges, POST /public/vendor_portal/step_up/google

#### admin — 19 files, 35 distinct paths (method-agnostic)

- `admin/src/app/admin/analytics/page.tsx`: GET /admin/sellers, GET /admin/sellers/stats
- `admin/src/app/admin/invoices/complaints/[complaintId]/page.tsx`: GET /admin/sellers/:store_id/suspensions, POST /admin/sellers/:store_id/suspensions
- `admin/src/app/admin/invoices/issuances/new/page.tsx`: GET /admin/sellers, GET /admin/sellers/:store_id
- `admin/src/app/admin/merchants/[id]/SellerAccountDeleteDialog.tsx`: DELETE /admin/accounts/:id
- `admin/src/app/admin/merchants/[id]/page.tsx`: DELETE /admin/accounts/:id, GET /admin/certifications/:id, GET /admin/certifications/:id/audit-logs, GET /admin/sellers/:store_id, GET /admin/sellers/:store_id/suspension, GET /admin/sellers/:store_id/suspensions, PATCH /admin/certifications/:id, POST /admin/sellers/:store_id/suspensions, POST /admin/sellers/:store_id/suspensions/:id/lift
- `admin/src/app/admin/merchants/page.tsx`: GET /admin/sellers, GET /admin/sellers/stats
- `admin/src/app/admin/page.tsx`: GET /admin/sellers/stats
- `admin/src/app/admin/seller-membership-plans/page.tsx`: GET /admin/seller_membership_plans, PATCH /admin/seller_membership_plans/:plan_code
- `admin/src/app/admin/seller-prospects/[id]/ConvertPanel.tsx`: GET /admin/sellers
- `admin/src/app/admin/seo/page.tsx`: GET /admin/sellers, PATCH /admin/sellers/:store_id/seo
- `admin/src/app/admin/violation-reports/[source]/[id]/page.tsx`: GET /admin/food_safety_reports/:id/fda_export, GET /admin/violation_reports, PATCH /admin/food_safety_reports/:id, PATCH /admin/seller_violation_reports/:id, POST /admin/food_safety_reports/:id/ack, POST /admin/food_safety_reports/:id/mark_fda_reported, POST /admin/food_safety_reports/:id/notify_seller
- `admin/src/app/admin/violation-reports/page.tsx`: GET /admin/violation_reports
- `admin/src/app/admin/violations/[id]/page.tsx`: GET /admin/violations/:id, PATCH /admin/violations/:id, POST /admin/violations, POST /admin/violations/:id/appeal/decide, POST /admin/violations/:id/revoke
- `admin/src/app/admin/violations/new/page.tsx`: GET /admin/violations/:id, PATCH /admin/violations/:id, POST /admin/violations
- `admin/src/features/invoices/overview.ts`: POST /admin/violations
- `admin/src/features/invoices/types.ts`: POST /admin/violations
- `admin/src/services/product-certifications.ts`: GET /admin/certifications
- `admin/src/services/seller-prospect-media.ts`: GET /admin/seller_prospects, POST /admin/seller_prospects, PUT /admin/seller_prospects/:id/media
- `admin/src/services/seller-prospects.ts`: DELETE /admin/seller_prospects/:id, DELETE /admin/seller_prospects/:id/invites/:invite_id, GET /admin/seller_prospects, GET /admin/seller_prospects/:id, GET /admin/seller_prospects/duplicates, GET /admin/seller_prospects/reconciliation, POST /admin/seller_prospects, POST /admin/seller_prospects/:id/convert, POST /admin/seller_prospects/:id/invites, POST /admin/seller_prospects/:id/restore, POST /admin/seller_prospects/merge, POST /admin/seller_prospects/preview, PUT /admin/seller_prospects/:id

#### buyer-app — 17 files, 11 distinct paths (method-agnostic)

- `buyer-app/lib/features/address/data/tgos_address_service.dart`: GET /app/address/suggestions, GET /app/address/validate
- `buyer-app/lib/features/category/data/category_marketplace_model.dart`: GET /public/marketplace
- `buyer-app/lib/features/category/data/category_repository.dart`: GET /public/marketplace, GET /public/sellers/:id
- `buyer-app/lib/features/category/data/product_item_model.dart`: GET /public/sellers/:id/products
- `buyer-app/lib/features/chat/data/chat_model.dart`: POST /app/chat/messages
- `buyer-app/lib/features/follow/data/follow_repository.dart`: GET /public/sellers/:id/stats
- `buyer-app/lib/features/product/data/product_repository.dart`: GET /app/share_url, GET /public/sellers/:id/products
- `buyer-app/lib/features/product/presentation/search_screen.dart`: GET /public/marketplace
- `buyer-app/lib/features/seller/data/seller_model.dart`: GET /public/sellers/:id/rating
- `buyer-app/lib/features/seller/data/seller_repository.dart`: GET /public/sellers/:id, GET /public/sellers/:id/products, GET /public/sellers/:id/rating
- `buyer-app/lib/features/seller/presentation/merchant_screen.dart`: GET /public/sellers/:id/products, GET /public/sellers/:id/rating, GET /public/sellers/:id/stats
- `buyer-app/lib/features/seller/presentation/widgets/merchant_chat_tab.dart`: GET /public/sellers/:id
- `buyer-app/lib/features/seller/presentation/widgets/merchant_home_tab.dart`: GET /public/sellers/:id/posts
- `buyer-app/lib/features/seller/presentation/widgets/merchant_shop_tab.dart`: GET /public/sellers/:id/products
- `buyer-app/lib/features/social/data/social_repository.dart`: GET /public/posts, GET /public/sellers/:id, GET /public/sellers/:id/posts
- `buyer-app/lib/features/wallet/presentation/wallet_screen.dart`: GET /public/sellers/:id
- `buyer-app/lib/shared/widgets/product_card.dart`: GET /public/marketplace, GET /public/sellers/:id

#### seller-app — 25 files, 26 distinct paths (method-agnostic)

- `seller-app/lib/core/session/session_model.dart`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `seller-app/lib/core/session/session_provider.dart`: DELETE /app/sellers/me, GET /app/sellers/me, GET /app/sellers/me/payuni/status, PUT /app/sellers/me
- `seller-app/lib/features/auth/presentation/auto_login_controller.dart`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `seller-app/lib/features/chat/data/chat_picker_remote_service.dart`: GET /public/sellers/:id/products
- `seller-app/lib/features/chat/data/chat_repository.dart`: POST /app/chat/messages
- `seller-app/lib/features/dashboard/data/dashboard_order_stats_provider.dart`: GET /app/sellers/me/dashboard
- `seller-app/lib/features/post/data/post_model.dart`: GET /public/marketplace, GET /public/posts, GET /public/sellers/:id/posts
- `seller-app/lib/features/post/data/post_repository.dart`: GET /app/share_url, GET /public/posts, GET /public/sellers/:id/posts
- `seller-app/lib/features/post/data/seller_page_stats_remote_service.dart`: GET /public/sellers/:id/stats
- `seller-app/lib/features/post/presentation/widgets/page_echo_tab.dart`: GET /public/sellers/:id
- `seller-app/lib/features/product/data/product_repository.dart`: GET /app/share_url
- `seller-app/lib/features/product/presentation/product_preview_screen.dart`: GET /public/sellers/:id/rating
- `seller-app/lib/features/seller/data/payuni_status_notifier.dart`: GET /app/sellers/me/payuni/status
- `seller-app/lib/features/seller/data/seller_repository.dart`: DELETE /app/certifications/:id, DELETE /app/sellers/me, GET /app/businessRegistration/:ban, GET /app/certifications/:id, GET /app/certifications/:id/reviews, GET /app/sellers/me, GET /app/sellers/me/dashboard, GET /app/sellers/me/payuni/status, GET /app/sellers/me/violations, GET /app/sellers/me/violations/:id/appeal, GET /public/sellers/:id, GET /public/sellers/:id/rating, GET /public/sellers/:id/stats, PATCH /app/certifications/:id, PATCH /app/sellers/me/onboarding, POST /app/certifications, POST /app/sellers, POST /app/sellers/me/payuni/status_syncs, POST /app/sellers/me/violations/:id/appeal, PUT /app/certifications/:id/progress, PUT /app/sellers/me, PUT /app/sellers/me/cover, PUT /app/sellers/me/logo
- `seller-app/lib/features/seller/data/seller_violation_model.dart`: GET /app/sellers/me/violations
- `seller-app/lib/features/seller/presentation/payuni_review_status_screen.dart`: GET /app/sellers/me/payuni/status
- `seller-app/lib/features/seller/presentation/profile_screen.dart`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `seller-app/lib/features/seller/presentation/seller_appeal_screen.dart`: DELETE /app/sellers/me, GET /app/sellers/me, GET /app/sellers/me/violations, PUT /app/sellers/me
- `seller-app/lib/features/seller/presentation/seller_certification_upload_screen.dart`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `seller-app/lib/features/seller/presentation/seller_info_screen.dart`: DELETE /app/sellers/me, GET /app/sellers/me, POST /app/sellers, PUT /app/sellers/me
- `seller-app/lib/features/seller/presentation/suspension_notice_screen.dart`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `seller-app/lib/features/settings/data/shipping_settings_remote_service.dart`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `seller-app/lib/features/settings/data/tgos_address_service.dart`: GET /app/address/suggestions, GET /app/address/validate
- `seller-app/lib/features/wallet/presentation/wallet_screen.dart`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me
- `seller-app/lib/shared/utils/seller_avatar_helper.dart`: DELETE /app/sellers/me, GET /app/sellers/me, PUT /app/sellers/me



### 5.1 Where "seller suspended" (9240 / HTTP 409) is handled today

- website: `lib/api/client.ts:36-41,372-374,604-607,632-633` (authed calls with code 9240 → window event `hk-seller:suspended`); `features/seller-center/components/SellerCenterShell.tsx:77-105` (redirect to `/seller-center/suspended` except whitelisted pages; skip calls while suspended); `components/shared/useSellerIdentity.ts`, `lib/seller/repo.ts:69 isSellerSuspendedLifecycle` (from `/app/sellers/me` lifecycle), `lib/hooks/useSellerInvoices.ts`, `lib/invoices/seller-repo.ts:114`, `lib/query/keys.ts:375`, `app/(seller)/seller-center/suspended/page.tsx`.
- seller-app: `lib/core/networking/dio_provider.dart:35,201,267-287` (suspension interceptor), `lib/core/exceptions/seller_suspension_codes.dart:10` (9240), `lib/core/session/session_provider.dart:168,359`, `session_model.dart:55`, `features/seller/presentation/suspension_notice_screen.dart`, `features/seller/data/payuni_status_notifier.dart`, `features/product/data/product_upload_service.dart:222`, `features/notification/data/push/device_registrar.dart:53`.
- buyer-app: only the buyer-side code `SellerSuspensionCode.sellerSuspendedProductUnavailable` in `features/cart/presentation/cart_controller.dart` and `widgets/checkout_payment_sheet.dart` (not 9240).
- admin: none.

Plan A3 replaces 9240+409 with `permission_denied` + `ErrorInfo.reason=SELLER_SUSPENDED` (ADR `2026-09-25-api-error-model.md` D10). No seller proto exists yet under root `api/hawkr/` (only admin, common, content, identity).

## 6. Recommended-seller ranking (stays behind for A11 "discovery")

- RPC: `GetRecommendedSellers` (1). Routes: `GET /public/sellers/recommended` (`handler/recommended_sellers.go`), `GET /public/marketplace` (`handler/marketplace.go`, uses GetRecommendedSellers + GetSellerBatch). Clients: website `features/market/products/MarketProductsListPage.tsx`, `lib/api/index.ts`, `lib/api/cached.ts`, home bootstrap; marketplace also buyer-app and seller-app.
- Implementation: `services/seller-service/internal/usecase/recommended_sellers.go` (216 lines): product `ListStoreIDsWithPublicProductsInternal` → order `GetTopSellersByRevenue` (Java) → `sellerRepo.GetByStoreIDs` / `GetApprovedExcluding` (reads `sellers`) → buyer `BatchGetSubscriberCounts`. **No ranking-owned tables in seller**; the ranking inputs are the three remote calls plus the `sellers` table. Related (not in seller): ai-service `seller_ranking_facts` fed by `seller.membership_changed` + `order.completed` (procurement ranking, A9/A11 input).
- Serving until A11 — options: (a) keep a tiny legacy piece: gateway handler calls a monolith endpoint that composes the ranking in `internal/httpapi` (or a temporary `internal/legacy` adapter) using seller's public read API + remote Java order/product + buyer module; (b) keep seller-service running only for this RPC (contradicts "delete old service"); (c) move the use case into a `discovery` module now as a stub-free port. ADR D2 already lists `discovery` → {ordering, catalog, buyer, seller, review}, so (a)/(c) fit; the seller module itself must not import buyer/ordering/catalog.

## 7. Monolith coupling to seller-service

- `backend/internal/legacy/sellerservice/sellerservice.go` (74 lines): connect-go gRPC client (`internal/legacy/gen/seller`, generated from `api/proto/seller` via `buf.gen.legacy.yaml`), `Store(ctx, storeID)` → `GetSeller`, 1200 ms deadline, `Operable()` = lifecycle `active`. Wired in `cmd/hawkr/main.go:36,112-113,151` (`SELLER_SERVICE_ADDR`), consumed by `internal/legacy/delegationservice/server.go:23-29,93` (Stores interface) and its integration test. delegation module receives `StoreState` (delegation AGENTS.md:42,74,97; `internal/modules/delegation/internal/model/delegation_rules.go:254`). After A3: seller module public API supplies store state; delegation → seller is an allowed D2 edge.
- `internal/legacy/accountservice/server.go` serves `BindSeller`, `RemoveRole`, `GetAccountInfo` (+ `VerifyGoogleIdToken` for gateway step-up) to seller-service; `events.go` publishes `account.deleted` that seller consumes. identity schema keeps `account_roles.source_id` fence only for async seller/buyer role removal (`internal/modules/identity/db/schema.sql:41`, identity AGENTS.md:44,82) — to remove in A3.
- `internal/arch/modules.go:19`: seller may depend on `identity`, `media` only; fulfillment, catalog, invoice, cart, coupon, ordering, aftersales, review, im, social, discovery, delegation, agent, notification depend on seller.
- `internal/legacy/gen/seller` + `sellerconnect` exist only for the above client; after A3 the same generated code can back a legacy **server** for Java/Go callers.
- Characterization: per the plan the shared part is `internal/httpapi/characterize`; the via-gateway run was kept up to A2 (commit `bafe4b2a3` retired identity's characterization tests by Will's decision) — whether A3 writes a REST+Connect characterization for seller is a question for the plan owner.

## 8. Design questions surfaced

1. Legacy SellerService gRPC server in the monolith: which of the 14 (+4 gateway) RPCs to serve, and whether internal Go services (im, social, notification, review, invoice, ai) and the gateway get it via `internal/legacy` too (like `accountservice`).
2. Gateway-wide dependencies that outlive the seller REST routes: store resolution (`GetCurrentStoreID`, 130 call sites) and suspension guard (~150 routes + 9 inject + 3 view guards) for modules still in the gateway — keep calling the monolith's legacy server, or move `store_id`/suspension into the identity/session token the monolith forwards?
3. D2 violations in current seller logic (buyer subscriber stats and HasActiveBuyer, catalog product names for view stats, review stats in GetSellerStats, social HasPublishedPost for KYC, order active-order check which is a stub) — ports vs http composition vs drop.
4. Events that must keep flowing to RabbitMQ for Java/unmigrated consumers (seller.deleted/suspended/unsuspended/onboarded/profile.changed/membership_changed, merchant.verification.*) and consumers the monolith must host (review.created/hidden, seller.review.result); governance events have no exchange — delete?
5. Vendor portal (invites, sessions, step-up) is used only by ai-service procurement + notification: does it stay in seller or move to agent (A9)?
6. Dead surface to delete: 7 RPCs without callers, 7 routes without clients; `PUT /app/buyers/me/seller_binding` (website pre-grants SELLER without a stint before `POST /app/sellers`) becomes redundant once create+grant is one tx; `seller.governance.*` producer-only events, dormant membership payment results.
7. D9: compose defaults `TGOS_BASE_URL`（舊公司 Cloud Run） egress proxy and `MEDIA_BASE_URL` 指向舊公司的 test-media 網域.
