# 決策簡報：賣家功能的前置條件閘（KYC／PayUni）由誰判斷

**決定（Will 2026-10-08）**：採 (c)，exposure 同一刀收成 `PUBLIC_LISTING`（c'）；落在 10c 第 3-0 刀。

基準：HEAD 04a6507f4 加上未 commit 的 seller-app 2-4。只讀調查，沒有改 repo。

## 結論（先講）

業界基準是**伺服器算「這個賣家現在能做什麼、每一項還缺什麼」，client 只照著畫，伺服器逐方法再擋一次**（Stripe Connect 的 capability＋requirements）。我們已經有這套骨架（`SellerCapability`＋gateway 依能力擋路由），只是它只看停權與生命週期，沒有看前置條件；前置條件被拆到 `exposure`（公開曝光）和各 client 自己的推導裡，所以才會有兩個來源互相矛盾。

建議 **(c)**：把 capability 擴成「每個能力帶狀態與缺少的條件」，新增細粒度能力「物流設定」，由 `CapabilitiesOf` 一併看 KYC 流程；gateway 的物流建店路由改宣告這個能力，伺服器就會擋。client 只看能力。2-4 現在把物流閘改成和「已認證」同一份 `GetMyStore` 回應（exposure），包在一個判斷函式裡，契約那一刀只換函式本體。

## 1. 現況：每個閘的 client 規則、伺服器規則、真正的外部約束

| 功能 | client 規則 | 伺服器規則 | 真正的外部約束 |
|---|---|---|---|
| 物流建店（7-11 C2C、黑貓） | website 頁面：`approved`＝exposure 不缺 `KYC_APPROVED`（`website/lib/seller/cert-gate.ts:1-10`、`ShippingClient.tsx:105,125-126`；但檔頭註解 `:53` 還寫「KYC 流程狀態不是 APPROVED」，已過時）；website 進度卡同樣用 exposure（`onboardingTasks.ts:189-190`）；App 2-4 用 `GetMyKYC.flow.status == APPROVED`（`onboarding_tasks.dart:216`、`seller_certification_gate.dart:28`） | **沒有 KYC 檢查**。gateway 只要求 `CAP_PRODUCT_MANAGE`（`backend/gateway/internal/router/routes_shipping.go:45-53`）。Java shipping-service 要求 `payuni_mer_id` 非空才送 PayUni（`OnboardingService.java:1050-1060`），但單體回的 MerID 一律由 store_code 組成、不看註冊有沒有成立（`backend/internal/legacy/sellerserver/server.go:174-185`），所以這個檢查形同虛設，最後是 PayUni 自己回錯 | **REAL**：PayUni 物流建店以 MerID 呼叫，前提是這個 MerID 在 PayUni 已是合作商店（`第三方整合/PayUni 物流官方 API 總覽.md:63-68`；錯誤碼 `AGENT02001` 查無商店物流資料、`AGENT02003` C2C 物流尚未開通，`PayUni 代理商專用 API 總覽.md:659-667`）。B2C 大宗寄倉明確要求會員層與商店層都核准（`詳細設計書/68-…:59-61`）。**未查證**：C2C／宅配是否在 PayUni 審核中（FORWARDED、UNDER_REVIEW，已有 MerID）就能建店，需要 sandbox 實測 |
| 建立商品 | B9 已拿掉 KYC 閘 | gateway `CAP_PRODUCT_MANAGE`；`can_manage_products` 不要求 KYC（`sellerserver/server.go:174-176`） | 無（ADR 2026-10-05：可曝光前可上架，買家看不到；`step10-decision-brief.md:155`） |
| 優惠券、發票 | 10b D3 決定拿掉，只看能力 | coupon、invoice 都不查 KYC（`step10b-decision-brief.md:108-111`） | 無（`step10b-decision-brief.md:125`） |
| 結帳（買家下單） | — | **有**：order-service 讀 `can_accept_orders`＝Exposable（`sellerserver/server.go:174,186`；`CheckoutService.java`） | REAL：錢進 PayUni 子商店，必須 PayUni 核准 |
| 錢包交易列表、提領 | website：已認證且 PayUni approved 才顯示（`website/lib/seller/wallet-transactions.ts:5-10`）；App 錢包整頁只在核准後顯示（`wallet_screen.dart:1938-1939`，K6） | 提領在 PayUni 端，平台沒有賣家提領路由（`rg -n -i "withdraw\|payout" backend/gateway/internal/router/*.go` 只命中 admin 與無關路由） | REAL：撥款需要 PayUni 核准 |

附帶發現：
- 10b D3 的「shipping 只用 `BatchGetSellerMeta` 取店名」不完整：它也檢查 MerID 非空，只是這個值永遠非空。
- 能力分類不一致：契約與單體說物流屬於 `ORDER_FULFILL`（`seller.proto:215-216`、`model/capability.go:20-21`），gateway 的物流建店卻掛 `CAP_PRODUCT_MANAGE`（`routes_shipping.go:45-53`）。

## 2. 契約現在提供什麼

- `SellerCapability`：14 個粗粒度能力，「client 以它決定畫面上哪些操作可用，伺服器仍會逐方法檢查」（`seller.proto:205-237`）。只有一個推導點 `model.CapabilitiesOf(status, suspended)`（`backend/internal/modules/seller/internal/model/capability.go:63-81`），檢查點 `Authorize`（`:89-101`）、Connect 的宣告（`backend/internal/httpapi/seller/declaration.go:22-38,174`）、legacy 路由靠單體送的 `X-Seller-Capabilities` header（`backend/internal/legacy/gatewayproxy/proxy.go:49,117-135`；gateway 只檢查成員，`backend/gateway/internal/authz/capability.go:23-26`）。
- 被拿掉的能力**不帶原因**；原因只有在呼叫被拒時以錯誤碼出現（停權 PermissionDenied＋`ReasonSellerSuspended`，等待刪除 FailedPrecondition，`capability.go:89-101`）。
- `Exposure`：「店家公開曝光的狀態」，`exposable`＋`missing_requirements`（`seller.proto:239-256,322-328`），唯一規則 `model.MissingExposure`（`model/exposure.go:54-91`）。這是目前**唯一**「帶缺少條件」的結構，所以 client 借它來判斷「已認證」。
- `KYCFlow.status`：給顯示用的細分流程狀態（`model/payuni.go:410-437`），APPROVED＝PayUni 核准。
- 沒有「每個能力的前置條件」這個概念。能力 ADR 當初就預留了第二個維度（tier）：`GrantedCapabilities(SellerContext)` 的簽章要能加維度，「能力層架構零改動」（`backend/docs/adr/2026-09-18-seller-capability-authorization.md:82-85`）。

## 3. 業界基準（從零設計會怎麼做）

- **Stripe Connect**（確定）：Account 有 `capabilities`（例如 `card_payments`、`transfers`），每項狀態 `active`／`inactive`／`pending`；`requirements` 有 `currently_due`、`eventually_due`、`past_due`、`pending_verification`、`disabled_reason`、`errors`；每個 Capability 物件也有自己的 `requirements`。平台依 capability 狀態與 `charges_enabled`／`payouts_enabled` 開放功能，Stripe 伺服器對 API 呼叫再擋一次。
- **Adyen for Platforms**（確定）：account holder 的 `capabilities` 每項有 `allowed`、`enabled`、`verificationStatus`（valid／pending／invalid／rejected）與 `problems`（`verificationErrors`，含要補的東西）。
- **Shopify**（部分推測）：Admin API 的 `shop.features` 由伺服器回傳商店能用的功能；Payments 啟用前就能建商品、設折扣（10b 已引用）。後台 setup guide 由伺服器驅動是推測。
- **Amazon Seller Central**（推測）：Account Health 與類別上架限制都由伺服器判定、逐項列出缺的文件。

共同原則：**一個伺服器推導點產出「能力＋每項缺什麼」，client 只渲染，伺服器逐方法擋**。client 自己從原始狀態拼閘不是業界做法。

## 4. 現有機制為什麼存在

- 公司時期的 client 讀 REST 的狀態字串（`overallCertificationStatus` 等，`wallet_screen.dart:82,117-133`）自己推導閘；伺服器沒有對應規則（10b D3 查證）。
- `Exposure` 是為「買家看得到店面、可以結帳」設計的（ADR 2026-10-03、2026-10-05），剛好含 `KYC_APPROVED`，所以 website 10b 借它當「已認證」的來源（K5）。它的語意是公開曝光，不是「能不能設定物流」。
- `SellerCapability` 是為停權白名單設計的（能力 ADR、設計書 46），只有兩個維度：生命週期與停權。

## 5. 選項

| 選項 | 內容 | 取捨 |
|---|---|---|
| (a) client 從 exposure 推導 | App 改成和 website 一樣看 `missing_requirements` 不含 `KYC_APPROVED` | 最快，一份回應內不會自相矛盾。但規則仍在兩個 client 各寫一次；伺服器不擋；把「公開曝光條件」挪作「物流前置條件」，哪天曝光條件改了（例如加「至少一個商品」）就會連帶影響物流；第三個 client 還要再抄一次 |
| (b) 能力依前置條件授予 | `CapabilitiesOf(status, suspended, flow)`：新增 `SHIPPING_SETUP`，KYC 不是 APPROVED 時不授予；gateway 物流建店路由改宣告它 | 一個推導點、伺服器會擋。缺點：client 拿到「沒有這個能力」卻不知道為什麼，只好再讀 `GetMyKYC` 或 exposure 來決定文案，又回到兩個來源 |
| **(c) (b)＋每個能力帶狀態與缺的條件（Stripe 形）** | `GetMyStoreResponse` 的 `capabilities` 改成 `repeated CapabilityStatus { capability; bool active; repeated SellerRequirement missing; }`，`SellerRequirement` 合併現在的 `ExposureRequirement` 並加上 `NOT_PENDING_DELETION` 等原因；停權、等待刪除、KYC 都成為「缺的條件」 | 一次回應就能畫出閘與原因，client 只看能力；伺服器以同一個函式擋；以後新增前置條件（例如 B3 的物流商核准、tier）只改伺服器。代價：契約改形（v1alpha1、未上線，可以改），兩個 client 改讀法，gateway proxy 每個賣家請求多讀 KYC 申請與 PayUni 註冊 |
| (c') 同 (c)，再把 exposure 收成能力 `PUBLIC_LISTING` | exposure 也變成一個 `CapabilityStatus` | 模型最一致；但 exposure 還服務買家端清單、結帳與公告時間（`model/exposure.go:20-27`），那些內部用法不變，只是契約少一個欄位。可以和 (c) 同一刀，也可以晚一步 |

## 6. 約束帳

| 約束 | 推離基準的方向 | 標記 | 來源 |
|---|---|---|---|
| PayUni 物流建店需要 MerID 已是 PayUni 合作商店 | 決定「物流設定」的前置條件內容，不推離基準 | REAL（第三方） | 物流 API 總覽 `:63-68`；代理商 API 總覽 `:659-667` |
| 不知道 C2C／宅配能否在 PayUni 審核中建店 | 前置條件暫定 APPROVED（較嚴） | REAL（缺資料），sandbox 實測後可放寬為「已註冊」 | 同上；68 號 `:59-61` 只講 B2C |
| shipping-service 是 Java，B3 fulfillment 才搬進單體 | 不推離：伺服器端的擋在 gateway 宣告與單體送的 header，兩者都在我們手上，不必動 Java | 非約束 | `routes_shipping.go:45-53`；`proxy.go:117-135`；計畫 `2026-09-24-go-modular-monolith.md:243` |
| 提領、結帳要 PayUni 核准 | 不推離（伺服器已擋結帳） | REAL | `sellerserver/server.go:174-186` |
| 2-4 已寫好、第 3 刀的切法 | 推向 (a) | SELF-IMPOSED（計畫分段） | `step10c-decision-brief.md:239-241` |
| 改契約與兩端的工作量、與 website 現況一致 | 推向 (a) | SELF-IMPOSED（工作量、一致性；AGENTS.md 明定一致性不能單獨限縮選項） | 根 `AGENTS.md`「上線前不受相容性約束」 |
| proxy 每次多兩個查詢 | 推向 (a)/(b) | SELF-IMPOSED（效能取捨，`GetMyStore` 已為 exposure 讀同樣的資料，`profile.go:257-269`） | — |

沒有 REAL 約束推離基準，所以建議就是基準。

## 7. 建議：(c)＋(c')（已採用）

**終點**：伺服器一個函式（`CapabilitiesOf` 擴充後）產出每個能力的狀態與缺的條件；Connect 宣告與 gateway 都以它擋；client 只讀 `GetMyStore` 的能力狀態決定開關與文案，`GetMyKYC.flow` 只用來顯示 KYC 頁的細分進度。
**觸發**：
- PayUni sandbox 確認 C2C／宅配可在審核中建店 → `SHIPPING_SETUP` 的條件從 KYC_APPROVED 放寬成「PayUni 已註冊」，只改伺服器。
- B3 fulfillment 搬進單體 → 物流 Connect 方法宣告 `SHIPPING_SETUP`，刪 gateway 路由。
- B7 coupon、B10 finance → 各自的前置條件（如果有）同樣加進能力，不在 client。

## 8. 變更清單

**契約（`api/hawkr/seller/v1alpha1/seller.proto`）**
- 新增 `SELLER_CAPABILITY_SHIPPING_SETUP`（物流建店、開關配送方式）；`ORDER_FULFILL` 的註解刪掉「物流」設定的部分，只留出貨。
- 新增 `SellerRequirement`（STORE_ACTIVE、NOT_SUSPENDED、NOT_PENDING_DELETION、KYC_APPROVED、STORE_NAME、LOGO、TOWN_CODE）與 `CapabilityStatus`；`GetMyStoreResponse.capabilities` 改成 `repeated CapabilityStatus`。（c'）再把 `exposure` 換成 `PUBLIC_LISTING` 能力。
- 重新生成 TS、Dart、Go。

**backend**
- `model.CapabilitiesOf` 加入 KYC 流程輸入，回傳每個能力的狀態與缺的條件；`Authorize` 對前置條件造成的拒絕回 FailedPrecondition＋新的 `ErrorReason`（例如 `SELLER_REQUIREMENT_MISSING`，ErrorInfo metadata 帶缺的條件）。
- `gatewayproxy.currentStoreOf` 讀 KYC 流程（沿用 `exposure.KYCFlows`），header 只放 active 的能力。
- gateway：`authz/capability.go` 加 `CapShippingSetup`；`routes_shipping.go:45,47,53` 的建店與開關改宣告它，GET 保留可讀；`TestCapabilityTokensMatchGateway` 跟著改。
- 測試：`TestCapabilitiesOf` 加 KYC 維度；物流建店在未核准時被 gateway 擋（先紅後綠）。
- 更新 seller 模組 AGENTS.md:12,60、能力 ADR 修訂（加前置條件維度）。

**website**
- `lib/seller/cert-gate.ts` 的 `approved` 與 `ShippingClient.tsx` 改看 `SHIPPING_SETUP` 能力；`onboardingTasks.ts:189-190` 的 `shippingOpen` 同樣；`isStoreCertified` 只留給「已認證」的顯示（或 c' 時改讀 `PUBLIC_LISTING` 缺的條件）；修正 `ShippingClient.tsx:53` 過時註解。

**seller-app**
- `onboarding_tasks.dart:216`、`seller_certification_gate.dart:28`、`profile_screen.dart` 的 `shippingUnlocked` 改看 `SHIPPING_SETUP`；`my_store.dart` 加 `can(capability)` 與 `missingFor(capability)`。

**K5／K6 的處置**
- K5（已認證看 exposure、細分看 GetMyKYC）：保留；c' 時改成看 `PUBLIC_LISTING` 的缺項。
- K6 的「物流閘由 `flow.status == APPROVED` 推導」：作廢，改成「物流閘看 `SHIPPING_SETUP` 能力」。

**落在哪一刀**：契約＋backend＋gateway＋兩端讀法放一刀（建議在第 3 刀之前，命名 3-0），因為第 3 刀要動錢包與物流設定頁，先有能力就不必再寫一次 client 推導。

## 9. 2-4 未 commit 的程式現在怎麼做（不丟掉東西）

- 把物流閘集中成一個判斷（例如 `MyStore.shippingSetupAllowed`），2-4 的本體先用 **exposure 不缺 `KYC_APPROVED`**——和進度卡「KYC 已通過」讀同一份 `GetMyStore` 回應，就不會再出現「已通過卻要先認證」；`onboarding_tasks.dart:216`、`seller_certification_gate.dart:28`、`profile_screen.dart` 都呼叫它。
- `GetMyKYC`（`current_kyc.dart`、`kyc_flow.dart` 的文案表）保留，只用於細分文案；讀不到時文案退回通用的「尚未通過商家認證」，不影響開關。
- 3-0 只改這個判斷的本體成 `can(SHIPPING_SETUP)`，其他程式與測試都留著。
