# MQ 與事件管理規範

> 現況（作品集轉 public 步驟 2，D7）：公司時期的部署環境設定已刪除——`infra/mq/envs/{dev,prod}`、`mq-topology.yml` 等部署 workflow、`monitoring/` 的 prod exporter 與 `prod-infra.json`、image-gateway 的 dev／production 環境與 custom domain、website／admin 的 `wrangler.jsonc` env 區塊。下文提到這些的部署、plan／apply 與環境操作段落是公司時期的歷史紀錄，不是現行流程；部署設定等選定 demo 部署目標時重寫。

> **適用範圍（2026-09-24 起）**：只適用遷移期間仍在運作的舊服務。單體不使用 RabbitMQ：需要原子性的狀態變更用同一個 transaction 內的 in-process 事件，其餘改用 River job（`docs/adr/2026-09-24-go-modular-monolith.md` D4、D6）。RabbitMQ 在遷移計畫階段 C 移除，本規範隨之退場。


> **本規範為 MQ / Event 設計流程規範**，與 `docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md`（內容 SSOT）配合使用。
>
> - **本文件回答**：如何新增 / 修改 / 查詢 MQ 與事件？流程是什麼？審查重點？
> - **總覽文件回答**：目前有哪些 MQ / Event？對應什麼 service？payload 是什麼？
> - **topology 佈建回答**（exchange / queue / binding 由誰建立、怎麼改、怎麼刪）：`docs/基礎建設/MQ Topology 管理（Terraform）.md`（權威），操作速查 `infra/mq/README.md`

> ⚠️ **2026-07-24 起 broker 資源改由 Terraform 集中管理**：dev / prod 的服務啟動時走 **passive declare**（Go `MQ_PASSIVE_DECLARE=true`、Java `SPRING_RABBITMQ_DYNAMIC=false`），**應用程式不再能建立任何 exchange / queue / binding**，唯一 owner 是 `infra/mq/topology/topology.yaml`。queue 沒登記 → 服務照常啟動、pod 不重啟、**該筆訂閱失效且幾乎無聲**（Go 端只在啟動當下印一行 `[MQ][ERROR] skipping queue ...`，之後全無跡象；Java 端行為待坐實，見 §7.2）。本規範中涉及 queue 生命週期的章節（§1.3 / §2.1 / §3.1 / §3.3 / §四 / §6.1 / §7.2）皆以此為前提。local（OrbStack）不設 passive flag，仍由 app 自建 topology，不受影響。

---

## 一、核心原則（強制）

### 1.1 集中管理

| 規則 | 說明 |
|------|------|
| **唯一 SSOT** | `docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md` 是所有 MQ / Event 的唯一真實來源 |
| **禁止分散管理** | 不得僅在各服務設計書中描述 MQ 設計，必須同步更新 36 號設計書 |
| **新增前查 SSOT** | 新增 MQ / Event 前，必須先閱讀 36 號設計書確認無重複 |
| **修改先改 SSOT** | 修改 routing key / payload / queue 前，必須先更新 36 號設計書 |
| **程式碼與文件不一致 = bug** | 任何漂移視為 bug，必須立即修正或標註「需確認」 |

### 1.2 命名規範

依 36 號設計書 §1.4，所有 MQ 物件命名必須遵循：

| 物件 | 規範 |
|------|------|
| Exchange | `{domain}.events` 或 `hawkr.{domain}` |
| DLX | `{exchange}.dlx` |
| Queue | `{consumer-service}.{event-source}.{action}` |
| DLQ | `{queue-name}.dlq` |
| Routing Key | `{domain}.{action}` 或 `{domain}.{action}.{result}`（全小寫） |
| Payload Class | `{Event}Payload` 或 `{Event}Event` |
| Consumer Class | `{EventSource}{Topic}Consumer` |
| Publisher Class | `{Domain}EventPublisher` |

### 1.3 程式碼層強制

| 規則 | 違規後果 |
|------|---------|
| Java Publisher 必須用 `obsRabbitPublisher.publish(...)` | trace / correlation 遺失，違反 `docs/開發規範/Java 微服務開發規範.md` §8.2 |
| Outbox 排程器投遞必須用 `obsRabbitPublisher.publishConfirmed(...)`（等 broker ack 才標 SENT） | broker 未收下卻誤標 SENT → 事件遺失（issue #1349，見 §8.2） |
| Java Consumer 必須用 `@RabbitListener` + `manualAckContainerFactory` | 訊息可能遺失，違反 `docs/開發規範/Java 微服務開發規範.md` §10.2（Manual ACK 由 `RabbitMqAutoConfiguration` 提供） |
| Exchange / Queue / Routing Key 必須走 `MqConstants` 常數 | 修改易遺漏，命名漂移 |
| `RabbitMqConfig` 必須宣告對應 `@Bean Queue` + `@Bean Binding`（**意圖宣告**：passive declare 下 `SPRING_RABBITMQ_DYNAMIC=false`，Spring 不再實際佈建，僅表達「本服務需要這條 queue」並供 review 與 topology 對照） | 讀者無從得知該 listener 依賴哪條 queue，topology 對照失去依據 |
| Consumer 監聽的 queue 必須登記於 `infra/mq/topology/topology.yaml`（唯一佈建來源） | **該訂閱失效且幾乎無聲**（**Go 端；Java 待坐實**）——服務正常啟動、pod 不重啟、不進 DLQ（queue 根本不存在），發給它的訊息直接丟棄；Go 端僅啟動時印一行 `skipping queue`（issue #1795） |
| 禁止 `Jackson2JsonMessageConverter` 全域 Bean | byte[] 雙重序列化，違反 `docs/開發規範/Java 微服務開發規範.md` §8.6 |
| 跨語言（Java ↔ Go）同邏輯領域必須用同名 exchange | 訊息漏路由 |

---

## 二、新增 MQ / Event 流程

### 2.1 完整流程（強制順序）

依 `docs/開發規範/開發工作流程.md` §1 強制順序（設計書 → Code Gen → DB → 程式碼 → 測試），新增 MQ 事件具體展開：

| 步驟 | 操作 | 禁止事項 |
|------|------|---------|
| 1 | 在 36 號設計書 §三 對應領域新增事件條目（含 routing key / payload / producer / consumer / DLQ / idempotency） | 禁止跳過設計書 |
| 2 | 同步更新對應服務的詳細設計書（例：訂單事件 → 03 號訂單設計書 §事件章節） | 禁止只改 36 而不改服務設計書 |
| 3 | 更新 `docs/代碼管理（CodeMeta）/內部技術碼.md` 對應 EventType enum（若新增 routing key） | 禁止直接改 Java Enum |
| 4 | 執行 `cd docs/代碼管理（CodeMeta）/scripts && ./generate.sh --java` | 嚴禁手動修改生成檔案 |
| 5 | 更新 `MqConstants.java` 加入新的 Exchange / Queue / Routing Key 常數（若 generate.sh 未涵蓋） | 禁止字面值硬編碼 |
| 6 | Publisher 服務實作 `Publisher` 方法，使用 `obsRabbitPublisher.publish(EXCHANGE, ROUTING_KEY, payloadBytes, eventType)` | 禁止用 `RabbitTemplate.convertAndSend` 直接寫 |
| 7 | Consumer 服務在 `RabbitMqConfig` 加 `@Bean Queue` + `@Bean Binding`（必要時加 `@Bean DLQ` + DLX 配置）——意圖宣告，不再實際佈建（§1.3） | 禁止只寫 `@RabbitListener` 不寫 Config |
| 8 | **同一個 PR** 加 `infra/mq/topology/topology.yaml` 條目（宣告式：`retry:` / `dlq:` policy；`.retry` / `.dlq` 與其 binding 由展開規則自動產生），並在本機跑三道閘門：<br>`cd infra/mq/tools && python3 validate.py --topology ../topology/topology.yaml`<br>`python3 -m unittest discover -p 'test_*.py'`<br>（回 repo root）`python3 scripts/mq-consumer-registration-check.py` | 禁止只改程式碼不登記 topology（訂閱失效，症狀與排查見 §7.2）；**policy 會展開的衍生資源不要手寫**（`.dlq` / `.retry` 與其 binding，手寫會重複產生）——共用 DLQ 目的地與自建 retry queue 是例外，見 §6.1 |
| 9 | Consumer 實作 `@RabbitListener(queues = MqConstants.QUEUE_XXX, containerFactory = "manualAckContainerFactory")` | 禁止用預設 container factory |
| 10 | 關鍵路徑（金流 / 訂單核心）加 idempotency 保護（dedup 表或 DB 樂觀鎖） | — |
| 11 | 本地測試（含模擬重複訊息 + 失敗重投） | 禁止只驗 happy path |

> 步驟 8 的 topology 條目怎麼寫（五種 queue 形狀、policy 展開規則、`#` 綁定注意事項）見 `infra/mq/README.md`；展開規則的權威出處是 `docs/基礎建設/MQ Topology 管理（Terraform）.md`「展開規則」段。

### 2.2 是否為「跨團隊契約事件」判定

| 判定 | 處置 |
|------|------|
| Producer 與 Consumer 都在我們 9 個 Java 服務內 | 一般流程 |
| Producer 在我們、Consumer 在跨團隊（Go 服務） | Outbound 契約：必須先與對方團隊對齊 schema |
| Producer 在跨團隊（Go 服務）、Consumer 在我們 | Inbound 契約：必須先請對方確認 exchange/RK 規範 |
| Producer 與 Consumer 都在跨團隊 | 不在本規範範圍，但仍要在 36 號設計書中記錄（協同契約） |

---

## 三、修改 MQ / Event 流程

### 3.1 變更類型與允許範圍

| 變更類型 | 是否允許 | 原因 |
|---------|---------|------|
| Payload 新增欄位（向後相容） | ✅ 允許 | Consumer 可忽略未知欄位 |
| Payload 修改欄位型別 | ❌ 禁止 | Consumer 反序列化失敗 |
| Payload 刪除欄位 | ❌ 禁止 | 舊 Consumer 可能依賴 |
| Routing Key 重新命名 | ❌ 禁止 | 改名 = 新事件 + 棄用舊事件 |
| Exchange 重新命名 | ❌ 禁止 | 跨服務契約破壞 |
| Queue 重新命名（Consumer 端） | ⚠️ 需停機 | 必須先停舊 listener、確認 queue 空、再切到新 queue |
| 新增 / 修改 Queue arguments（含啟用 `retry:` / `dlq:` policy） | ⚠️ 對既有環境破壞性 | RabbitMQ 不支援 in-place 改 args，Terraform 算成 `must be replaced`（先刪後建）→ **queue 內未消費的訊息一起消失**。**必走 §3.3 SOP**（見下） |
| 刪除 queue | ⚠️ 對既有環境破壞性 | 同上，`destroy` 與 replace 在 CI 的 destructive gate 眼中同級。**必走 §3.3 SOP** |
| 新增 Idempotency 保護（純 application 層 dedup 表） | ✅ 允許 | 不涉及 queue args |
| 修改 Idempotency 機制 | ⚠️ 需評估 | 變更 dedup 表結構需 DB migration |

### 3.2 破壞性變更處理

當必須做破壞性變更（如 payload 改型別、routing key 改名）：

| 步驟 | 操作 |
|------|------|
| 1 | 在 36 號設計書 §三 加新版本事件條目（如 `order.paid.v2`） |
| 2 | Producer 端同時發送舊 + 新版本（過渡期） |
| 3 | Consumer 端逐一遷移至新版本 |
| 4 | 確認所有 Consumer 已遷移後，Producer 停止發送舊版本 |
| 5 | 從 36 號設計書移除舊事件條目 |

### 3.3 Queue arg 變更 / 刪除 SOP（強制）

> ⚠️ **任何修改既有 queue arguments 或刪除 queue 的變更**（含 Go `pkg/mq.Subscription.MaxRetries` 從 0 → 3 啟用 retry/DLQ、TTL 設定、max-length、quorum queue 切換、DLX 指向變更）**必走本 SOP**。2026-07-24 起 broker 資源的唯一 owner 是 `infra/mq/topology/topology.yaml`，本節所有動作都在 Terraform 流程內完成——**不再有任何「SSH 進 dev / prod 手刪 queue 讓服務重新宣告」的步驟**。passive declare 下沒有人會重建被刪掉的 queue，手刪 = 該訂閱永久失效。

#### 3.3.1 為什麼「改 args」等於「重建 queue」

RabbitMQ 不支援 in-place 修改 queue arguments。Terraform 因此把 `arguments` 的變更算成
`must be replaced`——plan 的 `actions` 是 `["delete", "create"]`，也就是**先刪後建**。

| 事實 | 後果 |
|---|---|
| queue 被 replace / destroy | **裡面未消費的訊息一起消失**（與直接刪 queue 同級，CI 的 destructive gate 也一視同仁） |
| passive declare 下服務不會自己重建 queue | queue 消失而 topology 沒補回 → 該訂閱失效，且服務照常運行不重啟（見 §7.2）；即使事後補上 topology 並 apply，**訂閱也不會自癒**，要 `kubectl rollout restart` |
| plan 的 `must be replaced` / `destroy` 是唯一預警 | PR 階段不跑 destructive gate，**CI 綠燈不代表這個 PR 沒有破壞性改動**，要自己讀 plan |

#### 3.3.2 PR 描述強制 checklist

在 PR description 中**必須**列出（缺一項即不可 merge）：

| 項目 | 說明 |
|---|---|
| 受影響的 queue 清單 | 本 PR 讓 plan 出現 `must be replaced` / `destroy` 的每一條 queue（含 policy 展開出來的 `.dlq` / `.retry` 衍生資源） |
| 預期新 args | 例：`x-dead-letter-exchange=hawkr.content.dlx`；以 `topology.yaml` 的 `retry:` / `dlq:` policy 形式說明 |
| dev + prod 兩份 plan 的判讀結論 | 逐環境列出 replace / destroy 的 address 清單，並附 plan 末尾的 `Plan: X to add, Y to change, Z to destroy`（CI 貼的是 `tail -c 60000`，長 plan 前段會被截掉，以這行數字為準） |
| 「queue 內訊息是否可接受消失」評估 | 逐條確認 `messages=0`；in-flight message 不可丟 → 走 §3.3.5 |
| destructive apply 的執行窗口 | 破壞性變更需人工 `workflow_dispatch` 觸發，指定低峰時段 |

#### 3.3.3 標準流程（強制順序）

1. 改 `infra/mq/topology/topology.yaml`（與服務程式碼**同一個 PR**）
2. 本機跑三道閘門（秒級、零連線）：

   ```bash
   python3 -m pip install -r <repo>/infra/mq/tools/requirements.txt   # 首次或依賴更新時（PyYAML）

   cd <repo>/infra/mq/tools
   python3 validate.py --topology ../topology/topology.yaml
   python3 -m unittest discover -p 'test_*.py'

   cd <repo>
   python3 scripts/mq-consumer-registration-check.py
   ```

3. （建議）對 dev 跑**唯讀** plan 看清楚會動到什麼：`cd infra/mq/envs/dev` → `terraform plan -input=false -detailed-exitcode -refresh=false`（credentials 取得與 `-refresh=false` 的理由見 `infra/mq/README.md`「本機驗證 ③」）。⚠️ **本機對 dev / prod 執行 `apply` 一律禁止**——R2 state 沒有 locking，並行 apply 會互相覆蓋
4. push → 開 PR。CI 兩處各司其職：
   - `mq-topology.yml`：schema validate → **對 dev + prod 各跑一次 plan**，貼進 step summary
   - `static-gates.yml` 的「MQ consumer ↔ topology 登記一致性」step：雙向登記檢查（blocking，**只掃 Go**）
5. **人工讀兩份 plan**，搜 `must be replaced` 與 `destroy`。⚠️ 兩個陷阱：**(a)** step summary 貼的是 `tail -c 60000`（plan 的**最後 60KB**，長 plan 前段會被截掉）——以 plan 末尾那行 `Plan: X to add, Y to change, Z to destroy` 的數字為準，數字對不上你的預期就別 merge；**(b) PR 階段的 plan 含 destroy 不會讓 CI 變紅**（destructive gate 有 `if: mode == 'apply'`），綠燈不等於安全
6. Review → merge 到 `dev`：
   - **非破壞性**（plan 只有 `create` / `update`）：push 觸發 CI 自動 apply dev；promote 到 `main` 後自動 apply prod
   - **破壞性**（`actions` 含 `delete`，replace 亦然）：destructive gate 會列出所有 address 並**失敗、不 apply**。人工再審一次 plan 確認每條都是預期的，於**低峰時段**手動觸發：

     ```bash
     gh workflow run mq-topology.yml --ref dev  -f env=dev  -f allow_destructive=true
     gh workflow run mq-topology.yml --ref main -f env=prod -f allow_destructive=true
     ```

     ⚠️ **`--ref` 與 `env` 必須對齊，而且沒有任何機制會替你檢查**：workflow 的 `actions/checkout` 不指定 ref（用觸發 ref，`gh` CLI 與 UI 預設都是 default branch `main`），`decide` job 的 workflow_dispatch 分支也只讀 `inputs.env`、不比對 ref。從 `main` 派 `env=dev` = 拿 **main 版 topology 覆蓋 dev**，dev-only 的 queue 全被 destroy——而 `allow_destructive=true` 正好把唯一的 gate 關掉。本 repo 2026-07-13 已有同類事故（dispatch 漏帶 `--ref` 打到 prod）。
7. 驗證（每環境各做一次）：

   ```bash
   # 目標 queue 以新 args 存在，且 consumer 已重新連上（consumers ≥ 1）
   rabbitmqctl list_queues name arguments messages consumers | grep -E "<queue1>|<queue2>"
   ```

   再抽樣 publish 一則測試訊息，確認 routing → consume → ack 路徑完整。**consumer 數才是判準**——queue 存在但 `consumers=0` 代表服務側沒接上（passive declare 不會為此報錯）。

   > ⚠️ **apply 成功不代表訂閱恢復**：`pkg/mq/resilient.go` 只在啟動／reconnect 時 declare，開機時被 skip 的 subscription **不會自癒**，而純 topology 的 PR 也不會重啟任何 pod。`consumers=0` 時執行 `kubectl rollout restart deployment/<service>` 讓它重新接上。這屬於「讓已部署的配置生效」的維運動作（後端工程師戴維運帽子主動做，見 `docs/開發規範/後端服務責任邊界規範.md`），**不是**用 restart 取代 code 的 CI/CD 部署——沒有 rebuild image 的假部署仍然禁止。

#### 3.3.4 跨環境順序

| 環境 | 順序 |
|---|---|
| local（OrbStack） | 不設 passive flag，app 仍自建 topology——開發者自行刪本機 queue（`docker exec hawkr-rabbitmq rabbitmqctl delete_queue <queue>`）或整個 reset volume（`./dev.sh down` 之後 `docker volume rm backend-payuni_rabbitmq_data`）即可，**不需要也不會動到 Terraform**。⚠️ `./dev.sh down` **不轉傳任何參數**（`cmd_down` 只跑 `docker compose down`），`-v` 會被吃掉、volume 不會被清；`./dev.sh clean` 也明示不動 volumes |
| dev | merge 到 `dev` → CI 自動 apply；破壞性則人工 `gh workflow run mq-topology.yml --ref dev -f env=dev -f allow_destructive=true` |
| prod | dev 驗證通過後 promote `main` → CI 自動 apply；破壞性同上但 `--ref main -f env=prod`，指定低峰窗口 |

> ⚠️ **`--ref` 漏帶或與 `env` 不對齊 = 用另一個環境的 topology 覆蓋目標環境**（`gh` 預設 ref 是 `main`，workflow 不做對齊檢查，理由見 §3.3.3 步驟 6）。

> **禁止**：SSH 進 dev / prod broker 手跑 `rabbitmqctl delete_queue`（Terraform 不知道的 drift，且 passive declare 下沒有人會把它建回來）、在本機對 dev / prod `terraform apply`、以「dev 先手動處理」的方式繞過 CI。**唯讀查詢不在此限**——`list_queues` / management UI 看 args、messages、consumers 隨時可做，也是排查的第一手證據；受禁的是**寫入性**操作（`delete_queue`、手建 queue/binding、本機 apply）。CI 全掛時的緊急後門見 `infra/mq/README.md`，事後必須補回 `topology.yaml` 並確認 nightly drift 歸零。

#### 3.3.5 例外：queue 含 in-flight message 不可丟

replace / destroy 會讓 queue 內未消費的訊息一起消失。若不可丟（例：訂單付款事件）：

| 步驟 | Java 服務（Spring config 動態） | Go 服務（無動態 flag） |
|---|---|---|
| 1 暫停 producer | feature flag / `application.yml` toggle → `/actuator/refresh` | `kubectl scale deployment/<publisher> --replicas=0` 停 publisher container |
| 2 等 queue 排空 | RabbitMQ console 確認 `messages=0`（consumer 仍在消化） | 同左 |
| 3 執行 destructive apply | `gh workflow run mq-topology.yml --ref <dev\|main> -f env=<dev\|prod> -f allow_destructive=true`（§3.3.3 步驟 6） | 同左 |
| 4 恢復 producer | toggle 回 / config reload | `kubectl scale deployment/<publisher> --replicas=N` |

排不空又非動不可的極端情況：先以 `rabbitmqadmin get ackmode=ack_requeue_true` 備份 payload，apply 完成後用 **default exchange 直投該 queue** 重放（**禁止投回原 exchange**——會讓綁同一 routing key 的其他 queue 重複收到）。

#### 3.3.6 教訓 / 過去事件

> 前兩起發生於 2026-07-24 集中化**之前**（app auto-declare 時代，`amqp.QueueDeclare` 的 assert 語意會回 406 PRECONDITION_FAILED）。保留的理由是失效模式仍然成立：**動 queue args 會讓既有 queue 不可用，而部分服務對此完全沉默。** 這兩起「逐環境手動 ops」的失效面正是 Terraform 集中化要消除的東西——現在 dev / prod 吃同一份 `topology.yaml`，不再有「dev 做過 ≠ prod 做過」。

- **2026-06-16 PR #989**（issue #1010）：notification-service 啟用 `MaxRetries=3` 後，framework 對 `notification.merchant.follower` queue assert 新 args 失敗 → 80+ 次 reconnect → dev notification 全鏈不通。**直接原因**：PR 描述未列受影響 queue 清單，部署時遺漏對應的 ops 動作。
- **2026-07-23 issue #1625**：`pkg/mq` 啟用 DLX args 的版本上 prod 時 dev 有做 ops、prod 漏做 → prod 6 條 queue（crm/buyer/seller×3/social）406 斷線 5 天：crm 因單一 subscription 整服務 reconnect loop，其餘服務**靜默 skip**（僅啟動時 log 一行，外表正常）；同時 RabbitMQ prometheus plugin aggregated 模式無 per-queue label，`RabbitMQNoConsumers` 告警從未生效。修復＝零窗口重建 ＋ prometheus `/metrics/detailed` per-queue 告警（`RabbitMQQueueNoConsumersStalled`：業務 queue consumers=0 即告警，不要求有堆積）。教訓：**部分 queue 被 skip 的服務外表正常，必須靠 queue 級監控抓**。
- **2026-07-25 issue #1795**（passive declare 上線後第一起）：`crm.aftersales.refund.completed` 這條新 queue 晚於 topology 匯出才 merge，broker 上根本不存在 → 該筆 subscription 失效（服務正常啟動、不進 DLQ，Go 端只有啟動時一行 `skipping queue` 沒人看到），退款事件在 dev 完全沒進 crm-service。防線＝§2.1 步驟 8 ＋ `static-gates.yml` 的「MQ consumer ↔ topology 登記一致性」step（**只掃 Go**）＋ notification-service 的 `TestNotificationQueuesExistInTerraformTopology`（單元測試，不同步就測試 fail）；**Java consumer 沒有自動攔截**，只能靠 §6.1 人工對照。

---

## 四、刪除 MQ / Event 流程

🔴 **順序鐵則：code 必須先部署到「不再訂閱」的版本，才能 apply 刪除。**
步驟 1–2 的 grep 證明的是**原始碼**，不是**執行期**——跑著的 pod 仍可能在舊版本上訂閱著它。
反過來做＝把 queue 從一個還連著的 consumer 底下抽掉：服務**不會 crash**，broker 送 `basic.cancel`
→ consumer 重連 → 重連時 passive declare 查不到 → **靜默跳過該筆訂閱**，與「新增時漏登記 topology」
同一個失效模式（鏈路四段與行號見《MQ Topology 管理（Terraform）》「刪除 queue」節）。
所以步驟 12 是承重的，不是形式；⚠️ 且步驟 13 的 push 會讓 `MQ Topology` workflow 變紅（plan 算出
destroy → destructive gate `exit 1`），**那是預期行為**，它在等你走完步驟 12 再手動 dispatch。

| 步驟 | 操作 |
|------|------|
| 1 | grep 全專案確認所有 Producer 已停止發送（含 Java + Go）——**這是原始碼層**，不代表執行期 |
| 2 | grep 全專案確認所有 Consumer 已停止監聽——同上，執行期由步驟 12 把關 |
| 3 | RabbitMQ console 確認對應 queue 已無訊息積壓 |
| 4 | 從 36 號設計書移除條目 |
| 5 | 從對應服務設計書移除引用 |
| 6 | 從 `docs/代碼管理（CodeMeta）/內部技術碼.md` 對應 EventType 移除 |
| 7 | 執行 `generate.sh --java` 重生 Enum |
| 8 | 從 `MqConstants` 移除常數（若 generate.sh 未涵蓋） |
| 9 | 刪除 Publisher / Consumer / Payload Class 程式碼 |
| 10 | 刪除 `RabbitMqConfig` 中對應 `@Bean Queue` + `@Bean Binding` |
| 11 | 從 `infra/mq/topology/topology.yaml` 移除該 queue 條目（`retry:` / `dlq:` 展開出來的 `.retry` / `.dlq` 會一併消失，不必也不可手動列出），跑 §3.3.3 步驟 2 的三道閘門 |
| 12 | 🔴 **執行期閘門（不可略過）**：merge 後**等 deploy 完成**，確認服務真的跑到不再訂閱的版本（用內容定址 tag 對帳，不是看 workflow 綠燈），再確認該 queue `consumers = 0`：<br>`kubectl exec -it <rabbitmq-pod> -- rabbitmqctl list_queues name consumers \| grep <queue>`<br>仍 >0 就是還有 pod 連著，**此時 apply 會造成靜默失效**，先查是哪個服務沒滾到新版 |
| 13 | 走 §3.3 SOP 完成 destructive apply：plan 會出現 `destroy` → CI destructive gate 擋下 → 人工審 plan 後於低峰時段 `gh workflow run mq-topology.yml --ref <dev\|main> -f env=<dev\|prod> -f allow_destructive=true`（**`--ref` 必須與 `env` 對齊**，見 §3.3.3 步驟 6）。**禁止在 RabbitMQ console / SSH 手動刪除 dev 或 prod 的實體 queue**（Terraform 不知道的 drift；local 環境不受此限） |

---

## 五、查詢 MQ / Event 流程

### 5.1 「我需要消費 X 事件，怎麼找？」

1. 打開 `docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md`
2. 在 §三「完整事件清單」找對應領域
3. 確認 Routing Key / Exchange / Payload Class
4. 在 §五「Consumer 完整清單」確認是否已有其他 Consumer 監聽（避免衝突）
5. 在對應服務的 `RabbitMqConfig` 新增 `@Bean Queue` + `@Bean Binding`
6. **同一個 PR 把該 queue 登記進 `infra/mq/topology/topology.yaml`**（否則 broker 上不會有這條 queue，該訂閱失效，見 §2.1 步驟 8 與 §7.2）

### 5.2 「我要新增一個事件，但不知道應該用哪個 exchange？」

1. 打開 36 號設計書 §二「Exchange 清單」
2. 找對應業務領域的 exchange
3. 若無對應領域 → 新增 exchange（罕見，需強烈理由）

### 5.3 「我看到一個 routing key，想知道是誰發、誰收？」

1. 36 號設計書 §三 用瀏覽器 Ctrl+F 搜 routing key
2. 對應行即顯示 Producer + Consumer + Payload + DLQ + Idempotency

### 5.4 「我發現程式碼與 36 號設計書不一致？」

| 情境 | 處置 |
|------|------|
| 文件描述某事件存在，程式碼找不到 | 標記「設計書漂移」於 36 號設計書 §9.5，不擅自補完 |
| 程式碼有事件，文件未記錄 | 更新 36 號設計書補上條目，並回頭通知該事件原開發者 review |
| 文件描述的 routing key 與程式碼字面值不一致 | 程式碼為準，更新文件並 commit |

---

## 六、Code Review 審查清單

### 6.1 新增 MQ / Event PR 必查項

- [ ] 36 號設計書已同步更新（§三 加入事件條目）
- [ ] CodeMeta `內部技術碼.md` 已加入 EventType enum 值（若 routing key 新增）
- [ ] `MqConstants.java` 已加入對應常數（無字面值硬編碼）
- [ ] Publisher 使用 `obsRabbitPublisher.publish`
- [ ] Consumer `@RabbitListener` 指定 `manualAckContainerFactory`
- [ ] `RabbitMqConfig` 含對應 `@Bean Queue` + `@Bean Binding`（意圖宣告，見 §1.3）
- [ ] **`infra/mq/topology/topology.yaml` 已加對應條目**，且本機 `validate.py` + `unittest` + `mq-consumer-registration-check.py` 三道全綠（漏登記 = 訂閱失效，Go 端只有啟動時一行 log，實務上等於沒人會發現）
- [ ] **DLQ / retry 以 `retry:` / `dlq:` policy 宣告**，未手寫 policy 會展開的衍生資源（`{queue}.dlq`、`{queue}.retry`、鏡射 binding——手寫會重複產生）。**三類例外必須手寫**：① `{exchange}.dlx` exchange 本身（展開不產生它）② 共用 DLQ 目的地（`aftersales.events.dlq`、`payment.rescue.refund.dlq`，以裸 queue 條目自行存在）③ 自建的延遲重試 queue（6 條 product-import `*.retry`，帶 `x-message-ttl` 走 `dlq:`，非 `retry:` policy 產物）
- [ ] **已讀過 PR 上 dev + prod 兩份 `terraform plan`**，確認沒有非預期的 `must be replaced` / `destroy`（⚠️ PR 階段含 destroy **不會**讓 CI 變紅，綠燈不等於安全）
- [ ] **Java consumer 另需人工對照**——`static-gates.yml` 的「MQ consumer ↔ topology 登記一致性」step 只掃 Go（`**/*.go`），Java 端漏登記無自動攔截
- [ ] 關鍵路徑有 idempotency 保護（dedup 表或 DB 樂觀鎖）
- [ ] 關鍵路徑有 DLQ 配置（`topology.yaml` 的 `dlq:` / `retry:` policy；Java 側另有 `@Bean DLQ` + DLX argument 作為意圖宣告）
- [ ] Payload 為 typed class（避免 Map 動態 payload）
- [ ] 對應服務的詳細設計書 §事件章節已更新
- [ ] 跨團隊事件已與對方團隊對齊
- [ ] 本地測試含重複訊息驗證

### 6.2 修改 MQ / Event PR 必查項

- [ ] 變更類型符合 §3.1 允許範圍
- [ ] 若為破壞性變更，已走 §3.2 過渡流程
- [ ] 若動到 queue arguments 或刪除 queue，已走 §3.3 SOP（PR 描述含 §3.3.2 checklist、兩份 plan 判讀結論、destructive apply 窗口）
- [ ] 36 號設計書已同步更新
- [ ] 對應服務設計書已同步更新
- [ ] payload 向後相容（新增欄位於 class 結尾，consumer 不會 fail）

---

## 七、常見陷阱

### 7.1 RabbitMQ 訊息搶奪（多環境共用同一 broker 時）

**症狀**：RabbitMQ 對綁在同一個 queue 上的多個 consumer 採 round-robin 分派，因此多環境共用同一 broker 時訊息會被不同環境的 consumer 互相搶走，導致部分訊息被舊版程式碼處理、資料不一致。

**防護**：每個環境必須使用獨立的 RabbitMQ instance 或 vhost，嚴禁跨環境共用同一 broker。現況為 Local（OrbStack）／ dev ／ prod 三套各自獨立，`mq-topology.yml` 也因此對 dev + prod 各跑一次。

### 7.2 Queue 未登記於 topology.yaml（訂閱靜默失效）

**症狀**（**以下為 Go 端實測；Java 端待坐實**）：**服務正常啟動、pod 不重啟、監控無異常**——passive declare 查不到 queue 只是跳過該筆訂閱。publisher 發出的訊息在 exchange 上找不到符合的 binding，**直接丟棄**：不進 DLQ、不留痕跡。唯一的外顯是「怎麼都沒收到」。

**Go 端不是完全無聲**：`pkg/mq.partitionConsumable` 會在**啟動當下**印一行
`[MQ][ERROR] skipping queue <name> (NOT consumed): ...`（內文直接指向 topology.yaml 與 SOP），之後就再無任何跡象——log 一輪就被沖掉，所以實務上仍是「錯過就查不到」。
兩個特例：該服務**全部** subscription 都查無 queue 時會回 `no consumable subscriptions` 並進 reconnect（這種反而看得見）；**Java 端漏登記的實際行為未經坐實**（Spring `missingQueuesFatal` 預設值是否讓它啟動失敗，尚待驗證），不要假設它也是無聲的。

**排查**：

| 步驟 | 動作 |
|---|---|
| 1 | **先 grep 服務啟動 log 的 `skipping queue`**（Go 服務；`kubectl logs deployment/<svc> \| grep "skipping queue"`，pod 重啟過就看不到，必要時 `rollout restart` 再看）——命中即直接指出是哪條 queue |
| 2 | 把 consumer 認得的 queue 名（Java `@RabbitListener(queues=...)` / `MqConstants`；Go `Subscription{Queue:...}` / `pkg/mq` 常數）與 `infra/mq/topology/topology.yaml` 逐條對照 |
| 3 | Go 側直接跑 `python3 scripts/mq-consumer-registration-check.py`（雙向比對，秒級零連線；加 `--list` 只列現況） |
| 4 | 查 broker 上該 queue 是否存在、`consumers` 是否 ≥ 1：`rabbitmqctl list_queues name messages consumers`（唯讀，management UI 或 SSH 皆可） |
| 5 | 確認 exchange → queue 的 binding 存在——exchange 有、queue 有、binding 沒有，訊息一樣被丟棄且無任何 log |
| 6 | 補完 topology 並 apply 後，**記得 `kubectl rollout restart deployment/<service>`**——被 skip 的訂閱不會自癒（§3.3.3 步驟 7） |

**防護**：§2.1 步驟 8（同 PR 登記 topology.yaml）＋ `static-gates.yml` 的「MQ consumer ↔ topology 登記一致性」step（blocking，**只掃 Go**）＋ notification-service `TestNotificationQueuesExistInTerraformTopology`（**單元測試**，直接讀 `topology.yaml` 核對，不同步就測試 fail）。**Java consumer 無自動攔截**，靠 §6.1 人工對照。

**歷史教訓**：
- `order.shipping.label` 缺少 `@Bean` 定義：auto-declare 時代症狀是「新環境啟動失敗」，直接把問題砸在臉上；passive declare 下同樣的漏登記**只剩啟動時一行 log**（Go 端）——症狀從「服務起不來」降級成「幾乎看不見」，這正是它現在更危險的原因。
- issue #1795：`crm.aftersales.refund.completed` 漏登記 topology → 退款事件在 dev 完全沒進 crm-service，**沒有任何人看到那行啟動 log**，靠「怎麼沒進 CRM」反推才發現。

### 7.3 Jackson2JsonMessageConverter 全域 Bean

**症狀**：訊息 byte[] 雙重序列化，consumer 反序列化失敗。

**防護**：`docs/開發規範/Java 微服務開發規範.md` §8.6 禁止全域宣告 `Jackson2JsonMessageConverter` Bean。

**曾發生過的違規形狀**：`services/java/payuni-webhook-service/.../config/RabbitMqConfig.java` 曾宣告過此 Bean（現已移除，該檔目前只剩兩個 `TopicExchange`；紀錄見 36 號設計書 §9.5）。

### 7.4 Routing Key 硬編碼

**症狀**：修改 routing key 時遺漏某處，事件路由失敗。

**防護**：必須走常數來源——`MqConstants`，或 CodeMeta 生成的 EventType enum（`XxxEventType.YYY.value()`，見 §2.1 步驟 3 / 5），禁止字面值。

**曾發生過的違規形狀**：`coupon-service/CouponEventPublisher.java` 曾硬編碼 `"coupon.issued"`（現已改用 `CouponEventType.COUPON_ISSUED.value()`；紀錄見 36 號設計書 §9.5）。

### 7.5 跨語言 Exchange 命名不一致

**症狀**：Java publish 到 `payment.events`，Go consume 從 `hawkr.payment`，訊息漏路由。

**防護**：Java `MqConstants` 與 Go `pkg/mq/exchanges.go` 對齊；新增 exchange 時兩端同步。

**現有風險**：Payment 領域（見 36 號設計書 §9.3）。

### 7.6 同 Routing Key 多 Consumer 沒分 Queue

**症狀**：同一事件被多個 consumer 競爭消費，預期由 A 處理的訊息被 B 拿走，資料遺失。

**防護**：同一 routing key 給多個 consumer 必須**各自獨立 queue**，分別 binding。RabbitMQ Topic exchange 對同一 queue 內訊息採 round-robin。

**範例**：`order.paid` 同時給 shipping-service 與 notification-service：
- shipping queue：`shipping.order.paid`
- notification queue：`notification.order.paid`

### 7.7 Inbox / Outbox 表保留期未設

**症狀**：表無限增長導致 DB 撐爆。

**防護**：每個 dedup / inbox / outbox 表必須有清理排程（`@Scheduled`）。

**待確認**：`payment_event_dedup` / `shipping_webhook_dedup` 等表的保留期（36 號設計書 §12 需確認 #8/#9）。

### 7.8 Outbox 在 @Transactional 內呼叫 gRPC

**症狀**：長事務 + 跨服務呼叫，DB 連線池耗盡。

**防護**：Outbox 排程器在 @Transactional 外執行；publish 失敗僅標記 retry，不 rollback DB。

---

## 八、特殊場景

### 8.1 PayUni Webhook 入口模式

PayUni 是外部 webhook 來源，特殊處理：

| 階段 | 說明 |
|------|------|
| 接收 | `payuni-webhook-service` HTTP 入口接收 PayUni POST |
| T1 | INSERT `webhook_inbox`（REQUIRES_NEW）→ UNIQUE 約束去重 |
| T2 | INSERT `outbox_events`（REQUIRES_NEW）→ 排程器非同步發送 |
| 解密 | 由 webhook-service 解密 PayUni payload |
| 分發 | OutboxPublisher 發送至 `payuni.events` exchange，由各業務服務消費 |

**新增 PayUni event type**：
1. 更新 `PayUniEventType` enum（CodeMeta `內部技術碼.md`）
2. webhook-service `PayUniEventClassifier` 加入分類邏輯
3. 對應業務服務新增 Consumer

### 8.2 Outbox 模式（雙保證）

當業務寫入 DB 後**必須保證**事件投遞時使用：

| 服務 | 表 | 排程器 |
|------|-----|--------|
| payment-service | `outbox_events` | `OutboxEventPublisherScheduler` 每 5s |
| payuni-webhook-service | `outbox_events` | `OutboxPublisherScheduler` 每 5s |

**新增 Outbox 寫入**：
1. 業務寫入主表時，**同事務**寫入 outbox_events（status=PENDING）
2. 排程器掃描 PENDING / 到期 FAILED 記錄，呼叫 publisher（走 `ObsRabbitPublisher.publishConfirmed`，等 broker confirm）
3. broker ack → status=SENT；nack / timeout / unroutable → 指數退避（5s→15s→45s→135s→405s）→ 5 次後標 DEAD

**Transactional Outbox with Immediate Dispatch（afterCommit 即時 + 排程器兜底，issue #1349）**：

對「延遲敏感」的關鍵事件（如付款結果 `payment.success`），純排程器模式最多有一個輪詢週期（5s）的延遲。此時在 outbox 寫入元件（如 `PaymentOutboxWriter.enqueueAndDispatch`）內於**同事務**做兩件事，兼得原子性與即時性：

1. `INSERT outbox_events(status=PENDING)`（payload JSON 於此凍結一次，afterCommit 送與排程器兜底送同一份 bytes）
2. `TransactionSynchronizationManager.registerSynchronization` 註冊 `afterCommit` callback，於事務 commit **成功後**在原執行緒立即呼叫 publisher（同一 `publishConfirmed`）

投遞語意：
- **正常路徑**：afterCommit ack → `markSENT`，下游**即時**收到、排程器不再撿（消除輪詢延遲）
- **兜底路徑**：afterCommit nack / timeout / 例外 → `markFailed` 或留 `PENDING`，交排程器每 5s 掃描重投（涵蓋 afterCommit 送失敗、或送成功但 `markSENT` 前崩潰的視窗）
- **消費端冪等吸收 at-least-once**：afterCommit 與兜底可能重送同一鍵，消費端須以業務鍵（如 `trade_no`）`ON CONFLICT DO NOTHING` dedup

**afterCommit callback 鐵則**：callback 內**必須 `try/catch` 吞掉所有例外、不得 rethrow**。afterCommit 在 commit 之後於原請求執行緒同步執行，若拋例外會上拋至 commit 呼叫方，造成「業務已成功（如付款 PAID）卻對呼叫方回錯」。confirm 失敗只 `markFailed` 交排程器兜底，主流程仍正常返回。

**投遞確認（publisher-confirm，issue #1349）**：
- Outbox 排程器投遞**必須**用 `ObsRabbitPublisher.publishConfirmed(exchange, routingKey, payloadBytes, event, timeoutMs)`——以 `CorrelationData` + `future.get(timeout)` 等待 broker confirm；`!isAck` / timeout / unroutable（`getReturned != null`）回 `false` → 排程器 markFailed 重投。**禁止**在 outbox 投遞路徑用不確認的 `publish(...)`（broker 未收下也會誤標 SENT → 事件遺失，等同無 outbox）。
- 全域啟用 `spring.rabbitmq.publisher-confirm-type=correlated` + `publisher-returns=true` + `template.mandatory=true`（置 `observability-defaults.properties`）。correlated 為**非破壞性**：既有 `publish(...)`（fire-and-forget）caller 不 block、語意不變，僅新增 `publishConfirmed` 供 outbox 路徑使用。

**規則**：
- 業務主流程禁止直接 publish event（必須走 outbox）
- 排程器禁止在事務內執行
- Outbox 投遞必須 publisher-confirm 確認 broker ack 才標 SENT

### 8.3 best-effort 模式

非關鍵路徑事件（例：order events 通知下游）可用 best-effort：

| 規則 | 說明 |
|------|------|
| 不在 @Transactional 內 publish | 避免事務拉長 |
| Publish 失敗僅 log，不重試 | 由下游 consumer 端的 dedup 保證 |
| 不需要 outbox 表 | 接受偶發遺失 |

**判定關鍵**：「這個事件遺失會不會造成資料不一致或金流錯誤？」是 → 必須 outbox；否 → best-effort 可。

---

## 九、與其他規範的關係

| 相關規範 | 關係 |
|---------|------|
| `docs/代碼管理（CodeMeta）/README.md` | CodeMeta SSOT 流程（含 EventType enum 生成） |
| `docs/開發規範/開發工作流程.md` | 開發工作流程（設計書 → Code Gen → DB → 程式碼 → 測試） |
| `docs/開發規範/Java 微服務開發規範.md` | Java 微服務通用規範；MQ 相關條款在 §8（Observability / `ObsRabbitPublisher` / 禁 `Jackson2JsonMessageConverter` 全域 Bean）與 §10.2（RabbitMQ Manual ACK） |
| `docs/開發規範/PostgreSQL 設計規範與最佳實踐.md` | Inbox/Outbox/Dedup 表設計 |
| `docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md` | 內容 SSOT |
| `docs/基礎建設/MQ Topology 管理（Terraform）.md` | **topology 佈建權威**：passive declare、policy 展開規則、CI/CD 與 destructive gate |
| `infra/mq/README.md` | 操作速查：`topology.yaml` 五種 queue 形狀、本機驗證三道閘門、plan 判讀 |

---

## 十、違規回報

若發現程式碼或文件違反本規範：

1. 確認違規（與本規範條款對比）
2. 在 36 號設計書 §9 對應子節（孤兒 / 不一致 / 違規）新增條目
3. 若為破壞性違規（高優先級），建立修正 PR 並於 §13.1 後續行動清單追蹤
4. 若為設計書漂移（低優先級），標註「需確認」並通知原開發者 review

**禁止**：靜默修復違規而不更新 36 號設計書（造成 SSOT 與程式碼再次漂移）。
