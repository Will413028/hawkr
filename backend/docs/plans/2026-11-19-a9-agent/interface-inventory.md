# A9-0 介面盤點：ai-service（含 procurement）（唯讀，快照 dev `bc2c732c`）

標記：【量】＝跑過指令或打開檔案看過；【推】＝讀程式推論。路徑縮寫：`B=backend`、`S=backend/services/ai-service`、`G=backend/gateway/internal`、`M=backend/internal`、`P=B/api/proto/ai/ai.proto`、`CSV=B/docs/plans/2026-09-24-go-modular-monolith`。
去向欄只是建議【推】，不是決定。Connect 方法名未擬（A9 的 D7 還沒決定 SSE 取捨），去向只寫「種類」與落點。所有指令從 `backend/` 執行。
先講一個容易誤導的事：`CSV/rpc-servers.csv`、`rpc-calls.csv`、`events.csv` 對 ai-service 都**過期**（§0 列出差異），本檔以現行程式為準。

## 0. 數字與指令（和主計畫、CSV 的差異先講）

| 項目 | 量到 | 主計畫／CSV | 指令與備註 |
|---|---|---|---|
| gRPC RPC 總數 | **59**（AIImageService 2、AIAdminService 7、RecommendationService 1、AIAssistantService 13、AIProcurementService 36） | CSV 寫 61（AIAssistantService 15，含已刪的 `SellerPrice`、`BuyerSeasonal`） | `rg -c '^\s*rpc ' api/proto/ai/ai.proto`＝59；逐 service 用 `awk '/^service X/,/^}/' api/proto/ai/ai.proto \| rg -c '^\s*rpc '`【量】。`gateway/internal/grpc/ai_service_conn.go:12-14` 註解仍寫「15 RPCs」，是舊註解 |
| 其中 server streaming | **12**（AIAssistantService 10＋AIProcurementService 2：`SubmitTurnStream`、`WatchRunEvents`） | 主計畫「12 支 SSE」 | `awk '/^service AIAssistantService/,/^}/' P \| rg -c 'returns \(stream'`＝10；procurement 同法＝2【量】。**主計畫的 12 成立**，但口徑是「12 條 SSE 路由」，不是「AIAssistantService 有 12 支 stream」（`gateway/internal/grpc/ai_assistant_client.go:11`、`proxy/ai_assistant_stream.go:96`、`proxy/ag_ui_events.go:69` 的註解寫「12 AIAssistantService」是過期寫法）【量】 |
| 有呼叫端的 RPC | **59／59**（呼叫端全是 gateway） | CSV 61 列全是 gateway | gateway 的 `AIClient`、`AIAssistantClient`、`AIProcurementClient` 共 59 個方法，逐一在 `G/handler` 有 ≥1 個非測試呼叫點（§1 列 file:line）【量】。**沒有呼叫端的 RPC：0** |
| 非 gateway 的 RPC 呼叫端 | **0** | — | Java：`rg -l 'AIProcurementServiceGrpc\|AIAssistantServiceGrpc\|AIImageServiceGrpc\|AIAdminServiceGrpc\|RecommendationServiceGrpc' services/java --glob '*.java'` 無命中；其他 Go 服務：`rg -l 'ai-service\|AI_SERVICE' services/*-service --glob '!services/ai-service/**'` 只命中 notification-service 的 migration／測試與 `internal/grpc/di.go` 的註解（沒有 ai client）【量】 |
| 對外 REST operation（gateway） | **59**（路由 46＋6＋6＋1） | 簡報沒寫 | `rg -c 'h\.Proxy\|h\.VendorPortal\|h\.GetRecommendations' G/router/routes_ai.go routes_procurement_quote.go routes_vendor_portal.go routes_buyer.go`＝46／6／6／1；`openapi.yaml` 符合 pattern（assistant、procurement、ai_styles、ai_quotas、image_transformations、recommendations、vendor_portal）的 operation 也是 59【量】。逐路徑對照有 `G/router/openapi_coverage_test.go` 守門，這裡沒另外 diff【推】 |
| 其中 SSE 路由 | **12**，全在 `openapi.yaml` 且標 `text/event-stream`（另有 `notifications/stream` 不屬 ai） | 12 | 見 §4.2。`python3 -I` 掃 `openapi.yaml` 的 path 區塊找 `text/event-stream`＝13 個 path，扣掉 `/app/notifications/stream`＝12【量】 |
| 非測試 Go 行數 | **37,949**（procurement 23,901、grpc 3,713、usecase 1,850、agent 1,658、cache 1,384、handler 341），**不含** `gen/`；測試 51,836 | — | `find S -name '*.go' ! -name '*_test.go' ! -path '*/gen/*' \| xargs cat \| wc -l`；各目錄同法【量】。procurement 占 63%，是主體 |
| 表 | **40**（`CREATE TABLE`；sessions／events／app_states／user_states 是 ADK session 表，ai_styles／ai_usage／ai_quotas／ai_usage_reservations 是圖片功能，candidate_leads 1，其餘 procurement_* 31） | CSV `db-tables.csv` 41 | `rg --no-filename -o 'CREATE TABLE (IF NOT EXISTS )?[a-z_."]+' S/db/schema`＝40；migration 檔 51 個（`ls S/db/migrations \| wc -l`，含 atlas.sum）、sqlc query 檔 17【量】。差 1 張未追，【推】CSV 把某個 view 或註解算進去 |
| 發布的事件 | **2** routing key（同一 exchange `procurement.events`）；經 outbox 表 `procurement_rfq_publication_outbox`；另有 `procurement_order_provisioning_outbox` 是內部補單用，不上 MQ | CSV 2 | §2 |
| 消費的事件 | **0** | CSV 寫 2（`order.completed`、`seller.membership_changed`，queue `ai_service.*`） | `rg -n 'order\.completed\|membership_changed\|NewConsumer\|\.Subscribe\(' S --glob '!*_test.go'` 無命中；`infra/mq/topology/topology.yaml` 沒有 `ai_service.*` queue【量】。CSV 這兩列過期（推薦的「ranking facts」已不存在，`recommend_usecase` 只剩匿名模式，見 §1.1 #10） |
| 出站 RPC（client adapter） | 6 個目標服務、**15 個 RPC 方法**（order 5、product 3、seller 3、buyer 1、category 1、media 3 經 `pkg/media` 的 helper，見 §3） | CSV 21 列含 pricebook 6 列 | `rg -o 'c\.client\.[A-Z]\w+' S/internal/client`（§3）；pricebook 已刪（ADR 2026-10-27），CSV 6 列 pricebook 過期【量】 |
| 出站 HTTP／SDK | LLM：Gemini（genai SDK、REST 圖片模型）、DeepSeek（OpenAI 相容）、OpenRouter、OpenAI 相容端點；沒有 object storage 直連（走 media-service） | — | §3.2 |
| 舊服務還在跑的 Java 呼叫端 | **0** | — | 上面兩條指令【量】 |
| 接 gateway 的 SSE 路由設定 | 單 turn 上限 120s、寫入每次 150s、keepalive 註解行 | — | §4.3 |

## 1. gRPC RPC（59 支，`P`）

呼叫方一律是 gateway（G）。ai-service 的 handler 在 `S/internal/grpc/`（`handler.go` 圖片、後台、推薦；`assistant_handler.go` 助理；`procurement_handler.go`、`procurement_seller_response_handler.go`、`procurement_run_events_handler.go`、`rfq_publication_invite_view.go` 採購）。`P` 欄是 proto 的行號。每支 RPC 都經 method-policy（`S/internal/grpc/method_policy.go`，未登記一律拒絕）與 actor 解析【量】。

### 1.1 AIImageService、AIAdminService、RecommendationService（10 支）

| # | RPC | P 行 | 呼叫方 file:line | REST | 去向（建議） |
|---|---|---|---|---|---|
| 1 | TransformImage | 14 | G `grpc/ai_client.go:145` ← `handler/ai.go:280` | `POST /app/image_transformations` | api：Connect unary（賣家 App／web 拍照美化；上限 20MB 訊息，`ai_service_conn.go:30-31`；外呼 Gemini 圖片模型，§3.2） |
| 2 | ListAiStyles | 15 | `ai_client.go:69` ← `ai.go:187` | `GET /app/ai_styles`（PrivateNoStore） | api：Connect unary（回應帶 weeklyUsage／weeklyLimit） |
| 3 | ListAiStylesForAdmin | 21 | `ai_client.go:79` ← `ai.go:307` | `GET /admin/ai_styles` | api：admin Connect（Cloudflare Access 後；見 §7 U5 的 admin 歸屬） |
| 4 | CreateAiStyle | 22 | `ai_client.go:87` ← `ai.go:374`（樣式圖經 media，§3） | `POST /admin/ai_styles` | api：admin Connect |
| 5 | UpdateAiStyle | 23 | `ai_client.go:95` ← `ai.go:425` | `PUT /admin/ai_styles/{id}` | api：admin Connect |
| 6 | DeleteAiStyle | 24 | `ai_client.go:103` ← `ai.go:447` | `DELETE /admin/ai_styles/{id}` | api：admin Connect |
| 7 | GetAiQuota | 25 | `ai_client.go:114` ← `ai.go:468` | `GET /admin/ai_quotas/{store_id}` | api：admin Connect |
| 8 | UpdateAiQuota | 26 | `ai_client.go:122` ← `ai.go:510` | `PUT /admin/ai_quotas/{store_id}` | api：admin Connect |
| 9 | DeleteAiQuota | 27 | `ai_client.go:130` ← `ai.go:535` | `DELETE /admin/ai_quotas/{store_id}` | api：admin Connect |
| 10 | PersonalizedRecommend | 34 | `ai_client.go:41` ← `handler/recommendation.go:62` | `GET /public/recommendations` | api：Connect unary，或 removed。gateway 固定傳 `accountID=0`（匿名，`recommendation.go:17,62`），個人化沒有接線；website／buyer-app 的消費者要查（`rg -n 'recommendations' website/lib buyer-app/lib -l` 只命中 buyer-app 的 `category_landing_screen.dart`）【量】，是否保留是 §7 U4 |

### 1.2 AIAssistantService（13 支，`S/internal/grpc/assistant_handler.go`；hander 邏輯在 `S/internal/handler/{seller,buyer}_assistant.go`、`S/internal/agent/`）

10 支 stream（#11–15、#18–22，回 `stream StreamEvent`），3 支 unary。SSE 請求 body 全是 `{sessionId, message, imageUrl}`（`G/handler/ai.go:84-88`），store／buyer 身分由 gateway 放進 gRPC metadata（`grpcclient.WithStoreID`、`WithBuyerID`）。

| # | RPC | P 行 | 呼叫方 file:line | REST | 去向 |
|---|---|---|---|---|---|
| 11 | SellerChat | 200 | G `ai_assistant_client.go:44` ← `handler/ai.go:576` | `POST /app/sellers/me/assistant/chat`（SSE，CapMarketing） | api 或 http（D7）：見 §4 |
| 12 | SellerOrders | 201 | `…client.go:48` ← `ai.go:606` | `POST …/assistant/orders`（SSE） | 同上 |
| 13 | SellerInventory | 202 | `…client.go:52` ← `ai.go:636` | `POST …/assistant/inventory`（SSE） | 同上 |
| 14 | SellerPayment | 203 | `…client.go:56` ← `ai.go:666` | `POST …/assistant/payment`（SSE） | 同上 |
| 15 | SellerProductListingChat | 204 | `…client.go:60` ← `ai.go:1766` | `POST …/assistant/product-listing/chat`（SSE） | 同上 |
| 16 | SellerProductAutofill | 205 | `…client.go:111` ← `ai.go:1797`（30s deadline，`autofillTimeout`） | `POST …/assistant/product-autofill`（JSON） | api：Connect unary |
| 17 | SellerLatestSession | 206 | `…client.go:121` ← `ai.go:840` | `GET …/assistant/sessions/latest` | api：Connect unary |
| 18 | BuyerOrders | 209 | `…client.go:70` ← `ai.go:724` | `POST /app/buyers/me/assistant/orders`（SSE） | 同 #11 |
| 19 | BuyerTracking | 210 | `…client.go:74` ← `ai.go:752` | `POST …/assistant/tracking`（SSE） | 同上 |
| 20 | BuyerReturnPolicy | 211 | `…client.go:78` ← `ai.go:781` | `POST …/assistant/return-policy`（SSE；固定文案 `S/internal/agent/return_policy.go`） | 同上；`CARRYOVER.md`（content 模組）已寫退貨文案 SSE 在 content-service 刪除時改回 ai-service 固定文案，A9 不再有外部依賴【量：`M/modules/content/CARRYOVER.md:22`】 |
| 21 | BuyerSellerMatch | 212 | `…client.go:82` ← `ai.go:809` | `POST …/assistant/seller-match`（SSE） | 同上 |
| 22 | BuyerChat | 213 | `…client.go:66` ← `ai.go:696` | `POST …/assistant/chat`（SSE） | 同上 |
| 23 | BuyerLatestSession | 214 | `…client.go:128` ← `ai.go:864` | `GET …/assistant/sessions/latest` | api：Connect unary |

### 1.3 AIProcurementService（36 支，`P:1422-1488`）

所有 procurement REST 都在 `/app/buyers/me/procurement/*`（買家，JWT＋`requireBuyer`）、`/app/sellers/me/procurement/*`（賣家，CapMarketing）、`/public/vendor_portal/rfqs/*`（站外業者，單體簽的 vendor session，§4.1）。呼叫點都在 `G/handler/ai.go`（簡稱 ai.go）、`ai_procurement_seller_response.go`（簡稱 sr.go）、`vendor_portal*.go`。

| # | RPC | P 行 | 呼叫方 file:line | REST | 去向 |
|---|---|---|---|---|---|
| 24 | SubmitTurn | 1424 | G `grpc/ai_procurement_client.go:47` ← `ai.go:912` | `POST /buyers/me/procurement/turns` | api：Connect unary |
| 25 | SubmitTurnStream | 1425 | `…client.go:56` ← `ai.go:971`（`ProxyProcurementTurnStream` :939） | `POST …/turns/stream`（SSE） | 依 D7，§4 |
| 26 | GetSnapshot | 1426 | `…client.go:60` ← `ai.go:993` | `GET …/requests/{id}/snapshot` | api |
| 27 | StartCandidateSearch | 1429 | `…client.go:66` ← `ai.go:1018` | `POST …/requests/{id}/candidate-searches` | api |
| 28 | GetCandidateSearch | 1430 | `…client.go:72` ← `ai.go:1043` | `GET …/candidate-searches/{runId}` | api |
| 29 | ToggleSelection | 1431 | `…client.go:78` ← `ai.go:1075` | `POST …/requests/{id}/selections` | api |
| 30 | SubmitAnswer | 1434 | `…client.go:84` ← `ai.go:1173` | `POST …/requests/{id}/answers` | api |
| 31 | Rewind | 1435 | `…client.go:90` ← `ai.go:1220` | `POST …/requests/{id}/rewinds` | api |
| 32 | PreviewRfqPrice | 1438 | `…client.go:98` ← `ai.go:1273` | `POST …/rfq-price-previews` | api（拿掉 pricebook 後的形狀，見 §7 U2） |
| 33 | ConfirmRfqSnapshot | 1441 | `…client.go:107` ← `ai.go:1319` | `POST …/rfq-confirmations` | api |
| 34 | PreviewRfqPublication | 1445 | `…client.go:116` ← `ai.go:1375` | `POST …/rfq-publications/preview` | api |
| 35 | ExecuteRfqPublication | 1450 | `…client.go:125` ← `ai.go:1434` | `POST …/rfq-publications`（寫 outbox，§2） | api |
| 36 | GetRfqPublicationInviteSellerView | 1453 | `…client.go:257` ← `handler/vendor_portal.go:93` | `GET /public/vendor_portal/rfqs/{rfq_recipient_id}` | api：vendor portal 的 RFQ 讀取，是 A9 要把 `VendorPortalRFQPrefix` 改成 Connect 的那條（`M/legacy/gatewayproxy/proxy.go:53-54` 註解寫「過渡到 A9」）【量】 |
| 37 | ListConversations | 1456 | `…client.go:131` ← `ai.go:1459` | `GET …/conversations` | api |
| 38 | GetConversationMessages | 1457 | `…client.go:137` ← `ai.go:1485` | `GET …/conversations/{id}/messages` | api |
| 39 | GetRuntimeSnapshot | 1458 | `…client.go:143` ← `ai.go:1509` | `GET …/conversations/{id}/runtime-snapshot` | api |
| 40 | ListSellerPublications | 1462 | `…client.go:150` ← `sr.go:342` | `GET /sellers/me/procurement/publications` | api（賣家） |
| 41 | GetSellerPublication | 1463 | `…client.go:156` ← `sr.go:377` | `GET …/publications/{id}` | api |
| 42 | UpsertSellerResponseDraft | 1464 | `…client.go:162` ← `sr.go:425` | `PUT …/publications/{id}/response-draft` | api |
| 43 | GetVendorPortalResponseDraft | 1465 | `…client.go:168` ← `vendor_portal_response_draft.go:167` | `GET /public/vendor_portal/rfqs/{id}/response_draft` | api（vendor portal） |
| 44 | UpsertVendorPortalResponseDraft | 1466 | `…client.go:174` ← `vendor_portal_response_draft.go:137` | `PUT …/response_draft` | api（vendor portal） |
| 45 | SubmitVendorPortalResponse | 1468 | `…client.go:182` ← `vendor_portal_response_submission.go:87` | `POST …/response_submissions` | api（vendor portal；需 Google 驗證，§4.1） |
| 46 | WithdrawVendorPortalResponse | 1469 | `…client.go:188` ← `vendor_portal_response_submission.go:132` | `POST …/seller_responses/{id}/withdrawals` | api（vendor portal） |
| 47 | DeclineVendorPortalRfqPublication | 1470 | `…client.go:194` ← `vendor_portal_response_submission.go:176` | `POST …/declines` | api（vendor portal） |
| 48 | SubmitSellerResponse | 1471 | `…client.go:200` ← `sr.go:472` | `POST /sellers/me/procurement/publications/{id}/seller-responses` | api（賣家） |
| 49 | WithdrawSellerResponse | 1472 | `…client.go:206` ← `sr.go:517` | `POST …/seller-responses/{id}/withdraw` | api |
| 50 | DeclineRfqPublication | 1473 | `…client.go:212` ← `sr.go:559` | `POST …/publications/{id}/decline` | api |
| 51 | ListSellerResponses | 1476 | `…client.go:218` ← `sr.go:588` | `GET /buyers/me/procurement/requests/{id}/seller-responses` | api |
| 52 | PutShortlistEntry | 1477 | `…client.go:224` ← `sr.go:623` | `PUT …/shortlist/{seller_response_id}/versions/{v}` | api |
| 53 | DeleteShortlistEntry | 1478 | `…client.go:230` ← `sr.go:649` | `DELETE` 同路徑 | api |
| 54 | AcceptSellerQuote | 1479 | `…client.go:236` ← `sr.go:690`（Idempotency-Key header） | `POST …/buyer-quote-acceptances` | api（成交後排 order provisioning outbox → `CreateRfqOrder`，§3） |
| 55 | RenameConversation | 1480 | `…client.go:242` ← `ai.go:1542` | `PATCH …/conversations/{id}` | api |
| 56 | DeleteConversation | 1481 | `…client.go:249` ← `ai.go:1575` | `DELETE …/conversations/{id}` | api |
| 57 | WatchRunEvents | 1484 | `…client.go:278` ← `ai.go:1617`（`ProxyProcurementRunEvents` :1603） | `GET …/runs/{runId}/events`（SSE，`Last-Event-ID`） | 依 D7；§4 |
| 58 | CancelRun | 1487 | `…client.go:285` ← `ai.go:1656` | `POST …/runs/{runId}/cancel` | api |
| 59 | ResumeRun | 1488 | `…client.go:294` ← `ai.go:1695` | `POST …/runs/{runId}/resume` | api |

（編號 #1–#59 連續：1.1 是 #1–10，1.2 是 #11–23，1.3 是 #24–59。）

沒有呼叫端的 RPC：**無**【量，§0 指令】。因為 `ProcurementReplaySseEnabled` 是 feature flag（`ai.go:1604`，`WatchRunEvents` 關閉時回 503 `CAPABILITY_UNAVAILABLE`），#57 在 flag 關的環境等於沒有流量【量】；flag 現在的值是設定問題，這裡沒查【未驗證】。

## 2. 事件

### 2.1 發布（2 個 routing key，exchange `procurement.events`，`B/pkg/mq/exchanges.go:20,89,91`）

| # | routing key | 產生處 file:line | payload 來源 | 消費者 | 去向 |
|---|---|---|---|---|---|
| E1 | `procurement.rfq_publication_invite.created`（PLATFORM lane：站內賣家） | `S/internal/procurement/publicationoutbox/worker.go:57,66`；列在 `commerce/publication_execute.go:249,269` 與 `ExecuteRfqPublication` 同交易寫入 `procurement_rfq_publication_outbox` | outbox 列 payload（邀請 ID、賣家 store 等） | notification-service `services/notification-service/internal/worker/consumer.go`（推播＋站內信，seller:visible；CSV `events.csv`） | in-tx-event：進單體後 `agent` 模組在 `ExecuteRfqPublication` 同交易發內部事件，notification（A8）訂閱，取代 outbox＋MQ（`publicationoutbox` 整包可刪，【推】需過 necessity-condition：同 DB 同交易後 relay 不再需要） |
| E2 | `procurement.rfq_publication_invite.external_created`（EXTERNAL lane：站外業者） | 同上 `worker.go:68` | 同上；notification 呼叫 seller 的 `EnsureVendorPortalInvite`（`services/notification-service/internal/client/seller/client.go:55` → 單體 `M/legacy/sellerserver/invites.go`）建邀請再寄信 | notification-service（email 邀請） | 同 E1；注意「邀請由 notification 向 seller／agent 要」這條反向流程，進單體後變 notification→agent 的 in-process 呼叫（`arch/modules.go:42` 已允許 notification→agent） |

`procurement_order_provisioning_outbox`（`S/internal/procurement/orderprovisioning/{worker,repository}.go`，`commerce/buyer_acceptance.go:388` 同交易寫入）是**內部補單 worker**，不上 MQ：worker 呼叫 order 的 `CreateRfqOrder`（§3）。去向：ordering 是 Java（B8），A9 期間仍需此 worker 跨服務打 Java；ordering 進單體後改同交易建單【推】。

### 2.2 消費

**0**（§0）。`S/internal/config/config.go:58` 有 `RABBITMQ_URL`，只給發布用（`publicationoutbox/di.go:86` 的 `mq.NewPublisher`，連線 lazy，URL 空則 worker 略過每個 tick）。`CSV/events.csv` 的 `order.completed`、`seller.membership_changed` 兩列要在 A9 結案時更正或刪除【量】。

### 2.3 非 MQ 的背景排程

| 排程 | 位置 | 去向 |
|---|---|---|
| ADK session 清理（`outbox.NewScheduler`） | `S/internal/agent/cleaner.go:35` | job：改 River periodic job（單體現行機制） |
| publication outbox drain＋monitor＋cleanup | `publicationoutbox/di.go:153-175` | removed（見 E1） |
| order provisioning worker | `orderprovisioning/di.go` | job（ordering 進單體前）；之後 removed |

## 3. 出站呼叫

### 3.1 對其他服務（`S/internal/client/*`，目標位址由 env 決定，`S/internal/config/config.go:23-31`）

| # | 目標 | RPC | file:line | 目前由誰服務 | 去向 |
|---|---|---|---|---|---|
| C1 | buyer（`BUYER_SERVICE_*`，預設 9941） | GetBuyerEntitlement | `client/buyer/adapter.go:61`；用於 `procurement/module.go:146`（entitlement 閘） | **單體** `M/legacy/buyerserver/server.go:260`（compose 指向 `host.docker.internal:9941`，`docker-compose-local.yml:24-25`）【量】 | in-process：`buyer.Module` 的 entitlement 讀取（A3 已搬進 buyer，`a3-buyer-seller.md:32` 寫明「GetBuyerEntitlement 仍被 ai-service 使用」） |
| C2 | seller（`SELLER_SERVICE_*`，預設 9941） | GetSellerStats | `client/seller/adapter.go:37` | 單體 `M/legacy/sellerserver/server.go:363`（評分來自 review，經 `cmd/hawkr/seller.go:73` 的 `legacySellerRatings`；ai-service 只讀 rating 與 followers，`server.go:360`） | in-process：seller 模組＋review 的 `StoreRatingStats`（見 §5.3） |
| C3 | seller | BatchGetSellerMeta | `adapter.go:68` | 單體 `server.go:311` | in-process：seller 模組 |
| C4 | seller | SearchSellerProspects | `adapter.go:91`（`procurement/usecase/candidate_*` 的 EXTERNAL lane 候選搜尋） | 單體 `server.go:497` | in-process：`seller.SearchProspects`（`M/modules/seller/CARRYOVER.md:170` 已預留：「ai-service 進單體（A9）後直接呼叫 SearchProspects，期間由 internal/legacy 提供那一支 RPC」）；`prospect_id` 是 BIGINT（`CARRYOVER.md:176,370`，最後一個整數 ID 消費者） |
| C5 | order（`ORDER_SERVICE_*`，預設 9915，Java） | GetSellerOrders、GetMyOrders、GetOrder、GetSellerOrderDetail | `client/order/adapter.go:71,123,164,206` | Java order-service | 暫留 gRPC client 到 Java；ordering 進單體（B8）前走 `internal/legacy/orderservice` 之類的 adapter【推】 |
| C6 | order | CreateRfqOrder | `adapter.go:249`（`orderprovisioning/worker.go`）；`B/api/proto/order/order.proto:32` | Java `OrderGrpcService.java:427`（「僅接受內部服務呼叫」） | 同 C5；是 procurement 成交到訂單的唯一出口，要有 characterization（冪等鍵 `acceptance_id`，Java `V15__add_order_acceptance_id.sql`） |
| C7 | product（`PRODUCT_SERVICE_*`，預設 9913，Java） | GetProductList、GetStock、GetProduct | `client/product/adapter.go:36,76,109,143,189`（`SearchProducts` 在 :129，PLATFORM lane 候選搜尋，`procurement/usecase/candidate_platform_lane.go:76`） | Java product-service | 暫留；catalog 搬進單體前不動 |
| C8 | category | GetCategoryTree | `client/category/adapter.go:35`；ai-service 自己建快取（`internal/cache/category_index.go`、`category_alias*.go`、`category_tree_snapshot.go`） | Java product-service（CSV：`product-service CategoryService`）；單體沒有 `GetCategoryTree`（`rg -l GetCategoryTree M` 無命中）【量】 | 暫留；快取與 `cmd/category-alias-gen`、`cmd/prospect-category-gen` 是離線工具，進單體時決定保留位置 |
| C9 | media（`MEDIA_SERVICE_*`，預設 9937，Java） | FinalizeImages、LinkImages、ReleaseReferences | `client/media/client.go:48,55` 包 `B/pkg/media` 的 `FinalizeAndLink`、`ReleaseReferences`（後台 AI 樣式圖的生命週期） | Java media-service（單體 `M/modules/media` 已有，但這條還走舊 RPC）【推】 | in-process：`media` 模組（`arch/modules.go:39` 的 agent 允許依賴 media） |

CSV 另有 6 列 pricebook（`GetForecastCurve` 等）：服務已刪（ADR 2026-10-27），ai-service 現行程式沒有 pricebook client（`ls S/internal/client` 只有 buyer、category、media、order、product、seller）【量】。

### 3.2 外部 API（LLM 與圖片模型）

| # | 目的 | 位置 | 設定（env 名，不含值） | 去向 |
|---|---|---|---|---|
| L1 | Gemini 圖片轉換（REST，`gemini-2.5-flash-image:generateContent`，base64 inline，`x-goog-api-key`） | `S/internal/usecase/transform_usecase.go:62,358-371`（硬寫 URL） | `GEMINI_API_KEY` | 隨 agent 模組；金鑰進單體的 secret 管理；硬寫 URL 要不要抽設定是沿用盤點項 |
| L2 | 助理與採購的 LLM（ADK `model/gemini`、`model/openaimodel`） | `S/internal/llmprovider/provider.go:36-37,109-111`（`knownExtractorModels`：gemini-2.5-flash、deepseek-flash） | `GEMINI_API_KEY`、`GEMINI_MODEL`、`DEEPSEEK_API_KEY`、`OPENROUTER_API_KEY`、`LLM_API_URL`、`LLM_API_KEY`、`LLM_MODEL`、`EXTRACTOR_MODEL`、`EXTRACTOR_PROVIDER`（已 stale）、`LLM_TOKEN_BUDGET_PER_MINUTE` | 同上；`LLM_*`（OpenAI 相容預設）與 `EXTRACTOR_PROVIDER` 是否還有人用要在 A9 設計審查盤點（`config/strict_env.go`、`env_tag_coverage_test.go` 有守門） |
| L3 | Object storage | **沒有直連**：圖片輸入 `imageUrl` 由 gateway／client 傳 URL，樣式圖走 media-service（C9）【推：`rg -n 'storage\|gcs\|s3\|bucket' S/internal` 沒有 SDK 命中，只有 URL 欄位】 | — | in-process media |
| L4 | Redis | compose 有 `REDIS_HOST`，但 `S/internal/config/config.go` 沒有 Redis 欄位；`rg -il redis S --glob '*.go' --glob '!*_test.go'` 只命中 `client/product/adapter.go` 與 `config/strict_env.go`，是否真的連 Redis 沒開檔確認【未驗證】 | — | 進單體前確認 |

## 4. REST／SSE（經 `backend/gateway`）

### 4.1 路由總表（59 條，全在 `openapi.yaml`）

| 群組 | 數量 | 掛載 | 認證與中介 |
|---|---|---|---|
| App 圖片與樣式 | 2 | `routes_ai.go:14,18`（`/app/ai_styles`、`/app/image_transformations`） | JWTAuth；樣式列表 PrivateNoStore |
| Admin 樣式與額度 | 7 | `routes_ai.go:25-39` | admin namespace（單體把 `Cf-Access-Jwt-Assertion` 轉 Bearer）；刻意不掛 PrivateNoStore（`:22-24` 註解） |
| 賣家 AI 助理 | 7（SSE 5＋JSON 2） | `routes_ai.go:49-58` | PrivateNoStore→JWTAuth→`CapMarketing`（即使唯讀也鎖，2026-07-10 裁決） |
| 買家 AI 助理 | 6（SSE 5＋JSON 1） | `routes_ai.go:61-69` | PrivateNoStore→JWTAuth→`requireBuyer` |
| 買家採購 | 24（SSE 2） | `routes_ai.go:72-107` | 同上；SSE 起手有 per-buyer 5 條並發上限（`handler/procurement_stream_connlimit.go:8`，超過回 429） |
| 賣家採購 | 6 | `routes_procurement_quote.go:24-39` | JWTAuth＋`CapMarketing`（選 Marketing 而非 OrderContract 的理由在 `:17-21`） |
| vendor portal | 6 | `routes_vendor_portal.go:13-40`（`/public/vendor_portal/rfqs/*`） | `VendorPortalSession`（單體簽發的 session，gateway 讀單體注入的 `X-Vendor-Portal-*` header 重建身分，`M/legacy/gatewayproxy/proxy.go:53-61,365`）；寫入類再加 `RequireVendorPortalGoogleVerified` |
| 推薦 | 1 | `routes_buyer.go:12`（`/public/recommendations`） | 匿名 |

請求／回應細節：採購 turn 的 body 是 `{requestId, message, clientRequestId}`，`message` 上限 2000 rune，`clientRequestId` 必填、同買家同 key 重送回既有結果（`handler/procurement_response_mapping.go:10-14`、`ai.go:939-970`）【量】。其餘請求形狀寫在各 handler 的 swagger 註解與 `openapi.yaml`。

### 4.2 12 條 SSE 逐條（`format: text/event-stream`，POST 除了 run events 是 GET）

| # | 路由 | 請求 body／header | RPC | 事件型別（AG-UI frame，`data: <json>\n\n`，沒有 `event:` 行） |
|---|---|---|---|---|
| S1 | `POST /app/sellers/me/assistant/chat` | `{sessionId,message,imageUrl}` | SellerChat | 見下表「助理共通」 |
| S2 | `…/sellers/me/assistant/orders` | 同 | SellerOrders | 同 |
| S3 | `…/inventory` | 同 | SellerInventory | 同 |
| S4 | `…/payment` | 同 | SellerPayment | 同 |
| S5 | `…/product-listing/chat` | 同 | SellerProductListingChat | 同，另有 `CUSTOM draft_patch` |
| S6 | `POST /app/buyers/me/assistant/orders` | 同 | BuyerOrders | 同 |
| S7 | `…/tracking` | 同 | BuyerTracking | 同 |
| S8 | `…/return-policy` | 同 | BuyerReturnPolicy | 同 |
| S9 | `…/seller-match` | 同 | BuyerSellerMatch | 同（web 端 `website/lib/buyer/seller-match-chat.ts`） |
| S10 | `…/chat`（買家） | 同 | BuyerChat | 同 |
| S11 | `POST /app/buyers/me/procurement/turns/stream` | `{requestId,message,clientRequestId}` | SubmitTurnStream | 助理共通＋`activity`、`run_event` 等 procurement 事件；RUN_ERROR 另帶 `reason／remaining／resetAt`（`ag_ui_events.go:73-80`）；`RUN_STARTED` 延後到第一個事件（`deferRunStarted`，用 ledger run_id） |
| S12 | `GET /app/buyers/me/procurement/runs/{runId}/events` | `Last-Event-ID`（十進位 ledger seq；缺省從頭重播）；游標過舊 410、過新 409，皆以 in-band 錯誤 frame 呈現，HTTP 恆 200 | WatchRunEvents | `RUN_ERROR`＋`CUSTOM name=<由 run event type 決定的 slug>`＋文字訊息（`ai_run_events_stream.go:142,148,208`）；有 `id:` 行（`writeAGUIFrameWithID`） |

事件對映（`G/proxy/ai_assistant_stream.go:440-560`、`ag_ui_events.go`，gRPC `StreamEvent` oneof 共 10 個 payload：text、done、error、draft_patch、tool_result、step、message、turn_result、run_accepted、activity）【量】：

| gRPC payload | AG-UI frame |
|---|---|
| text | `TEXT_MESSAGE_START`（首次）、`TEXT_MESSAGE_CONTENT` |
| done | `TEXT_MESSAGE_END`＋`RUN_FINISHED` |
| error | `TEXT_MESSAGE_END`（若有開）＋`RUN_ERROR` |
| draft_patch | `CUSTOM name=draft_patch` |
| tool_result | `TOOL_CALL_START(tool_result)`＋`TOOL_CALL_END`＋`TOOL_CALL_RESULT` |
| step | `STEP_STARTED`／`STEP_FINISHED`＋`CUSTOM name=step_detail` |
| activity | `CUSTOM name=activity` |
| message | `TEXT_MESSAGE_START／CONTENT／END`（整段） |
| turn_result | `STATE_SNAPSHOT`（`:561`）＋`RUN_FINISHED`（帶 interrupt 陣列，`agUIInterruptEntry`） |
| run_accepted | 不直接輸出，只決定 `RUN_STARTED` 的 runId |

另 `STATE_SNAPSHOT` 在 turn_result 路徑送出（`ai_assistant_stream.go:561`，`ag_ui_events.go:126`），所以 procurement 的 S11 事件型別集合還要加 `STATE_SNAPSHOT`【量】。契約黃金檔：`G/proxy/agui_golden_test.go`、`testdata/agui-frames/*.sse`；web 端以 `@ag-ui/core` zod 驗證（`website/lib/ai-chat/stream-client.ts`、`agui-dispatch.ts`），seller-app 有 `agui_sse_decoder`（`seller-app/test/core/networking/agui_sse_decoder_test.dart`）。

### 4.3 gateway 怎麼代理 SSE（不是 reverse proxy，是 gRPC→SSE 翻譯層，`G/proxy/ai_assistant_stream.go`）

| 項目 | 值 | file:line |
|---|---|---|
| 單 turn 上限 | **120s**（`context.WithTimeout` 衍生自 request ctx，client 斷線即取消上游 gRPC；gRPC 把 deadline 傳給 ai-service，LLM 呼叫繼承）。「產品決定，不是量出來的 p99」；實測 2026-08-21 成功 turn 16–46s | `:82,207` |
| 每次寫入 deadline | **150s**（`deadlineResponseWriter`，防 client 不讀卻不斷線） | `:32,242` |
| keepalive | 靜默期寫 `: ping\n\n` 註解行（`sseKeepaliveInterval`，原因：Cloudflare 約 100s idle 會斷） | `:36-67,403-407` |
| buffering | `Content-Type: text/event-stream`、`Cache-Control: private, no-store`、`Connection: keep-alive`、`X-Accel-Buffering: no`，每個 frame 後 `Flush()` | `:230-233,266,322,399,407`；run events 同（`ai_run_events_stream.go:325`） |
| 開不了串流 | 不重試（有副作用的 LLM 呼叫），回 HTTP 424 envelope（`FailClientFacing`，避開 Cloudflare 換掉 5xx body） | `:210-225` |
| gateway HTTP server | 無 `ReadTimeout`／`WriteTimeout`（會砍 SSE），`ReadHeaderTimeout` 10s、`IdleTimeout` 120s，body 讀取由 BodyGuard | `G/infrastructure/di.go:177-187` |
| gRPC 連線 | 一條共用連線（`ai_service_conn.go`，env `AI_SERVICE_HOST`／`AI_SERVICE_GRPC_PORT` 預設 `127.0.0.1:9939`），收發上限 20MB；circuit breaker（stream interceptor） | `ai_service_conn.go:24-35`、`grpc/circuit_breaker_stream_interceptor_test.go` |
| 單體前面那一層 | 單體的 gateway proxy（`httputil.ReverseProxy`）`FlushInterval: -1`（逐 frame 轉送）、`dialTimeout` 5s、**`ResponseHeaderTimeout` 60s**、`IdleConnTimeout` 90s | `M/legacy/gatewayproxy/proxy.go:70-71,259-276` |
| 速率限制 | 三個 AI 群組原本在 rate-limit 豁免名單，現在**刻意移出**（有測試 `TestAIRoutesAreNotRateLimitExempt`） | `G/middleware/ratelimit.go:115-119` |

【推】`ResponseHeaderTimeout` 60s 對 S11 有風險：procurement 串流的 `RUN_STARTED` 延後到第一個上游事件才寫（`deferRunStarted`），HTTP header 隨第一次 Flush 才送出；若第一個事件超過 60 秒，單體的 proxy 先斷。實測 turn 16–46s 在界內，但沒有量過 procurement 第一事件的延遲分布【未驗證】。進單體後這層消失，但 D7 若選 Connect server streaming 要重新定 header 與 keepalive。

## 5. 與單體的耦合

### 5.1 單體現在對 ai-service／procurement 的引用【量】

指令：`rg -n 'ai-service|ai_service|aiservice|procurement|AIProcurement|AIAssistant' M cmd --glob '!*_test.go' --glob '!**/gen/**'`。

| # | 位置 | 引用 | 性質 |
|---|---|---|---|
| R1 | `M/arch/modules.go:39` | `"agent": {"ordering","catalog","buyer","seller","media"}`；`:42` notification 可依賴 agent；`:97` agent 內部圖只有 `vendorportal`、`model`、`store` | 單體已有 **`agent` 模組**，但只放 vendor portal（A3 步驟 8i 提前建，`a3-buyer-seller.md:129,438-440` 方案 b）。ai-service 的其餘部分（assistant、procurement、圖片、推薦）**尚未**進入 |
| R2 | `M/modules/agent/{AGENTS.md,CARRYOVER.md,ports.go}`、`cmd/hawkr/agent_events.go` | vendor portal 的邀請、兌換、session 與 prospect 合併／封存事件；agent 依賴 seller 的 `ProspectContact`（`ports.go`） | A9 要在這個模組上長出 ai-service，不是新建 |
| R3 | `M/legacy/sellerserver/{server.go,invites.go}`、`M/legacy/buyerserver/server.go` | 為 ai-service（與 Java）保留的 SellerService／BuyerService 讀取面：GetSellerStats、BatchGetSellerMeta、SearchSellerProspects、GetBuyerEntitlement、EnsureVendorPortalInvite、MarkVendorPortalInviteDelivered | **A9 完成後可刪的 legacy 面**（只要 notification 與 ai-service 都進單體） |
| R4 | `M/legacy/gatewayproxy/proxy.go:53-61,148-229,365` | vendor portal RFQ 路徑前綴的 session 驗證與 header 注入；註解「過渡到 A9 把 RFQ 路由改成 Connect 為止」 | A9 內刪（RFQ 路由改 Connect 後不再需要 header 注入） |
| R5 | `M/httpapi/rpc/session.go:167` | 註解提到「收件人 ID 由 ai-service 依序發號」（用來決定 cookie 淘汰順序） | 注意：recipient ID 的發號者是 ai-service 的 DB；搬進單體後 ID 方案（BIGINT IDENTITY→UUIDv7）要和 row-identity ADR 對齊 |
| R6 | `M/platform/codemeta/code_meta.go:1217-1286` | `ProcurementRunStatus`、`ProcurementStepStatus` 等代碼（`B/pkg/codemeta` 與 admin 的 `codemeta/generated.ts` 也有） | CodeMeta 凍結（ADR 2026-09-25）；搬時改成 proto enum＋Go 型別＋CHECK |
| R7 | `M/modules/content/CARRYOVER.md:22` | 退貨文案 SSE 的後備句在 ai-service | 已知，A9 吸收 |
| R8 | `M/httpapi/gen/hawkr/admin/seller/v1alpha1/prospect.pb.go:400` | 註解「ai-service 用主要的 WEBSITE」 | 僅註解 |

單體沒有任何對 `hawkr.ai.*` 的 client、沒有 proxy 路由專屬於 ai（gateway 的 `/api/v1/*` 是整批轉發，沒有逐路徑清單）、沒有 `internal/legacy/aiservice`（`ls M/legacy`）【量】。

### 5.2 單體與 ai-service 的資料／事件耦合

- ai-service 的 DB 是獨立的 `hawkr_ai`（`config.go:44`），與單體不共庫【量】；單體 `agent` 模組的 schema `agent` 與 ai-service 的 `procurement_rfq_publication_invites` 之間以 `rfq_recipient_id` 對應（`M/legacy/sellerserver/invites.go:25` 註解）。【推】A9 搬進單體後兩邊要合併，`rfq_recipient_id` 的唯一性由哪邊保證要在設計書定。
- 事件：E1、E2 的消費者是 notification-service（舊 Go 服務，A8 才進單體）。

### 5.3 A9 是否依賴 A3（buyer、seller）或 A6（review、im、crm）

**結論：前提「A9 與 A3／A6 無關」不成立，但依賴方向是『A9 是它們已提供之介面的消費者』，沒有任何一條依賴會擋住 A9 開工。**

| 依賴 | 證據 | 狀態 | 對 A9 的影響 |
|---|---|---|---|
| A3 buyer：`GetBuyerEntitlement` | `S/internal/client/buyer/adapter.go:61`；單體已服務（`buyerserver/server.go:260`）；A3 計畫寫「仍被 ai-service 使用」（`a3-buyer-seller.md:32`） | buyer 模組**已進單體**（legacy façade 還在） | 不阻擋；A9 把 C1 改成 in-process 呼叫 `buyer.Module`，並刪 façade 這一支 |
| A3 seller：SearchSellerProspects／GetSellerStats／BatchGetSellerMeta／vendor portal invites | §3.1 C2–C4；`modules/seller/CARRYOVER.md:170,176,370`（明寫「ai-service 進單體（A9）後直接呼叫 `SearchProspects`」、`prospect_id` BIGINT 是最後一個整數主鍵消費者） | seller 模組**已進單體** | 不阻擋；但 A9 是 `prospect_id` BIGINT→UUID 的**觸發者**（`CARRYOVER.md:370`），要排進 A9 設計 |
| A3 agent（vendor portal） | R1、R2；`a3-buyer-seller.md:129,438-440` | 已進單體（提前建模組） | A9 的 agent 模組要擴充既有結構，不是新建 |
| **A6 review**：間接依賴 | ai-service 的 C2 `GetSellerStats` 在單體由 `legacySellerRatings{reviews}` 取評分（`cmd/hawkr/seller.go:70-74`、`sellerserver/server.go:43`），來源是舊 review-service 的 `ListStoreRatingStats` | review **仍在舊服務**；A6-1 會把 review 搬進單體（`a6-review-im-crm/interface-inventory.md` #8） | 不直接依賴：ai-service 只透過 `GetSellerStats` 這支 RPC 讀 rating；A6 若先完成，這支 RPC 的實作換成 `review.Module.StoreRatingStats`，對 ai-service 透明。若 A9 先完成，則 agent 直接依賴 seller（rating 來自 seller 對 review 的 port），需把 `arch/modules.go:39` 的 agent 依賴補上或確認走 seller 取得評分 |
| A6 im、crm | `rg -n 'im-service\|crm-service\|review-service\|IMService\|CrmService\|ReviewService' S --glob '*.go' -l` 無命中；ai-service 引用的 proto 套件（`go-pkg/proto/*`）只有 ai、buyer、seller、category 加上 order、product、media 經各 adapter；`arch/modules.go:39` 的 agent 依賴清單沒有 review／im／crm【量】 | — | **無依賴** |
| 反向：A6 依賴 ai-service？ | 同上方向的 `rg 'ai-service' B/services/{review,im,crm}-service`、`docs/plans/2026-11-19-a6-review-im-crm/*.md` 只在 `client-inventory.md:250,270` 提到「採購助理的 e2e 綁著 `/app/chat/*`」 | — | **一個耦合**：4 個 website e2e spec（`procurement-*.spec.ts`）為了載入頁面 mock 了 `/app/chat/*` 背景路由；A6 改 chat 契約時 mock 失效，A9 動採購頁時也會碰到。要在兩邊的 e2e 修改清單互相引用【量：`client-inventory.md:270`；spec 內容未開】 |
| Java 與 order／product／media／category | §3.1 C5–C9 | ordering、catalog 還是 Java（B8 之後） | A9 期間 ai-service 仍須跨進程呼叫 Java；這是 A9 與 **B8（ordering、catalog 進單體）** 的耦合，不是 A3／A6。A9 進單體後 agent 對 ordering、catalog 的依賴先走 `internal/legacy` adapter（與 A6-1 對 ordering 的處理同型）|

給計畫的修正建議【推】：主計畫「A9 獨立於 A3／A6」改寫為「A9 依賴 A3 已完成的 buyer／seller／agent 介面（不阻擋），間接讀 A6 的評分（不阻擋、可任意順序），與 A6 只有 e2e mock 與 `/app/chat/*` 的前端耦合；真正的前置是 B8 的 ordering／catalog 若未搬則 A9 走 legacy adapter」。

## 6. 根目錄 `api/` 既有的 Connect proto

【量】`ls api/hawkr`＝admin、agent、buyer、common、content、discovery、identity、moderation、payment、seller、social。

| 項目 | 現況 |
|---|---|
| 覆蓋 ai／assistant／procurement 的 proto | **沒有 AI 或採購的 service**。`rg -l 'agent\|procurement' -i api/hawkr/buyer/v1alpha1/buyer.proto api/hawkr/seller/v1alpha1/seller.proto` 只命中 seller.proto（註解）。唯一相關的是 `api/hawkr/agent/v1alpha1/agent.proto`：`VendorPortalPublicService`（PreviewInvite、ExchangeInvite）與 `VendorPortalInviteeService`（GetSession、VerifySession），即 vendor portal 的**存取層**，不含 RFQ 讀取、草稿、回價（那些還是 gateway REST，§1.3 #36、#43–47）【量】 |
| ErrorReason | `rg -n 'PROCUREMENT\|AGENT\|AI_' api/hawkr/common/v1alpha1/error_reason.proto` 無命中，即沒有 procurement／AI 的 reason【量】；gateway 的 procurement 錯誤用自己的 `reason` 字串（`S/internal/grpc/procurement_errors.go`、`procurement_error_reason_registry_test.go`、`G/procurement/wire.go`）；seller-app 有自己的 `procurement_error.dart` |
| 舊 proto | `B/api/proto/ai/ai.proto`（1489 行，單一檔、5 個 service、package `hawkr.ai`，產出到 `B/pkg/proto/ai`）。A9 要新增 `api/hawkr/agent/v1alpha1`（或拆 `assistant`、`procurement`、`imaging`、`admin`）的 Connect 契約，依 `docs/共通規範/API 設計規範（Connect）.md` |

## 7. 未決與要審查的點（每項附證據；去向欄只是建議）

| # | 問題 | 證據 | 建議 |
|---|---|---|---|
| U1 | D7：SSE 要改 server streaming，還是保留 AG-UI 並列為第二套契約 | 12 條 SSE 的 AG-UI 事件已被 web（zod）、seller-app（decoder）與 golden fixture 固定；`STATE_SNAPSHOT`、`TOOL_CALL_*`、`CUSTOM` 是 AG-UI 開放格式的詞彙（§4.2）；Connect 的 server streaming 要自訂 `StreamEvent` oneof（`P:49-90` 已經有，是同一個 oneof） | 先在 A9 設計用 methodology-brief 的「前提問題」：兩個前端已能吃 AG-UI，Connect streaming 的收益是單一契約與型別，成本是改兩個 client 與 golden；12 條裡 11 條是同一個 `StreamEvent`，決定一次即可 |
| U2 | 拿掉 pricebook 後的形狀與命名 | 主計畫 A9 項（`go-modular-monolith.md:254-256`）；`PreviewRfqPrice`（#32）與 `procurement_price_previews` 已刪行情欄位；`cmd/resolver-eval` 的 `observe` 還照舊形狀解析 | A9 設計審查觸發點已寫明 |
| U3 | 59 支 RPC 要不要全留 | 無呼叫端的 RPC 為 0，但 `PersonalizedRecommend` 固定匿名（#10）、`WatchRunEvents` 受 flag 控制（#57） | #10 與 #57 的去留要 Will 決定 |
| U4 | 圖片轉換（AIImageService）與後台樣式／額度（AIAdminService）是不是 agent 模組的職責 | 它們和採購、助理沒有共用資料（只共用 LLM 供應商金鑰與 `ai_usage`）；`arch/modules.go` 的模組邊界以資料擁有者為準 | 設計時評估拆成 `imaging` 模組或併入 agent；這決定 `agent` 模組內部 package 數量 |
| U5 | admin 端 7 條 REST 的歸屬 | 單體 admin 路由在 `M/httpapi/*` 與 `api/hawkr/admin/*`，ai 的 7 條還在 gateway `/admin/ai_*` | 與 U4 一起決定 |
| U6 | 40 張表與 51 個 migration 的搬遷方式 | `CSV/db-tables.csv`、單體每模組一個 schema 的慣例（`agent` schema 已存在且含 2 張表） | 合併進 `agent` schema 或分 schema；ADK 的 session 表（sessions、events、app_states、user_states）由 ADK 套件管，搬遷方式要查 ADK（`S/internal/agent/di.go`、`session_dsn_test.go`） |
| U7 | notification 對 agent 的反向呼叫（Ensure／Mark invite） | §2.1 E2 | 進單體後是 notification→agent 的 in-process，`arch/modules.go:42` 已允許 |
| U8 | 沿用盤點（從零設計會怎麼做） | outbox＋MQ、BIGINT ID、`LLM_*` 舊 env、硬寫 Gemini URL、`ai_service_conn` 的「共用一條連線」、procurement 內部 `graphspike`／`goldenset`／`testkit` 套件是否隨搬遷 | 寫進 agent 模組 `CARRYOVER.md` 的沿用盤點表（AGENTS.md 規定） |
