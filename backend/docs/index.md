# Hawkr 後端文件總覽

> 本專案所有文件的索引。點擊連結查看各文件。

---

## 基本設計書

**外部設計**：這個功能為什麼存在、誰在用、業務怎麼運作、錢與發票怎麼流、有哪些畫面。
先讀它再讀詳細設計書。[完整總覽與公版章節](basic-design/00-overview.md)

| 編號 | 基本設計書 | 對應詳細設計書 |
|---|---|---|
| 56 | [ADM 代行營運（Delegated Operations）](basic-design/56-delegated-operations-basic-design.md) | [56](detailed-design/56-delegated-operations-detailed-design.md) |
| 58 | [電子發票服務（Invoice Service）](basic-design/58-invoice-service-basic-design.md) | [58](detailed-design/58-invoice-service-detailed-design.md)、[58A](detailed-design/58a-e-invoice-frontend-implementation-specification.md) |
| 06 | [物流微服務（Shipping Service）](basic-design/06-shipping-service-basic-design.md) | [06](detailed-design/06-shipping-service-detailed-design.md) |
| 65 | [物流履約平台（Fulfillment Platform）](basic-design/65-fulfillment-platform-basic-design.md)（共用骨架） | [65](detailed-design/65-fulfillment-platform-detailed-design.md) |

🔴 **物流領域（06／65）編號一一對應基本設計書。**

---

## 詳細設計書

系統程式碼的設計規格文件。[完整總覽](detailed-design/00-overview.md)｜新增設計書請套用[詳細設計書範本](detailed-design/00-detailed-design-template.md)

### 核心微服務（01-11）

| 編號 | 設計書 | 對應程式碼 |
|------|--------|-----------|
| 01 | [閘道微服務（Gateway）](detailed-design/01-gateway-detailed-design.md) | `gateway/` |
| 02 | [產品微服務（Product Service）](detailed-design/02-product-service-detailed-design.md) | `services/java/product-service/` |
| 03 | [訂單微服務（Order Service）](detailed-design/03-order-service-detailed-design.md) | `services/java/order-service/` |
| 04 | [購物車微服務（Cart Service）](detailed-design/04-cart-service-detailed-design.md) | `services/java/cart-service/` |
| 06 | [物流微服務（Shipping Service）](detailed-design/06-shipping-service-detailed-design.md) | `services/java/shipping-service/` |
| 07 | [優惠券微服務（Coupon Service）](detailed-design/07-coupon-service-detailed-design.md) | `services/java/coupon-service/` |
| 08 | [多媒體微服務（Media Service）](detailed-design/08-media-service-detailed-design.md) | `services/java/media-service/` |
| 09 | [售後微服務（AfterSales Service）](detailed-design/09-aftersales-service-detailed-design.md) | `services/java/aftersales-service/` |
| 10 | [PayUni Webhook 服務](detailed-design/10-payuni-webhook-service-detailed-design.md) | `services/java/payuni-webhook-service/` |
| 11 | [PayUni Java Client](detailed-design/11-payuni-java-client-detailed-design.md) | `services/java/shared/payuni-java-client/` |

### 輔助微服務（12-16）

| 編號 | 設計書 | 對應程式碼 |
|------|--------|-----------|
| 12 | [通知微服務（Notification Service）](detailed-design/12-notification-service-detailed-design.md) | 已退場，規格見 `backend/docs/modules/notification.md`（`internal/modules/notification/`） |
| 13 | [通知模板引擎與收件箱（Notification Template）](detailed-design/13-notification-template-detailed-design.md) | 已退場，規格見 `backend/docs/modules/notification.md`（`internal/modules/notification/`） |
| 14 | [即時通訊微服務（IM Service）](detailed-design/14-im-service-detailed-design.md) | `services/im-service/` |
| 15 | [AI 微服務（AI Service）](detailed-design/15-ai-service-detailed-design.md) | `services/ai-service/` |
| 16 | [顧客管理微服務（CRM Service）](detailed-design/16-crm-service-detailed-design.md) | `services/crm-service/` |

### 身分與認證（17-19）

| 編號 | 設計書 | 對應程式碼 |
|------|--------|-----------|
| 19 | [農業認證申請與審核（Product Certification Application & Review）](detailed-design/19-certification-verification-detailed-design.md) | `services/java/product-service/` |

### 功能模組與跨服務主題（20-56）

| 編號 | 設計書 | 對應程式碼 |
|------|--------|-----------|
| 20 | [結帳模組（Checkout Module）](detailed-design/20-checkout-module-detailed-design.md) | 跨服務模組 |
| 21 | [運費計算模組（Shipping Fee Module）](detailed-design/21-shipping-fee-module-detailed-design.md) | `services/java/shared/common-core/` |
| 22 | [商品配送驗證（Product Shipping Validation）](detailed-design/22-product-shipping-validation-detailed-design.md) | 跨服務模組 |
| 23 | [門市管理模組（Store Pickup Management）](detailed-design/23-store-pickup-management-detailed-design.md) | `services/java/shipping-service/` |
| 24 | [節假日管理（Holiday Management）](detailed-design/24-holiday-management-detailed-design.md) | `services/java/shipping-service/` |
| 26 | [輸入驗證模組（Java InputValidator）](detailed-design/26-java-inputvalidator-detailed-design.md) | `services/java/shared/common-core/` |
| 29 | [Admin 異常訂單管理 API](detailed-design/29-admin-anomalous-order-management-api-detailed-design.md) | 跨 Payment / Shipping / AfterSales / Order |
| 31 | [內容檢舉與封鎖（Content Reporting & Block）](detailed-design/31-content-reporting-and-block-detailed-design.md) | `gateway/`、`services/{social,im,review}-service/`、`pkg/{blocklist,moderation}` |
| 32 | [商家治理與食安通報（Seller Governance & Food Safety）](detailed-design/32-seller-governance-and-food-safety-detailed-design.md) | `admin/`（商家治理畫面；行為見 `backend/docs/modules/seller.md`） |
| 33 | [商品 Hashtag 模組（Product Hashtag）](detailed-design/33-product-hashtag-detailed-design.md) | `services/java/product-service/` |
| 34 | [貼文 Hashtag 模組（Post Hashtag）](detailed-design/34-post-hashtag-detailed-design.md) | `services/social-service/` |
| 35 | [商品列表排序與銷量反正規化（Product Sort & Sales Count）](detailed-design/35-product-sort-and-sales-count-detailed-design.md) | `services/java/product-service/`、`services/java/order-service/`、`gateway/` |
| 36 | [MQ 與事件總覽（MQ & Event Catalog）](detailed-design/36-mq-and-event-catalog.md) | 跨全部 Java / Go 微服務（MQ 設計 SSOT） |
| 38 | [Flyway 自動同步與 DB 自動初始化](detailed-design/38-flyway-auto-sync-and-db-auto-init-detailed-design.md) | 10 個 Java 微服務、`services/java/shared/common-core/` |
| 41 | [Realtime SSE 推播平台（Realtime SSE Push Platform）](detailed-design/41-realtime-sse-push-platform-detailed-design.md) | `gateway/`、`services/social-service/`、`services/notification-service/` |
| 43 | [評論微服務（Review Service）](detailed-design/43-review-service-detailed-design.md) | `services/review-service/`、`gateway/` |
| 44 | [帳號與賣家買家身份模型（Account & Seller-Buyer Identity）](detailed-design/44-account-and-seller-buyer-identity-detailed-design.md) | `gateway/`、跨角色守衛（賣家身份見 `backend/docs/modules/seller.md` §2.1；單體 identity、buyer、accountdeletion 的規格在 `backend/docs/modules/`） |
| 46 | [賣家能力權限層（Seller Capability Authorization）](detailed-design/46-seller-capability-authorization-detailed-design.md) | `gateway/internal/authz/`（能力推導見 `backend/docs/modules/seller.md` §10） |
| 47 | [GA4 分析追蹤（GA4 Cross-Platform Analytics）](detailed-design/47-ga4-cross-platform-analytics-detailed-design.md) | `hawkr-website/`、`hawkr_buyer/`、`hawkr_seller/` |
| 48 | [SEO 優化與現況（SEO Optimization）](detailed-design/48-seo-optimization-detailed-design.md) | `hawkr-website/lib/seo/`、`hawkr-admin/` |
| 49 | [多媒體儲存與圖片服務（Cloudflare R2 & Image Gateway）](detailed-design/49-cloudflare-r2-and-image-gateway-detailed-design.md) | `services/java/media-service/`、`workers/image-gateway/` |
| 52 | [微服務間內部授權（Internal Service Authorization）](detailed-design/52-internal-service-authorization-detailed-design.md) | `services/java/shared/common-core/`、`pkg/middleware/grpc/interceptor/` |
| 53 | [Web 前端快取架構與版本一致性（Web Frontend Cache & Deployment Consistency）](detailed-design/53-web-frontend-cache-and-deployment-consistency-detailed-design.md) | `hawkr-website/lib/api/cached.ts`、`hawkr-website/open-next.config.ts`、`hawkr-website/app/api/revalidate/route.ts` |
| 55 | [Hawkr 幫找採購助手（Procurement Assistant）](detailed-design/55-procurement-assistant-detailed-design.md) | `services/ai-service/internal/procurement/`、`gateway/internal/handler/ai.go`、`hawkr-website/features/procurement-assistant/` |
| 56 | [ADM 代行營運（Delegated Operations）](detailed-design/56-delegated-operations-detailed-design.md) | `internal/modules/delegation/`（規格見 `modules/delegation.md`）、`internal/legacy/delegationservice/`、`gateway/internal/authz/`、`gateway/internal/router/`、`api/proto/common/delegation.proto`、`hawkr-admin/src/features/delegated-operations/` |
| 64 | [Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）](detailed-design/64-rfq-publication-seller-quote-buyer-acceptance-detailed-design.md) | 由另一條 lane 維護；本列僅補登記（`00-overview.md` 已有，`_sidebar`／`index` 先前漏登） |
| 65 | [物流履約平台（Fulfillment Platform）](detailed-design/65-fulfillment-platform-detailed-design.md) | `services/java/order-service/`、`services/java/shipping-service/`、`services/java/payment-service/`（已實作的履約骨架）|

---

## 開發規範

| 文件 | 說明 |
|------|------|
| [API 設計規範](dev-guidelines/api-design-guidelines.md) | **★ 最高優先級** — 所有 API 新增/修改必須以此為最高依據 |
| [開發工作流程](dev-guidelines/development-workflow.md) | 設計書 → Code Gen → DB → 程式碼 → 測試 強制順序、設計書品質、環境變數配置流程 |
| [Git 分支與交付路徑規範](../../docs/conventions/git-branch-and-delivery-path-guidelines.md) | 分支策略（PR base 一律 dev）、各 repo 交付路徑與把關者、Flutter 放行六條件、跨 repo 破壞性遷移判準 |
| [Java 微服務開發規範](dev-guidelines/java-microservices-development-guidelines.md) | Java 微服務通用開發規範 |
| [Proto 管理規範](dev-guidelines/proto-management-guidelines.md) | Protobuf 定義與同步流程 |
| [Java gRPC 錯誤處理規範](dev-guidelines/java-grpc-error-handling-guidelines.md) | gRPC 錯誤碼與例外處理 |
| [Go gRPC 錯誤處理規範](dev-guidelines/go-grpc-error-handling-guidelines.md) | Go 服務的 gRPC 錯誤碼與 usecase 錯誤轉譯 |
| [PostgreSQL 設計規範與最佳實踐](dev-guidelines/postgresql-design-guidelines-and-best-practices.md) | 資料庫設計規範（含時區策略） |
| [DB Migration 相容性規範（expand-contract）](dev-guidelines/db-migration-compatibility-expand-contract.md) | 新增欄位／改型別／刪欄位的擴張—收縮流程與回填要求 |
| [MQ 與事件管理規範](dev-guidelines/mq-and-event-management-guidelines.md) | 新增/修改 MQ 事件的強制流程（含 topology.yaml 同 PR 登記） |
| [新增微服務指南](dev-guidelines/new-microservice-guide.md) | 從零建立一個新微服務的骨架與接線步驟 |
| [後端服務責任邊界規範](dev-guidelines/backend-service-responsibility-boundaries.md) | 什麼寫進 application code、什麼走一次性維運 SQL/script |
| [賣家能力權限層規範](dev-guidelines/seller-capability-authorization-guidelines.md) | 賣家路由必宣告 capability、停權 policy、deny-by-default |
| [多媒體與圖片 URL 規範](dev-guidelines/media-and-image-url-guidelines.md) | DB/MQ 只存 objectKey、URL 組裝唯一入口、preset 白名單 |
| [Web 前端契約同步規範](dev-guidelines/web-frontend-contract-synchronization-guidelines.md) | 後端契約變更時與 web 前端的同步責任 |
| [固定 IP 出口與第三方轉發規範（Egress Proxy）](dev-guidelines/fixed-ip-egress-proxy-guidelines.md) | 第三方要求來源 IP 白名單時一律收斂到 egress-proxy |
| [Cloudflare Worker 與 Serverless 部署治理規範](dev-guidelines/cloudflare-workers-and-serverless-deployment-governance.md) | Worker/Cloud Run 源碼入版控、CI 部署、zone 配置快照稽核 |
| [Gateway Swagger 註解規範](dev-guidelines/gateway-swagger-annotation-guidelines.md) | Gateway handler Swagger 必要欄位、DTO 規範、test mock 同步 |
| [硬上限依賴接入治理規範（配額與容量）](dev-guidelines/hard-limit-dependency-governance-quota-and-capacity-guidelines.md) | 有硬上限的依賴（配額 / maxmemory / 連線數 / rate limit）接入前必備：指標＋告警＋預算數字 |
| [多副本排程搶佔規範](dev-guidelines/multi-replica-scheduling-preemption-guidelines.md) | `replicas ≥ 2` 下排程不重複執行：**prod 副本數現況與查證方式（§〇，判定輸入）**、搶佔必在副作用前、一律 lease 不用永久旗標、永久旗標六步判準、租約長度 < 候選窗口、例外 A/B |

---

## 第三方整合

| 文件 | 說明 |
|------|------|
| [PayUni 金流官方 API 總覽](third-party-integrations/payuni-official-payment-api-overview.md) | PayUni 金流 API 規格 |
| [PayUni 逆向金流 API 總覽](third-party-integrations/payuni-reverse-payment-api-overview.md) | PayUni 退款/取消 API |
| [PayUni 代理商專用 API 總覽](third-party-integrations/payuni-reseller-api-overview.md) | 代理商多功能收付/撥款/合作商店 API |
| [PayUni 費用一覽](third-party-integrations/payuni-fees.md) | PayUni 各項費率 |
| [TGOS API 整合指南](third-party-integrations/tgos-api-integration-guide.md) | 內政部 TGOS 地址正規化 / 定位 API 介接 |

---

## 代碼管理（CodeMeta）

| 文件 | 說明 |
|------|------|
| [README](codemeta/README.md) | CodeMeta 體系總覽 |
| [錯誤碼](codemeta/error-codes.md) | 系統錯誤碼定義 |
| [業務狀態碼](codemeta/business-status-codes.md) | 業務狀態碼定義 |
| [業務類型碼](codemeta/business-type-codes.md) | 業務類型碼定義 |
| [內部技術碼](codemeta/internal-technical-codes.md) | 內部技術碼定義 |
| [踩坑與變遷紀錄](codemeta/pitfalls-and-change-history.md) | CodeMeta 生成體系的演進決策與踩坑 |

---

## 基礎建設

| 文件 | 說明 |
|------|------|
| [系統架構與資料流向](infrastructure/system-architecture-and-data-flow.md) | 全系統元件、對外入口與資料流向總圖 |
| [CI/CD 自動化部署（K8s + Helm）](infrastructure/ci-cd-automated-deployment-k8s-helm.md) | 部署流程與配置 |
| [系統監控架構（Monitoring）](infrastructure/monitoring.md) | 監控與告警架構 |
| [監控架構完整教學 (Monitoring Architecture Guide)](infrastructure/monitoring-architecture-guide.md) | 監控 stack 從零理解與操作的教學版 |
| [Discord 監控 Bot 設計書](infrastructure/discord-monitoring-bot-design.md) | 監控告警轉 Discord 的 bot 設計 |
| [Cloudflare CDN 與 DDoS 防護配置指南](infrastructure/cloudflare-cdn-and-ddos-protection-configuration-guide.md) | CDN 與安全配置 |
| [Discord 告警通知設定指南](infrastructure/discord-alert-notification-setup-guide.md) | 從零建立 Discord Server 到告警通知 |
| [業務告警分流與 dev-警報 路由設定](infrastructure/business-alert-routing-and-dev-alert-route-configuration.md) | 業務告警→dev-警報/prod-警報 頻道分流路由基建（#898/#1353） |
| [MQ Topology 管理（Terraform）](infrastructure/mq-topology-management-terraform.md) | RabbitMQ exchange/queue/binding 的新增、修改、刪除流程與環境現況 |
| [Prod RabbitMQ 3-Node Cluster](infrastructure/production-rabbitmq-3-node-cluster.md) | Prod MQ 三節點 cluster 的節點配置、bootstrap 與 cutover |
| [Tailscale VPN 連線 Prod 機器指南](infrastructure/tailscale-vpn-production-server-connection-guide.md) | 透過 Tailscale subnet router 連 Prod 私網（SSH / DB / k8s API） |

---

## 速查參考

| 文件 | 說明 |
|------|------|
| [常用指令](quick-reference/common-commands.md) | 開發常用指令速查 |
| [資料庫管理命令速查表](quick-reference/database-management-command-reference.md) | DB 管理指令 |
| [Redis 快取管理](quick-reference/redis-cache-management.md) | Redis key 清冊（含 session / 限流）、清除方式、**§5 512MB 容量契約與失效模型** |
| [前端屏幕與 API 映射表](quick-reference/frontend-screens-and-api-mapping.md) | 前端頁面對應的 API 端點 |

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
| [45 首頁動態區塊排序與營運干預提案（PM 版）](supplementary/45-homepage-dynamic-section-ordering-and-operations-intervention-proposal.md) | 首頁動態區塊排序與營運干預的 PM 版需求提案原文 |
