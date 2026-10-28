# Hawkr 後端文件總覽

> 本專案所有文件的索引。點擊連結查看各文件。

---

## 基本設計書

**外部設計**：這個功能為什麼存在、誰在用、業務怎麼運作、錢與發票怎麼流、有哪些畫面。
先讀它再讀詳細設計書。[完整總覽與公版章節](基本設計書/00-總覽.md)

| 編號 | 基本設計書 | 對應詳細設計書 |
|---|---|---|
| 56 | [ADM 代行營運（Delegated Operations）](基本設計書/56-ADM%20代行營運%EF%BC%88Delegated%20Operations%EF%BC%89基本設計書.md) | [56](詳細設計書/56-ADM%20代行營運%EF%BC%88Delegated%20Operations%EF%BC%89詳細設計書.md) |
| 58 | [電子發票服務（Invoice Service）](基本設計書/58-電子發票服務%EF%BC%88Invoice%20Service%EF%BC%89基本設計書.md) | [58](詳細設計書/58-電子發票服務%EF%BC%88Invoice%20Service%EF%BC%89詳細設計書.md)、[58A](詳細設計書/58A-電子發票前端實作規格.md) |
| 06 | [物流微服務（Shipping Service）](基本設計書/06-物流微服務%EF%BC%88Shipping%20Service%EF%BC%89基本設計書.md) | [06](詳細設計書/06-物流微服務%EF%BC%88Shipping%20Service%EF%BC%89詳細設計書.md) |
| 65 | [物流履約平台（Fulfillment Platform）](基本設計書/65-物流履約平台%EF%BC%88Fulfillment%20Platform%EF%BC%89基本設計書.md)（共用骨架） | [65](詳細設計書/65-物流履約平台%EF%BC%88Fulfillment%20Platform%EF%BC%89詳細設計書.md)、[65A](詳細設計書/65A-物流履約前端實作規格.md) |
| 66 | [Uber Direct 即時配送整合](基本設計書/66-Uber%20Direct%20即時配送整合基本設計書.md) | [66](詳細設計書/66-Uber%20Direct%20即時配送整合詳細設計書.md) |
| 67 | [定期配送訂閱（Subscription Delivery）](基本設計書/67-定期配送訂閱%EF%BC%88Subscription%20Delivery%EF%BC%89基本設計書.md) | [67](詳細設計書/67-定期配送訂閱%EF%BC%88Subscription%20Delivery%EF%BC%89詳細設計書.md) |
| 68 | [7-ELEVEN 大宗寄倉](基本設計書/68-7-ELEVEN%20大宗寄倉（B2C）基本設計書.md) | [68](詳細設計書/68-7-ELEVEN%20大宗寄倉%EF%BC%88B2C%EF%BC%89詳細設計書.md) |
| 69 | [商家自有出貨（Merchant Self-Delivery）](基本設計書/69-商家自有出貨%EF%BC%88Merchant%20Self-Delivery%EF%BC%89基本設計書.md) | [69](詳細設計書/69-商家自有出貨%EF%BC%88Merchant%20Self-Delivery%EF%BC%89詳細設計書.md) |
| 70 | [物流財務結算（Logistics Settlement & Receivable）](基本設計書/70-物流財務結算%EF%BC%88Logistics%20Settlement%20%26%20Receivable%EF%BC%89基本設計書.md) | [70](詳細設計書/70-物流財務結算%EF%BC%88Logistics%20Settlement%20%26%20Receivable%EF%BC%89詳細設計書.md) |

🔴 **物流領域（06／65／66／67／68／69／70）已建立編號一一對應的基本設計書**（2026-09-11）。
65A 是前端實作規格，不建立對應的基本設計書。

---

## 詳細設計書

系統程式碼的設計規格文件。[完整總覽](詳細設計書/00-總覽.md)｜新增設計書請套用[詳細設計書範本](詳細設計書/00-詳細設計書範本.md)

### 核心微服務（01-11）

| 編號 | 設計書 | 對應程式碼 |
|------|--------|-----------|
| 01 | [閘道微服務（Gateway）](詳細設計書/01-閘道微服務%EF%BC%88Gateway%EF%BC%89詳細設計書.md) | `gateway/` |
| 02 | [產品微服務（Product Service）](詳細設計書/02-產品微服務%EF%BC%88Product%20Service%EF%BC%89詳細設計書.md) | `services/java/product-service/` |
| 03 | [訂單微服務（Order Service）](詳細設計書/03-訂單微服務%EF%BC%88Order%20Service%EF%BC%89詳細設計書.md) | `services/java/order-service/` |
| 04 | [購物車微服務（Cart Service）](詳細設計書/04-購物車微服務%EF%BC%88Cart%20Service%EF%BC%89詳細設計書.md) | `services/java/cart-service/` |
| 05 | [支付服務（Payment Service）](詳細設計書/05-支付服務%EF%BC%88Payment%20Service%EF%BC%89詳細設計書.md) | `services/java/payment-service/` |
| 06 | [物流微服務（Shipping Service）](詳細設計書/06-物流微服務%EF%BC%88Shipping%20Service%EF%BC%89詳細設計書.md) | `services/java/shipping-service/` |
| 07 | [優惠券微服務（Coupon Service）](詳細設計書/07-優惠券微服務%EF%BC%88Coupon%20Service%EF%BC%89詳細設計書.md) | `services/java/coupon-service/` |
| 08 | [多媒體微服務（Media Service）](詳細設計書/08-多媒體微服務%EF%BC%88Media%20Service%EF%BC%89詳細設計書.md) | `services/java/media-service/` |
| 09 | [售後微服務（AfterSales Service）](詳細設計書/09-售後微服務%EF%BC%88AfterSales%20Service%EF%BC%89詳細設計書.md) | `services/java/aftersales-service/` |
| 10 | [PayUni Webhook 服務](詳細設計書/10-PayUni%20Webhook%20服務%EF%BC%88payuni-webhook-service%EF%BC%89詳細設計書.md) | `services/java/payuni-webhook-service/` |
| 11 | [PayUni Java Client](詳細設計書/11-PayUni%20Java%20Client%EF%BC%88payuni-java-client%EF%BC%89詳細設計書.md) | `services/java/shared/payuni-java-client/` |

### 輔助微服務（12-16）

| 編號 | 設計書 | 對應程式碼 |
|------|--------|-----------|
| 12 | [通知微服務（Notification Service）](詳細設計書/12-通知微服務%EF%BC%88Notification%20Service%EF%BC%89詳細設計書.md) | `services/notification-service/` |
| 13 | [通知模板引擎與收件箱（Notification Template）](詳細設計書/13-通知模板引擎與收件箱%EF%BC%88Notification%20Template%EF%BC%89詳細設計書.md) | `services/notification-service/` |
| 14 | [即時通訊微服務（IM Service）](詳細設計書/14-即時通訊微服務%EF%BC%88IM%20Service%EF%BC%89詳細設計書.md) | `services/im-service/` |
| 15 | [AI 微服務（AI Service）](詳細設計書/15-AI%20微服務%EF%BC%88AI%20Service%EF%BC%89詳細設計書.md) | `services/ai-service/` |
| 16 | [顧客管理微服務（CRM Service）](詳細設計書/16-顧客管理微服務%EF%BC%88CRM%20Service%EF%BC%89詳細設計書.md) | `services/crm-service/` |

### 身分與認證（17-19）

| 編號 | 設計書 | 對應程式碼 |
|------|--------|-----------|
| 18 | [賣家實名認證（Merchant Certification）](詳細設計書/18-賣家實名認證%EF%BC%88Merchant%20Certification%EF%BC%89詳細設計書.md) | `services/java/product-service/` |
| 19 | [農業認證申請與審核（Product Certification Application & Review）](詳細設計書/19-農業認證驗證%EF%BC%88Certification%20Verification%EF%BC%89詳細設計書.md) | `services/java/product-service/` |

### 功能模組與跨服務主題（20-56）

| 編號 | 設計書 | 對應程式碼 |
|------|--------|-----------|
| 20 | [結帳模組（Checkout Module）](詳細設計書/20-結帳模組%EF%BC%88Checkout%20Module%EF%BC%89詳細設計書.md) | 跨服務模組 |
| 21 | [運費計算模組（Shipping Fee Module）](詳細設計書/21-運費計算模組%EF%BC%88Shipping%20Fee%20Module%EF%BC%89詳細設計書.md) | `services/java/shared/common-core/` |
| 22 | [商品配送驗證（Product Shipping Validation）](詳細設計書/22-商品配送驗證%EF%BC%88Product%20Shipping%20Validation%EF%BC%89詳細設計書.md) | 跨服務模組 |
| 23 | [門市管理模組（Store Pickup Management）](詳細設計書/23-門市管理模組%EF%BC%88Store%20Pickup%20Management%EF%BC%89詳細設計書.md) | `services/java/shipping-service/` |
| 24 | [節假日管理（Holiday Management）](詳細設計書/24-節假日管理%EF%BC%88Holiday%20Management%EF%BC%89詳細設計書.md) | `services/java/shipping-service/` |
| 26 | [輸入驗證模組（Java InputValidator）](詳細設計書/26-輸入驗證模組%EF%BC%88Java%20InputValidator%EF%BC%89詳細設計書.md) | `services/java/shared/common-core/` |
| 27 | [地址搜尋與驗證（Address Proxy）](詳細設計書/27-地址搜尋與驗證%EF%BC%88Address%20Proxy%EF%BC%89詳細設計書.md) | `gateway/` |
| 28 | [商品瀏覽統計（Product View Stats）](詳細設計書/28-商品瀏覽統計%EF%BC%88Product%20View%20Stats%EF%BC%89詳細設計書.md) | `services/seller-service/`、`gateway/` |
| 29 | [Admin 異常訂單管理 API](詳細設計書/29-Admin%20異常訂單管理%20API%20詳細設計書.md) | 跨 Payment / Shipping / AfterSales / Order |
| 30 | [Sign in with Apple 認證（Apple OAuth）](詳細設計書/30-Sign%20in%20with%20Apple%20認證%EF%BC%88Apple%20OAuth%EF%BC%89詳細設計書.md) | `internal/modules/identity/`、`internal/httpapi/identity/` |
| 31 | [內容檢舉與封鎖（Content Reporting & Block）](詳細設計書/31-內容檢舉與封鎖%EF%BC%88Content%20Reporting%20&%20Block%EF%BC%89詳細設計書.md) | `gateway/`、`services/{social,im,review}-service/`、`pkg/{blocklist,moderation}` |
| 32 | [商家治理與食安通報（Seller Governance & Food Safety）](詳細設計書/32-商家治理與食安通報%EF%BC%88Seller%20Governance%20&%20Food%20Safety%EF%BC%89詳細設計書.md) | `gateway/`、`services/seller-service/`、`pkg/mq` |
| 33 | [商品 Hashtag 模組（Product Hashtag）](詳細設計書/33-商品%20Hashtag%20模組%EF%BC%88Product%20Hashtag%EF%BC%89詳細設計書.md) | `services/java/product-service/` |
| 34 | [貼文 Hashtag 模組（Post Hashtag）](詳細設計書/34-貼文%20Hashtag%20模組%EF%BC%88Post%20Hashtag%EF%BC%89詳細設計書.md) | `services/social-service/` |
| 35 | [商品列表排序與銷量反正規化（Product Sort & Sales Count）](詳細設計書/35-商品列表排序與銷量反正規化%EF%BC%88Product%20Sort%20&%20Sales%20Count%EF%BC%89詳細設計書.md) | `services/java/product-service/`、`services/java/order-service/`、`gateway/` |
| 36 | [MQ 與事件總覽（MQ & Event Catalog）](詳細設計書/36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md) | 跨全部 Java / Go 微服務（MQ 設計 SSOT） |
| 38 | [Flyway 自動同步與 DB 自動初始化](詳細設計書/38-Flyway%20自動同步與%20DB%20自動初始化%EF%BC%88Flyway%20Auto%20Sync%20&%20DB%20Auto%20Init%EF%BC%89詳細設計書.md) | 10 個 Java 微服務、`services/java/shared/common-core/` |
| 39 | [買家服務（Buyer Service）](詳細設計書/39-買家服務%EF%BC%88Buyer%20Service%EF%BC%89詳細設計書.md)（指引頁） | `internal/modules/buyer/`、`internal/httpapi/buyer/`、`internal/legacy/buyerserver/` |
| 40 | [商品匯入精靈服務（Product Import Service）](詳細設計書/40-商品匯入精靈服務%EF%BC%88Product%20Import%20Service%EF%BC%89詳細設計書.md) | `services/product-import-service/` |
| 41 | [Realtime SSE 推播平台（Realtime SSE Push Platform）](詳細設計書/41-Realtime%20SSE%20推播平台%EF%BC%88Realtime%20SSE%20Push%20Platform%EF%BC%89詳細設計書.md) | `gateway/`、`services/social-service/`、`services/notification-service/` |
| 43 | [評論微服務（Review Service）](詳細設計書/43-評論微服務%EF%BC%88Review%20Service%EF%BC%89詳細設計書.md) | `services/review-service/`、`gateway/` |
| 44 | [帳號與賣家買家身份模型（Account & Seller-Buyer Identity）](詳細設計書/44-帳號與賣家買家身份模型%EF%BC%88Account%20&%20Seller-Buyer%20Identity%EF%BC%89詳細設計書.md) | `internal/modules/seller`（身份部分）、`gateway/`、跨角色守衛（單體 identity、buyer、accountdeletion 的規格在 `docs/modules/`） |
| 45 | [首頁區塊動態排序與營運干預（Homepage Dynamic Layout & Merchandising）](詳細設計書/45-首頁區塊動態排序與營運干預%EF%BC%88Homepage%20Dynamic%20Layout%20&%20Merchandising%EF%BC%89詳細設計書.md) | draft，尚未實作（無對應程式碼） |
| 46 | [賣家能力權限層（Seller Capability Authorization）](詳細設計書/46-賣家能力權限層%EF%BC%88Seller%20Capability%20Authorization%EF%BC%89詳細設計書.md) | `gateway/internal/authz/`、`services/seller-service/` |
| 47 | [GA4 分析追蹤（GA4 Cross-Platform Analytics）](詳細設計書/47-GA4%20分析追蹤%EF%BC%88GA4%20Cross-Platform%20Analytics%EF%BC%89詳細設計書.md) | `hawkr-website/`、`hawkr_buyer/`、`hawkr_seller/` |
| 48 | [SEO 優化與現況（SEO Optimization）](詳細設計書/48-SEO%20優化與現況%EF%BC%88SEO%20Optimization%EF%BC%89詳細設計書.md) | `hawkr-website/lib/seo/`、`hawkr-admin/` |
| 49 | [多媒體儲存與圖片服務（Cloudflare R2 & Image Gateway）](詳細設計書/49-多媒體儲存與圖片服務%EF%BC%88Cloudflare%20R2%20&%20Image%20Gateway%EF%BC%89詳細設計書.md) | `services/java/media-service/`、`workers/image-gateway/` |
| 50 | [食譜內容管理（Recipe Content Management）](詳細設計書/50-食譜內容管理%EF%BC%88Recipe%20Content%20Management%EF%BC%89詳細設計書.md) | `internal/modules/content/`、`internal/httpapi/content/`、`hawkr-admin/` |
| 51 | [簡訊發送與手機 OTP（Mitake SMS & Phone OTP）](詳細設計書/51-簡訊發送與手機%20OTP%EF%BC%88Mitake%20SMS%20&%20Phone%20OTP%EF%BC%89詳細設計書.md) | `serverless/egress-proxy/`、`internal/modules/identity/`、`internal/httpapi/identity/` |
| 52 | [微服務間內部授權（Internal Service Authorization）](詳細設計書/52-微服務間內部授權%EF%BC%88Internal%20Service%20Authorization%EF%BC%89詳細設計書.md) | `services/java/shared/common-core/`、`pkg/middleware/grpc/interceptor/`、`k8s/external-services/networkpolicy.yaml` |
| 53 | [Web 前端快取架構與版本一致性（Web Frontend Cache & Deployment Consistency）](詳細設計書/53-Web%20前端快取架構與版本一致性%EF%BC%88Web%20Frontend%20Cache%20%26%20Deployment%20Consistency%EF%BC%89詳細設計書.md) | `hawkr-website/lib/api/cached.ts`、`hawkr-website/open-next.config.ts`、`hawkr-website/app/api/revalidate/route.ts` |
| 55 | [Hawkr 幫找採購助手（Procurement Assistant）](詳細設計書/55-Hawkr%20幫找採購助手%EF%BC%88Procurement%20Assistant%EF%BC%89詳細設計書.md) | `services/ai-service/internal/procurement/`、`gateway/internal/handler/ai.go`、`hawkr-website/features/procurement-assistant/` |
| 56 | [ADM 代行營運（Delegated Operations）](詳細設計書/56-ADM%20代行營運%EF%BC%88Delegated%20Operations%EF%BC%89詳細設計書.md) | `internal/modules/delegation/`（規格見 `modules/delegation.md`）、`internal/legacy/delegationservice/`、`gateway/internal/authz/`、`gateway/internal/router/`、`api/proto/common/delegation.proto`、`hawkr-admin/src/features/delegated-operations/` |
| 64 | [Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）](詳細設計書/64-Hawkr%20Agent%20%E5%95%86%E5%8B%99%E7%B7%9A%EF%BC%88RFQ%20Publication%E3%80%81Seller%20Quote%E3%80%81Buyer%20Acceptance%EF%BC%89%E8%A9%B3%E7%B4%B0%E8%A8%AD%E8%A8%88%E6%9B%B8.md) | 由另一條 lane 維護；本列僅補登記（`00-總覽.md` 已有，`_sidebar`／`index` 先前漏登） |
| 65 | [物流履約平台（Fulfillment Platform）](詳細設計書/65-物流履約平台%EF%BC%88Fulfillment%20Platform%EF%BC%89詳細設計書.md) | `services/java/shipping-service/`（擴充）、`services/java/payment-service/`（線 B ⇒ Logistics Finance module）、本波新增 Recurring（Go）、`api/proto/shipping/`（draft，僅稽核附錄與骨架已定版） |
| 65A | [物流履約前端實作規格](詳細設計書/65A-物流履約前端實作規格.md) | 四個前端 repo（website／admin／buyer／seller），43 個畫面（draft，骨架已定版） |
| 66 | [Uber Direct 即時配送整合](詳細設計書/66-Uber%20Direct%20即時配送整合詳細設計書.md) | `services/java/shipping-service/`（provider adapter）、`services/java/payment-service/`（Logistics Finance module）（draft） |
| 67 | [定期配送訂閱（Subscription Delivery）](詳細設計書/67-定期配送訂閱%EF%BC%88Subscription%20Delivery%EF%BC%89詳細設計書.md) | 本波新增 `services/recurring-service/`、`services/java/payment-service/`（mandate 擴充）（draft） |
| 68 | [7-ELEVEN 大宗寄倉（B2C）](詳細設計書/68-7-ELEVEN%20大宗寄倉%EF%BC%88B2C%EF%BC%89詳細設計書.md) | `services/java/shipping-service/`、`services/java/payuni-webhook-service/`（draft） |
| 69 | [商家自有出貨（Merchant Self-Delivery）](詳細設計書/69-商家自有出貨%EF%BC%88Merchant%20Self-Delivery%EF%BC%89詳細設計書.md) | `services/java/shipping-service/`、賣家 App（draft） |
| 70 | [物流財務結算（Logistics Settlement & Receivable）](詳細設計書/70-物流財務結算%EF%BC%88Logistics%20Settlement%20%26%20Receivable%EF%BC%89詳細設計書.md) | 🔴 **Runtime Owner ＝ `services/java/payment-service/`（Logistics Finance module）**、`services/java/shipping-service/`、`services/invoice-service/`（draft） |

---

## 開發規範

| 文件 | 說明 |
|------|------|
| [API 設計規範](開發規範/API%20設計規範.md) | **★ 最高優先級** — 所有 API 新增/修改必須以此為最高依據 |
| [開發工作流程](開發規範/開發工作流程.md) | 設計書 → Code Gen → DB → 程式碼 → 測試 強制順序、設計書品質、環境變數配置流程 |
| [Git 分支與交付路徑規範](開發規範/Git%20分支與交付路徑規範.md) | 分支策略（PR base 一律 dev）、各 repo 交付路徑與把關者、Flutter 放行六條件、跨 repo 破壞性遷移判準 |
| [Java 微服務開發規範](開發規範/Java%20微服務開發規範.md) | Java 微服務通用開發規範 |
| [Proto 管理規範](開發規範/Proto%20管理規範.md) | Protobuf 定義與同步流程 |
| [Java gRPC 錯誤處理規範](開發規範/Java%20gRPC%20錯誤處理規範.md) | gRPC 錯誤碼與例外處理 |
| [Go gRPC 錯誤處理規範](開發規範/Go%20gRPC%20錯誤處理規範.md) | Go 服務的 gRPC 錯誤碼與 usecase 錯誤轉譯 |
| [PostgreSQL 設計規範與最佳實踐](開發規範/PostgreSQL%20設計規範與最佳實踐.md) | 資料庫設計規範（含時區策略） |
| [DB Migration 相容性規範（expand-contract）](開發規範/DB%20Migration%20相容性規範（expand-contract）.md) | 新增欄位／改型別／刪欄位的擴張—收縮流程與回填要求 |
| [MQ 與事件管理規範](開發規範/MQ%20與事件管理規範.md) | 新增/修改 MQ 事件的強制流程（含 topology.yaml 同 PR 登記） |
| [新增微服務指南](開發規範/新增微服務指南.md) | 從零建立一個新微服務的骨架與接線步驟 |
| [後端服務責任邊界規範](開發規範/後端服務責任邊界規範.md) | 什麼寫進 application code、什麼走一次性維運 SQL/script |
| [賣家能力權限層規範](開發規範/賣家能力權限層規範.md) | 賣家路由必宣告 capability、停權 policy、deny-by-default |
| [多媒體與圖片 URL 規範](開發規範/多媒體與圖片%20URL%20規範.md) | DB/MQ 只存 objectKey、URL 組裝唯一入口、preset 白名單 |
| [Web 前端契約同步規範](開發規範/Web%20前端契約同步規範.md) | 後端契約變更時與 web 前端的同步責任 |
| [固定 IP 出口與第三方轉發規範（Egress Proxy）](開發規範/固定%20IP%20出口與第三方轉發規範（Egress%20Proxy）.md) | 第三方要求來源 IP 白名單時一律收斂到 egress-proxy |
| [Cloudflare Worker 與 Serverless 部署治理規範](開發規範/Cloudflare%20Worker%20與%20Serverless%20部署治理規範.md) | Worker/Cloud Run 源碼入版控、CI 部署、zone 配置快照稽核 |
| [PayUni 錯誤回應收斂規範](開發規範/PayUni%20錯誤回應收斂規範.md) | PayUni 錯誤碼三層語意分離（平台 / Provider SoT / Snapshot） |
| [Gateway Swagger 註解規範](開發規範/Gateway%20Swagger%20註解規範.md) | Gateway handler Swagger 必要欄位、DTO 規範、test mock 同步 |
| [硬上限依賴接入治理規範（配額與容量）](開發規範/硬上限依賴接入治理規範（配額與容量）.md) | 有硬上限的依賴（配額 / maxmemory / 連線數 / rate limit）接入前必備：指標＋告警＋預算數字 |
| [多副本排程搶佔規範](開發規範/多副本排程搶佔規範.md) | `replicas ≥ 2` 下排程不重複執行：**prod 副本數現況與查證方式（§〇，判定輸入）**、搶佔必在副作用前、一律 lease 不用永久旗標、永久旗標六步判準、租約長度 < 候選窗口、例外 A/B |

---

## 第三方整合

| 文件 | 說明 |
|------|------|
| [PayUni 金流官方 API 總覽](第三方整合/PayUni%20金流官方%20API%20總覽.md) | PayUni 金流 API 規格 |
| [PayUni 逆向金流 API 總覽](第三方整合/PayUni%20逆向金流%20API%20總覽.md) | PayUni 退款/取消 API |
| [PayUni 物流官方 API 總覽](第三方整合/PayUni%20物流官方%20API%20總覽.md) | PayUni 物流 API 規格 |
| [PayUni 代理商專用 API 總覽](第三方整合/PayUni%20代理商專用%20API%20總覽.md) | 代理商多功能收付/撥款/合作商店 API |
| [PayUni 費用一覽](第三方整合/PayUni%20費用一覽.md) | PayUni 各項費率 |
| [TGOS API 整合指南](第三方整合/TGOS%20API%20整合指南.md) | 內政部 TGOS 地址正規化 / 定位 API 介接 |
| [Uber Direct API 總覽](第三方整合/Uber%20Direct%20API%20總覽.md) | Uber Direct（DaaS）API 規格：認證、報價建單、狀態值域、webhook 驗簽 |

---

## 代碼管理（CodeMeta）

| 文件 | 說明 |
|------|------|
| [README](代碼管理%EF%BC%88CodeMeta%EF%BC%89/README.md) | CodeMeta 體系總覽 |
| [錯誤碼](代碼管理%EF%BC%88CodeMeta%EF%BC%89/錯誤碼.md) | 系統錯誤碼定義 |
| [業務狀態碼](代碼管理%EF%BC%88CodeMeta%EF%BC%89/業務狀態碼.md) | 業務狀態碼定義 |
| [業務類型碼](代碼管理%EF%BC%88CodeMeta%EF%BC%89/業務類型碼.md) | 業務類型碼定義 |
| [內部技術碼](代碼管理%EF%BC%88CodeMeta%EF%BC%89/內部技術碼.md) | 內部技術碼定義 |
| [踩坑與變遷紀錄](代碼管理%EF%BC%88CodeMeta%EF%BC%89/踩坑與變遷紀錄.md) | CodeMeta 生成體系的演進決策與踩坑 |

---

## 基礎建設

| 文件 | 說明 |
|------|------|
| [系統架構與資料流向](基礎建設/系統架構與資料流向.md) | 全系統元件、對外入口與資料流向總圖 |
| [CI/CD 自動化部署（K8s + Helm）](基礎建設/CI_CD%20自動化部署%EF%BC%88K8s%20+%20Helm%EF%BC%89.md) | 部署流程與配置 |
| [系統監控架構（Monitoring）](基礎建設/系統監控架構%EF%BC%88Monitoring%EF%BC%89.md) | 監控與告警架構 |
| [監控架構完整教學 (Monitoring Architecture Guide)](基礎建設/監控架構完整教學%20%28Monitoring%20Architecture%20Guide%29.md) | 監控 stack 從零理解與操作的教學版 |
| [Discord 監控 Bot 設計書](基礎建設/Discord%20監控%20Bot%20設計書.md) | 監控告警轉 Discord 的 bot 設計 |
| [Cloudflare CDN 與 DDoS 防護配置指南](基礎建設/Cloudflare%20CDN%20與%20DDoS%20防護配置指南.md) | CDN 與安全配置 |
| [Discord 告警通知設定指南](基礎建設/Discord%20告警通知設定指南.md) | 從零建立 Discord Server 到告警通知 |
| [ATM 退款結果 Webhook 遺失維運處置](基礎建設/ATM%20退款結果%20Webhook%20遺失維運處置.md) | ATM 退款 §6.7 webhook 遺失卡單的診斷與回復 runbook |
| [業務告警分流與 dev-警報 路由設定](基礎建設/業務告警分流與%20dev-警報%20路由設定.md) | 業務告警→dev-警報/prod-警報 頻道分流路由基建（#898/#1353） |
| [MQ Topology 管理（Terraform）](基礎建設/MQ%20Topology%20管理%EF%BC%88Terraform%EF%BC%89.md) | RabbitMQ exchange/queue/binding 的新增、修改、刪除流程與環境現況 |
| [Prod RabbitMQ 3-Node Cluster](基礎建設/Prod%20RabbitMQ%203-Node%20Cluster.md) | Prod MQ 三節點 cluster 的節點配置、bootstrap 與 cutover |
| [Tailscale VPN 連線 Prod 機器指南](基礎建設/Tailscale%20VPN%20連線%20Prod%20機器指南.md) | 透過 Tailscale subnet router 連 Prod 私網（SSH / DB / k8s API） |

---

## 速查參考

| 文件 | 說明 |
|------|------|
| [常用指令](速查參考/常用指令.md) | 開發常用指令速查 |
| [資料庫管理命令速查表](速查參考/資料庫管理命令速查表.md) | DB 管理指令 |
| [Redis 快取管理](速查參考/Redis%20快取管理.md) | Redis key 清冊（含 session / 限流）、清除方式、**§5 512MB 容量契約與失效模型** |
| [前端屏幕與 API 映射表](速查參考/前端屏幕與%20API%20映射表.md) | 前端頁面對應的 API 端點 |
| [Apple Pay Merchant Cert 輪替 SOP](速查參考/Apple%20Pay%20Merchant%20Cert%20輪替%20SOP.md) | Apple Pay 商家憑證到期前的輪替步驟 |

---

## Runbook

| 文件 | 說明 |
|------|------|
| [食安事件處理 Runbook](runbooks/food-safety-incident-runbook.md) | `food_safety_reports`（critical，4 小時 SLA）從受理到處置的操作步驟與 SQL |
| [賣家違規管理 Runbook](runbooks/seller-violation-management-runbook.md) | `seller_violation_reports` high/medium/low 檢舉與 admin 主動建立 violation |
| [賣家治理共用判斷標準](runbooks/seller-governance-shared.md) | 上兩份 runbook 共用的 penalty 選用、public_summary 撰寫、申訴決議標準（SSOT） |
| [App 版本門檻 Runbook](runbooks/app-version-gate.md) | 強更沒生效或全體被鎖的排查、各版本請求佔比的讀法、舊版 App 的放行判準 |
| [商品 re-key backfill gate](runbooks/product-rekey-backfill-gate.md) | seller/store aggregate re-key 的人工 gate（跨 `hawkr_seller`／`hawkr_product` 兩 DB） |

---

## 補充資料

| 文件 | 說明 |
|------|------|
| [商品匯入精靈 E2E 樣本 CSV](補充資料/product_import_e2e_sample.md) | 40 號商品匯入精靈的 E2E 測試樣本資料 |
| [45 首頁動態區塊排序與營運干預提案（PM 版）](補充資料/45-首頁動態區塊排序與營運干預提案%EF%BC%88PM版%EF%BC%89.md) | 45 號設計書的 PM 版需求提案原文 |
