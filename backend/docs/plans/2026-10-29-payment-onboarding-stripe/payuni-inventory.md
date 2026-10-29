# PayUni 依賴盤點（Stripe Connect 換掉 PayUni 的遷移計畫輸入）

快照 commit：`e865c68`（`git -C <repo> rev-parse --short HEAD`），repo `/Users/will/second-brain/.projects/personal/hawkr`。唯讀盤點，沒跑 e2e、沒打任何 API。
「去向」欄一律 `TBD`（規劃者填），「建議」欄是盤點者的建議。

## 0. 先讀這幾點

1. PayUni 在這個 repo 有三個不同角色，不是只有「金流」：(a) 賣家子商店（sub-merchant，MerID = `HAWK`+store_code）的開通與審核；(b) 買家付款／退款／撥款（UPP 信用卡、Apple Pay、Google Pay、ATM）；(c) **物流**（超商、黑貓宅配的建單、列印、貨態 webhook）。Stripe Connect 只覆蓋 (a)(b)；(c) 沒有替代品，但它和 (a)(b) 共用同一個 MerID 與同一支 webhook 服務。
2. 單體（live、新）只有 (a)：`backend/internal/modules/seller`（子商店綁定／同步／觀測）。(b)(c) 全在 Java legacy（`backend/services/java/`），階段 B 的 B2 payment、B3 fulfillment、B8 ordering、B9 aftersales、B10 finance 才會搬進單體；現在單體裡沒有任何買家付款程式。
3. 「legacy 有沒有部署」：沒有。`docs/plans/2026-09-29-a3-buyer-seller.md` 前提寫「gateway 與舊服務不會部署（A2 前提），單體→gateway 的身分 header 只存在於本機與 CI」，根 `AGENTS.md`「系統從未上線」。legacy 是本機（`dev.sh`、`docker-compose-local.yml`）與 CI（`backend-java-tests.yml`、`backend-integration-tests.yml`）才跑的程式，但單體仍經 `internal/legacy` 和 gateway :9900 呼叫它，所以是「活的依賴」不是死碼。
4. 重複實作三份 PayUni client：Go 單體 `internal/platform/payuni`、Go legacy `backend/pkg/payuni`（seller-service 與 gateway 用）、Java `shared/payuni-java-client`。加密（AES-256-GCM／雜湊簽章）各自實作。
5. 資料列上 PayUni 名稱已部分中性化：Java 表欄用 `provider_*`（`provider_trade_no`、`provider_status`、`provider_refund_no`），但 `grant_*`、`mer_id`、`payuni_audit_logs`、aftersales `payuni_status`／`payuni_message` 仍是 PayUni 語意。proto 註解裡寫死 PayUni 的欄位很多（見 §3）。
6. 噪音：108 個檔案（165 行）出現的是舊 repo 名 `backend-payuni`，不是 provider（README、admin、`.agents` 註解、Makefile 提示等）。下面的計數都已排除（pattern `(?<!backend-)pay_?uni`）。
7. 單體的 `store_requirement_flags.kyc_approved` view 把 PayUni `outcome='APPROVED' AND bind_status='BOUND'` 併進 KYC 核准：它決定店家可曝光、可收單、可建物流（`CapShippingSetup`、`CapOrderFulfill`）。換成 Stripe 時這個閘門的語意要重寫，不是只換 client。
8. 賣家入駐同意書有一種叫 `PAYUNI_TERMS`（proto enum `AGREEMENT_KIND_PAYUNI_TERMS`、DB CHECK `agreement_acceptances_kind_check`、website legal manifest），計畫已有「依代理商契約更正 PAYUNI_TERMS」待辦（`go-modular-monolith.md` L322）。

## 1. 計數與產生它們的指令

全部在 repo 根目錄執行。

| 指令 | 結果 |
|---|---|
| `git grep -Il -i payuni \| wc -l` | 1349 個 tracked 檔含 payuni（含舊 repo 名噪音） |
| `git -c core.quotepath=off grep -IlP -i "(?<!backend-)pay_?uni"` | **1261** 個檔（排除 `backend-payuni`）；其中 `rg -c` 非測試檔 961 |
| 同上，依路徑 grep -E 分層（括號為其中測試檔數） | 見下表 |
| `rg -il payuni`（不走 git，含 build 產物 `target/`）| 比 git grep 多（Java `target/classes` 有生成物，不要用 rg 全域數；全域 rg 還會超過 2 分鐘） |

| 層 | 檔數 (測試) | 說明 |
|---|---|---|
| 單體 `backend/internal/modules/seller` | 101 (31) | 52 在 `internal/`、19 在 `db/` |
| 單體其他模組 reviewqueue／social／discovery／content／buyer | 7 (2) | 多為註解與 CARRYOVER |
| 單體 `internal/platform` | 10 (2) | `payuni/` 4、`uitext` 4、`adminperm`、`codemeta` |
| 單體 `internal/httpapi`（含 gen） | 22 (9) | |
| 單體 `internal/legacy` | 12 (5) | sellerrelay、sellerconsumer、sellerserver、gen |
| 單體 `internal/arch` + `cmd/hawkr` | 1 + 7 | 設定、事件接線、dev seed |
| **單體小計** | **約 160** | |
| Java `backend/services/java` | 421 (106) | payment 109、shared 122（payuni-java-client、common-core）、order 44、shipping 85、aftersales 32、payuni-webhook 24 |
| Go legacy `seller-service` | 151 (59) | |
| 其他 Go legacy（ai、invoice、notification、crm、im、review、social） | 18 (1) | 多為註解／舊 repo 名／通知模板 SQL |
| `backend/gateway` | 40 (11) | |
| `backend/pkg`（`payuni` 4、`proto` 生成 9、codemeta／config／errors 等 5） | 18 | |
| `backend/api/proto` + `api/openapi` | 6 + 1 | |
| backend infra／monitoring／shared／scripts／Makefile／compose／.env.template | 17 | |
| `backend/docs` | 156 | |
| `api/`（Connect protos） | 5 | admin seller、seller、reviewqueue、discovery、common store_card |
| `packages/` | 25 (6) | api-ts 5、hawkr_api(dart) 9、ui-text-ts 7、ui_text(dart) 4 |
| website | 107 (30) | lib 55、features 38、tests 18、docs 5 |
| admin | 19 (7) | |
| buyer-app | 28 (1) | |
| seller-app | 89 (26) | |
| `.github/workflows` | **0** | 沒有任何 workflow 提到 PayUni；間接影響見 §9 |

其他計數：

- 環境變數名 `rg -oIN "\bPAYUNI_[A-Z0-9_]+"`：設定層（env）名稱見 §8；其餘 `PAYUNI_*` 是 enum 值或錯誤碼常數。
- River job kind（單體）：4 個 `seller.payuni_sync`／`_poll`／`_observe`／`_sweep`（`rg -n "Kind\(\) string" backend/internal/modules/seller/internal/payuni/jobs.go`）；另 legacy relay 5 個 `legacy.seller_payuni_*`。
- 單體 sqlc 查詢 `db/queries/payuni.sql` 17 個 named query；seller migration 中 10 個提到 payuni（`db/migrations/2026100214*_payuni.sql` 起）。
- Connect proto 識別字（`rg -oiI "[A-Za-z_]*payuni[A-Za-z_]*" api --glob '*.proto'`）：`PayUni*` 訊息／enum 約 30 種、4 個 admin RPC、1 個 review queue kind、1 個 agreement kind。
- 外部系統登記：`backend/docs/plans/2026-09-24-go-modular-monolith/external-systems.csv` 有 5 列 PayUni（gateway、seller-service、payment-service、shipping-service、payuni-webhook-service）；`events.csv` 有 9 列 `payuni.*`／`seller.review.result`。

## 2. 能力群組總表

欄位：現況在哪（單體 live ／ Java legacy ／ Go legacy）、檔案、公開契約、表、消費者、測試、建議、去向。

### G1 子商店開通／綁定（sub-merchant onboarding & binding）

| 項 | 內容 |
|---|---|
| 現況 | **單體 live**（A3 seller 已搬）；Go legacy `seller-service` 同功能舊版（outbox 模式，不會部署） |
| 指令 | `rg -il payuni backend/internal/modules/seller`、`rg -il payuni backend/services/seller-service` |
| 單體檔 | `internal/modules/seller/payuni.go`（公開面）、`internal/payuni/{intent,sync,jobs,observe,storedata,convert,payuni}.go`、`internal/kyc/payuni.go`、`internal/model/payuni*.go`、`internal/model/kyc_registration.go`；外部呼叫 `internal/platform/payuni/{client,crypto,limiter}.go`；接線 `cmd/hawkr/seller.go`（`readPayUni`）、`cmd/hawkr/seller_commands.go` |
| 外部 API | PayUni 代理商 API：`bind_merchant`、`update_merchant`、`search_merchant_status`（見 `backend/docs/第三方整合/PayUni 代理商專用 API 總覽.md`，938 行） |
| 送出的資料 | `BindMerchantRequest`：Name、UBN、Email、MerID、MerEmail、MerName、MerEngName、MerURL、MerAddr(+Post)、MerDesc、ContactName/Mobile/Tel、MerTel；企業另帶 MBC*、MC*（負責人姓名／身分證）；固定參數 MerType、MerMCC、GoodsRate、Payment、DeliverTime、**EscrowType**、MerUnitPrice、SellType（`sync.go` 的常數）。**KYC 解密資料（真實姓名、身分證號、統編）經 `KYC.BindingData` 在 transaction 外轉給 PayUni** |
| 契約 | `api/hawkr/admin/seller/v1alpha1/seller.proto`：`GetPayUniRegistration`、`RebindPayUni`、`ListPayUniObservations`、`RefreshPayUniStatus`；`api/hawkr/seller/v1alpha1/seller.proto`：`KYCApplication.payuni`（`SellerPayUni`）、`PayUniState` enum（8 值）、`KYCFlowStatus` 的 PAYUNI_* 值、`KYCResubmitUnavailableReason` 7 個 PAYUNI_* 值、`StoreActionUnavailableReason` 3 個 PAYUNI_* 值；`reviewqueue` 的 `REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY`；`discovery`／`common/store_card` 只在註解 |
| 表 | `seller.payuni_registrations`（PK `store_id`；CHECK：`mer_id ~ '^HAWK[0-9]{1,10}$'`、`bind_status IN (PENDING,BOUND,FAILED)`、`bind_failed_at`、`bound_at`、`intent_seq/synced_seq`、`outcome IN (...)`、`purged`）、`seller.payuni_observations`（`source IN (WEBHOOK,POLL,REFRESH,BIND)`、`(store_id,event_key)` 唯一）；view `seller.store_requirement_flags`（`objects.sql`）；函式 `seller.review_payuni_bind_retry`；`agreement_acceptances_kind_check` 含 `PAYUNI_TERMS` |
| 事件 | `PayUniForwarded/Approved/PendingInfo/UnderReview/Rejected/BindFailed`（`seller/events.go`，同 transaction 內發）→ `internal/legacy/sellerrelay` 轉成 RabbitMQ `merchant.verification.*`（notification-service 消費）；`cmd/hawkr/seller_events.go` 訂閱 |
| 權限 | `adminperm.SellerPayUni{RegistrationsGet,RegistrationsRebind,RegistrationsRefresh,ObservationsList}`（`adminauth.go`） |
| 消費者 | website／admin／seller-app（見 §6）；Java `order-service` 與 `shipping`／`aftersales` 經 legacy gRPC 取 `payuni_mer_id`（`internal/legacy/sellerserver/server.go:187` 由 `FormatPayUniMerID(StoreCode)` 組出，`GetSellerMeta`）|
| 測試 | 單體 31 個測試檔（`payuni_integration_test.go`、`payuni_state_integration_test.go`、`internal/payuni/*_test.go`、`internal/model/payuni*_test.go`、`platform/payuni/client_test.go`）、`internal/legacy/sellerserver/server_integration_test.go`（`approvingPayUni` 替身）、`cmd/hawkr/payuni_config_test.go`、`httpapi/seller/admin_kyc_test.go`、`httpapi/reviewqueue/review_queue_test.go` |
| 文件 | `backend/docs/modules/seller.md` §13–14（69 處）、ADR `2026-10-19-seller-kyc-payuni-consolidated.md`、`2026-10-02-seller-kyc-payuni-in-monolith.md`、seller `CARRYOVER.md`、`AGENTS.md` L40 |
| 建議 | 這是替換的主戰場。Stripe Connect 對應物是 Connected Account（Express／Custom）＋ Account Link onboarding；MerID 規則（`HAWK`+store_code）、`bind_merchant` 的欄位組裝、2 QPS 限流、AGENT04017 認領流程都是 PayUni 特有，不要搬。保留的概念：一店一列的 provider 註冊、意圖序號（`intent_seq`）、觀測紀錄、sweep 對帳。表名改 provider-中性（上線前不受相容性約束）。要決定：KYC 資料是否還需要自己蒐集後轉送（Stripe Express 由 Stripe 蒐集）— 這會影響 `seller.kyc_*` 的 PII 範圍與 ADR `2026-07-20-seller-kyc-pii-encryption-scope` |
| 去向 | TBD |

### G2 KYC 資料轉送（KYC data forwarded）

| 項 | 內容 |
|---|---|
| 現況 | 單體 live |
| 檔案 | `internal/payuni/sync.go` `bindRequest`（讀 `KYC.BindingData`、`KYC.CompanyPhone`）、`internal/kyc/payuni.go`、`internal/kyc/identity.go`、`internal/model/kyc*.go`（欄位對映 `KYCFieldNames`／`StoreFieldNames` ↔ PayUni 錯誤碼）、`payuni/storedata.go`（`StoreData`／`MissingStoreFields`／`SyncedFieldsChanged`） |
| 表 | `seller.stores` 的 `contact_name`、`contact_mobile`、`support_email`、`contact_phone`、`town_code`、`address_detail`（因 PayUni 必填而加，見 migration `20261007072959_seller_contact_to_store.sql`）；`seller.kyc_*`（加密 PII） |
| 契約 | `seller.proto` 的 `StoreContact`（欄位註解寫 PayUni 的 MerTel／ContactName／ContactMobile／MerEmail）、`SubmitKYC` 的 failed_precondition 文字、`KYCApplication.missing_store_fields`、`PayUniProblem.store_fields` |
| 鎖 | 身分欄位鎖：註冊有 MerID 且 PayUni 已收下後，店名、介紹、KYC 身分欄位不能改（`IdentityLocked`／`CheckNameUnlocked`；seller.md §14），原因是 PayUni 規格 `update_merchant` 不可改核心身分 |
| 測試 | `kyc_*_integration_test.go`、`storedata_test.go`、website `payuni-locked-fields.unit.test.ts`、seller-app 對應 test |
| 建議 | 若 Stripe 自己蒐集身分文件，`bindRequest`、身分鎖、`StoreData` 必填欄位檢查（PayUni 必填的地址／聯絡人）都可退場；店家聯絡欄位（contact_name 等）只在 PayUni 才強制，需重新評估是否保留為必填 |
| 去向 | TBD |

### G3 狀態同步與 remedy（status sync / remedy）

| 項 | 內容 |
|---|---|
| 現況 | 單體 live（webhook 經 Java 轉入）；Go legacy 舊版 |
| 檔案 | `internal/payuni/observe.go`（Receive／Observe／Poll／Refresh／Sweep）、`internal/model/payuni.go`（`PayUniOutcomeOf`、`ClassifyPayUniCode` 錯誤碼目錄、`ProblemOf`）、`internal/model/payuni_state.go`（賣家看到的 8 態與 remedy 類別）、`internal/model/kyc_resubmit.go`、`internal/legacy/sellerconsumer/consumer.go`（RabbitMQ `seller.review.result` → `ReceivePayUniNotification`） |
| 狀態對映 | `(MerStatus, MemberStatus)` → outcome（NONE／UNDER_REVIEW／PENDING_INFO／APPROVED／REJECTED／UNRECOGNIZED），終止碼 `2/8/92/99`、`91`（seller.md §14.1 表）；錯誤碼 `AGENTxxxxx` 目錄 `internal/model/payuni.g.go`（生成） |
| Job | `seller.payuni_sweep` 每 15 分鐘、`seller.payuni_poll`（已綁定且 outcome 未定論且 >2h 沒問）、`seller.payuni_observe`（webhook 套用，event_key 唯一）、`seller.payuni_sync` |
| 契約 | `PayUniObservationSource` enum、`PayUniStatusCodes`、`PayUniBindStatus` enum（含 `PAYUNI_BIND_IN_PROGRESS`）、`ErrorReason` 含 PAYUNI_*（`rg -n "PAYUNI_" api`）|
| 文案 | `internal/platform/uitext/catalog/{zh-TW,en}.json` → `catalog_gen.go`（`SellerPayUniState`、`SellerSellerPayUniFixFieldsHeading`、`...ReasonHeading`），TS `@hawkr/ui-text`（`sellerPayUniState` key、golden）、Dart `hawkr_ui_text`；生成物 CI：`ui-text-generated-diff.yml`、`ui-text-guard.yml` |
| 客戶端 remedy | website `lib/seller/payuni.ts`、`kyc-flow.ts`、`payuni-locked-fields.ts`、`features/seller-center/components/PayuniStatusBody.tsx`、`wallet/WalletClient.tsx`（錢包頁內嵌 PayUni 狀態，獨立的 `/seller-center/me/payuni` 頁已退役）、`profile/certifications/CertificationsClient.tsx`、`dashboard/DashboardPage.tsx`（onboarding 任務）；seller-app `features/seller/data/{payuni,kyc_flow,kyc_wizard,onboarding_tasks,seller_status_refresh}.dart`、`presentation/widgets/payuni_status_body.dart`、`seller_certification_gate.dart`；admin `src/app/admin/merchants/[id]/{PayUniCards.tsx,page.tsx,kyc-application.ts,use-store-detail.ts}`、`src/lib/unavailability.ts` |
| 測試 | website `lib/seller/payuni.unit.test.ts`（49 處）、`kyc-flow.unit.test.ts`、`CertificationsClient.unit.test.ts`（51）、`WalletClient.unit.test.ts`；admin `tests/merchant-kyc-application.test.ts`（79）、`unavailability.test.ts`；seller-app `test/features/seller/*`（13 檔）、`test/features/wallet/*`；e2e（mock，不跑）`website/tests/e2e/helpers/mock-gateway.ts`、`seller-dashboard-onboarding-actions.spec.ts` |
| 建議 | 狀態機是 PayUni 特有（MerStatus／MemberStatus 兩軸、「會員層要賣家自己去 PayUni 後台驗 email」）。Stripe 是 `charges_enabled`／`payouts_enabled`／`requirements.currently_due`／`disabled_reason`，改成 provider-中性的「收款資格」狀態，remedy 由 Account Link 導向。`PayUniState` 8 態、uitext 目錄、三個 client 的 remedy UI 全部要重寫；shared catalog 與 enum manifest 守門（`check-enum-text`）會擋掉遺漏 |
| 去向 | TBD |

### G4 付款建立與方式（payment creation & methods）

| 項 | 內容 |
|---|---|
| 現況 | **只在 Java legacy**（B2 payment 尚未開始）；gateway 轉發 |
| 指令 | `rg -il payuni backend/services/java/payment-service`、`rg -n "createCreditCardPayment\|createApplePayPayment\|createGooglePayPayment\|createAtmPayment" backend/services/java` |
| Java | `payment-service/.../application/service/PaymentApplicationService.java`（`createPayment`、`handleCreditCardPayment`／`ApplePay`／`GooglePay`／`Atm`、`syncPaymentStatus`、`closePaymentByOrderNo`、零元訂單不走 PayUni）、`PaymentCloseService`、`DuplicatePaymentService`、`AtmReactivationService`、`PaymentPageService`、`PaymentRedirectFormStore`、`ApplePayMerchantValidator`、`adapter/http/PaymentReturnController`、`adapter/grpc/PaymentGrpcService`、`config/{PayUniConfig,ApplePayProperties}`、`constant/PayUniConstants`；client：`shared/payuni-java-client/.../payment/{PaymentClient,PayUniPaymentEndpoint}`、`crypto/PayUniCryptoService`、`http/PayUniHttpClient`；限額 `shared/common-core/.../PaymentMethodLimitRules.java`（信用卡 1~199,999、ATM 15~49,999，PayUni 業務確認）；`PaymentMethod` enum（2001 信用卡、2002 Apple Pay、2003 Google Pay、2004 ATM） |
| 流程 | 信用卡走 UPP 表單 POST 到 PayUni（`UppFormData`）→ 瀏覽器 auto-submit 中轉頁（gateway `handler/payuni_redirect.go`，host allowlist）→ PayUni `ReturnURL` POST 回 gateway `/payment/return` → payment gRPC 解密。Apple／Google Pay 走 PayUni 幕後 API，token 由 client 原樣送、後端 base64（`website/lib/payment/{apple-pay,google-pay}.ts`、`buyer-app/lib/core/payment/pay_token_service.dart`）。多賣家合單走代理商 MerID（`consolidated`＋`PAYUNI_AGENT_MER_ID`）|
| 契約（REST，legacy） | `backend/api/openapi/openapi.yaml`：`POST /payments`、`GET /payments/{tradeNo}`、`.../history`、`POST .../status_syncs`、`PATCH /payments/{tradeNo}`（close）、`/payments/apple_pay/validate_merchant`、`/payment/return`、`/payment/redirect`、`/store_map/*`；admin `/admin/payment_transactions`、`/admin/payment_refunds`、`/admin/grants`；`gateway/internal/router/routes_payment.go`、`constants.go` |
| 契約（gRPC） | `backend/api/proto/payment/payment.proto`（1753 行；`DecryptPayUniResult` RPC、`action_url=https://api.payuni.com.tw/api/upp`、`mer_id`、`grant_status/grant_message`）、`order/order.proto`（`payment_mer_id`、`consolidated`、`mer_id`、`need_refund_page`、付款方式可用性區間）、`seller` proto 的 `payuni_mer_id` |
| 表（payment DB） | `payment_transactions`（`trade_no`=MerTradeNo、`mer_id`、`provider_trade_no`、`provider_status`、`granted_at`、`grant_status`、`provider_create_at/paid_at`、`rescued_at`）、`payment_refunds`、`payment_webhooks`（`webhook_type` 預設 `'8241'` = `WebhookProvider.PAYUNI`）、`payment_logs`、`payuni_audit_logs`（payment V3、shipping V5 各一張）、`outbox_events`；order DB `orders.mer_id`、`orders.need_refund_page`、`payment_event_dedup` |
| 客戶端 | website `features/checkout/screens/{CheckoutClient,CheckoutFailClient}.tsx`（「PayUni 安全付款」文案）、`features/payments/screens/{PaymentLandingClient,PaymentReturnClient}.tsx`、`lib/payments/{status,usePaymentResolution}.ts`、`lib/checkout/repo.ts`、`lib/orders/{repo,types}.ts`（`paymentMerId`）、`lib/payment/*`、Apple domain association（`lib/payment/applePayDomainAssociation.ts`）、`tests/native-pay-apple-routing.test.ts`；buyer-app `core/payment/pay_token_service.dart`、`features/order/*`（14 檔）、`config/flavor.dart`（測試／正式環境各綁不同 PayUni）|
| 測試 | Java：`payment-service` 測試 + `payuni-java-client` `PaymentClientRequiredFieldsTest`、`PayUniCryptoServiceTest`；gateway `payuni_redirect_test.go`（34）；website 見上 |
| 建議 | 這是 Stripe 取代的第二戰場，但**現況不在單體，也未排進近期階段**。規劃要先決定：是在 Java 內加 Stripe adapter（違反 B2 要改寫的方向），還是等 B2 payment 搬進單體時直接寫 Go 的 payment port。建議把「payment port」介面寫在 B2，PayUni 實作留在 Java 到 B8 為止或直接不搬。Apple Pay／Google Pay 的 gateway merchant id 流程（`gatewayMerchantId` = PayUni MerID，每賣家動態）在 Stripe 是 PaymentRequest／PaymentElement，client 要重寫；ATM 虛擬帳號在 Stripe 台灣沒有等價（需確認 bank_transfer／OXXO 之類支援；這是 Stripe 產品問題，未驗證）|
| 去向 | TBD |

### G5 通知／webhook 驗證（notify / webhook verify）

| 項 | 內容 |
|---|---|
| 現況 | Java legacy 服務 `payuni-webhook-service`（單一入口）；單體只消費 `seller.review.result` |
| 檔案 | `payuni-webhook-service/.../adapter/http/{PayUniWebhookController,WebhookSimulatorController}.java`、`service/{WebhookReceiveService,WebhookInboxTxOps,WebhookOutboxTxOps,FailedInboxRecorder,EventKeyBuilder}.java`、`infrastructure/mq/{PayUniEventRouting,OutboxPublisher...}`、`config/{PayUniConfig,RabbitMqConfig}`；驗證解析 `shared/payuni-java-client/.../webhook/{PayUniWebhookParser,WebhookType,event/*}`（7 種事件：PaymentUopEvent、RefundNotifyEvent、RefundAcceptedEvent、ShippingStatusEvent、ShippingPrintEvent、AgentNotifyEvent、PayUniWebhookEvent）；gateway `routes_payment.go`（`POST /webhook/payuni/notify` 反向代理，**刻意維持 502**，因為 PayUni 5xx＝重送）、`routes_webhook_simulator.go` |
| 契約 | 入站 HTTP `/webhooks/payuni/notify`（EncryptInfo／HashInfo）；內部 RabbitMQ exchange `payuni.events`，routing key `payuni.payment.uop`、`payuni.refund.notify`、`payuni.refund.accepted`、`payuni.shipping.status`、`payuni.unknown.fallback`、`payuni.payment.recurring`（`events.csv` L169–179）、`seller.review.result` |
| 表 | `webhook_inbox`、`outbox_events`（payuni-webhook-service V1、V2 `relax_webhook_inbox_for_multi_provider`）；payment `payment_webhooks`；shipping `shipping_webhook_dedup` |
| 同服務夾帶 | Uber webhook 切片（`webhook/uber/*`）與 PayUni 同服務；計畫 B3 要刪（`2026-10-27 ecommerce-scope-cut`），刪的時機影響此服務去留 |
| 設計 | ADR `2026-09-18-webhook-two-phase-transaction.md`、`2026-09-12-inbound-payload-raw-bytes.md`、設計書 10、11 |
| 測試 | `payuni-webhook-service` 的 `WebhookInboxDedupIntegrationTest`、`EventKeyBuilderTest`、`OutboxPublisher*Test`；`payuni-java-client` `PayUniWebhookParserTest`；`common-core` `PayUniWebhookEnvelopeTest` |
| 建議 | Stripe 簽章驗證（`Stripe-Signature`）、事件型別、冪等（event.id）都不同；單體規則 5 已規定「webhook 先寫 inbox」，Stripe 版直接用單體的 inbox 模式。`payuni-webhook-service` 同時承載物流 webhook，只要物流還在 PayUni，這個服務就不能整個退場 |
| 去向 | TBD |

### G6 退款（refund）

| 項 | 內容 |
|---|---|
| 現況 | Java legacy（payment＋aftersales＋order 三個服務協作） |
| 檔案 | payment：`RefundApplicationService`（`createRefund`、`createRescueRefund`、`handleCreditCardRefund`、`handleAtmRefundWebhook`、`handleAtmRefundAccepted`）、`RefundDomainService`、`RefundStatusRules`、`mq/{PayUniRefundNotifyConsumer,PayUniRefundAcceptedConsumer,RescueRefundConsumer}`；client `PaymentClient.refund/cancelAuthorization/offlineRefund/offlineRefundPage/cancelOfflineRefund`；aftersales `scheduler/RefundTaskWorker`、`service/{AtmRefundReconciler,RefundPageService,SellerCancelService}`、`mq/{RefundResultConsumer,UnknownRefundFallbackConsumer}`、`grpc/client/PaymentGrpcClient`；order `worker/{AutoRefundWorker,AutoRefundTxHandler}` |
| 流程特例 | **ATM 退款要買家去 PayUni 託管頁填銀行帳號**（`need_refund_page`、`awaiting_buyer_page`、`RefundPageService`、gateway `POST` 取得 URL `openapi.yaml` L15763）；信用卡走 `cancelAuthorization`／`refund`；rescue refund（EXPIRED 後 PayUni 才回 SUCCESS）|
| 表 | payment `payment_refunds`（`provider_refund_no`、`provider_status`）；aftersales `refund_tasks`（`payuni_status`、`payuni_message`、`mer_id`、`current_attempt`）；order `refund_requests`、`auto_refund_tasks`、`financial_logs`；計畫 B9 要把 `refund_tasks`／`auto_refund_tasks`／`payment_refunds` 收斂成一張退款命令表 |
| 契約 | `payment.proto` Refund RPC；`aftersales.proto`（`refund_failure_message` 直接透傳 `refund_tasks.payuni_message`「PayUni 原話」、`provider_status`、`redirect_url`）；`order.proto` `need_refund_page` |
| 客戶端 | website `lib/after-sales/{status,types,buyer-repo,return-failure-guidance}.ts`、`lib/orders/repo.ts`；buyer-app／seller-app `features/order/*`、`features/report`；CodeMeta `RefundStatus`、`AtmRefund...` 值域 |
| 測試 | aftersales／payment 的 Java 測試；website `after-sales-return-failure-guidance.test.ts`、`after-sales-repo.test.ts` |
| 建議 | 計畫 B9 重寫時以 payment port 的 `Refund(paymentID, amount, idempotencyKey)` 為界；Stripe refund 是同步＋webhook 非同步結果，沒有「買家要去託管頁」這步，`need_refund_page` 路徑與 `AwaitingBuyerRefundPagePayload`、通知模板 `awaiting_buyer_page` 全部可退場（要確認 Stripe 台灣支援的付款方式）|
| 去向 | TBD |

### G7 撥款／代理商費／託管（payout / grant / escrow）

| 項 | 內容 |
|---|---|
| 現況 | Java legacy；單體 bind 時送 `EscrowType`（開通參數） |
| 檔案 | order `worker/GrantPaymentWorker.java`（撥款資格閘門 backend#3262、payout hold）、`repository/GrantPaymentTaskRepository`、`service/OrderCompletionTxHandler`（訂單 DELIVERED→COMPLETED 同交易建立 `grant_payment_tasks`）、`scheduler/PaymentDeadlineScheduler`；payment `PaymentApplicationService.grantPayment/singleGrant/reconcileGrantViaQuery/invokeGrantWithRetry`、`GrantResponseRules`、`agentfee/{AgentFeeService,AgentFeeGrossUp,AgentFeeRatePolicy}`；client `PaymentClient.grant/agentFee/agentFeeQuery/grantQuery` |
| 契約 | `payment.proto` `GrantPayment` RPC、`grant_outcome`（含 `PROVIDER_TOOK_OVER`）、`grant_status/grant_message`；admin REST `GET /admin/grants`；order `PayoutCompletedPayload` MQ；CodeMeta 值域（撥款狀態）|
| 表 | order `grant_payment_tasks`（`uk_grant_payment_order`）；payment `payment_transactions.grant_*`、`granted_at`；物流財務 `logistics_receivables`、`logistics_collection_attempts`（PayUni 收款嘗試）、`seller_logistics_accounts`、`logistics_adjustments`、`ledger_transactions`、`ledger_entries`、`provider_settlement_lines`、`fulfillment_exception_cases`（payment V9）|
| 費用 | `AGENT_FEE_*` env（RATE_NUMERATOR／DENOMINATOR、MIN、MAX、ROUNDING、POLICY_VERSION）；ADR `2026-09-11-payuni-channel-fee-bearer`、`channel-fee-grossup-in-adapter`、`ledger-carrier-model-compatibility`；`website/docs/transaction-policy.md`（36 處）|
| 客戶端 | website `features/seller-center/wallet/*`、`lib/seller/wallet-transactions.ts`、seller-app `features/wallet/*` 與 `test/features/wallet/*`（90 處；錢包頁顯示撥款與物流費）|
| 建議 | PayUni 是**代理商先收款→按訂單完成撥款給子商店（平台掌握撥款時點，30 秒內撥）**的託管模型；Stripe Connect 對應是 destination charge／separate charges and transfers＋手動 payout schedule 或 `transfer_group`。GrantPaymentWorker 的資格閘門、hold 邏輯、`PROVIDER_TOOK_OVER` 收斂是業務邏輯，應保留在 port 之上（B10 finance）；`agentFee` 反推（gross-up）是 PayUni 費率模型，Stripe 手續費不同，ADR channel-fee-bearer 需重審。**此為最大的語意差異，需要設計決策（premise 問題）**|
| 去向 | TBD |

### G8 對帳／輪詢 job（reconciliation / polling）

| 項 | 內容 |
|---|---|
| 現況 | 單體：賣家狀態 sweep／poll（G3）；Java：付款與撥款對帳 |
| Java | payment `scheduler/{PendingPaymentReconcileScheduler,PendingPaymentReconcileService,ExpiredTransactionScheduler,ExpiredTransactionCloseService,OutboxEventPublisherScheduler,DataCleanupScheduler}`、`config/ReconcileProperties`、`PaymentClient.queryTransaction/queryTransactionByProviderTradeNo/grantQuery`；aftersales `AtmRefundReconciler`；runbook `backend/docs/基礎建設/ATM 退款結果 Webhook 遺失維運處置.md`；postmortems（`git grep -il payuni -- backend/docs/postmortems` → 0 檔）|
| 單體 | `seller.payuni_sweep`（15 分鐘）、`seller.payuni_poll`、手動 `RefreshPayUniStatus` |
| 監控 | `backend/monitoring/prometheus/rules`（2 檔）、`grafana/provisioning`（2 檔）、`sql-exporter/sql_exporter.yml`（payuni 相關表／指標）、`monitoring/README.md`；`seller-service` metrics.go |
| 建議 | Stripe 有 webhook＋`/events` 重播，對帳以 `Balance`／`PayoutReconciliation` report；PayUni 輪詢程式不搬。單體的 sweep 概念（補排遺失的 sync）可保留為通用模式 |
| 去向 | TBD |

### G9 物流貨態耦合（ship-status coupling）

| 項 | 內容 |
|---|---|
| 現況 | Java legacy；**與支付無關但與 PayUni MerID／webhook 服務強耦合**，換掉金流時物流仍留在 PayUni |
| 檔案 | `shared/payuni-java-client/.../shipping/*`（`ShippingClient`：createCvsShipment、createHomeShipment、createHomeRefund、scheduleHomeLabel、printCvsLabel、callTcat、queryShipment、onboardCvsShipping、onboardHomeDelivery）、`shipping-service/.../provider/payuni/{AbstractPayUniAdapter,PayUniCvsAdapter,PayUniHomeAdapter}`、`service/{ShipmentStatusMapper,ShippingService}`、`audit/ShippingPayUniAuditLogger`；order `mq/ShippingEventConsumer`（PayUni 狀態碼 31／32／11／33/43/44/46/51~56/81/82 → 訂單 IN_TRANSIT／WAITING_PICKUP／SHIPPING_ABNORMAL；送達 11 觸發 DELIVERED，完成後建撥款任務）；aftersales `mq/{ReturnShipmentStatusConsumer,ShipmentLabelConsumer}`；gateway `handler/{shipping,store_map,shipment_anomaly,admin_shipping,logistics_finance}.go` |
| 契約 | REST 物流端點、`order.proto` `shipping_status`（直接是 PayUni 狀態碼字串）、`shipping_status_desc`；CodeMeta：`PayUniSubmitStatus`（4301 未送／4303 已送）、`ShipmentAnomalyType.PAYUNI_LGS_NO_CODE`、賣家物流建店狀態、`PayuniMerchantVerificationStatus` |
| 表 | shipping `shipments`、`seller_shipment_settings`、`seller_shipment_onboarding_logs`、`shipping_webhook_dedup`、`payuni_audit_logs`；aftersales `aftersale_cases.return_provider_status/message`、`return_delivered_at` 等（PayUni 退貨貨態）|
| 單體關聯 | `CapShippingSetup` 前置條件 = KYC 流程核准（含 PayUni 核准）：「物流商以 PayUni 已成立的子商店建店」（`model/capability.go:41,66`、`seller.proto` L254、L268）|
| 設計 | 設計書 06、65、70、21、22、23；ADR `2026-09-11-logistics-*`、`responsibility-based-cost-attribution`、`return-leg-classifier-canonical` |
| 建議 | 這是金流替換計畫的邊界問題：若物流繼續用 PayUni，則 `payuni_mer_id` 仍是物流的必要輸入，單體需要保留「PayUni 物流子商店」的註冊（至少 MerID 與物流建店狀態），不能把 `seller.payuni_registrations` 整張退場。要選：(a) 保留 PayUni 作為純物流 provider，把註冊改名成 logistics-provider 註冊；(b) 物流也換掉（超出本次範圍）。另外「訂單完成→撥款」是物流狀態推進的，需要 payment port 與 fulfillment 之間的事件契約 |
| 去向 | TBD |

### G10 同意書／條款（agreements / terms）與法務文件

| 項 | 內容 |
|---|---|
| 現況 | 單體 live（`AcceptSellerAgreements`）＋ website 文件 |
| 檔案 | `internal/modules/seller/internal/model/agreement*.go`、`agreement_gen.go`（manifest 生成）、`db/queries`（agreement acceptances）、`seller/agreement_integration_test.go`、`agreement_queries_test.go`；website `docs/legal/{seller-agreements.json,seller-terms.md,seller-kyc-notice.md,self-issue-attestation-terms.md}`、`features/legal/documents/sellerAgreements.gen.ts`、`scripts/generate-legal.ts`、`scripts/legal-markdown.unit.test.ts`；CI `legal-generated-diff.yml` |
| 契約 | `AgreementKind` enum：`SELLER_TERMS`／`KYC_PRIVACY_NOTICE`／`PAYUNI_TERMS`；SubmitKYC 時同意 `PAYUNI_TERMS` |
| 待辦 | `go-modular-monolith.md` L318–322：律師確認三份條文；依代理商契約更正 PAYUNI_TERMS（取得條款正本存 `website/docs/legal/`，manifest 改 `doc`）— 這項因換 provider 可能整個作廢 |
| 建議 | 換成 Stripe Connected Account Agreement（由 Stripe 在 onboarding 流程內收，不必自己存）；`PAYUNI_TERMS` 值、DB CHECK、proto enum、manifest 與 website 條文可一起退場（上線前不受相容性約束）|
| 去向 | TBD |

### G11 其他附帶（不歸上面各群）

| 項 | 內容 | 建議 | 去向 |
|---|---|---|---|
| 代理商受託代銷（procurement agent） | ai-service `procurement/orderprovisioning/worker.go` 提到 `PAYUNI_AGENT_MER_ID`（2026-09-21 起 EXTERNAL lane 不再用）；`docs 64-Hawkr Agent 商務線` | 確認 `rg -n PAYUNI_AGENT ai-service` 是否仍有實際使用（註解） | TBD |
| 買家會員帳務週期 | ADR `2026-08-29-membership-billing-payuni-period.md`；payment V5–V8 建表、**V17 已 drop**（`drop_membership_billing_tables`），`payuni.payment.recurring` routing key 仍在 `events.csv` | 已退場，只剩文件與事件目錄殘留，清掉 | TBD |
| 發票 | invoice-service 用 `provider_paid_at` 決定開立期間，從 payment_transactions 讀（`internal/client/payment/client.go`）| 換 provider 後欄位語意（付款時間）要由 port 提供 | TBD |
| 通知模板 | notification-service migrations（`merchant.verification.*`、`aftersale awaiting_buyer_page` 模板，文案寫 PayUni）| 文案中性化；ATM 頁模板隨 G6 退場 | TBD |
| 7-11 門市地圖 | gateway `handler/store_map.go`、`PAYUNI_STORE_MAP_*` env、website `app/(commerce)/store-map/return/route.ts`、`lib/checkout/store-pickups.ts`、seller-app `store_map_webview_screen.dart` | 屬 PayUni 物流（G9），不隨金流替換 | TBD |
| 錯誤收斂規範 | `backend/docs/開發規範/PayUni 錯誤回應收斂規範.md` + gateway `response/bizhttp.go`、codemeta `registry.go`／`labels.go`、`pkg/errors/bizcode`（錯誤碼 9700 系列為 PayUni；commit `099967a` 剛補回仍在用的 9700）| payment port 要定義自己的錯誤分類；規範改寫 | TBD |
| 設計 token／其他 | `website/styles/features/seller-center.css`、`admin/src/styles/pages/MerchantCertificationDetail.css` 的 class 名含 payuni | 純命名 | TBD |

## 3. Connect protos 與 REST／gRPC 中 provider 專屬的欄位與 enum（契約清單）

指令：`rg -oiI "[A-Za-z_]*payuni[A-Za-z_]*" api --glob '*.proto' | sort | uniq -c | sort -rn`（`-h` 是 help，要用 `-I`）。

Connect（`api/hawkr/**`，生成物 `packages/hawkr-api-ts/src/gen`、`packages/hawkr_api/lib/gen` 由 `buf` 生成，不得手改）：

- admin seller（84 處 / `admin/seller/v1alpha1/seller.proto`）：`GetPayUniRegistration`、`RebindPayUni`、`ListPayUniObservations`、`RefreshPayUniStatus`＋各自 Request/Response、`PayUniRegistration`、`PayUniObservation`、`PayUniObservationSource`、`PayUniBindStatus`、`PayUniStatusCodes`、`StoreActionUnavailableReason` 的 `PAYUNI_BOUND`／`PAYUNI_BIND_IN_PROGRESS`／`PAYUNI_MERCHANT_TERMINAL`。
- seller（73 處 / `seller/v1alpha1/seller.proto`）：`SellerPayUni`、`PayUniState`、`PayUniProblem`、`KYCFlowStatus`（`PAYUNI_REJECTED` 等）、`KYCResubmitUnavailableReason`（`PAYUNI_REVIEWING`／`PENDING_INFO`／`RETRY_LATER`／`FIX_VIA_SUPPORT`／`CONTACT_SUPPORT`／`APPROVED`／`TERMINATED`）、`AgreementKind.PAYUNI_TERMS`、`StoreContact` 欄位註解（MerTel／ContactName／ContactMobile／MerEmail）、能力描述（物流建店需 PayUni 核准）、`ErrorReason` 的 PAYUNI_* 原因。
- reviewqueue：`REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY = 12`（對應 SQL 函式 `seller.review_payuni_bind_retry`、`rq.PayUniBindRetry`、權限 `SellerPayUniRegistrationsGet`）。
- discovery、common/store_card：只在註解（曝光條件含 PayUni 審核通過）。
- 生成物影響：`buf breaking` 在上線前 v1alpha1 不擋（根 AGENTS「上線前不受相容性約束」），改契約同一次變更要更新 `api/`、Go `internal/httpapi/gen`、TS、Dart 與四個 client。

Legacy（`backend/api/proto/**`，生成 `backend/pkg/proto/**`，Java 也用）：

- `payment/payment.proto`（`DecryptPayUniResult`、`CreatePaymentResponse` 的 UPP 表單、`mer_id`、`grant_*`、client_type／app_type 決定回跳出口）。
- `order/order.proto`：`payment_mer_id`、`consolidated`、`mer_id`、`need_refund_page`、`provider_status/message`、`shipping_status`（PayUni 狀態碼）、付款方式可用性。
- `aftersales/aftersales.proto`：退款頁 URL、`refund_failure_message`、`provider_status`、`return_provider_status`、`ship_trade_no`。
- `shipping/shipping.proto`、`seller/seller.proto`（`payuni_mer_id`）、`account/account.proto`（`reserved 10; // was: merchant_id (payuni_mer_id string)`）。
- `backend/scripts/proto/numeric-allowlist.txt`、`image-tag.sh` 提到 payuni（檢查用）。
- REST：`backend/api/openapi/openapi.yaml`（約 25 處描述／參數，如 `payuniCollectionFee`、`PAYUNI_LGS_NO_CODE`、ATM 退款頁）。

## 4. 單體內事件與 RabbitMQ 契約

| 事件 | 來源 | 去向 | 備註 |
|---|---|---|---|
| `seller.review.result`（AGENT_NOTIFY） | Java payuni-webhook-service | 單體 `internal/legacy/sellerconsumer` → `ReceivePayUniNotification` | queue 前綴 `hawkr.monolith`；格式是 Java `buildEventEnvelope` 的 snake_case envelope |
| `merchant.verification.{forwarded,approved,pending_info,under_review,rejected}` | 單體 seller（job `legacy.seller_payuni_*`） | notification-service（Go legacy）| `PayUniBindFailed` 沒有舊 routing key，不轉送 |
| `payuni.payment.uop`／`refund.notify`／`refund.accepted`／`shipping.status`／`unknown.fallback` | Java webhook service | Java payment／aftersales／shipping | 單體不涉及 |
| `shared/events/events.json` L290 `trade_no: PAYUNi交易編號` | 事件 schema 註解 | | 隨 payment 事件 |
| infra | `backend/infra/mq/{README.md,tools,topology}`（3 處）宣告 `payuni.events` exchange 與 queue | | topology 管理（Terraform）文件 `基礎建設/MQ Topology 管理（Terraform）.md` |

## 5. 資料庫彙總

單體 `seller` schema（live）：`payuni_registrations`、`payuni_observations`、view `store_requirement_flags.kyc_approved`、函式 `review_payuni_bind_retry`、`agreement_acceptances.kind='PAYUNI_TERMS'`；sqlc 生成到 `internal/store`（不得手改，`make sqlc`）；`sellerserver` 回傳 `payuni_mer_id`。

Go legacy seller-service DB（不部署）：`payuni_registrations`、`payuni_status_history`、`payuni_merchant_profile_updates`、`merchant_verification_outbox`、`payuni_merchant_profile_outbox`。

Java DB（legacy，各服務一個）：payment（`payment_transactions`、`payment_refunds`、`payment_webhooks`、`payment_logs`、`payuni_audit_logs`、`outbox_events`、物流財務 9 張）、payuni-webhook（`webhook_inbox`、`outbox_events`）、shipping（`shipments`、`seller_shipment_settings`、`seller_shipment_onboarding_logs`、`shipping_webhook_dedup`、`payuni_audit_logs`）、order（`orders.mer_id`、`payment_event_dedup`、`auto_refund_tasks`、`grant_payment_tasks`、`refund_requests`）、aftersales（`refund_tasks`、`aftersale_cases.return_provider_*`）。

migration 規則：已套用的 migration 不得修改（`TestSchemaFileMatchesMigrations`）；單體改 `db/schema.sql` 後 `atlas migrate diff`。Java 用 Flyway `V*__*.sql`。

## 6. 客戶端彙總

| 客戶端 | 檔案群（去重後主要） | 性質 |
|---|---|---|
| website | `lib/seller/{payuni,kyc-flow,payuni-locked-fields,shipment,shipping,wallet-transactions}.ts`、`features/seller-center/{components/PayuniStatusBody,wallet/WalletClient,profile/*,dashboard/*}`、`lib/payment/*`、`lib/payments/*`、`features/{checkout,payments,after-sales,account}`、`lib/codemeta/generated.ts`（生成，23 處）、`docs/{transaction-policy,seller-payment-logistics-flow}.md`、legal 文件與 manifest | 賣家 remedy UI（G3）、買家付款（G4）、法務（G10）、物流（G9）|
| admin | `src/app/admin/merchants/[id]/{PayUniCards.tsx,page.tsx,kyc-application.ts,use-store-detail.ts}`、`store-row.ts`、`src/lib/unavailability.ts`、`src/lib/codemeta/generated.ts`（21）、`tests/merchant-kyc-application.test.ts`、`unavailability.test.ts` | 後台重新綁定／刷新／觀測（G1/G3）|
| seller-app | `lib/features/seller/**`（19 檔）、`features/order/**`（13）、`features/settings/**`（運費設定／門市地圖）、`features/report`、`features/wallet`、`lib/core/codemeta`；`test/features/seller`（13）、`test/features/wallet`（2，90 處）| 同 website 賣家端 |
| buyer-app | `core/payment/pay_token_service.dart`、`features/order/**`（14）、`core/codemeta/code_meta.dart`（52）、`config/flavor.dart`、`features/cart`、`features/wallet` | 買家付款與退款 UI |
| `@hawkr/ui-text`／`hawkr_ui_text` | 見 G3，生成物，來源 `backend/internal/platform/uitext/catalog/*.json`，`make uitext` | 文案 |
| CodeMeta（凍結，`backend/docs/代碼管理（CodeMeta）/`；Go `pkg/codemeta`；生成物 website／admin／buyer-app／seller-app） | `PayuniMerchantVerificationStatus`、`PayUniSubmitStatus`（4301–4303）、PayUni 退款交易狀態、`ShipmentAnomalyType.PAYUNI_LGS_NO_CODE`、WebhookProvider.PAYUNI（8241）、ClientType／AppType 描述 | 依 ADR `codemeta-retirement`，隨 Java 消費者退場，不新增值域 |

## 7. 文件

- `backend/docs/第三方整合/`：`PayUni 金流官方 API 總覽.md`（1000 行）、`PayUni 代理商專用 API 總覽.md`（938）、`PayUni 逆向金流 API 總覽.md`（677）、`PayUni 物流官方 API 總覽.md`（994）、`PayUni 費用一覽.md`（305）。
- 設計書：05 支付服務（609 處）、06 物流（622）、09 售後（251）、10 PayUni Webhook（107）、11 PayUni Java Client（182）、03 訂單（67）、36 MQ 與事件（68）、58 發票（49）、29 Admin 異常訂單（48）、20 結帳、65、70 等；基本設計書 06、65、70。
- 規範：`開發規範/PayUni 錯誤回應收斂規範.md`、`開發規範/硬上限依賴接入治理規範（配額與容量）.md`（PayUni 2 QPS）、`單體開發規範.md`（規則 5 列 PayUni 為外部 provider）。
- ADR（37 個提到）：核心 `2026-10-19-seller-kyc-payuni-consolidated`、`2026-10-02-seller-kyc-payuni-in-monolith`、`2026-09-11-payuni-channel-fee-bearer`、`2026-09-18-webhook-two-phase-transaction`、`2026-08-29-membership-billing-payuni-period`、`2026-09-25-codemeta-retirement`；其餘為順帶提到。
- 模組規格：`modules/seller.md` §13–14（及 §1.3、§10 能力表）、`modules/reviewqueue.md`。
- 計畫：`plans/2026-09-24-go-modular-monolith.md`（B2、B3、B8–B10、L318–322 條文待辦）、`plans/2026-09-29-a3-buyer-seller/*`（step10e-5a/5b 設計各 225/104 處）、`plans/2026-10-24-portfolio-public/inventory.md`（82 處）。
- 運維：`基礎建設/ATM 退款結果 Webhook 遺失維運處置.md`、`速查參考/Apple Pay Merchant Cert 輪替 SOP.md`、`基礎建設/{Discord 告警…,系統監控架構,Prod RabbitMQ…}`；`docs/runbooks/`：只有 `uber-quote-retention.md` 提到 PayUni（Uber 將刪）。
- Java 服務內文件：`payment-service/測試與部署指南.md`、`枚舉變更清單.md`。
- website：`docs/{transaction-policy,seller-payment-logistics-flow,analytics-events,content-copy-audit}.md`。

## 8. 設定／環境變數（僅名稱）

指令：`rg -oIN "\bPAYUNI_[A-Z0-9_]+"` 於 `.env.template`、`docker-compose-local.yml`、`Makefile`、`cmd/hawkr/seller.go`、Java `application*.yml`。

- 單體（`cmd/hawkr/seller.go` `readPayUni`；`Makefile` 本機預設）：`PAYUNI_BASE_URL`、`PAYUNI_AGENT_ID`、`PAYUNI_HASH_KEY`、`PAYUNI_HASH_IV`、`PAYUNI_TIMEOUT`、`PAYUNI_MAX_QPS`、`PAYUNI_STOREFRONT_BASE_URL`。四個核心變數必須一起設定，沒設則 PayUni 關閉（綁定記 `PAYUNI_NOT_CONFIGURED`）；測試 `cmd/hawkr/payuni_config_test.go`。
- Java／legacy（`.env.template`、`docker-compose-local.yml`、application.yml）：`PAYUNI_AGENT_ID`、`PAYUNI_AGENT_KEY`、`PAYUNI_AGENT_IV`、`PAYUNI_AGENT_MER_ID`、`PAYUNI_API_URL`、`PAYUNI_NOTIFY_URL`、`PAYUNI_RETURN_URL`、`PAYUNI_SHIPPING_NOTIFY_URL`、`PAYUNI_STORE_MAP_MER_ID`、`PAYUNI_STORE_MAP_RETURN_URL`、`PAYUNI_SANDBOX`、`PAYUNI_SANDBOX_MER_ID`、`PAYUNI_AGENT_FEE_{RATE_NUMERATOR,RATE_DENOMINATOR,MIN,MAX,ROUNDING,POLICY_VERSION}`、`PAYUNI_WEBHOOK_SERVICE_{HOST,PORT}`（gateway→webhook 服務）、`PAYUNI_BACKED_DEADLINE_SEC`。
- 命名分歧（推斷，未逐檔驗證）：`rg` 在整個 backend 還看到 `PAYUNI_HASH_KEY`／`PAYUNI_HASH_IV`／`PAYUNI_MERCHANT_{ID,KEY,IV}`／`PAYUNI_AGENT_HASH_{KEY,IV}` 等名稱（`pkg/config/server.go`、seller-service `config.go` 與文件），與 Java 的 `PAYUNI_AGENT_{KEY,IV}` 並存；實際哪個服務讀哪個名稱沒對過。
- Apple Pay：`ApplePayProperties`（商家憑證）、website `applePayDomainAssociation.ts`、`.well-known`。

## 9. CI／workflows／基礎設施

- `.github/workflows` 內沒有任何 PayUni 字串（`git grep -niE "payuni|payment|webhook" -- .github` → 0）。
- 間接受影響的 workflow：`backend-monolith.yml`（單體 unit＋integration，含 seller PayUni 測試）、`backend-java-tests.yml`、`backend-integration-tests.yml`、`backend-contract-tests.yml`、`backend-gateway-json-contract.yml`、`backend-proto-generated-diff.yml`、`backend-sqlc-generated-diff.yml`、`backend-migration-inventory.yml`（`scripts/check-migration-inventory.py` 要求刪舊服務前 disposition 全填，涵蓋 PayUni 的 RPC／事件／副本）、`api-contract.yml`（buf lint／breaking，proto 改動）、`ui-text-generated-diff.yml`、`ui-text-guard.yml`、`legal-generated-diff.yml`（法務 manifest）、`website-ci.yml`、`admin-ci.yml`、`buyer-ci.yml`、`seller-ci.yml`、`website-e2e.yml`（e2e 不在本機跑）。
- 本機基礎設施：`backend/docker-compose-local.yml`、`dev.sh`、`.env.template`、`Makefile`、`infra/seed-test-data.sh`、`infra/mq/*`（exchange／queue）、`monitoring/{prometheus/rules,grafana/provisioning,sql-exporter}`。
- 沒有部署設定（K8s／Helm／Terraform）在 repo 的可見範圍內被驗證為 PayUni 專屬；設計文件 `基礎建設/CI_CD 自動化部署（K8s + Helm）.md` 提到 PayUni，但公司時期部署與維運 workflow 已不在現行範圍（根 AGENTS.md）。

## 10. 測試彙總（依賴 PayUni 的）

- 單體：`modules/seller` 31 檔（含 integration，需 Docker）、`platform/payuni/client_test.go`、`legacy/sellerserver/*_test.go`（替身 `approvingPayUni`）、`httpapi/{seller,reviewqueue}/*_test.go`、`cmd/hawkr/{payuni_config_test.go,dev_seed_integration_test.go}`、`internal/devseed`。
- legacy Go：`seller-service` 59 檔、`gateway` 11 檔（`payuni_redirect_test.go` 34 處、`proxy/reverse_proxy_rewrite_test.go`、`response/bizhttp_payuni_merchant_profile_test.go`）、`pkg/payuni/{client,crypto}_test.go`。
- Java：106 個測試檔（payment、payuni-webhook、order `GrantPaymentWorker*Test`、payuni-java-client、common-core）。
- 客戶端：website 30 檔（單元；e2e 在 `tests/e2e`，mock，本機不跑，CI 才跑）、admin 7、seller-app 26、buyer-app 1、packages 6（含 ui-text golden）。
- 生成物守門：`ui-text` golden、`enum-manifest`、codemeta 生成。

## 11. 無法分類／需要確認的項目

1. Java 側仍有 4 個 seller PayUni 事件類別（`common-core/.../mq/events/SellerPayuni{Register,Result,StatusCheck,StatusUpdated}Event.java`）與 `shared/java-proto/src/main/proto/seller/seller.proto`（第二份 seller proto 副本）；單體 A3 已搬走 seller-service，這些類別是否還有 Java 生產者／消費者沒查（`git grep -n SellerPayuni -- backend/services/java`）。
2. `PAYUNI_BACKED_DEADLINE_SEC`、`PAYUNI_RFD_77`、`PAYUNI_DATE_FMT` 等是 Java 常數還是 env，沒逐一追。
3. 付款方式在 Stripe 台灣的可用性（ATM 虛擬帳號、Apple／Google Pay 子商店模型）是 Stripe 產品事實，沒驗證（任務限制不打網路）。
4. `website/docs/transaction-policy.md`（36 處）與 `seller-payment-logistics-flow.md` 是產品政策文件，內容是否有對賣家／買家的承諾（例如撥款時點）需要產品決定，盤點只標記。
5. 錯誤碼 `9700` 系列（`pkg/errors/bizcode`、CodeMeta）與 gateway `response/bizhttp.go` 的 PayUni 錯誤收斂是否有 client 端依賴的 reason 字串，沒逐一比對 client。
6. `backend/pkg/geocode/tgos_client.go`、`pkg/urlsafety/validate.go`、`pkg/httpclient/httpclient.go`、`pkg/config/media.go` 對 PayUni 的提及性質（allowlist／註解）未逐行確認。
7. 計畫的 B-階段順序（B2 payment 排在 B8 ordering 之前，B10 才撥款）與 Stripe 切換的時點關係，需規劃者決定（premise：這些 Java 程式是否本來就預定改寫——是，B2/B3/B8–B10 都排定改寫，所以在 Java 加 Stripe adapter 通常不值得）。
