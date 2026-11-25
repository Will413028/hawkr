# A8-7 刪除盤點

盤點日 2026-11-25，dev `668e789c3`。A8-7 開工時發現計畫的範圍只列了服務本身與 gateway 路由，漏掉只為舊通知存在的周邊（gateway 的 SSE 平台、payload golden 契約、兩個 relay），也漏掉 A8-3 沒做完的一項交接。本檔列出每一項的去向，計畫 A8-7 的子步驟照本檔分工。

指令（在 repo 根目錄執行）：

```bash
rg -n "notification-service|hawkr_notification|notificationpb|notification\.proto" --hidden -g '!.git' -g '!**/docs/**' -g '!**/*.md' -g '!.worktrees/**' -g '!**/node_modules/**' -g '!**/target/**' -l
```

§3 的 route 分析：讀 `infra/mq/topology/topology.yaml` 的 queue 綁定與 `internal/legacy/rabbitmq/testdata/routes.json`，以 AMQP topic 規則比對，列出每個 route 除了 `notification.*` 以外還有哪些 queue（主 session 以一次性 Python 跑，未存檔）。

## 1. 刪除（A8-7a：舊服務、gateway、設定）

| # | 對象 | 依據 | 去向 |
|---|---|---|---|
| 1 | `backend/services/notification-service/`（含 `relay_routes_test.go` 的 `notifyAllowlist`、模板、`system_notification_logs`、FCM `SendToTopic`、`MailConfig`） | 盤點 §1–§6、缺陷 #21–#25 | 刪 |
| 2 | gateway 通知 handler：`notification.go`、`notification_inbox.go`、`notification_preference.go`、`admin_notification_template.go` 與它們的測試、`misc_response_shape_test.go`／`envelope_tier2_shape_test.go`／`admin_category_response_shape_test.go` 裡的通知案例 | 盤點 §1 | 刪（其他案例保留） |
| 3 | gateway gRPC client：`notification_client.go`、`notification_inbox_client.go`、`notification_admin_client.go`、`testing.go` 的通知 helper、`di.go` 接線 | 同上 | 刪 |
| 4 | gateway 路由：`routes_notification.go`、`routes_admin_notification.go`、`routes_buyer.go`／`routes_seller.go` 的通知偏好路由、`seller_route_assert`／`seller_capability_test` 的通知列 | 同上 | 刪。`authz.CapNotification` 保留：單體 seller 模組仍送 `CAP_NOTIFICATION`，gateway 不認得會拒絕整組能力（`gatewayproxy.TestCapabilityTokensMatchGateway`）；2026-11-25 實作時查出，原列「刪」不成立 |
| 5 | gateway SSE 平台：`internal/sse`、`internal/realtime`（只有通知與賣家工作區兩個 channel）、`internal/mq`（只服務 realtime）、`handler/sse_stream.go`、`holder.go` 的 `SSEHub`／`RealtimeReg`／`NotifChannel`、`ratelimit.go` 的 `/api/v1/app/notifications/stream` 豁免 | U10（`fact-checks.md`）；D19：賣家工作區改接 `WatchNotifications`，website 已改（A8-2d） | 刪。`/notifications/stream` 是唯一的 SSE 路由（`rg -n "sse|Stream" gateway/internal/router/routes*.go`），AI 的串流是 `proxy` 直通，不經 hub |
| 6 | `backend/api/openapi/openapi.yaml` 的通知 operation（`/app/notifications*`、`/buyers|sellers/me/notification-preferences`、`/admin/notification-templates*`、`/admin/notifications/broadcast`）與 `scripts/gateway-json-contract-baseline.json` 對應列 | 規則：刪路由時一起改（backend `AGENTS.md`） | 刪；發票的 `notification_resends` 不是通知服務的路由，保留 |
| 7 | `backend/api/proto/notification/`、`backend/pkg/proto/notification/`、`services/java/shared/java-proto/src/main/proto/notification/`（U11：與 `api/proto` 逐字相同，Java 0 處 import） | U11 | 刪 |
| 8 | `pkg/mq`：`ExchangeNotification`、`RoutingKeyNotification*`、`QueueNotification*`、`NotificationInboxNewEvent`、`NotificationUnreadChangedEvent`、`PublishNotification*`、`ExchangeSellerWorkspace`（只有 gateway realtime 讀） | 生產者與唯一消費者（gateway realtime）都刪 | 刪；先 `rg` 確認其他 module 沒有引用 |
| 9 | CodeMeta `NotificationEventType` 值域（`內部技術碼.md` §NotificationEventType）與生成的 Java `NotificationEventType.java` | 使用服務只有 notification 與 gateway，兩者都刪；CodeMeta README「刪除代碼」 | 從 `.md` 移除、重新生成；`MqConstants.EXCHANGE_NOTIFICATION*` 一起刪 |
| 10 | `internal/legacy/accountservice/events.go`（`account.deleted` relay）與 `cmd/hawkr/main.go` 的訂閱 | 盤點 §8.2 #7 寫「im 與 notification 還在訂閱」而保留；im-service 已刪（`ls backend/services` 只剩 ai、invoice、java、notification），notification 已改訂 `identity.AccountDeleted`（A8-1）。§3 的分析：`hawkr.account account.deleted` 只有 `notification.account.deleted` 綁定 | 刪（取代 §8.2 #7 的「保留」）；`accountservice/server.go` 不動 |
| 11 | `internal/legacy/reviewrelay` 的 `notify_seller_account_id`、`buyer_nickname`、`buyer_avatar_key` 與 `cmd/hawkr/review.go` 的對應註解 | A8-3d1 進度：只給舊服務，留到 A8-7 | 刪欄位與其 wire／golden 斷言；`review.created`、`review.hidden` 給 Java order、product 的部分保留 |
| 12 | `internal/legacy/rabbitmq` 的 `TestPublishRoutesManifest` 與 `testdata/routes.json` | 檔頭：manifest 只因 notification-service 是另一個 module 而存在 | 刪 manifest 與其測試；`TestPublishedRoutesAreBoundInTopology` 保留（單體送出的 route 要有人綁，對 Java 消費者仍有意義） |
| 13 | `infra/mq/topology/topology.yaml` 的 `notification.*` queue（`notification.procurement` 除外，見 §2）、`hawkr.notification`、`hawkr.notification.dlx`；沒有任何 queue 綁定、也沒有發送者的 exchange | 消費者刪除 | 刪 17 個 queue 與 7 個 exchange（上述兩個加 `hawkr.account`、`hawkr.content`、`hawkr.delegation`、`hawkr.im`、`hawkr.merchant`）；`hawkr.seller_workspace`、`hawkr.invoice`、`hawkr.marketing` 保留（Java order、aftersales、invoice 等仍是發送者並自己宣告）；`test_expand.py` 的數量釘改成 32/87/0/106（依現行 topology 重算） |
| 14 | 本機環境（`RESEND_API_KEY` 保留：單體 `cmd/hawkr/notification.go` 讀它）：`go.work`、`backend/Makefile`、`dev.sh`（`GO_SERVICES`、`GO_SEED_SERVICES`、`hawkr_notification`）、`docker-compose-local.yml`（服務與 gateway 的 `NOTIFICATION_SERVICE_HOST`）、`scripts/init-databases.sh`、`.env.template`（`NOTIFICATION_SERVICE_*`、`MAIL_*`、`RESEND_API_KEY`、只有舊服務讀的 FCM 設定；單體仍讀的保留） | 盤點 §6 #5、#12、#13 | 刪；每個變數先 `rg` 確認單體與其他服務沒有讀 |
| 15 | CI 與供應鏈：`backend-contract-tests.yml`（整檔：唯一的 job 跑已刪的模板契約測試；原列在 §4，2026-11-25 提前刪，golden 檔仍留到 A8-7b）、`backend-integration-tests.yml`、`backend-go-checks.yml`（notification-service 與 routes.json 的路徑規則）、`.github/dependabot.yml` 的 notification-service 項、`scripts/govulncheck-allowlist.json` 的 notification-service 條目 | 同上 | 刪；allowlist 條目若同時列其他 module 只拿掉這個 module |
| 16 | 監控：`service-alerts.yml` 的 `notification-service` 存活告警、`business-alerts.yml` 與 `tests/vendor-portal-alerts.test.yml` 的站外業者邀請信告警（consumer 已於 `5c0292db2` 刪除）、`invoice-slo.rules.yml` 的 `notification_email_*` 紀錄規則（指標只由舊服務產生） | 盤點 §6 #5 | 刪；`promtool test rules` 能跑就跑 |
| 17 | `internal/httpapi/session.go` 的 `DELETE /api/v1/app/notifications/device-token` 停權放行（A7-D18）與 `suspension_test.go` 的對應案例 | 路由刪除；Connect 的放行在 A8-2b | 刪 |

## 2. 交接缺口（A8-3f：採購邀請）

`procurement/CARRYOVER.md` 的 relay 列寫明「交接點（A8-3）：`rfq_invited` 意圖 job 接手投遞時，同時接手 `MarkInviteSent`／`MarkInviteFailed`，刪 `procurementrelay`」。A8-3c 建了 `rfq_invited` 型別與訂閱（`notification/internal/intake/subscribers_governance.go:213`），沒有接手狀態寫入，也沒有刪 relay：`rg -n "MarkInvite" backend/internal/modules/notification` 0 hit。

- 刪 relay 之前，邀請的 `delivery_state`（proto `InviteDeliveryState`，賣家端 summary 顯示）要有新的寫入者，否則永遠 `QUEUED`。
- 進行中的分支 `a9-6`（另一個 session）在 `ea5a87391` 把 relay 接進 `cmd/hawkr`；dev 上刪 relay 會與它衝突。
- `notification.procurement` queue 與 relay 同步刪：relay 還在時，`TestPublishedRoutesAreBoundInTopology` 要它（§3）。
- 去向：D30 已決 (b)（Will 2026-11-25）：刪 `delivery_state` 與 relay，由 A8-3f 做；不在 A8-7a 做。
- 原寫的「與 `a9-6` 分支協調」已不需要：a9-6 已合併進 dev。

## 3. 拿掉 `notification.*` queue 之後

單體 route（`routes.json` 17 筆）只剩兩筆沒有別的 queue：`hawkr.account account.deleted`（§1 #10 刪 relay）、`procurement.events procurement.rfq_publication_invite.created`（§2）。

Java 與舊 Go 服務送出、只有 `notification.*` 綁定的 key 會變成無人接收（例：`hawkr.invoice`、`hawkr.marketing`、`hawkr.merchant`、`hawkr.content` 全部，`hawkr.order`、`aftersales.events`、`hawkr.shipping`、`payment.events` 的一部分）。後果依發送方式：

- Go `pkg/mq`：`mandatory=false`（`pkg/mq/publisher.go:215`），訊息被 broker 丟棄，沒有錯誤。
- Java `ObsRabbitPublisher.publish`：`template.mandatory=true` 加 `publisher-returns`，只觸發 return callback 的 log，發送端視為成功。
- Java `publishConfirmed`：把無人接收當失敗。呼叫者只有 `PaymentOutboxPublisher`（無人接收的 key 例：`payment.rescue.refund_completed`，會走退避後進 DEAD）與 aftersales `ReturnStuckAlertScheduler`（`shipment.anomaly.detected`，每輪排程重試並記錯誤）。

系統未上線，影響只在本機與 CI；這是「各發送端搬進單體前那些事件沒有通知」（Will 2026-11-23）的機械後果，計畫記在 A8-7 的完成後狀態，不改舊服務。

## 4. 退場的 payload golden 契約（A8-7b）

`backend/tests/contract/golden/`（`event_payload_keys.json`、`known_pending.json`、`known_pending_identity.json`）是「notification 模板讀的 key ⊆ 發送端送出的 key」的契約（目錄 `README.md`）；它自己的規則寫明沒有模板消費的條目要刪。模板退場後整份契約沒有消費者。

| 讀取者 | 去向 |
|---|---|
| `services/notification-service/internal/worker/template_contract_test.go` | 隨 §1 #1 刪 |
| `internal/legacy/reviewrelay/golden_test.go`、`internal/legacy/buyerrelay/relay_test.go` 的 golden 斷言、`internal/legacy/procurementrelay/golden_test.go`（隨 A8-3f） | 刪 golden 斷言；欄位對 Java 消費者的約束由各自的 `wire_test` 守 |
| Java：aftersales `AwaitingBuyerRefundPagePayloadGoldenTest`、`RefundCompletedPayloadGoldenTest`、`GoldenPayloadKeys`，product `ProductCertificationEventGoldenTest`，shipping `ShippingPayloadGoldenContractTest` | 刪（只比對 golden；刪前逐檔確認沒有對 Java 消費者的其他斷言，有就留下那部分） |
| invoice-service `event_notify_routing_test.go`、ai-service `golden_contract_test.go` | 同上 |
| `.github/workflows/backend-contract-tests.yml`（已於 A8-7a 刪）、`backend-monolith.yml` 的 golden 路徑 | `backend-monolith.yml` 的 golden 路徑隨 golden 檔刪 |

## 5. 允許的殘留

驗收的 `rg` 只會命中下列項目；其餘一律 0 hit。

| 類別 | 例 | 理由 |
|---|---|---|
| 新契約的生成與引用 | `api/hawkr/**/notification.proto`、`backend/internal/httpapi/gen`、`packages/hawkr-api-ts/src/gen`、`packages/hawkr_api`、`packages/hawkr_ui_text`、`website/lib/notifications/watch.ts` | 計畫的 `rg` 樣式 `notification\.proto` 也命中 A8-2 的新 proto；這些是現行契約 |
| 還沒搬的服務的註解 | `services/java/**`、`services/invoice-service/**`、`services/ai-service/**`、`pkg/outbox`、`pkg/cmd/protonumericlint`、`api/proto/account/delegation.proto` 與其生成碼 | 舊服務「只做維持運作所需的修改」（backend `AGENTS.md`）；各自搬進單體時隨「模組完成的定義」的通知型別條目一併處理 |
| 凍結的 CodeMeta 生成檔 | `ShipmentAnomalySeverity` 的說明文字（Go、TS、Dart、Java 的生成檔） | CodeMeta 凍結，該值域仍由 Java 使用；隨 shipping 搬遷退場 |
| 測試 fixture 的任意路徑 | `website/lib/api/session-ended.unit.test.ts`、`website/tests/sse-revive.test.ts`、`backend/internal/httpapi/entry_test.go` 的 `/notifications/stream` | 測的是 `SseClient` 與反代的串流行為，路徑只是字串；`SseClient` 仍有 AI 串流使用 |

## 6. A8-7a 實作結果與偏差（2026-11-25）

- 分支 `a8-7a`（dev `974e91530` 起）。`events.csv` 與 `rpc-servers.csv` 補 36 列 disposition（`scripts/check-migration-inventory.py` 的 gate 要求）。
- 偏差已寫進 §1 各列：#4（`CapNotification` 保留）、#13（`hawkr.seller_workspace`、`hawkr.invoice`、`hawkr.marketing` 保留）、#14（`RESEND_API_KEY` 保留）、#15（contract workflow 提前刪）。
- #16 多刪兩條 `NotificationEmailQuotaNearLimit`、`NotificationEmailRejected` 告警：指標 `notification_email_*` 已沒有產生者。
- #11 連帶刪 `reviewrelay.Worker` 只為那三欄存在的 `Buyers`、`Stores` port（`NewWorker` 少兩個參數）。
- 留下的死碼：`backend/pkg/mail`（`MAIL_*` 的讀取者）已沒有 importer；`MqConstants.java` 的 `EXCHANGE_ACCOUNT`、`EXCHANGE_CONTENT`、`EXCHANGE_IM` 等指向已不存在 exchange 的常數。兩者屬還沒搬的舊服務，隨各自搬遷退場（§5）；`pkg/mail` 在 A8-8 收尾時確認並刪。
- CodeMeta 重新生成時帶出的 `registry.go`、`InvoiceInboxDisposition.java`、`ReportTargetType.java` 說明文字，來源 `.md` 早已改過，只是生成物同步。
- 本機沒跑：Java 測試、integration、docker、`promtool`（未安裝）。
- 與 `origin/dev` 合併時（a9-6 已刪 ai-service 與 gateway 的 AI 代理）：`ProdOtlpServiceAbsent` 告警兩個目標服務都已不在，整條刪除；`ai_assistant_stream.go` 隨 dev 刪除；`maxRESTOperations` 改 246（合併後實數）。
