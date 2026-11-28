# detailed-design總覽

> 📘 **要先看懂「這個服務在做什麼、誰會看到它」，先讀 [basic-design](../basic-design/00-overview.md)。**
> 兩者的分工是**外部設計 vs 內部設計**：basic-design講業務流程、畫面一覽與外部介面的用途；
> 本系列講實作契約（DDL／proto／端點簽章／狀態機／陷阱）。同一件事只有一個權威版本，不互相摘要。


> 本文件分成兩部分：
> 1. 已編號detailed-design索引
> 2. 目前代碼庫與部署設定的參考現況
>
> 說明：
> - 本文件的主體仍是「已維護的detailed-design索引」
> - 後半段的服務現況僅作為閱讀與定位程式碼的輔助參考
> - detailed-design未覆蓋所有實際存在的服務屬正常情況；文件覆蓋範圍以目前已建立並持續使用的設計書為準

---

## 已編號設計書索引

| 編號 | 設計書名稱 | 對應程式碼路徑 | HTTP Port | gRPC Port | 備註 |
|------|-----------|---------------|-----------|-----------|------|
| 01 | [閘道微服務（Gateway）](01-gateway-detailed-design.md) | `gateway/` | 9900 | - | 對外 HTTP 入口 |
| 02 | [產品微服務（Product Service）](02-product-service-detailed-design.md) | `services/java/product-service/` | 9912 | 9913 | Java / Spring Boot |
| 03 | [訂單微服務（Order Service）](03-order-service-detailed-design.md) | `services/java/order-service/` | 9914 | 9915 | Java / Spring Boot |
| 04 | [購物車微服務（Cart Service）](04-cart-service-detailed-design.md) | `services/java/cart-service/` | 9926 | 9927 | Java / Spring Boot |
| 06 | [物流微服務（Shipping Service）](06-shipping-service-detailed-design.md) | `services/java/shipping-service/` | 9930 | 9931 | Java / Spring Boot |
| 07 | [優惠券微服務（Coupon Service）](07-coupon-service-detailed-design.md) | `services/java/coupon-service/` | 9950 | 9951 | Java / Spring Boot |
| 08 | [多媒體微服務（Media Service）](08-media-service-detailed-design.md) | `services/java/media-service/` | 9936 | 9937 | Java / Spring Boot |
| 09 | [售後微服務（AfterSales Service）](09-aftersales-service-detailed-design.md) | `services/java/aftersales-service/` | 9922 | 9923 | Java / Spring Boot |
| 10 | [PayUni Webhook 服務](10-payuni-webhook-service-detailed-design.md) | `services/java/payuni-webhook-service/` | 9946 | 9947 | 獨立服務，非共用 Payment Port |
| 11 | [PayUni Java Client](11-payuni-java-client-detailed-design.md) | `services/java/shared/payuni-java-client/` | - | - | 共用 Java library |
| 12 | [通知微服務（Notification Service）](12-notification-service-detailed-design.md) | `services/notification-service/` | -（gRPC-only） | 10037 | Go 服務 |
| 13 | [通知模板引擎與收件箱](13-notification-template-detailed-design.md) | `services/notification-service/` | -（gRPC-only） | 10037 | 建立於通知服務之上 |
| 14 | [即時通訊微服務（IM Service）](14-im-service-detailed-design.md) | `services/im-service/` | — | 9933 | gRPC-only；Gateway public WebSocket 透過 typed ChatStream 連入 |
| 15 | [AI 微服務（AI Service）](15-ai-service-detailed-design.md) | `services/ai-service/` | 9938 | 9939 | Go 服務 |
| 16 | [顧客管理微服務（CRM Service）](16-crm-service-detailed-design.md) | `services/crm-service/` | -（gRPC-only） | 10030 | Go 服務 |
| 19 | [農業認證申請與審核](19-certification-verification-detailed-design.md) | `services/java/product-service/` | - | - | 商品認證的申請／政府查驗／ADM 審核；`product.certifications` 為 approved-only 公開投影 |
| 20 | [結帳模組（Checkout Module）](20-checkout-module-detailed-design.md) | 跨服務模組 | - | - | 主要跨 Cart / Order / Payment / Coupon / Shipping |
| 21 | [運費計算模組（Shipping Fee Module）](21-shipping-fee-module-detailed-design.md) | `services/java/shared/common-core/` | - | - | 共用 Java 元件 |
| 22 | [商品配送驗證](22-product-shipping-validation-detailed-design.md) | 跨服務模組 | - | - | 主要由 Product / Shipping 實作 |
| 23 | [門市管理模組](23-store-pickup-management-detailed-design.md) | `services/java/shipping-service/` | - | - | 物流子領域 |
| 24 | [節假日管理（Holiday Management）](24-holiday-management-detailed-design.md) | `services/java/shipping-service/` | - | - | 物流子領域 |
| 26 | [輸入驗證模組（Java InputValidator）](26-java-inputvalidator-detailed-design.md) | `services/java/shared/common-core/` | - | - | 共用 Java 元件 |
| 29 | [Admin 異常訂單管理 API](29-admin-anomalous-order-management-api-detailed-design.md) | 跨 Payment / Shipping / AfterSales / Order | - | - | 跨服務 Admin 查詢層，PayUni 相關失敗紀錄的列表+詳情 |
| 31 | [內容檢舉與封鎖（Content Reporting & Block）](31-content-reporting-and-block-detailed-design.md) | `gateway/`、`services/{social,im,review}-service/`、`pkg/{blocklist,moderation}` | - | - | Apple App Store Guideline 1.2 合規，跨多 service 整合 |
| 32 | [商家治理與食安通報（Seller Governance & Food Safety）](32-seller-governance-and-food-safety-detailed-design.md) | `admin/` | - | - | hawkr-admin 的商家治理畫面（行為見 `docs/modules/seller.md`） |
| 33 | [商品 Hashtag 模組（Product Hashtag）](33-product-hashtag-detailed-design.md) | `services/java/product-service/` | - | - | product-service 內部子模組；DB schema、normalize、counter、auto-complete、熱門查詢 |
| 34 | [貼文 Hashtag 模組（Post Hashtag）](34-post-hashtag-detailed-design.md) | `services/social-service/` | - | - | social-service 內部子模組（Go）；鏡像 33 號商品 Hashtag，共用 normalize / 字元 / 長度 / 數量規則與錯誤碼 |
| 35 | [商品列表排序與銷量反正規化（Product Sort & Sales Count）](35-product-sort-and-sales-count-detailed-design.md) | `services/java/product-service/`、`services/java/order-service/`、`gateway/` | - | - | 5 個 sort 值完整支援；product.sales_count 反正規化由 order.sales.accumulated 事件累計 |
| 36 | [MQ 與事件總覽（MQ & Event Catalog）](36-mq-and-event-catalog.md) | 跨全部 Java / Go 微服務 | - | - | MQ / Event 的唯一真實來源（SSOT）：Producer / Consumer / Exchange / Queue / Routing Key / Payload / DLQ / 冪等；新增或修改 MQ 前必先更新本文件 |
| 38 | [Flyway 自動同步與 DB 自動初始化（Flyway Auto Sync & DB Auto Init）](38-flyway-auto-sync-and-db-auto-init-detailed-design.md) | 10 個 Java 微服務、`services/java/shared/common-core/` | - | - | V1 凍結 + V2+ 漸進式 migration；DatabaseBootstrapInitializer 服務啟動自動建 DB 與 extension |
| 41 | [Realtime SSE 推播平台（Realtime SSE Push Platform）](41-realtime-sse-push-platform-detailed-design.md) | `gateway/`（SSE Hub + MQ Consumer）、`services/social-service/`、`services/notification-service/` | - | - | 通用 SSE Hub + Channel 註冊；首版 2 channel：留言（per-post，三端）/ 通知（per-user，僅 web；mobile 走 FCM） |
| 43 | [評論微服務（Review Service）](43-review-service-detailed-design.md) | `services/review-service/`、`gateway/`（review handler） | 10032 | 10033 | 訂單商品評論全生命週期（提交 / 查詢 / 統計 / B-135 隱藏 / review.created·hidden 事件）；review-service 首份設計書 |
| 44 | [帳號與賣家買家身份模型（Account & Seller-Buyer Identity）](44-account-and-seller-buyer-identity-detailed-design.md) | `gateway/`、跨角色守衛 im/social/cart/coupon/order | - | - | ResolvePrincipals 跨角色守衛、gateway 身份解析（賣家身份見 `docs/modules/seller.md` §2.1）；單體 identity／buyer／accountdeletion 的規格在 `docs/modules/`；**取代舊 42** |
| 46 | [賣家能力權限層（Seller Capability Authorization）](46-seller-capability-authorization-detailed-design.md) | `gateway/internal/authz/`、`gateway/internal/router/` | - | - | Gateway deny-by-default 賣家授權層：`requireSeller(cap)` 宣告=掛載合一、豁免清單、收斂點 fail-closed backstop（能力推導見 `docs/modules/seller.md` §10） |
| 47 | [GA4 分析追蹤（GA4 Cross-Platform Analytics）](47-ga4-cross-platform-analytics-detailed-design.md) | `hawkr-website/lib/analytics/`、`hawkr_buyer/lib/core/analytics/`、`hawkr_seller/lib/core/analytics/`、`services/java/order-service/`（GA4 backstop） | - | - | 跨 4 repo 主題：web(gtag)／buyer+seller app(Firebase) 的 client-side 埋點，以及後端 GA4 Measurement Protocol purchase backstop 的契約；含 GA4 從零建置 Console SOP |
| 48 | [SEO 優化與現況（SEO Optimization）](48-seo-optimization-detailed-design.md) | `hawkr-website/lib/seo/`、`hawkr-admin/src/app/admin/seo/`、`gateway/internal/handler/admin_seller.go`、`internal/modules/seller/`、`services/java/product-service/` | - | - | 跨 3 repo 主題：website 技術 SEO（metadata／JSON-LD／robots／sitemap）的模組責任，以及賣家 SEO override 全鏈路（admin→gateway→seller 模組→revalidate→網站） |
| 49 | [多媒體儲存與圖片服務（Cloudflare R2 & Image Gateway）](49-cloudflare-r2-and-image-gateway-detailed-design.md) | `services/java/media-service/`、`workers/image-gateway/`、`services/java/shared/common-core/`、`pkg/mediaurl/` | - | - | OSS→Cloudflare R2 遷移＋Image Gateway Worker（media.example.com，preset 白名單 deny-by-default）＋全平台圖片欄位 objectKey 單一真實來源與 URL 組裝收斂 |
| 52 | [微服務間內部授權（Internal Service Authorization）](52-internal-service-authorization-detailed-design.md) | `services/java/shared/common-core/`、`services/java/shared/observability-common/`、`pkg/middleware/grpc/interceptor/`、各 Go 服務 `internal/grpc/di.go`、`k8s/external-services/networkpolicy.yaml`、`k8s/values/*.yaml` | - | - | 跨服務主題：微服務間 gRPC 內部身分以 HMAC-SHA256 簽章判定（mode `off`／`permissive`／`enforcing`）＋per-service NetworkPolicy ingress 白名單＋歸屬判定不用魔數哨兵＋內部 PII RPC 縱深校驗。與 46（Gateway 端授權）、44（身分解析）正交 |
| 53 | [Web 前端快取架構與版本一致性（Web Frontend Cache & Deployment Consistency）](53-web-frontend-cache-and-deployment-consistency-detailed-design.md) | `hawkr-website/lib/api/cached.ts`、`hawkr-website/next.config.ts`、`hawkr-website/open-next.config.ts`、`hawkr-website/wrangler.jsonc`、`hawkr-website/app/api/revalidate/route.ts` | - | - | 網頁（Next.js on Cloudflare Workers）前端快取分層（瀏覽器／CF edge → Worker ISR → R2 incremental cache ＋ sharded tag cache／refresh queue／purge buffer 三個 DO）＋ document 與 RSC 的 `Cache-Control` 契約（四條規則、無 opt-out、**Workers Cache 不參與**）＋ 後端 mutation → `/api/revalidate` 的 tag 展開與失效鏈＋失效後收斂（合成讀者，§4.5）＋ candidate 0% staging ＋ rollout 傳播 gate 的部署版本一致性＋ R2 lifecycle；backend 觸發 revalidate 見責任邊界規範 §9 |
| 55 | [Hawkr 幫找採購助手（Procurement Assistant）](55-procurement-assistant-detailed-design.md) | `services/ai-service/internal/procurement/`、`services/ai-service/db/schema/procurement.sql`、`gateway/internal/handler/ai.go`、`hawkr-website/features/procurement-assistant/` | - | - | 買家採購需求釐清與候選呈現（MVP-A）：固定 slot engine + 純函式選題（模型只負責抽取與文案）、不可變需求版本與分支失效、雙 lane 候選與可驗證配對事實、LLM 不可用時降級續行；詢價／價格引擎／外部邀價／金流不在本設計範圍|
| 56 | [ADM 代行營運（Delegated Operations）](56-delegated-operations-detailed-design.md) | `internal/modules/delegation/`、`gateway/internal/authz/`、`gateway/internal/router/`、`api/proto/common/delegation.proto`、`hawkr-admin/src/features/delegated-operations/` | - | - | 跨 repo 主題：平台管理員在 ADM 內以短效、可撤銷、逐筆稽核的方式代賣家操作店面／商品／貼文，訂單唯讀、**永不取得任何履約能力**（UI／路由／授權三層鎖定，Gateway boot 斷言機械保證）。delegation 模組擁有工作階段／店家授權／操作稽核等資料表，既有賣家 RPC 加 `DelegationContext` 而非冒用賣家身分；稽核走 intent→confirm 兩階段（跨 DB 無法同交易）；9 個 capability 含 publish 分權。⚠️ 本書是 hawkr-admin 那一側（BFF＋UI＋契約測試）對應的後端契約，**capability 變更必須 admin 前端先發布** |
| 58 | [電子發票服務（Invoice Service）](58-invoice-service-detailed-design.md) | `services/invoice-service/`（含 `internal/client/amego/`）、`api/proto/invoice/invoice.proto`、`gateway/internal/{router/routes_invoice.go,handler/invoice*.go,grpc/invoice_service_client.go,proxy/reverse_proxy.go}` | - | 10041 | 新服務（Go／fx／sqlc／Atlas）：代賣家向光貿 AMEGO 開立買家電子發票——付款成功當下鎖不可變發票快照，退款時自動判定作廢或折讓，對法定期限倒數並留存每次送件原文；**平台不是發票人**。本書為兩冊之首（後端與契約唯一 SSOT，含附錄 C CodeMeta 裁決與附錄 D 前端契約對照），前端規格見 58A；碼表已於設計階段直接寫入 `docs/codemeta/` 四份 `.md` 與 36 號 MQ Catalog，不另存材料分冊 |
| 58A | [電子發票前端實作規格](58a-e-invoice-frontend-implementation-specification.md) | `hawkr-website/app/(account)/account/`、`hawkr-website/app/(seller)/seller-center/invoices/`、`hawkr-admin/src/app/admin/invoices/` | - | - | 58 號的**前端分冊**：三端共 34 個畫面的版面、元件、四態、文案與 a11y（Buyer `B01`–`B09`／Seller `S01`–`S12`／ADM `A01`–`A13`）。端點路徑、DTO 欄名、業務碼、狀態值一律沿用 58 §5／§6，本冊不自行改契約 |
| 64 | [Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）](64-rfq-publication-seller-quote-buyer-acceptance-detailed-design.md) | `services/ai-service/internal/procurement/`、`services/java/order-service/`、`services/java/payment-service/`、`api/proto/ai/ai.proto` | - | - | confirmed RFQ snapshot **之後**的一切：publication approval／execute、投遞與可證實送達、賣家 append-only 報價版本、買家 exact-version 接受、建單與付款接點、成交溫度計（#2450／#2539／#2538／#2537）。57 號 §12 明文把這條線排除在 A–I 之外，本書補上缺的契約 SSOT。⚠️ 兩個決策先行：擴大 `payload_hash` preimage（現行只含 4 欄，證明不了「賣家報價當時看到的那張詢價單」），以及接受／建單採 outbox ＋ 冪等命令而非分散式交易 |
| 65 | [物流履約平台（Fulfillment Platform）](65-fulfillment-platform-detailed-design.md) | `services/java/order-service/`、`services/java/shipping-service/`、`services/java/payment-service/`、`gateway/internal/router/` | - | - | 訂單政策快照與收貨確認軸（order-service）、provider adapter 與動作可用性（shipping-service）、責任判定案件與事件、帳本與 provider 帳單列（payment-service）；65 是 06 號的擴充。basic-design見 `docs/basic-design/65-…` |

---

## 目前代碼庫參考現況

### 已存在的可部署服務

目前實際可對上的服務共有 18 個，其中 `services/` 目錄下有 17 個微服務（Go 7 個 + Java 10 個），另加 `gateway/` 入口服務 1 個。帳號、買家、賣家、內容等能力由單體 `backend/cmd/hawkr` 的模組承接（`internal/modules/`，行為規格在 `docs/modules/`），不在下表：

| 服務 | 技術棧 | HTTP | gRPC | 目前狀態 |
|------|--------|------|------|----------|
| gateway | Go / Gin | 9900 | - | 已部署入口 |
| social-service | Go | 9917 | 9918 | 已存在，但尚無獨立編號設計書；貼文 Hashtag 見 34 |
| im-service | Go | — | 9933 | 已編號（14）；gRPC-only，無 service-local HTTP/WS |
| ai-service | Go | 9938 | 9939 | 已編號（15）；採購助手見 55 |
| crm-service | Go | -（gRPC-only） | 10030 | 已編號（16） |
| review-service | Go | 10032 | 10033 | 已編號（43） |
| notification-service | Go | -（gRPC-only） | 10037 | 已編號（12 / 13） |
| pricebook-service | Go | -（gRPC-only） | 10039 | 已編號（61 各品項自適應 P50 色階、62 全市場品項行情，皆為主題型） |
| product-service | Java / Spring Boot | 9912 | 9913 | 已編號（02） |
| order-service | Java / Spring Boot | 9914 | 9915 | 已編號（03） |
| aftersales-service | Java / Spring Boot | 9922 | 9923 | 已編號（09） |
| payment-service | Java / Spring Boot | 9924 | 9925 | 已編號（05） |
| cart-service | Java / Spring Boot | 9926 | 9927 | 已編號（04） |
| shipping-service | Java / Spring Boot | 9930 | 9931 | 已編號（06） |
| media-service | Java / Spring Boot | 9936 | 9937 | 已編號（08） |
| payuni-webhook-service | Java / Spring Boot | 9946 | 9947 | 已編號（10） |
| coupon-service | Java / Spring Boot | 9950 | 9951 | 已編號（07） |

### 共用模組

| 路徑 | 用途 |
|------|------|
| `services/java/shared/common-core/` | Java 共用核心元件 |
| `services/java/shared/java-proto/` | Java gRPC / protobuf 生成與共用定義 |
| `services/java/shared/observability-common/` | Java 觀測性共用配置 |
| `services/java/shared/payuni-java-client/` | PayUni Java client |
| `shared/events/` | 事件定義與共享資料 |
| `pkg/` | Go 共用套件，例如 config、database、mq、redis、otel |

---

## 尚未對應獨立設計書的服務

下列服務已存在於 repo 與 k8s values，但目前沒有對應的獨立編號detailed-design：

| 服務 | 程式碼路徑 | 備註 |
|------|-----------|------|
| Social Service | `services/social-service/` | 關注、粉絲、社交資料；僅貼文 Hashtag 子模組有 34 號 |
| Pricebook Service | `services/pricebook-service/` | 價格簿；主題型設計書見 61（自適應 P50 色階）、62（全市場品項行情），無服務級設計書 |

這裡僅作索引補充，方便讀者理解 repo 現況；不代表這些服務必須補齊正式detailed-design。

---

## 分類說明

### 對外入口

- `gateway/` 是目前唯一對外 HTTP 入口。
- Gateway 透過反向代理與 gRPC client 串接多個後端服務。

### Go 微服務

- 目前實際存在的 Go 服務共 11 個：account、ai、buyer、content、crm、im、notification、pricebook、review、seller、social（清單以 `go.work` 的 `use` 區塊為準，勿背誦本行）。
- 這批服務的現況可優先參考 `k8s/values/*.yaml` 與各自 `internal/config`，但是否有對應detailed-design，仍以實際文件維護範圍為準。

### Java 微服務

- 目前實際存在的 Java 服務共 9 個：product、order、aftersales、payment、cart、shipping、media、payuni-webhook、coupon（清單以 `ls services/java/` 為準，勿背誦本行）。
- 這批服務的實際端口以 `k8s/values/*.yaml` 為準；`application.yml` 內的預設值僅在完全不帶 env 時生效，部分服務（例如 shipping）兩者並不相同。

### 跨服務主題文件

- 17 號之後的編號多數是流程、共用能力或跨 repo 主題文件，不等同於獨立可部署服務（服務級設計書集中在 01-16、40、43）。
- 這些文件應描述跨服務協作，不應再用單一路徑硬性代表全部實作。
- 編號 42 不使用，且不回收——身份主題的設計書是 44 號《帳號與賣家買家身份模型》。

---

## 物流領域：basic-design ↔ detailed-design 一一對應

> 🔴 **編號一一對應，06 對 06、65 對 65、…、70 對 70。**
> 每一本detailed-design都有編號相同的basic-design，不共用、不概括。

| # | basic-design（外部設計：為什麼／誰在用／業務怎麼走／錢與發票怎麼流） | detailed-design（內部設計：怎麼做） |
|---|---|---|
| 06 | [物流微服務（Shipping Service）](../basic-design/06-shipping-service-basic-design.md) | [06 詳設](06-shipping-service-detailed-design.md) |
| 65 | [物流履約平台（Fulfillment Platform）](../basic-design/65-fulfillment-platform-basic-design.md)（**共用骨架**） | [65 詳設](65-fulfillment-platform-detailed-design.md) |

---

## 未編號文件

（目前無）

---

## 更新原則

- 服務端口以 `k8s/values/*.yaml` 為主，並交叉比對服務內建設定。
- 服務是否存在，以 repo 內 `services/<name>/` 與對應 k8s values 是否同時存在為準。
- 若設計書索引中的服務定位、端口或路徑明顯過時，可適度更新。
- 若 repo 現況與設計書覆蓋範圍不同，優先保留設計書索引定位，並在參考現況段落補充說明即可。
