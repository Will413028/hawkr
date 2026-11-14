# 7e 開通進度推導盤點（唯讀，快照 commit `7269bdf3`）

盤點時 website、seller-app、admin 各自推導開通任務的條件、資料來源與兩端差異；D31 的依據。英文是盤點者的原文。

Abbrev: W = website/features/seller-center/dashboard/onboardingTasks.ts (OT.ts); A = seller-app/lib/features/seller/data/onboarding_tasks.dart (ot.dart); ADM = admin/src/app/admin/merchants.
Catalog = packages/hawkr-ui-text-ts/src/messages/zh-TW.gen.json (shared server-ish text catalog, keys seller.KYCStatus.*, seller.PaymentAccountStatus.{,action.,description.}*, seller.SellerRequirement.*, common.unknown).

## 0. Shared inputs (what feeds the builder)
| input | website source | seller-app source | backend owner |
|---|---|---|---|
| store + capabilities (PUBLIC_LISTING.missing, SHIPPING_SETUP.active/missing), description | GetMyStore (SellerAppService) via useSellerSelf; store.ts:31-65 | GetMyStore; my_store.dart:158-193 | seller module: internal/httpapi/seller/app.go:179 GetMyStore -> model.CapabilitiesOf (model/capability.go:60-85 `needs` table) ; conditions from stint.WithConditions (seller/internal/stint/stint.go:~255-290) reading SQL view seller.store_requirement_flags |
| KYC status | GetMyKYC `application.status` (useMyKyc, hooks/useSellerSelf.ts:112; cert-gate.ts:51) | GetMyKYC (`MyKyc.reviewStatus = application?.status`, current_kyc.dart:49) | seller module: internal/httpapi/seller/kyc.go:195 |
| payment account | GetMyPaymentAccount (payment-account.ts:29-39, NotFound -> null); fetched only when KYC approved (cert-gate.ts:52-54) | GetMyPaymentAccount (payment_account.dart:79-89, NotFound -> null), accountPaymentAccount provider (:123) always fetched | payment module (internal/modules/payment; status derivation internal/modules/payment/internal/account/port.go:163 StatusOf: ENABLED iff transfers capability Active); RPC exposed by internal/httpapi/seller/payment.go:54 via Payments interface |
| shipping rows | REST GET /api/v1/app/shipping_onboardings (shipping.ts:71-81; hook lib/hooks/useShippingOnboardings.ts) -> `payuniSubmitStatus` | same REST (shipping_settings_remote_service.dart:14-19) | LEGACY: gateway/internal/router/routes_shipping.go (+constants.go:141); backing = Java shipping-service (services/java/.../ShippingGrpcServiceImpl.java). Not in the Go monolith modules. |
| product count | REST GET /api/v1/app/products?page=1&page_size=1&status=4004 -> `.total` (DashboardPage.tsx:90-97; seller-repo.ts:347-359; root "/app/products" :342; ProductStatus.ACTIVE="4004" codemeta/generated.ts:99-104) | REST GET /api/v1/app/products?view=manage&page=1&page_size=20 (no status) -> `records.length` (product_repository.dart:18-32, :304-319; ot.dart:299-306) | LEGACY gateway route (gateway/internal/router/constants.go:52-55) -> legacy product service (backend/internal/legacy/productservice); NOT catalog module in monolith (internal/modules has no product module) |

Web loading/availability: onboardingTasks gets `kycStatus = certGate.application?.status` (DashboardPage.tsx:224-231). useCertGate returns application=null/paymentAccount=null while loading or when !seller (cert-gate.ts:59-89).

## 1. Task table (required group first, then recommended)
Web order = array order OT.ts:117-169 then pushed other_requirement (:176-188) then recommended (:210-265). Group labels W :53-56 "公開前必須"/"建議完成"; A :16-23 same strings.

### 1.1 profile  (web id `profile`; app `OnboardingTaskId.profile`)
| | website | seller-app | diff |
|---|---|---|---|
| group/order | required, #1 (OT.ts:119-120) | beforePublish, #1 (ot.dart:149-150) | same |
| completed | `profileDone = missingProfile.length===0` (OT.ts:99); missingProfile = PROFILE_REQUIREMENTS(STORE_NAME,LOGO,TOWN_CODE; :59-63) filtered by `lacks` (:93-94: `missing===undefined || missing.includes(r)`), missing = PUBLIC_LISTING.missing (store.ts:61-65) | same logic: ot.dart:128-138; missing from my.missingFor(PUBLIC_LISTING) (my_store.dart:185-193, drops UNSPECIFIED) | same. PUBLIC_LISTING not reported => all three "missing" in both |
| locked | none | none | - |
| statusLabel | OT.ts:129-131 `profileDone ? "已完成" : "缺少：<names joined 、>"`; names = ut(catalog seller.SellerRequirement.*: 店名/logo/所在地) else commonUnknown("未知") (:96-98) | ot.dart:155 same; names via requirement.uiText(l) ?? commonUnknown (:134-137) | same |
| CTA/href | cta "去設定"; href /seller-center/me (:125-126) | cta "去設定" (:153); action: _openProfile -> onOpenProfile callback or MainRoutes.settings (onboarding_progress_card.dart:72-74,125-132) | route differs by platform only |
| backend | seller: stores.store_name/logo_key/town_code via view flags has_store_name/has_logo/has_town_code (objects.sql:17-31) | | |

### 1.2 platform_review (app `platformReview`)
| | website | seller-app | diff |
|---|---|---|---|
| completed | `reviewDone = paymentDone || kycStatus===KYC_STATUS_APPROVED` (OT.ts:100-101) | `reviewDone = platformApproved(reviewStatus)` = reviewStatus==APPROVED (ot.dart:141; payment_account.dart:42-43) | DIFF: web also true when onboarding_complete (server) is already satisfied even if GetMyKYC says otherwise/was not read; app ignores onboarding_complete |
| locked | none | none | |
| statusLabel | OT.ts:145-149: reviewDone -> catalog(KYC APPROVED)="已通過"; else kycStatus===undefined -> "未送出"; else catalog(kycStatus) (DRAFT 草稿, SUBMITTED 審核中, REJECTED 需修改, CANCELLED 已取消; unknown -> "未知") | ot.dart:165-167: `reviewStatus==null && reviewReadable ? '未送出' : (reviewStatus?.uiText(l) ?? commonUnknown)`; so unreadable GetMyKYC -> "未知" | DIFF: app has reviewReadable (card :44 `kyc != null`) -> "未知" on read failure/loading; web has no readable flag, undefined (loading, error, or none) -> "未送出". Also web shows APPROVED text when reviewDone via paymentDone even if status != APPROVED; app shows the real status text |
| CTA | `reviewDone ? undefined : "前往商家認證"` href CERT_PAGE_HREF=/seller-center/me/certifications (cert-gate.ts:104) (OT.ts:140-142) | same cta (ot.dart:163); tap -> context.go(MainRoutes.wallet) (card :75-77) | app goes to wallet tab (which shows gate + opens KYC wizard), web to cert page |
| source | GetMyKYC + onboarding_complete | GetMyKYC | server-side: seller.kyc_applications.status |

### 1.3 payment_account (app `paymentAccount`)
| | website | seller-app | diff |
|---|---|---|---|
| completed | `paymentDone = !lacks(ONBOARDING_COMPLETE)` (OT.ts:100) i.e. PUBLIC_LISTING reported AND missing lacks ONBOARDING_COMPLETE (server: KYC APPROVED AND payout_capabilities.transfers_active) | `paymentDone = paymentAccountEnabled(account)` = account.status==ENABLED (ot.dart:143; payment_account.dart:37-38), where account=effectivePaymentAccount (null unless reviewDone; NOT_CREATED if approved and no account; :53-62) | DIFF in source: web = seller view flag; app = payment module GetMyPaymentAccount. Equivalent in steady state (ENABLED iff transfers active, port.go:163-175; seller copy via PayoutAccountChanged subscriber same tx, schema.sql:1628) but app additionally requires KYC APPROVED via effective account; web requires nothing from payment RPC |
| locked | `locked = !reviewDone` (OT.ts:164) | `locked = account==null` (ot.dart:181) = !reviewDone | same effect (web reviewDone includes paymentDone) |
| statusLabel | OT.ts:165-167: !reviewDone -> "平台審核通過後開放設定"; else payment.badge = catalog seller.PaymentAccountStatus (準備中/審核中/待設定/已啟用/無法啟用) else "未知". Account fetch failure/null is silently treated as NOT_CREATED "準備中" (:103-108) | ot.dart:182-186: account==null -> const paymentAccountNotYetOpenNotice "平台審核通過後開放設定" (payment_account.dart:46); paymentUnreadable (= reviewDone && !paymentReadable && !paymentDone, :144) -> "未知"; else status.uiText ?? "未知" | DIFF: app distinguishes read failure ("未知"); web shows "準備中" on failure |
| CTA/href | cta = `paymentDone || !reviewDone ? undefined : paymentLink?.label` (OT.ts:161); href = paymentLink.href ?? CERT_PAGE_HREF (:159). paymentAccountLink (payment-account.ts:128-139): DISABLED+action -> {catalog "聯絡客服", /contact}; ACTION_REQUIRED+action -> {catalog "前往設定收款帳戶", /seller-center/me/certifications}; else {"前往商家認證", cert page} (CERT_ACTION_LABEL :122). Note NOT_CREATED/UNDER_REVIEW catalog action "重新整理" is NOT used as a link label | ot.dart:175-179: paymentDone||account==null -> null; paymentUnreadable -> '前往商家認證'; else account.status.action(l) (catalog: 待設定 "前往設定收款帳戶", DISABLED "聯絡客服", NOT_CREATED/UNDER_REVIEW "重新整理"; ENABLED none). Tap always context.go(MainRoutes.wallet) (card :75-77) | DIFF: web CTA label for NOT_CREATED/UNDER_REVIEW = "前往商家認證"(go to cert page); app label = "重新整理" but navigates to wallet. DISABLED: web -> /contact, app -> wallet (not contact) |
| source | onboarding_complete (GetMyStore) + GetMyPaymentAccount (only when KYC approved, cert-gate.ts:52-54) | GetMyKYC + GetMyPaymentAccount | payment module + seller view |

### 1.4 other_requirement
| | website | seller-app | diff |
|---|---|---|---|
| shown when | `unrecognized = (missing??[]).filter(r != UNSPECIFIED && !KNOWN_REQUIREMENTS.has(r))` nonempty (OT.ts:171-176). KNOWN = STORE_ACTIVE, NOT_PENDING_DELETION, NOT_SUSPENDED (HANDLED_BY_NOTICE_PAGE :69-73), ONBOARDING_COMPLETE, profile three (:75-79) | `missing.any(!_knownRequirements.contains)` (ot.dart:222-224); known set ot.dart:94-102 (same 7) | same set |
| completed/locked | false / none | false / none | |
| label/CTA | statusLabel "待完成" (:187); no cta; href /seller-center/me (:184); panelDescription "還有公開條件尚未滿足，請聯絡客服。" | "待完成" (:232); no cta; tap no-op (card :86-87) | same |
Extra app-only rows (no website equivalent; web redirects to notice page via pageCapabilities.ts:106-130 instead):
- app `notSuspended`: shown if PUBLIC_LISTING missing NOT_SUSPENDED (ot.dart:190-203): title "停權處分", desc "帳號目前處於停權。", cta "查看停權說明", status "停權中", completed false; tap -> SellerRoutes.suspensionNotice (card :78-79)
- app `storeActive`: shown if missing STORE_ACTIVE or NOT_PENDING_DELETION (ot.dart:204-221): "商店狀態"/"商店目前不是營業中。"/cta "查看說明"/status "未營業"; tap -> suspensionNotice (card :80-81)
Web note: OT.ts:65-68 says progress card is not shown in these states (outer shell redirects to notice page when the page capability is missing NOT_SUSPENDED/NOT_PENDING_DELETION/STORE_ACTIVE, pageCapabilities.ts:119-130; dashboard page capability MARKETING :33).

### 1.5 shipping (recommended #1 of group)
| | website | seller-app | diff |
|---|---|---|---|
| completed | `shippingDone = shippingRows.some(isApproved)` (OT.ts:206) isApproved = payuniSubmitStatus==="4303" (shipping.ts:284-286, :30-35) | `ShippingProgress.done = statuses.contains(PayUniSubmitStatus.succeeded.value)` (ot.dart:71-77) | same (either shipping type row suffices: HOME_DELIVERY "3652" / CVS "3651" not distinguished) |
| processing | `some(isProcessing)` = "4302" (:205; shipping.ts:276) | contains requested (ot.dart:75) | same |
| locked | `locked = !shippingOpen`; shippingOpen = shippingGateOf(my)==="open" (OT.ts:203-204,227; store.ts:83-96) | `locked = !shippingOpen` from my.shippingGate (ot.dart:237-238,254; my_store.dart:195-217) | same semantics. Gate: no SHIPPING_SETUP status->unknown; active->open; missing NOT_PENDING_DELETION->pending_deletion; NOT_SUSPENDED->suspended; (web: STORE_ACTIVE->unknown; ONBOARDING_COMPLETE->certification) (app: ONBOARDING_COMPLETE && !STORE_ACTIVE -> certification); else unknown. SHIPPING_SETUP needs ACTIVE,NOT_SUSPENDED,ONBOARDING_COMPLETE,NOT_PENDING_DELETION (capability.go:82) |
| statusLabel | OT.ts:228-236: !open -> (gate==="certification" ? "需先通過認證" : "未營業"); done "已開通"; processing "審核中"; else "待設定" | ot.dart:255-261 identical strings | same. Note pending_deletion/suspended/unknown all map to "未營業" |
| CTA/href | cta done "查看物流設定" / processing "查看審核進度" / else "設定物流"; href /seller-center/me/shipping (:220-224) | same cta (ot.dart:248-252); push SettingsRoutes.shipping (card :82-83). Locked => button disabled (web Button disabled OnboardingProgressCard.tsx:180; app onPressed null card :299) | same |
| read failure | `fetchShippingOnboardings` returns [] for non-array / `throwOnHttpError:false` (shipping.ts:71-81) -> not done | try/catch -> ShippingProgress() (ot.dart:292-297) | same |

### 1.6 product
| | website | seller-app | diff |
|---|---|---|---|
| completed | `productsDone = (productCount ?? 0) > 0` (OT.ts:207) ; count = products list `.total` with status=ACTIVE(4004) page_size=1 (DashboardPage.tsx:90-97) | `(productCount ?? 0) > 0` (ot.dart:239); count = `records.length` of first page (20) of view=manage list, NO status filter (ot.dart:299-306; product_repository.dart:304-319) | **DIFF**: web counts only ACTIVE (上架) products; app counts any product incl DRAFT/PENDING_REVIEW/INACTIVE. App also requires storeId!=null (:300). Failure: web productCount null (loading/err => "待上架"); app null => same |
| locked | none | none | |
| status | "已上架"/"待上架" (OT.ts:250) | same (ot.dart:270) | |
| CTA | "新增商品" -> /seller-center/products/new (:246-247) | "新增商品" -> ProductRoutes.picker (card :84-85) | |
| owner | legacy gateway /app/products -> productservice | same | legacy |

### 1.7 description
| | website | seller-app | diff |
|---|---|---|---|
| completed | `(my?.store?.description ?? "").trim() !== ""` (OT.ts:208) | `my?.description != null` where description=_nonEmpty(store.description) (trim) (ot.dart:240; my_store.dart:231,244-247) | same |
| status | "已完成"/"待完成" (:263) | same (:279) | |
| CTA | "去設定" /seller-center/me | "去設定" -> _openProfile | |
| owner | seller: stores.description (GetMyStore.store.description) | | |

## 2. Aggregates / UI behavior (both clients, client-side today)
- required tasks = tasks with group required (W OnboardingProgressCard.tsx:~90 `requiredTasks`; A card :50-52). completedRequired = count(completed); allRequiredComplete = total>0 && completed==total. progress% = round(c/total*100) (web :~95-97; app Header :152-154). Because other_requirement/notSuspended/storeActive rows add to total only when present, the denominator varies (web 3..4, app 3..5).
- suggestedTask = first incomplete required else first incomplete overall (web :~100-102); active task = stored id (dashboardUiStore) ?? suggested ?? tasks[0]. App has no active-task concept (list only).
- Collapsed = override ?? allRequiredComplete (web `collapsedOverride ?? defaultCollapsed` ; app card :56-57). Override persisted in localStorage store `sellerDashboardUi` (dashboardUiStore.ts:28-31,62-70); app: useState (not persisted).
- web-only presentation: icon names, panelTitle/panelDescription (OT.ts), step icons/locks in OnboardingStep (card :20-60). task.locked -> lock icon, muted; completed -> check; else activity. App: check / shield(locked) / clock.
- web-only: dashboard banner for payment status != ENABLED (DashboardPage.tsx:78-81, 191-205): badge + description from paymentAccountCopy, button -> CERT_PAGE_HREF.
- Refresh: web react-query staleTime 30s + refetchOnWindowFocus (useSellerSelf.ts:97-105,163-169); app onBecameVisible invalidates onboardingInputs and refreshes status with maxAge (card :61-68).

## 3. Related screens that derive overlapping state
- Web certifications page: `certified = isStoreCertified(seller)`; `shownStatus = certified ? APPROVED : application?.status` (CertificationsClient.tsx:356-359); `platformApproved = shownStatus===APPROVED` (:1053) -> PaymentAccountSection(platformApproved) (:1075). Certified sellers are redirected to /seller-center/wallet (:363-372). PlatformReviewSection badge: known ? catalog(status) ?? 未知 : "未送出" (PlatformReviewSection.tsx:49-54), time labels "核准時間："/"送出時間：" (:64-69), "取消申請" button when kycCancellable (:55,72-83).
- Web PaymentAccountSection.tsx: !platformApproved -> "平台審核通過後開放設定" (:154-165); loading "載入中…" (:184); confirming "正在確認資料…" (:186); load error or no badge -> defaults(Code.Internal) text + "重新整理" (:189-195); else badge/description/payoutsNote(from catalog sellerPaymentAccountPayoutsInactiveNote when ENABLED && !payoutsActive, payment-account.ts:114-117) (:198-202); action button branching DISABLED -> /contact; canOnboard (ACTION_REQUIRED && surface HOSTED|EMBEDDED, payment-account.ts:86-92) -> "開啟設定中…" begin session; ACTION_REQUIRED w/o surface -> none; other -> refetch button (:203-233); "上次更新 …" (:234-236).
- App certification_status_view.dart: same two sections: kycStatusTone (:23-29), stamps (:36-53, strings '核准時間：'/'送出時間：' :168-169), !approved -> paymentAccountNotYetOpenNotice (:189-195), loading '載入中…' (:199-203), error -> defaultErrorText + '重新整理' (:210-231), embedded notice '請到網頁版完成收款設定' (payment_account.dart:49; shown :278-281), '上次更新 $fetched' (:286). Action kind via paymentAccountActionKind (payment_account_action.dart, not read in full).
- App wallet_screen.dart: `approved = platformApproved(kyc.reviewStatus)` (:98-99); `account = effectivePaymentAccount(payment.value, approved)` (:117); `certified = my?.isCertified` (:118, = PUBLIC_LISTING lacks no ONBOARDING_COMPLETE, my_store.dart:220-228); account==null -> _CertificationGate (title certificationGateTitle "尚未通過商家認證", message "通過平台審核並啟用收款帳戶後，就能啟用錢包與收款。", "目前狀態：<KYC catalog or 未送出>", button "前往商家認證"; seller_certification_gate.dart:21-23, wallet_screen.dart:212-251); else !ENABLED -> _PaymentAccountPanel (:124-125); certified -> WalletAccountSection (:126). Load failure panel title '無法載入認證狀態' (:108, :272). So wallet mixes payment-RPC status (panel) and onboarding_complete (WalletAccountSection) in one screen.
- Admin (no onboarding task list; no combined status): 
  - list badge: rowBadge (store-row.ts:77-91): CLOSED "已關閉" gray > suspended "停權中" danger > PENDING_DELETION "等待刪除" warning > flowBadge(kycFlowStatus) (:66-74; staff text catalog sellerKYCFlowStatus; colors :48-60). rowState (:38-45): isPending flows set (:24-31), isRejected {REJECTED, PAYUNI_REJECTED} (:33-36). List filter tabs all/pending/rejected/approved ("已核准", page.tsx:204-243).
  - detail header (page.tsx:807-884): `flowBadge(kycSummary.flowStatus)` else "狀態不明"; no kyc summary -> "未申請認證"; plus lifecycle badge. `isApproved = isKycApproved(kycSummary?.status)` (page.tsx:259; "KYC manual decision", NOT incl. payment account/PayUni).
  - payment account is a separate card PaymentAccountSection (PaymentAccountCard.tsx; page.tsx:268, 1202-1206) via PaymentAdminService.GetPaymentAccount (admin/payment/v1alpha1; payment-account.ts:34-44): notFound -> "尚未建立（平台審核核准後自動建立）" (:141-144); error "收款帳戶讀取失敗" (:129-132); badge = staffTextOrUnknown(catalog PaymentAccountStatus) w/ variants NOT_CREATED gray, UNDER_REVIEW/ACTION_REQUIRED warning, ENABLED success, DISABLED danger (:45-51,146-149); loading "收款帳戶載入中…". Admin staff audience text, not the seller sentences.
  - => admin "已核准/成功" badge is KYC-flow only; it never reflects onboarding_complete. No admin equivalent of required-task completion.

## 4. Backend owners (item 1)
| data | owner | path |
|---|---|---|
| stores, capabilities, store_requirement_flags, kyc_applications, payout_capabilities | seller module | backend/internal/modules/seller (db/objects.sql, db/schema.sql:1628-, internal/model/capability.go, internal/model/exposure.go, internal/stint/stint.go) ; RPC handlers backend/internal/httpapi/seller/app.go:179 (GetMyStore), kyc.go:195 (GetMyKYC), payment.go:54 (GetMyPaymentAccount handler in seller httpapi, delegating to payment module) |
| payment account status, surface | payment module | backend/internal/modules/payment (internal/account/port.go:163 StatusOf) ; seller copy payout_capabilities written by PayoutAccountChanged subscriber |
| shipping onboardings | legacy gateway REST /api/v1/app/shipping_onboardings -> Java shipping-service | backend/gateway/internal/router/routes_shipping.go, constants.go:138-141, services/java/shipping-service |
| product count | legacy gateway REST /api/v1/app/products -> productservice | backend/gateway/internal/router/constants.go:52-55, backend/internal/legacy/productservice |
| ui text catalog | packages/hawkr-ui-text-ts (+ hawkr_ui_text dart) generated from enum manifest | packages/hawkr-ui-text-ts/src/messages/zh-TW.gen.json |

## 5. SQL: onboarding_complete (item 2)
Latest definition: backend/internal/modules/seller/db/migrations/20261106120300_payment_moved_to_payment_module.sql (CREATE OR REPLACE VIEW; identical to db/objects.sql:17-31). History: created 20261019014830_seller_pub_exposable_stores.sql (kyc_approved); renamed/changed 20261102122621_payment_gate_and_review.sql (DROP+CREATE); current uses payout_capabilities.
```sql
CREATE OR REPLACE VIEW seller.store_requirement_flags AS
SELECT s.store_id,
       s.account_id,
       s.status <> 'CLOSED' AS active,
       s.status <> 'PENDING_DELETION' AS not_pending_deletion,
       EXISTS (
           SELECT 1
           FROM seller.kyc_applications a
           JOIN seller.payout_capabilities p ON p.store_id = a.store_id
           WHERE a.store_id = s.store_id AND a.status = 'APPROVED' AND p.transfers_active
       ) AS onboarding_complete,
       (s.store_name IS NOT NULL)::boolean AS has_store_name,
       (s.logo_key IS NOT NULL)::boolean AS has_logo,
       (s.town_code IS NOT NULL)::boolean AS has_town_code
FROM seller.stores s;
```
Dependents:
- SQL function seller.pub_exposable_stores(at) (objects.sql:52-65; declared db/pub.sql:19): `WHERE f.active AND f.not_pending_deletion AND f.onboarding_complete AND f.has_store_name AND f.has_logo AND f.has_town_code AND NOT EXISTS (effective_measures(at) ...)`. Used by: seller exposure.Classify via query ExposableStoreIDs (db/queries/exposure.sql:65-72), social module (internal/modules/social/internal/pubsql/seller.go, visibility/visibility.go, purge/purge.go, social.go, perf/measure.go) via `EXISTS (SELECT 1 FROM seller.pub_exposable_stores(...) ...)`, and the many social golden SQL files under internal/modules/social/internal/reads/testdata/golden.
- Query StoreRequirementFlags (db/queries/exposure.sql:56-62) -> stint.WithConditions -> model.Conditions.OnboardingComplete (stint.go:~274) -> model.met (model/exposure.go:111-112) -> MissingExposure / CapabilitiesOf (SHIPPING_SETUP + PUBLIC_LISTING need it; capability.go:82,84-87).
- Not found depending on it: other pub_* review views (pub_review_items etc.) read review_* functions, not the flags view.
- Stale comments (do not trust): model/exposure.go:83 says "KYC 流程已核准（含 PayUni 核准）" but SQL reads kyc_applications.status='APPROVED' + payout_capabilities.transfers_active. objects.sql:13 comment says "還沒取消的 KYC 申請" but the SQL has no cancelled filter beyond status='APPROVED' (a store has at most one row? not verified: join a.store_id, EXISTS handles multiples).
- Where Conditions.OnboardingComplete is read: backend/internal/modules/seller/internal/stint/stint.go:274 (set), internal/model/exposure.go:111-112 (read), model/model.go:194 (field), delegation test fixtures set it true.

## 6. UI strings: server-ownable vs field labels vs screen text (item 3)
Server-ownable sentences (status/title/CTA label/description that depend on derived state; candidates to move into RPC response, ideally as catalog keys/enums + params):
- task titles + descriptions: 商店資料/店名、logo 與所在地。; 平台審核/商家資料與統編的平台審核。; 收款帳戶/向金流商設定身分與銀行資料。; 其他公開條件/還有公開條件尚未滿足。; 物流設定/設定配送與退貨資訊。; 上架商品/建立第一個可購買商品。; 商店介紹/說明你賣什麼、產地與品牌故事。 (OT.ts, ot.dart); app-only 停權處分/帳號目前處於停權。; 商店狀態/商店目前不是營業中。
- web-only panelTitle/panelDescription (完成商店資料… 設定物流與退貨 …; OT.ts:123-258) - presentation of a detail panel; server-ownable if web keeps panel
- group labels 公開前必須/建議完成
- statusLabel strings with branching: 已完成/缺少：X、Y; 未送出; (catalog KYC); 平台審核通過後開放設定; (catalog PaymentAccountStatus badge); 待完成; 需先通過認證; 未營業; 已開通; 審核中; 待設定; 已上架; 待上架; app-only 停權中; 未營業 (storeActive); 未知 fallback
- CTA labels: 去設定; 前往商家認證; 查看物流設定/查看審核進度/設定物流; 新增商品; catalog action texts (前往設定收款帳戶/聯絡客服/重新整理); app-only 查看停權說明/查看說明
- already catalog-owned (shared ui-text package, enum-keyed): KYCStatus names, PaymentAccountStatus badge/description/action, SellerRequirement nouns, common.unknown, payouts-inactive note
Field labels / structural headings (client-owned): 平台審核 / 收款帳戶 section headings, 核准時間：/送出時間：/上次更新, 取消申請, 開啟設定中…, 重新整理 button on error, 載入中…, 正在確認資料…
Pure screen text (client-owned): 開店進度, 賣家開店任務, 任務完成, 展開任務/收合, "N/M", progress %, aria-label, wallet gate copy (certificationGate* constants are shared between wallet/shipping gate but screen-specific), paymentAccountEmbeddedNotice 請到網頁版完成收款設定 (platform capability: app has no embedded component), '無法載入認證狀態'.
Routing (hrefs/routes), icons, tone/color, locked/disabled rendering: client-owned, but server should supply a stable action enum (e.g. OPEN_PROFILE, OPEN_CERTIFICATION, OPEN_SHIPPING, OPEN_PRODUCT_CREATE, OPEN_SUSPENSION_NOTICE, CONTACT_SUPPORT, NONE) because href (/contact) vs wallet route differs per client.

## 7. Cross-client differences to resolve before server reproduction (summary)
1. platform_review.completed: web `paymentDone || KYC==APPROVED` (OT.ts:101) vs app KYC==APPROVED only (ot.dart:141).
2. payment_account.completed: web onboarding_complete flag (OT.ts:100) vs app GetMyPaymentAccount ENABLED (ot.dart:143); web payment account unused for completion.
3. read-failure semantics: app has reviewReadable/paymentReadable -> "未知" (ot.dart:114-117,144,165-167,183-185); web has none ("未送出"/"準備中").
4. payment CTA label + destination (see 1.3).
5. product count: ACTIVE-only total (web) vs any-status first-page length (app).
6. extra required rows notSuspended/storeActive only in app; web delegates to notice-page redirect.
7. denominators therefore differ (3-4 vs 3-5).
8. web persists collapsed/active selection; app does not (UI-only).
## 8. Uncertainties
- Gateway `/app/products` with/without `view=manage`: not verified how legacy handler defaults; web omits view, app sends view=manage.
- CANCELLED KYC: web comment says cancelled => undefined (OT.ts:278), but proto/catalog has KYC_STATUS_CANCELLED ("已取消"); if GetMyKYC can return a cancelled application, both clients would show "已取消" and not "未送出". Not verified in kyc.go.
- Whether a store can have multiple kyc_applications rows (EXISTS handles it); admin `isKycApproved` impl not read.
- payment_account_action.dart (app action kinds) and admin kyc-application.ts not read in full.
- Line numbers marked "~" in OnboardingProgressCard.tsx were counted from a piecewise view; verify before citing.
