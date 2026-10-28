# MQ Topology（Terraform）

RabbitMQ topology（exchange/queue/binding）唯一入口。
完整說明：`docs/基礎建設/MQ Topology 管理（Terraform）.md`。
ADR：wiki backend-payuni decisions/2026-07-24-mq-topology-terraform-centralization。

## 範圍

本目錄現在只有 topology 的來源與驗證工具：`topology/topology.yaml`（唯一登記處）、
`topology/*.tf`（env 無關的 Terraform module）、`tools/`（schema 驗證、展開、測試）。
公司時期的 dev / prod 環境根設定（`envs/`，遠端 state 與 broker endpoint）、對應的部署 workflow
與一次性匯入工具已刪除；要部署 RabbitMQ 時，等選定部署目標再加環境根設定，module 直接重用。

```
改 topology/topology.yaml（+ 服務 consumer 程式碼，同一個 PR）
   ├─ 本機 ① validate.py + unittest ················· 秒級，任何改動都跑
   └─ 本機 ② consumer 登記一致性 ···················· 秒級，改 consumer 或 queue 時跑
```

## 改動類型決定你要做到哪一步

| 改動 | plan 會出現 | 風險 | 本機至少要跑 |
|---|---|---|---|
| 新增 queue / exchange / binding | `create` | 低 | ① |
| **新增／移除 consumer**（改 `Subscription{}`） | 視是否同時動 topology | **中**——漏登記會靜默失效（見下） | ① + ② |
| 改既有 queue 的 `arguments` | **`must be replaced`** | **高**——queue 重建，裡面未消費的訊息全部消失 | ① + ③ |
| 刪除 queue | `destroy` | **高**——同上 | ① + ② + ③ |
| 只改通知 template | 不動 topology | — | 寫 Atlas migration 就好，見下方 notification-service |

🔴 **刪 queue 有順序**：必須先讓 code 部署到「不再訂閱」的版本、確認該 queue `consumers = 0`，
**才**能把 queue 從 topology 移除並套用。反過來做＝把 queue 從還連著的 consumer 底下抽掉——服務不會
crash，而是重連後 passive declare 查不到、**靜默跳過該筆訂閱**，和「漏登記 topology」是同一個失效模式。
逐步 SOP 見《MQ 與事件管理規範》§四。

## topology.yaml 怎麼讀

### 先懂三個 RabbitMQ 概念

```
publisher                exchange              binding              queue          consumer
    │                   （無狀態，              （路由規則）      （有狀態，訊息    （訂閱 queue）
    │                    不存訊息）                                 存在這裡）
    │
    └── publish(exchange, routing_key, payload)
                            │
                            ├── 比對每一條 binding 的 key ──→ 符合就投一份進該 queue
                            │
                            └── 沒有任何一條符合 ──→ 訊息「直接消失」⚠️
```

- **exchange**：訊息的入口，**無狀態**——它不儲存任何東西，收到訊息就當場決定要複製到哪些 queue。
  publisher **從不直接發給 queue**。
- **queue**：訊息真正待著的地方，**有狀態**——訊息存在裡面直到被 consumer ack，或過期、或被 dead-letter。
- **binding**：把 exchange 接到某條 queue 的規則，上面寫一個 routing key。

#### 最容易搞混的：routing key 有「兩個」

```
publisher 發訊息時帶一個  routing key   ← 「這是什麼事件」    例：order.completed
binding 上寫一個          routing key   ← 「我想收什麼事件」  例：order.completed 或 #
                              ↓
                     兩者比對，符合才投遞
```

publisher **不知道有誰在聽**，也不指定 queue——它只說「我發生了 `order.completed` 這件事」，
剩下的由 binding 決定誰收得到。這是解耦的關鍵：新增一個 consumer 不需要動 publisher。

#### 一則訊息可以進很多條 queue

真實例子——`hawkr.account` 上發一則 `account.deleted`：

```
publish("hawkr.account", "account.deleted", {...})
        │
        ├──→ im.account.deleted             （im-service：聊天記錄 PII 抹除）
        └──→ notification.account.deleted   （notification：清通知匣 PII）
```

**2 條 queue 各收到一份自己的複本**，各自獨立 ack、獨立重試、互不影響。這就是「同一個 exchange
綁多條 queue」的意義。

#### topic exchange 的萬用字元

這個 repo 幾乎都是 `type: topic`，routing key 以 `.` 分段比對：

| binding 的 key | 收得到 | 收不到 |
|---|---|---|
| `order.completed` | 只有 `order.completed` | 其他全部 |
| `order.*` | `order.completed`、`order.paid`（**恰好一段**） | `order.item.shipped`（兩段） |
| `#` | **該 exchange 的全部訊息** | — |

notification-service 的 13 條 queue 全部用 `#`，理由見下方 notification-service 段。

#### 兩種「訊息不見了」，症狀完全不同

| 情況 | 會發生什麼 | 好不好查 |
|---|---|---|
| **exchange 不存在** | publish 當場 channel error（`404 NOT_FOUND`），連線被關 | **會報錯**，看得到 |
| **exchange 存在，但沒有任何 binding 符合** | 訊息**靜默丟棄**——不報錯、不進 DLQ、沒有任何痕跡 | ⚠️ **完全查不到**，只能靠「怎麼沒收到通知」反推 |

第二種是這套架構最陰的失效模式。`hawkr.payment` 目前就是這個狀態：exchange 存在、有
template 指向它，但**零 binding**，所以那些訊息發出去就沒了。

#### DLX 什麼時候才介入

**只在訊息「已經進了某條 queue」之後**——處理失敗被 nack、或 TTL 過期、或 queue 滿了。
上面那兩種「根本沒進 queue」的情況，DLX 幫不上忙，因為訊息從來沒到過 queue。

`.dlx` 結尾的 exchange 就是這個用途（dead letter exchange）。設定方式見下方 queue 的五種形狀。

### 檔案只有兩個頂層區塊

```yaml
exchanges:   # 有哪些入口
queues:      # 有哪些收件匣，以及各自接在哪個入口上
```

binding 沒有自己的區塊，它寫在 queue 的 `bindings:` 底下（因為 binding 天然屬於某條 queue）。

### 什麼是「policy 展開」

這份 YAML **不是 broker 資源的一對一清單**，是一份**意圖**宣告。你寫「這條 queue 要有重試」，
展開規則負責把它翻譯成 broker 真正需要的那幾個資源。

```
你在 topology.yaml 寫的             展開規則              broker 上實際存在的
─────────────────────              ────────              ──────────────────
36 exchanges                          →                    122 exchanges
114 queues                            →                    187 queues + 6 json-args
119 bindings                           →                    241 bindings
─────────────────────                                     ──────────────────
269 條宣告                                                  556 個資源
```

**多出來的 287 個就是展開產生的。** 你不會、也不該在 YAML 裡看到它們。

「policy」指的是 queue 條目上那幾個會觸發展開的欄位——`retry:`、`dlq:`、`args_json:`。
目前的分佈是：43 條用 `retry:`、58 條用 `dlq:`、6 條 `args_json:`、13 條沒有 policy（純裸 queue）。

**為什麼要這樣做**：`.retry` exchange、`.dlq` queue、dlx→dlq binding 與 dead-letter 參數這些東西是機械衍生的
——每次都長一樣、每次都要寫對三四個名字。手寫 184 條不只是麻煩，是**漏一條不會有人發現**
（plan 照樣乾淨、apply 照樣成功，只是死信沒地方去）。交給規則產生就不會漏。

**代價是你不能手寫那些衍生資源**：自己加一條 `im.account.deleted.dlq` 到 YAML 裡，展開規則會再產生
一次，terraform 就會看到重複的資源。所以規則是——**凡是 `.retry` / `.dlq` 結尾的東西，一律不要自己寫**。
（唯一的例外是**共用** DLQ，例如 `aftersales.events.dlq`——它不由任何 policy 產生，必須自己列一條裸
queue，見下方形狀 ①。）

**展開規則在兩個地方各實作一份**：`tools/expand.py`（給 CI 的驗證與單元測試用）與
`topology/main.tf` 的 `locals`（Terraform 真正拿去佈建的那份）。這是刻意的重複——讓 Terraform 去吃
Python 的產物會多一個「忘記重跑 generator」的失效面。兩份的一致性由容器 e2e（plan 必須 no-op）與
`test_expand.py` 的資源數 pin 守著。**兩邊不一致時以
`docs/基礎建設/MQ Topology 管理（Terraform）.md` 的「展開規則」段為準**，那是權威出處。

### `exchanges:` 條目

```yaml
- name: aftersales.events
  type: topic
```

`type` 決定路由方式（比對規則見上方「topic exchange 的萬用字元」），這裡幾乎都是 `topic`。
唯一的例外是 `payment.webhook.dlx`，它是 `direct`。

**命名慣例**：`.dlx` 結尾 = 死信交換機、`.retry` 結尾 = Go 重試框架展開出來的 fanout exchange。
兩者都不要手寫，由 policy 展開產生。

### `queues:` 條目的五種形狀

**① 裸 queue** —— 只產出自己 + 自己的 bindings，不衍生任何東西：

```yaml
- name: aftersales.events.dlq
  bindings:
  - exchange: aftersales.events.dlx     # 接在死信交換機上
    key: dlq                            # 用 "dlq" 這個 routing key 收
```

這條就是**共用死信信箱**：aftersales 相關的 queue 處理失敗時，訊息被 broker 丟到
`aftersales.events.dlx`，再由這條 binding 送進 `aftersales.events.dlq` 存起來等人工處理。
⚠️ 共用 DLQ **不會被任何 policy 自動生成，必須自己列一條**。

**② `retry:` —— Go 服務的重試框架**（`pkg/mq` 用）：

```yaml
- name: im.account.deleted
  retry:
    max_attempts: 3
  bindings:
  - exchange: hawkr.account
    key: account.deleted
```

一條宣告展開成 **7 個資源**：queue 本身、`im.account.deleted.dlq`、兩個 **fanout**
exchange（`im.account.deleted.retry` 讓重試不會扇出到共用同一 routing key 的其他 queue，
`im.account.deleted.dlx` 讓死信不會流進別人的 DLQ）、以及 3 條 binding
（原本那條 + `.retry`→自己 + `.dlx`→`.dlq`）。**寫一條、動七個**——這就是本機先看 plan 的價值。

⚠️ **dead-letter 參數不要手寫**，`x-dead-letter-exchange`（＝`{queue}.dlx`）由展開注入，
`validate.py` 會擋手寫。**沒有** `x-dead-letter-routing-key`——`{queue}.dlx` 是只綁
`{queue}.dlq` 的 fanout，fanout 不看 routing key，隔離是構造上的而不是靠標籤設對。

⚠️ DLX 的名字由 **queue** 推導，不是由 binding 的 exchange。所以綁幾個 exchange、
幾條 key 都與死信路由無關，也不需要事先宣告任何 `.dlx`——它跟 `.retry` 一樣自動生成。

**③ `dlq:` 用共用信箱** —— broker 原生 dead-letter，Java 服務為主：

```yaml
- name: aftersales.payment.refund.result
  dlq:
    via: aftersales.events.dlx          # 失敗時丟到這個 DLX
    queue: aftersales.events.dlq        # 由這條既有的共用 DLQ 承接
    routing_key: dlq
  bindings:
  - exchange: payment.events
    key: refund.failed
  - exchange: payment.events            # 一條 queue 可以綁多條 binding
    key: refund.succeeded
```

指定了 `dlq.queue` 就**不會**產出任何新 queue 或 binding——那條共用 DLQ 必須以形狀 ① 自己存在。

**④ `dlq:` 但不指定 `queue`** —— 自動產出專屬的 `{Q}.dlq` 與 binding `X.dlx` → `{Q}.dlq`
（routing key 是 queue 全名）。此時 `routing_key` 必須就是 `{Q}.dlq`，否則死信帶著原
routing key 進 `X.dlx`、沒有 binding 命中就靜默丟棄（`validate.py` 會擋）。

⚠️ **③④ 這條路徑維持共用 DLX，不改成 per-queue fanout。** Java 服務用 Spring 的
`RabbitAdmin` 主動宣告 queue，並在 `QueueBuilder` 裡硬寫 dead-letter args；Terraform
若改成別的值，服務啟動時 declare 會撞 `406 PRECONDITION_FAILED` 起不來。要改這半邊
必須與 Java 端同批。② 的 Go 那半沒有這個約束（dev／prod 走 `MQ_PASSIVE_DECLARE=true`，
passive declare 只驗存在、不驗 args）。

**⑤ `args_json: true`** —— `arguments` 含非字串值時必須加：

```yaml
- name: product.import.asset.download.retry
  dlq:
    via: hawkr.product.import
    queue: product.import.asset.download
    routing_key: asset.download
  args_json: true                        # 因為下面有 int
  arguments:
    x-message-ttl: 120000                # 停留 120 秒後死信回主 queue = 延遲重試
```

provider 的 `arguments` 是 `map[string]string`，數字型參數只能走 `arguments_json` 那條。

### 寫的時候要記得

- **`retry:` 與 `dlq:` 互斥**，同時寫會被 `validate.py` 擋下。
- `.retry` / `.dlq` 與其 binding **不要手寫**（見上方「什麼是 policy 展開」）。
- routing key 是 `#` 或 `*` 時必須加引號（`key: "#"`）——不加引號 YAML 會把它解析成 `null`，
  `validate.py` 會擋下。
- `via: ''`（空字串）是合法值，代表 RabbitMQ 的 **default（無名）exchange**——靠 routing key 直接
  比對 queue 名稱路由，不需要具名 DLX 也不需要 binding。5 條 product-import queue 是這個形狀，
  **不是漏填**。

⚠️ **新增 consumer 的 PR 必須同時登記 topology.yaml**。passive declare 下 broker 沒有的
queue 會 404、該筆 subscription 靜默失效（不進 DLQ，因為 queue 根本不存在）——issue #1795
就是這樣漏掉 `crm.aftersales.refund.completed`。**這件事現在由本機驗證 ② 與 CI 擋。**

## 本機驗證

下面的指令都從 **repo root** 出發（`<repo>` = `backend-payuni` 的 checkout 路徑），不必先 `cd` 到別處。

### ① schema（秒級，零連線）

```bash
cd <repo>/infra/mq/tools
python3 validate.py --topology ../topology/topology.yaml
python3 -m unittest discover -p 'test_*.py'
```

預期輸出：`topology.yaml OK`，然後 `Ran <N> tests ... OK (skipped=2)`
（兩個 skip 是 `TestRoundTripAgainstRealTopology`，`topology.json` 在 P1 被 `git rm` 後永久 skip）。

**這兩個指令是不同的東西**，都要跑：

#### `validate.py` — 驗你剛改的 `topology.yaml`

它擋的是 **`terraform plan` 看不出來、要到 `apply` 才炸或根本不炸的那類錯誤**。plan 只會告訴你
「要建這些資源」，不會告訴你「這個 binding 指向的 exchange 根本不存在」。規則有三類：

| 類別 | 規則 | 不擋會怎樣 |
|---|---|---|
| **形狀** | 未知欄位、`name` 必填、名稱重複、`type` 必須是合法值、`retry.max_attempts` 必須是正整數 | 拼錯的欄位（`retires:`、`routing_ky:`）**被靜默忽略**——你以為設了 retry，實際上沒有 |
| **互斥** | `retry:` 與 `dlq:` 不能同時出現 | 兩套 dead-letter 機制打架 |
| **參照完整性** | binding 的 `exchange` 必須已宣告；`retry:` queue 的每條 binding，其 `{exchange}.dlx` 必須已宣告；`dlq.via` 與 `dlq.queue` 必須指向已宣告的東西 | **`plan` 照樣乾淨產出**，`apply` 建 binding 時才炸 `404 (not_found): no exchange '...'`，並留下 **partial apply**（一半資源已建、一半沒建） |

第三類是它存在的主要理由——那是唯一「plan 綠燈但 apply 會壞」的情境。

#### `unittest` — 驗**工具本身**沒壞，外加一道資源數防線

36 個測試分三個檔：`test_validate.py`（25 個，schema 規則）、`test_expand.py`（11 個，展開規則）、
`test_definitions_to_topology.py`（2 個，DR 還原）。多數是拿合成 fixture 驗工具邏輯，跟你改的
`topology.yaml` 無關——**但有一個例外**：

`test_expand.py` 的 `test_expanded_resource_counts_are_pinned` 拿**真實的** `topology.yaml` 跑展開，
把結果釘死在 **122 exchanges / 187 queues / 6 json-args / 241 bindings / 556 total**。

這是「**過度產生**」的唯一防線。少產資源你會發現（服務連不上），多產不會——展開規則寫錯多生出
一批 queue，plan 會照建、apply 會成功、broker 上多一堆沒人用的東西，沒有任何人會察覺。所以那組
數字是刻意 pin 死的：**改動如果讓它變了，你必須是有意識地改**，順手更新數字前先確認多／少的那幾筆
是你要的。

### ② consumer 登記一致性（秒級，零連線）

```bash
cd <repo>
python3 scripts/mq-consumer-registration-check.py           # 加 --list 只列現況不判定
```

雙向比對 Go 端與 `topology.yaml`：

| 方向 | 判準 | 擋掉什麼 |
|---|---|---|
| ① code → topology | `pkg/mq` 的 `Queue*` 常數值、以及 `Subscription{Queue: "..."}` 的字面值，都要存在於 `topology.yaml` | **漏登記**——passive declare 404 → subscription 靜默失效（issue #1795 那類） |
| ② topology → code | 帶 `retry:` 的 queue（Go 重試框架專用形狀），queue 名要出現在某個非測試 `.go` 檔 | **宣告了卻沒 consumer**——訊息在 broker 堆積到過期 |

兩個實作細節，改動時要知道：

- **比對的是字串字面值，不解析 Go 語法。** queue 名在 code 裡有三種形狀（`pkg/mq` 常數、
  `Queue:` 欄位字面值、`notificationQueueByExchange` 那種 map value），比字面值三種都涵蓋得到，
  也不需要能編譯或跨 module 求值——那 30 條 subscription 分散在 6 個獨立的 go module 裡。
- **`_test.go` 排除在外。** `pkg/mq/resilient_republish_test.go` 用的 `notification.buyer.delivered`
  是合成 queue 名，刻意不在 topology 裡，掃進來會誤報。

之所以做成 script 而不是「每個 service 各寫一支 Go 測試」：新增第 7 個 service 時沒人會記得加測試，
**那正是「漏登記」問題本身的形狀**。script 的名單兩邊都從資料來（`topology.yaml` + `git ls-files`），
新 service 自動納入。

### ③ sandbox 容器（完全不碰共用環境）

驗證 apply 行為本身時用（例如改了展開規則，想確認資源數符合預期）：

```bash
docker run -d --name mq-tf -p 15673:15672 rabbitmq:3.13-management-alpine
# 另建一份指向 localhost:15673 的 provider 設定（含 module "topology" 的最小根設定，
# source 指向 topology/），用 local state；先用 HEAD 版本 apply 建出全部資源，
# 再換上改過的 topology.yaml 跑 `terraform plan`，看 `must be replaced` 與 `destroy`
docker rm -f mq-tf
```

`must be replaced` 與 `destroy` 要特別看：**queue 被重建等於裡面未消費的訊息全部消失**。

## notification-service

這個服務的 queue 設計跟其他服務不一樣，值得單獨看——**它是唯一一個「資料表內容會影響該收哪些訊息」
的服務**，而設計的重點就是切斷那個連動。

### 設計：每個來源 exchange 一條 queue，用 `#` 收全部

13 條 queue，對照表在 `worker/consumer.go` 的 `notificationQueueByExchange`：

```go
var notificationQueueByExchange = map[string]string{
    "aftersales.events": "notification.aftersales",
    "hawkr.buyer":  "notification.buyer",
    // … 13 條
}
```

每條都以 `#` 綁定，也就是**收下該 exchange 的所有訊息**，不管 routing key 是什麼。

### 訊息進來之後

```
訊息到達 notification.<來源> queue
   │
   ├─ 依 (exchange, routing_key) 查 template cache
   │
   ├─ 查到 → 處理（可能回傳多筆：買家版 + 賣家版，handler 迴圈跑）
   └─ 沒查到 → 直接 ack，不解析 payload（成本極低）
```

「沒查到就 ack」是 `#` 綁定成立的前提——收到不關自己的事件是常態，不能當錯誤。

### 新增一則通知會發生什麼

**寫一筆 Atlas migration 加進 `notification_templates`，這樣就好。**

不用動 `topology.yaml`、不用改 map、不用部署 infra。因為 `#` 綁定的關係，新 template 用什麼 routing key
都收得到——**新增 template = 零基建變更**，這正是這個設計要買的東西。

唯一的例外：**引進一個全新的來源 exchange**（現有 13 個以外的）。那時要同時加 map 條目與
`topology.yaml` 的 queue 條目（帶 `retry:` policy 與 `#` binding）。

### 為什麼不是「每個通知型別一條 queue」

因為試過，代價很大。那個粒度下 77 筆 template 開出 64 條主 queue（光 `aftersales.events` 一個 exchange
就 37 條），連同 dlq/retry 佔 132 queue + 94 exchange——**當時全 cluster 51% 的 queue、77% 的 exchange**。

而它買不到宣稱的故障隔離：所有 queue 都由同一個服務、同一條 connection、同一個 channel 消費，
head-of-line blocking 早就被 `pkg/mq` 的 ack + republish 重試機制在**訊息層級**解掉了。

真正的代價是 `queue_name` 那個無約束欄位讓「**新增一筆資料**」隱含「**必須佈建一條 queue**」——
**issue #1795 就是這樣發生的**。2026-07-25 改成現在的粒度，`queue_name` 欄位也已於 2026-07-26 完全移除。

### 兩道防線

| 防線 | 位置 | 抓什麼 |
|---|---|---|
| **建置期** | `worker/consumer_test.go` 的 `TestNotificationQueuesExistInTerraformTopology` | 直接讀 `topology.yaml`，逐條核對 map 裡每個 exchange/queue 的存在、`type == topic`、`retry.max_attempts`、`#` binding。不同步就編不過 |
| **開機期** | `consumer.go` 建 subscription 前的迴圈 | 掃過 template cache，**任何 template 的 exchange 沒登記在 map 裡就 `logger.Error`**。因為 passive declare 下 broker 什麼都不會說——subscription 根本不存在，這行 log 是該失效唯一會浮現的地方 |

第二道是刻意做成 loud 的。**一個 template 掛在沒有 queue 的 exchange 上，它永遠不會被投遞**，
而且沒有任何錯誤會自己冒出來。

## 例外（不歸 Terraform 管）

- `gateway.sse.<hostname>` per-pod queue（gateway active declare）
- local 環境（不設 passive flag，app 自建 topology）

## 緊急後門

CI 全掛時可 rabbitmqadmin 手動操作，但事後必須補回 `topology/topology.yaml`（broker
definitions → `tools/definitions_to_topology.py` → `tools/fold.py` 還原成宣告式 YAML）
+ 讓 nightly drift 歸零。
