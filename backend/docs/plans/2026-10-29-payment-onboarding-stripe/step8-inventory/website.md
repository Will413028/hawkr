# Step 8 client deletion inventory: TypeScript side (website, hawkr-ui-text-ts, hawkr-api-ts)

Worktree `hawkr-payment-onboarding`, branch `payment-onboarding`, commit `499c5ae8`. Read-only; nothing was built or run. All paths below are relative to the worktree root. "Read" = I opened the lines; "inferred" = concluded from grep context without opening the whole file.

## 0. Headline

Steps 7a/7b/7e2 already removed most of the PayUni onboarding UI from the website. What is left for step 8 on the TS side is small:

- The name/description lock (D17): one lib file plus its test to delete, and its uses in two screens.
- `flow.identityLocked` reads, the `useMyKycFlow` hook, and e2e mock fields.
- `PAYUNI_TERMS` in the legal manifest, generator and three tests (D11).
- Test fixtures that reference `KYCField` values for removed PII fields (D22).
- A set of user-visible copy strings that still describe the PayUni / ID-card / bank-book flow and are NOT covered by the approved drafts (section 2).

Production website code does not read `flow.status`, `KYCFlowStatus`, `PayUniState`, `SellerPayUni`, `PayUniRemedy`, `canResubmit` or any PII proto field. Command: `git grep -n -E '\bflow\b|canResubmit|resubmitUnavailab|SellerPayUni|PayUniState|PayUniBindStatus|PayUniProblem|KYCFlowStatus' -- website ':!website/features/procurement-assistant' ':!website/lib/codemeta' ':!website/lib/api/openapi-types.ts'` returns only `identityLocked` reads, tests and the e2e mock. The website also imports nothing PayUni-related from CodeMeta: `git grep -n -E 'PayuniMerchantVerificationStatus|PAYUNI_MERCHANT' -- website ':!website/lib/codemeta/generated.ts'` is empty (the only CodeMeta PayUni symbol used is the logistics `PayUniSubmitStatus`, D-class).

## 1. Inventory table

Base grep: `git grep -l -i payuni -- website packages/hawkr-ui-text-ts packages/hawkr-api-ts ':!**/gen/**'` = 116 files (109 website + 7 ui-text-ts generated; 0 in hawkr-api-ts hand-written). Second grep (PII/flow/locks): `git grep -c -i -E 'idCard|id_card|repIdCard|bankAccount|bankCode|bankBook|identityLocked|identity_locked|KYCFlowStatus|KycFlow|flow\.status|flowStatus|PAYUNI_TERMS|KYCField|KYC_FIELD_|PayUniRemedy|...'` on the same paths.

### 1a. Delete / edit (step 8 scope)

| # | Path | What is there | Category | Proposed disposition | Depends on | Tests / specs affected |
|---|---|---|---|---|---|---|
| 1 | `website/lib/seller/payuni-locked-fields.ts` (1 file, 55 lines) | `PAYUNI_LOCKED_NOTICE` :17, `PAYUNI_LOCK_ON_SUBMIT_NOTICE` :26, `PAYUNI_LOCKED_FIELDS` :30, `payuniLockedNoticeFor` :33, `stripPayuniLockedFields` :45 | D17-locks | delete file | `KYCFlow.identity_locked` (seller.proto:881) | row 2 |
| 2 | `website/lib/seller/payuni-locked-fields.unit.test.ts` (1 file) | tests of row 1 | D17-locks | delete file | — | itself |
| 3 | `website/features/seller-center/profile/SellerProfileClient.tsx` | `useMyKycFlow` import :21 and call :73; lock imports :29-31; `payuniLockedNotice`/`payuniLocked` :160-165; `stripPayuniLockedFields` :175, :215; `readOnly`/`description` on 商店名稱 :378-379 and 商店介紹 :501-502; copy "PayUni 會寄金流相關通知到這個信箱" :429 | D17-locks; PAYUNI-onboarding copy (:429) | edit: remove lock handling (fields become editable when `canWrite`); :429 copy is UNDECIDED (section 5, N1) | `GetMyKYC` → `application.flow.identity_locked` | `SellerProfileClient.unit.test.ts` :13, :68, :483-499 (test "locks the store name and description once PayUni holds the identity" goes away; `KYCFlowSchema` import and `useMyKycFlow` mock removed); `tests/delegated-readonly-surface.test.ts:237` (comment only; regex accepts `readOnly={!canWrite}`, read) |
| 4 | `website/features/seller-center/profile/certifications/CertificationsClient.tsx` (1888 lines) | lock import :122; `identityLocked` :370; `kycDraftDiscardable(application, identityLocked)` :1163; `locked={identityLocked}` :1302 and `SectionCompany` `locked` prop :1535, :1543, `readOnly={locked}` :1571, :1598, :1637, disabled lookup button :1578; lock-on-submit notice + comment :1386-1392; type picker subtitles "需身分證 + 存摺" :1460 and "需公司資訊 + 負責人身分證 + 公司帳戶存摺" :1470 | D17-locks; D22-PII copy (:1460, :1470) | edit: remove `identityLocked`, `locked` prop, notice :1386-1392; :1460/:1470 copy UNDECIDED (N2) | `KYCFlow.identity_locked`; `SaveKYCDraft`/`SubmitKYC`/`GetMyKYC` (stay) | `CertificationsClient.unit.test.ts` (row 5) |
| 5 | `website/features/seller-center/profile/certifications/CertificationsClient.unit.test.ts` (1635 lines) | fixtures using `KYCField.KYC_FIELD_BANK_BOOK` :443, :661 with expected text "存摺封面：封面反光看不清楚" :452, :851 (test :436 "退回指到表單已經沒有的欄位（舊申請）"); `flow: { canResubmit: true }` :825; negative assertions `not.toMatch(/PayUni|…/)` :1063, :1088, :1478 and absent-label lists :511-515, :1068-1078 | D22-PII (enum values in fixtures); D15 (`canResubmit` fixture) | edit: fixtures using deleted `KYCField` values must change or the tests be dropped (N6); negative assertions keep (they guard the approved state) | `KYCField` enum values 1-9, 15-18, 26 (seller.proto:675-705); `KYCFlow.can_resubmit` | itself |
| 6 | `website/lib/seller/kyc.ts` (654 lines; no "payuni" hit) | `kycDraftDiscardable(app, identityLocked)` :527-533; rest of the form is already PII-free (header comment :4-8, `KycForm` :40-50) | D17-locks | edit: drop the `identityLocked` parameter | `KYCStatus` (stays) | `kyc.unit.test.ts` :437-446 |
| 7 | `website/lib/seller/kyc.unit.test.ts` (575 lines) | `kycDraftDiscardable(…, true/false)` :437-446; `flow: { canResubmit: true }` :464; `KYCField.KYC_FIELD_BANK_NAME` :315; `KYC_FIELD_ID_CARD_FRONT` :554, :560 ("身分證正面照" :561), `KYC_FIELD_BANK_CODE` :555; mask regex :116 (negative, keep) | D17, D15, D22-PII (enum values in fixtures) | edit (same question as row 5, N6) | `KYCField`, `KYCFlow` | itself |
| 8 | `website/lib/hooks/useSellerSelf.ts` | `KYCFlow` type import :6; `useMyKycFlow` :131-147 (only consumer is row 3: `git grep -n useMyKycFlow -- website`); comment "PayUni 的審核結果由伺服器的背景程序與 PayUni 通知更新…回到分頁時重新讀一次" :113 above `refetchOnWindowFocus: true` :114 | D17-locks (hook has no consumer left); naming (comment) | edit: delete `useMyKycFlow`; whether `refetchOnWindowFocus` on the KYC query stays is UNDECIDED (N8) | `KYCFlow` message | `SellerProfileClient.unit.test.ts:68` mock |
| 9 | `website/docs/legal/seller-agreements.json` | `PAYUNI_TERMS` entry :13-17 (url `https://www.payuni.com.tw/terms`, version `2026-10-07`) | D11-terms | edit: remove entry | `AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS` (seller.proto:215); backend `AgreementKinds()` | rows 10-13 |
| 10 | `website/scripts/generate-legal.ts` | header comment :5; `KINDS = [..., "PAYUNI_TERMS"]` :17; manifest `version` field exists only for the doc-less kind (:20-25, :72 loop; inferred from header comment :5) | D11-terms | edit: remove kind; whether the manifest-level `version` path stays as a mechanism is UNDECIDED (N7) | writes `backend/internal/modules/seller/internal/model/agreement_gen.go` :28-29 (backend, regenerates) | `legal-markdown.unit.test.ts` |
| 11 | `website/scripts/legal-markdown.unit.test.ts` | test "沒有正本的 PAYUNI_TERMS 取 manifest 的版本" :115-133 | D11-terms | edit: rewrite/remove that case | — | itself |
| 12 | `website/features/legal/documents/sellerAgreements.gen.ts` | `PAYUNI_TERMS` :12-15 | generated (from rows 9-10, `pnpm gen:legal`) | regenerates | — | — |
| 13 | `website/lib/seller/agreements.unit.test.ts` | `AgreementKind.PAYUNI_TERMS` :53, :65-66, :69-70 | D11-terms | edit: remove the kind from the case (production `agreements.ts` has no PayUni reference, read) | `AgreementKind` enum | itself |
| 14 | `website/features/seller/onboarding/SellerOnboardingClient.unit.test.ts` | mocked `SELLER_AGREEMENTS.PAYUNI_TERMS` :88 | D11-terms | edit: remove mock entry (production sends only `SELLER_TERMS`, `SellerOnboardingClient.tsx:65`) | — | itself |
| 15 | `website/tests/e2e/helpers/mock-gateway.ts` (1218 lines; no "payuni" hit) | `mockMyKyc` option `identityLocked` :686 and response field :733; `canResubmit` :687, :735-737; `resubmitUnavailabilities` :688, :738-743; type name `MockKycFlowStatus` :466 (values already only NOT_STARTED…APPROVED) | D17-locks; D15 (depends on what happens to `KYCFlow`, N5) | edit: remove `identityLocked`; `canResubmit`/`resubmitUnavailabilities` follow N5. No spec passes these options (`git grep -n -E 'identityLocked|canResubmit|resubmitUnavailab' -- website/tests/e2e` hits only this file) | `GetMyKYC` response JSON shape | e2e specs that call `mockMyKyc` (`seller-dashboard-onboarding-actions.spec.ts` and others) keep working unchanged (inferred: they pass only `status` and payment options) |
| 16 | `website/tests/seller-e2e-fixtures.test.ts` | `flow: { identityLocked: true }` :147, assertion :151 | D17-locks | edit: fixture-shape test must use another field | `GetMyKYCResponseSchema` | itself |
| 17 | `packages/hawkr-api-ts/tests/field-mask.test.ts` | uses `KYCApplicationSchema` paths `business.rep_id_card_no` :15-16 and `individual.id_card_no` :57-58 as the nested / oneof examples | D22-PII (test fixture) | edit: switch to surviving paths (e.g. `business.tax_id`) | `IndividualKYC.id_card_no` (seller.proto:893), `BusinessKYC.rep_id_card_no` (:908) | itself |
| 18 | `packages/hawkr-api-ts/src/proto/field-mask.ts` | doc comment example `individual.id_card_no` :9 | naming-only (comment) | edit comment. Note: if `IndividualKYC` loses its only fields, the "oneof sub-message" exception documented here may have no remaining user (N9) | — | row 17 |
| 19 | `packages/hawkr-ui-text-ts/tests/format.test.ts` (hand-written) | uses `sellerKYCFlowStatusAction` :12, :93-97 and `seller.KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED` :154-158 as the example for "variant omit" and "staff audience" | D15-flow-status (test example) | edit only if the catalog loses `seller.KYCFlowStatus.*` (N5); otherwise keep | catalog keys `seller.KYCFlowStatus.*` | itself |
| 20 | `packages/hawkr-ui-text-ts/src/{enum-manifest.gen.json,keys.gen.ts,zh-tw.gen.ts,messages/en.gen.json,messages/zh-TW.gen.json}`, `tests/{golden-calls.gen.ts,golden.gen.json}` (7 files) | generated from `backend/internal/platform/uitext/catalog/` by `backend/cmd/uitext-gen` | generated | regenerates (keys in section 4) | catalog | `tests/golden.test.ts` |
| 21 | `packages/hawkr-api-ts/src/gen/**` | generated from `api/` | generated | regenerates | — | — |

### 1b. Comment-only / naming-only inside the onboarding area (no behaviour)

| Path | What | Proposed disposition |
|---|---|---|
| `website/lib/seller/repo.ts:5` | comment "認證與 PayUni 在 `kyc.ts`、`payuni.ts`" — `lib/seller/payuni.ts` does not exist (`ls website/lib/seller`) | edit comment (stale) |
| `website/features/delegated-operations/dashboard/DelegatedDashboardPage.tsx:34` | comment names `usePayuniStatus`, a hook that no longer exists (`git grep usePayuniStatus -- website` = this line only) | edit comment (stale) |
| `website/tests/delegated-readonly-surface.test.ts:237` | comment cites the PayUni lock as the example of a stricter `readOnly` | edit comment |
| `website/lib/seller/validators.ts:15, :41, :50`; `website/lib/seller/store-contact.ts:3`; `website/lib/seller/store-contact.unit.test.ts:87` | limits explained as "PayUni 的 MerEmail / ContactName 上限"; test title "長度上限等於 PayUni 的上限" | UNDECIDED wording: plan step 8 says "CARRYOVER 聯絡欄位長度一列改成平台自己的上限並結案" — values (50 / 20) are a backend decision; client comments follow it (N10) |
| `website/lib/media/secure-upload.ts:2-3, :22-23` | header says "KYC ID card, bank statements" and `submitCertification({ idCardFrontStagedToken, bankBookStagedToken })`; module itself hits legacy REST `/app/media/upload_sessions` and is still used by company documents (`CertificationsClient.tsx:72`), invoices and appeals | keep file (D24 Q2 + D-class REST); edit comment |
| `website/styles/features/seller-center.css:1952, :11892, :12112, :12342, :12530` | CSS comments mentioning PayUni. No selector contains "payuni" | naming-only; see N11 for dead-class candidates |
| `website/scripts/check-jsx-cjk-linebreak.ts:138` | self-test fixture string "送出後 PayUni 會回傳結果。" | keep or reword (linter fixture, not UI) |
| `website/features/seller-center/wallet/WalletClient.unit.test.ts:141` | negative assertion `not.toMatch(/逾期|停用|PayUni/)` | keep |

### 1c. D-class: stays (60 website files with "payuni" hits)

Told apart by backend: all of these hit the legacy REST gateway (`/api/v1`, `/app/...`) or are comments about it; none calls the monolith seller/admin-seller Connect service.

| Group (files) | Paths | Evidence |
|---|---|---|
| Buyer payment / checkout landing (17) | `app/[locale]/(commerce)/orders/[orderNo]/page.tsx`, `.../refund/page.tsx`, `components/analytics/PurchaseTracker.tsx`, `features/checkout/screens/{CheckoutClient,CheckoutFailClient}.tsx`, `features/payments/screens/{PaymentLandingClient,PaymentReturnClient}.tsx`, `features/account/screens/orders/OrderDetailClient.tsx`, `lib/analytics/purchase-store.ts`, `lib/checkout/repo.ts`, `lib/payment/{apple-pay,google-pay,useNativePayAvailability}.ts`, `lib/payments/{status,usePaymentResolution}.ts`, `lib/orders/{repo,types}.ts` | e.g. "PayUni 安全付款" `CheckoutClient.tsx:1767`, `gateway: "payuni"` `google-pay.ts:143` |
| Refund / after-sales (7) | `features/after-sales/components/AfterSaleRefundBanner.tsx`, `features/after-sales/screens/RefundResultClient.tsx`, `lib/after-sales/{buyer-repo,return-failure-guidance,status,types}.ts`, `lib/query/keys.ts` | ATM refund landing |
| Logistics / 7-ELEVEN map / shipping onboarding (22) | `app/(commerce)/store-map/return/route.ts`, `features/cart/components/StorePickupPickerSheet.tsx`, `lib/checkout/store-pickups.ts`, `styles/components/sheet.css`, `features/seller-center/profile/shipping/{ShippingClient.tsx,ShippingClient.unit.test.ts,prefill.unit.test.ts}`, `features/seller-center/dashboard/{DashboardPage.tsx,onboardingTasks.unit.test.ts}`, `features/seller-center/orders/{OrderDetailClient,SellerOrdersPage}.tsx`, `features/seller/components/AddressAutocompleteField.tsx`, `lib/addresses/schema.ts`, `lib/logistics/repo.ts`, `lib/seller/{shipping,shipment}.ts`, `lib/seller/{shipment,shipment-provider,shipment-progress-parity}.unit.test.ts`, `lib/shipping/anomalies.ts`, `lib/validation/display-length.ts`, `lib/intl/paths.ts` | `PAYUNI_SUBMIT_STATUS` `lib/seller/shipping.ts:30`; `DashboardPage.tsx:666` is the shipment-anomaly `providerMessage` (read); visible copy "地址已鎖定。如需變更請聯繫 PayUni 客服。" `ShippingClient.tsx:393` is shipping onboarding, not D17 |
| Other legacy-REST context (5) | `lib/hooks/useSellerWorkspaceRealtime.ts`, `lib/seller/canonical-redirect.ts:236`, `lib/sentry/scrub.ts:53` (`"payuni"` and `"bankaccount"` :36 are scrub keys; keep), `lib/codemeta/generated.ts` (generated; PayUni enums at :221, :243, :495, :1864), `docs/seller-payment-logistics-flow.md:188` | — |
| Docs (2) | `docs/transaction-policy.md` (36 hits: fees, Phase 1 "賣家自己的 PayUni 商店帳戶" :18, :198, "賣家綁定 PayUni 商店時" :109), `docs/analytics-events.md` | describes the company-era payment model; outside step 8's allow-list wording (plan allow-list names `backend/docs/**` only) → listed as N12 |
| Tests (7) | `tests/{after-sales-repo,after-sales-return-failure-guidance,date-formatting-taipei,native-pay-apple-routing,sentry-scrub,store-map-return-route}.test.ts`, `tests/e2e/checkout-fail-clears-pending-purchase.spec.ts` | — |

### 1d. Old repo name only (`backend-payuni#NNN` issue references; 21 files, keep)

`app/[locale]/error.tsx`, `features/procurement-assistant/{ClarifyQuestionCard.tsx,conversations.ts,errors.ts,session-logic.ts,useProcurementSession.interrupt.unit.test.ts}`, `features/seller-center/coupons/CouponRulesButton.tsx`, `features/seller-center/products-import/types.ts`, `features/seller-center/products/adapters/productFormAdapter.ts`, `lib/ai-chat/stream-client.ts`, `lib/api/openapi-types.ts`, `lib/chat/chatIdentity.ts`, `lib/media/cdn.ts`, `lib/seller/ai-listing-chat.ts`, `lib/social/repo.ts`, `tests/e2e/{procurement-assistant-clarify,procurement-crop-disambiguation-interrupt}.spec.ts`, `tests/{post-gif-media-contract,procurement-assistant-turn-stream,procurement-assistant-workspace-handoff,social-comment-tsid}.test.ts`. Unrelated to the payment provider.

### 1e. Copy / docs that still describe the old flow (UNDECIDED; details in sections 2 and 5)

`website/lib/content/data.ts:404`, `website/lib/content/seller-guide-data.ts:152, :161-170`, `website/features/seller/landing/components/SellerFeatureOverview.tsx:64`, `website/features/seller/landing/sellerFeatureOverviewData.ts:134`, `website/tests/seller-landing-original-plus-expansion.test.ts:267`, `website/docs/content-copy-audit.md:104, :120, :121`, `website/PRODUCT.md:37, :54`, `website/docs/legal/seller-terms.md:9` (+ `features/legal/documents/sellerTerms.gen.ts:21`).

No Storybook story touches KYC or PayUni: `git grep -l -i -E 'kyc|certif|payuni' -- 'website/**/*.stories.*'` is empty (38 story files exist).

## 2. User-visible changes beyond the approved drafts

Rule: layout, UX copy and flow changes need consent; pure contract-following does not.

| # | Screen | Change in step 8 | Covered by `step7-screen-drafts.md`? |
|---|---|---|---|
| V1 | 商店資訊 (`SellerProfileClient.tsx:378-379, :501-502`) | 商店名稱 and 商店介紹 stop being read-only after approval; helper text "資料已送至 PayUni，店名與商店介紹無法修改，如需變更請聯繫 PayUni" disappears | Drafts do not mention this screen. Covered by plan D17 ("刪除；店名是平台自己的欄位，不同步給 Stripe"). Contract-following (inferred) |
| V2 | 商店資訊 客服電子信箱 helper (`SellerProfileClient.tsx:429`) | "PayUni 會寄金流相關通知到這個信箱" is false after step 8; needs new wording or removal | NOT covered. Drafts §1 only say 客服信箱 is a store field; the payment contact email is the account email (D23 revision, `seller-kyc-notice.md:12`), so this helper has no approved replacement |
| V3 | 商家認證 confirm bar (`CertificationsClient.tsx:1392`) | sentence "送出後商店名稱與商店介紹將無法修改，請先確認無誤" under "所有必填欄位已完成" disappears | Drafts §1 confirm section lists "資料摘要、`KYC_PRIVACY_NOTICE` 同意勾選、缺漏清單" and the note "身分與銀行帳號會在審核通過後，由金流商收集，平台不經手" (already at :1331); the lock sentence is not in the draft. Removal follows D17 |
| V4 | 商家認證 type picker (`CertificationsClient.tsx:1460, :1470`) | subtitles "需身分證 + 存摺" and "需公司資訊 + 負責人身分證 + 公司帳戶存摺" are already wrong (form no longer asks for them); need new wording or removal | NOT covered. Drafts §1: "類型選擇：個人或公司。公司要填統編，所以這一步還需要" — no subtitle text. This is a leftover from 7b rather than a step 8 contract change |
| V5 | 商家認證 company section (`CertificationsClient.tsx:1571, :1578, :1598, :1637`) | 統編, 公司名稱, 販售類型 are no longer read-only for a draft with `identity_locked`; 捨棄草稿 button no longer hidden by the lock (`:1163`) | Not in drafts; follows D17 |
| V6 | 賣家指南 `/seller-guide` (`lib/content/seller-guide-data.ts`) | :152 "送出商家認證後，商店名稱與商店介紹將無法修改"; :161 "完成身分與銀行帳戶的認證"; :166 "「身分證明」：上傳…身分證正反面"; :167 "「銀行帳戶」…"; :168 "…英文姓名…"; :170 "…個人資料蒐集告知事項與金流服務條款的兩個同意項" — all describe the removed form | NOT covered. Drafts have no section on the guide. Already inaccurate since 7b |
| V7 | FAQ (`lib/content/data.ts:404`) | "需先完成商家認證，系統會自動代為向PayUni建立收款帳戶；待PayUni審核通過後即可正式收款。" | NOT covered |
| V8 | Seller landing (`SellerFeatureOverview.tsx:64`; `sellerFeatureOverviewData.ts:134`) | "需要商家認證、PayUni 或物流商開通的能力…"; "查看商家認證、銀行帳戶及 PayUni 建店狀態…" | NOT covered. `tests/seller-landing-original-plus-expansion.test.ts:267` asserts the public copy contains "PayUni", so any rewording changes that test |
| V9 | Shared catalog texts (section 4) | no website screen renders `seller.KYCFlowStatus.*`, `seller.PayUniState.*` or `seller.SellerPayUni.*` | n/a for website (admin `store-row.ts:71` renders `sellerKYCFlowStatus`; other agent's scope) |

Not changing visibly in step 8 (already done in 7b/7e2, read): the two-area status page (`PlatformReviewSection.tsx`, `PaymentAccountSection.tsx`), the PII-free form, the privacy-notice-only checkbox (`CertificationsClient.tsx:215`, :1350).

## 3. Legal documents and generated legal artifacts

| File | What it says | Plan coverage |
|---|---|---|
| `website/docs/legal/seller-agreements.json:13-17` | `PAYUNI_TERMS` → `https://www.payuni.com.tw/terms`, version `2026-10-07` | Covered: drafts §4 "`seller-agreements.json` 的 `PAYUNI_TERMS` 在步驟 8 拿掉（D11）：`GetSellerAgreements` 要求每個種類都有現行版本，必須和 `AgreementKinds()` 一起刪" |
| `website/scripts/generate-legal.ts:5, :17` | `KINDS` includes `PAYUNI_TERMS`; also emits backend `agreement_gen.go` (:28-29) | Covered implicitly by D11; must change in the same commit as the proto enum and backend `AgreementKinds()` (inferred from drafts §4) |
| `website/features/legal/documents/sellerAgreements.gen.ts:12-15` | generated copy of the manifest | regenerates |
| `website/docs/legal/seller-kyc-notice.md` (version line :3 = 2026-11-03) and `sellerKycNotice.gen.ts:24-26` | Already rewritten in 7b (commit `c9125b8e`): :12 platform collects 統編 and contact data; :13 identity and bank data collected by 金流商（目前為 Stripe）; :14 fraud hashes of 統編 and the provider's bank-account fingerprint, kept 5 years. No PayUni, no ID-card wording | Covered and done (drafts §4). Nothing left for step 8. Point :9 still says "包含公司統一編號與聯絡方式" (consistent) |
| `website/docs/legal/seller-terms.md:9` and `sellerTerms.gen.ts:21` | demo disclaimer: "不收集、不處理真實的個人資料，包含賣家的店家資料、身分證件與銀行帳號；請勿輸入真實資料。" | NOT covered by the plan or drafts (drafts §4 names only the KYC notice and the manifest). After D22 the platform has no field for 身分證件 or 銀行帳號. Changing it bumps the `SELLER_TERMS` version (version = the document's version line, `generate-legal.ts:5`), which the server compares on `ApplyStore` (inferred) → N4 |
| `website/docs/legal/self-issue-attestation-terms.md` | tax-identity attestation; no PayUni, no KYC PII | not affected |
| `website/features/legal/pages/*.tsx`, `LegalPage.tsx` | no PayUni or PII wording (`git grep -n -E '金流|身分證|銀行帳|KYC' -- website/features/legal/pages` shows only the notice import in `PrivacyPage.tsx:4, :36-37`) | not affected |
| `website/docs/transaction-policy.md` | internal policy memo, 36 PayUni mentions incl. seller binding (:109) and "賣家自己的 PayUni 商店帳戶" (:18, :198) | not covered; D-class by content (payout model deferred in plan "延後": 撥款模型與手續費重審) → N12 |

## 4. Shared catalog keys referencing removed enums / fields

Source of truth `backend/internal/platform/uitext/catalog/zh-TW.json` (+ `en.json`); TS output regenerates. Listed from `packages/hawkr-ui-text-ts/src/messages/zh-TW.gen.json`.

- `seller.KYCFlowStatus.<VALUE>` for all 11 values. PayUni-specific values: `KYC_FLOW_STATUS_BINDING` ("PayUni 審核中"), `BIND_FAILED` ("PayUni 建檔失敗，處理中"), `FORWARDED`, `UNDER_REVIEW` ("PayUni 審核中"), `PENDING_INFO` ("需補件"), `PAYUNI_REJECTED` ("PayUni 審核未通過").
- `seller.KYCFlowStatus.action.<VALUE>` (10 values, incl. the six above).
- `seller.KYCFlowStatus.banner_description.*`, `banner_title.*`, `gate_heading.*`, `gate_message.*` (5 values each: APPROVED, DRAFT, NOT_STARTED, REJECTED, SUBMITTED). Two texts mention PayUni: `banner_description.KYC_FLOW_STATUS_SUBMITTED` ("審核通過後將自動送出至 PayUni 進行金流審核。") and `gate_message.KYC_FLOW_STATUS_SUBMITTED`.
- `seller.PayUniState.PAY_UNI_STATE_{APPROVED,CONTACT_SUPPORT,FIXABLE,FIX_VIA_SUPPORT,PENDING_INFO,RETRY_LATER,REVIEWING,TERMINATED}`.
- `seller.SellerPayUni.fix_fields_heading`, `seller.SellerPayUni.reason_heading`.
- `seller.KYCField.*` for PII fields: `KYC_FIELD_BANK_ACCOUNT`, `BANK_BOOK`, `BANK_CODE`, `BANK_NAME`, `BIRTH_DATE`, `BUSINESS_REP_ID_CARD_NO`, `ENGLISH_NAME`, `ID_CARD_BACK`, `ID_CARD_FRONT`, `ID_CARD_ISSUE_DATE`, `ID_CARD_ISSUE_LOCATION`, `ID_CARD_ISSUE_TYPE`, `INDIVIDUAL_ID_CARD_NO`, `REAL_NAME`.
- `enum-manifest.gen.json` entries: `hawkr.seller.v1alpha1.KYCFlowStatus` :354, `PayUniBindStatus` :452, `PayUniState` :459, `hawkr.admin.seller.v1alpha1.PayUniObservationSource` :172.
- Key functions in `keys.gen.ts`: `sellerKYCFlowStatus` :423, `…Action` :443, `…BannerDescription` :458, `…BannerTitle` :473, `…GateHeading` :488, `…GateMessage` :503, `sellerPayUniState` :766, `sellerSellerPayUniFixFieldsHeading` :868, `sellerSellerPayUniReasonHeading` :873.

Consumers in my scope: none in `website`. Only `packages/hawkr-ui-text-ts/tests/format.test.ts:12, :93-97, :154-158` (hand-written). Command: `git grep -n -E 'seller(KYCFlowStatus|PayUniState|SellerPayUni)[A-Za-z]*\(' -- website admin/src seller-app/lib packages` → `admin/src/app/admin/merchants/store-row.ts:71` plus generated golden files and the format test. (seller-app reads `KYCFlowStatus` in `seller-app/lib/features/seller/data/{current_kyc,kyc_flow}.dart`; other agent's scope.)

Keys that mention identity/bank but belong to the new flow and stay: `seller.OnboardingTaskId.description.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT`, `…panel_description…`, `seller.PaymentAccount.payouts_inactive_note`, `seller.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED`.

## 5. Not decided by the plan (options only; no recommendation)

- **N1. 客服電子信箱 helper text** (`SellerProfileClient.tsx:429`). Options: (a) remove the helper; (b) reword to what the support email is actually used for; (c) reword to mention the payment provider generically. Evidence: the provider receives the account contact email, not the support email (`seller-kyc-notice.md:12`).
- **N2. Type picker subtitles** (`CertificationsClient.tsx:1460, :1470`). Options: (a) delete the second `<small>` on both cards; (b) reword to what is now required (individual: nothing extra; business: 統編與公司資料). Drafts §1 give no text.
- **N3. Marketing / help copy** describing the old flow: guide (`seller-guide-data.ts:152, :161-170`), FAQ (`data.ts:404`), landing (`SellerFeatureOverview.tsx:64`, `sellerFeatureOverviewData.ts:134`, test `seller-landing-original-plus-expansion.test.ts:267` requires the string "PayUni"). Options: (a) rewrite in step 8 (needs copy consent); (b) leave as D-class-adjacent and fix in a separate copy pass; (c) minimal factual fix (replace provider name with 金流商, drop the removed sections). Note the landing sentence at :64 also covers "物流商開通", which is D-class and still true.
- **N4. `seller-terms.md:9`** mentions 身分證件與銀行帳號. Options: (a) leave (it is a "do not enter real data" warning, still harmless); (b) edit and accept a `SELLER_TERMS` version bump; (c) edit without changing the version line. The plan covers only the KYC notice and the manifest.
- **N5. Fate of `KYCFlow` beyond the listed deletions.** Step 8 names `KYCFlowStatus` PayUni values plus BINDING/BIND_FAILED/FORWARDED/UNDER_REVIEW/PENDING_INFO and `IdentityLocked`; it does not say whether `KYCFlow`, `KYCFlowStatus` (remaining NOT_STARTED/DRAFT/SUBMITTED/REJECTED/APPROVED, which duplicate `KYCStatus`), `can_resubmit` and `resubmit_unavailabilities` (seller.proto:875-888; reasons 3-9 are PayUni, :856-862) survive. Website production code reads none of them (section 0), so either choice is deletion-only on the website: tests `CertificationsClient.unit.test.ts:825`, `kyc.unit.test.ts:464`, mock `mock-gateway.ts:687-743`, and the catalog test `format.test.ts`. Options: (a) keep the reduced `KYCFlowStatus` and `KYCFlow`; (b) delete `KYCFlow` and `KYCFlowStatus` entirely and let clients read `KYCStatus` and `GetMyOnboarding` (D26). Admin and seller-app do consume it.
- **N6. `KYCField` values for the non-ID personal fields and the "old application" tests.** D22 names 身分證號、銀行帳號、證件; the form and requirements also dropped `real_name`, `english_name`, `birth_date`, `id_card_issue_*`, `bank_name` (7a). Whether those proto fields and `KYCField` values (seller.proto:675-682, :936-944) are deleted in step 8 is not stated. On the website this decides the fate of tests that assert "a rejection pointing at a field the form no longer has still shows its name" (`CertificationsClient.unit.test.ts:436-453, :652-664, :846-873`; `kyc.unit.test.ts:308-318, :550-563`). Options: (a) delete those tests with the enum values (no such applications can exist; system never launched); (b) keep the behaviour and re-point the fixtures at a surviving non-form field such as `KYC_FIELD_WEBSITE_URL`.
- **N7. `generate-legal.ts` manifest-level `version`.** `PAYUNI_TERMS` is the only agreement without a source document (`:5`). Options: (a) remove the `version`/doc-less path together with the kind; (b) keep it for a future provider-terms kind (D11 says platform-collected provider consent would be added when a Taiwan provider is connected).
- **N8. `refetchOnWindowFocus` on the KYC query** (`useSellerSelf.ts:113-114`): its stated reason is PayUni background updates. Options: (a) keep with a new reason (admin decisions also arrive in the background); (b) remove.
- **N9. `IndividualKYC` after `id_card_no` goes** (seller.proto:893-894): if the message becomes empty or is removed, `field-mask.ts:9` documents an exception with no remaining example and `kyc.ts` mask paths for the individual branch change. Backend/proto decision; website follows.
- **N10. Contact-field limits wording** (`validators.ts:15, :41, :50`; `store-contact.ts:3`; `store-contact.unit.test.ts:87`): step 8 says the CARRYOVER row becomes "平台自己的上限" but not whether the numbers (email 50, name 20) change. Client constants and the test title follow that backend decision.
- **N11. Possibly dead CSS** in `website/styles/features/seller-center.css` (naming-only; inferred): 56 of 118 `.seller-wallet__*` / `.seller-cert__*` class names have no literal reference under `website/{app,features,components,lib}` (checked with `git grep -F` per class; dynamic class composition and CSS-contract tests not excluded). Includes the old document-upload cards (`.seller-cert__upload-card` :9676, `.seller-cert__upload-pair` :10694), the wallet stepper / type picker (`.seller-wallet__stepper` :11913, `.seller-wallet__picker` :11990, `.seller-wallet__type` :12024) and the "PayUni 聯絡資訊編輯卡片" block (`.seller-wallet__diff-*` :12342). Options: (a) remove in step 8; (b) separate cleanup; not named by the plan.
- **N12. Website internal docs** with PayUni seller-binding content: `docs/transaction-policy.md` (:18, :109, :198), `docs/content-copy-audit.md:104, :120, :121` (lists a `/seller-center/me/payuni` route that does not exist: `ls "website/app/[locale]/(seller)/seller-center/me"` = certifications, shipping), `PRODUCT.md:37, :54`. Step 8's allow-list names `backend/docs/**` only; its acceptance grep covers `backend/internal backend/cmd api/hawkr`, not `website`. Options: (a) leave (no acceptance check hits them); (b) header note like the backend PayUni docs ("B 階段刪除"); (c) edit.

## 6. Counts

Commands (run from the worktree root):

- `git grep -l -i payuni -- website packages/hawkr-ui-text-ts packages/hawkr-api-ts ':!**/gen/**' | wc -l` → 116 (109 website, 7 ui-text-ts generated, 0 hawkr-api-ts).
- `git grep -c -i -E '<PII/flow/lock pattern>' -- website packages/hawkr-ui-text-ts packages/hawkr-api-ts ':!**/gen/**'` → 41 files, of which in scope: the rows of table 1a (others are buyer-order ATM `atmBankCode`, `procurement-assistant` `flow.status`, `solidCards` false positives).

Classification of the 109 website files with a "payuni" hit (manual, from the tables above):

| Class | Files |
|---|---|
| delete | 2 (`lib/seller/payuni-locked-fields.ts`, its unit test) |
| edit (code, tests, manifest, generated legal) | 11 (rows 3, 4, 5, 8, 9, 10, 11, 12, 13, 14 and `SellerProfileClient.unit.test.ts`) |
| copy / docs UNDECIDED | 6 (`lib/content/data.ts`, `SellerFeatureOverview.tsx`, `sellerFeatureOverviewData.ts`, `tests/seller-landing-original-plus-expansion.test.ts`, `docs/content-copy-audit.md`, `PRODUCT.md`) |
| comment / naming-only in the onboarding area | 9 (table 1b minus `secure-upload.ts`, which has no "payuni" hit) |
| keep, D-class | 60 (table 1c) |
| keep, old repo name only | 21 (table 1d) |
| total | 109 |

Additional files with no "payuni" hit that step 8 touches: `website/lib/seller/kyc.ts`, `website/lib/seller/kyc.unit.test.ts`, `website/tests/e2e/helpers/mock-gateway.ts`, `website/tests/seller-e2e-fixtures.test.ts`, `packages/hawkr-api-ts/tests/field-mask.test.ts` (edit, 5); `packages/hawkr-api-ts/src/proto/field-mask.ts`, `website/lib/media/secure-upload.ts` (comment-only, 2); `packages/hawkr-ui-text-ts/tests/format.test.ts` (edit if N5 removes the keys, 1); `website/lib/content/seller-guide-data.ts`, `website/docs/legal/seller-terms.md` + `sellerTerms.gen.ts` (UNDECIDED copy, 3).

Totals for the TS side: delete 2; edit 16 certain (11 + 5) + 1 conditional; UNDECIDED copy/docs 9; comment/naming-only 11; D-class keep 60; unrelated keep 21; generated: 7 ui-text-ts files + `hawkr-api-ts/src/gen` + `sellerAgreements.gen.ts` (counted under edit).

## 7. Not covered / limits

- `admin/` and Flutter (`seller-app`, `buyer-app`, `packages/hawkr_*` Dart) are out of scope; mentioned only where a shared catalog key has a consumer there.
- Nothing was executed (no pnpm, tests, typecheck, codegen), so "edit" rows are from reading, not from compiler output; after the proto change `tsc` may surface further references that grep patterns missed.
- `CertificationsClient.tsx` (1888 lines) and its test (1635 lines) were read only around grep hits, not end to end.
- The 60 D-class and 21 old-repo-name files were classified from the grep line content; only `DashboardPage.tsx:656-667`, `ShippingClient` hits, `secure-upload.ts` header and `transaction-policy.md` seller lines were opened.
- The dead-CSS list (N11) is a literal-string heuristic.
- `.env*` and `secrets/` were not read. The file `backend/internal/platform/uitext/catalog/zh-TW.json` was only counted (146 matching lines), keys were taken from the generated TS JSON.
