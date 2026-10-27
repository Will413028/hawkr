# A7 決策簡報 v2：和業界基準比對、治本的版本（供 Will 拍板）

- 對象：`backend/docs/plans/2026-10-18-a7-social.md`（下稱「計畫」）的 A7-D6 到 A7-D13、「預設」段、步驟設計與範圍邊界。
- 快照：main `90661e594`（盤點快照 `172260d31`）。
- 標記：【量】＝打開檔案或跑了指令；【推】＝讀程式後推論；外部說法附 URL，沒查證的標「一般知識（未查證）」。
- 約束分類：REAL 只給第三方（Apple 1.2）、真的適用的法律、缺資料、我們不擁有的系統。我們的規則、ADR、依賴表、條款草稿、計畫分段都是 SELF-IMPOSED；符合業界基準的寫「符合基準，保留」。DSA 只當業界基準引用，不是義務（EU 法規，台灣、未上線的平台不受拘束）。
- 本版處理了 critique F1–F11，對照表在 §6。

---

## 0. 結論

**前提題：** A7 會產生一批過渡程式，之後在 A6、A8、A10 刪掉（§1.1）。已核可的 brief 把「im／review／crm 到 A6」「notification 到 A8」「代行編排到 A10」標成 REAL（`step10d-a7-social-brief.md:75-78,109-111,181-182,185`【量】）。三項都是 **SELF-IMPOSED**，找不到現在滿足不了的前提（§1.2）。

**v1 的錯在哪（F1）：** v1 只查了 RPC，沒查事件。A6 不管什麼時候做，都要寫往 Java 和 notification 的橋（review 的 relay、crm 的 Java consumer、coupon 的 legacy client）。這些是 **A6 自己的成本**，不是 S3 造成的，所以 v1 寫的「終點只剩 T1」是錯的（§1.3 終點已改寫）。

**S3 還值不值得：值得，理由比 v1 窄。** S3 省的是「A7 往舊 im／review 的橋」，約 2 個 session，外加兩個語意好處。S3 不會讓 A6 變大，A6 的工作量一樣，只是排在 A7 之後下一個。代價只有兩項：A7-5 到 A6 完成之間，本機聊天與評價的封鎖、檢舉不能用；以及 A6 要排在 A4、A5、A8 前面。v1 說「工期變兩倍」是誤導，已拿掉。

| 題 | 計畫 | 本簡報 v2 | 類別 | 估計 session 差 |
|---|---|---|---|---|
| S1 代行貼文 6 條路由併進 A7 | 留到 A10 | 併進（弱建議；只算 A7 是 +1.5，算到 A10 約 +0.5） | (a) 技術 | A7 +2、A7-3 −0.5、A10 −1 |
| S3 A7 不寫往舊 im／review 的橋，A6 接著做 | 寫橋 | 不寫；A6 是下一個階段 | (a) 技術＋(b) Q1 接受缺口 | A7 −1.5、A6 −0.5 |
| S4 A8 也提前 | 不提前，寫 relay | 同意計畫，寫 relay | (a) | 0 |
| D6 排程發布 | delegation 擁有 job | social 擁有 job；delegation 撤銷時在同一個 transaction 取消，並同步寫稽核（D）。A 可以當退路 | (a) | 0 |
| D7 內容下架措施 | 10 項 | 8 項同意；來源改多對多（和 10e 3b 同結構）；類別看 Q5 | (a)＋(b) Q5 | 0 |
| D8 平台佇列 | A | 改採 10e D′1 的 (c)：各模組自己的推導查詢，httpapi 依全域 keyset 合併；不做事件投影、不建模組（2026-10-18 修訂） | (a)，ADR D3 修訂交 Will | A7-1e −0.5 |
| D9 買家處分 | 拿掉 WARN | 處分紀錄帶型別、可選期限、可讀理由（技術）；階梯要不要、參數、理由頁、推播是 Q2、Q6 | (a)＋(b) | 階梯 +0.5～1 |
| D10 即時留言 | A：`LISTEN/NOTIFY` | 先回答 Q3；保留就同意 A | (b) Q3 | 拿掉 −0.5～1 |
| D11 計數 | 同 transaction；只計登入者瀏覽 | 計數同意；「本週主題」改由伺服器排序（技術）；匿名瀏覽計不計是 Q4 | (a)＋(b) Q4 | +0.5 |
| D12 website 封鎖名單頁 | 新增 | 同意新增；這是 UI 改動 | (b) Q5′ | +0.5 |
| D13 檢舉原因 | moderation 自己的 enum | 分流同意；要不要一份平台政策分類是 Q5 | (b) Q5 | +1（10e 檔） |

---

## 1. 前提題：A7 改的程式碼之後會不會被重寫、搬走或刪掉？

### 1.1 計畫產生的過渡程式（之後一定刪）

| # | 過渡物 | 規模 | 刪除點 | 證據 |
|---|---|---|---|---|
| T1 | `internal/legacy/socialrelay`：6 個 key | 前例 sellerrelay 非測試 439＋測試 577 行 | A8 | 計畫:83；行數【量】 |
| T2 | legacy `socialserver`：實作 8 支（代行 5、封鎖 3） | 前例 sellerserver 707＋1084 行 | 封鎖 A6、代行 A10 | 計畫:81,346-348【量】 |
| T3 | 「是否封鎖」legacy RPC＋im 2 個呼叫點、review 1 個 | `message.go:497,809`、`handler.go:43` | A6 | 【量】；`:809` 在對話列表迴圈內，變成每個對話一次網路呼叫【推】 |
| T4 | moderation 對 im／review 的 legacy port | im 沒有讀單則訊息的 RPC（`im.proto:12-51`），兩邊都沒有取消隱藏（`review.proto:43-46`、`im.proto:54-57`）。要在 A6 就刪的服務裡**新增** 2–3 支 RPC；計畫的停止條件會在這裡觸發（計畫:351） | A6 | 【量】 |
| T5 | gateway 留著的部分：`SocialClient` 8 支、`admin_delegated_post.go` 708 行、`/app/blocks*` 3 條 | | A6／A10 | 【量】 |
| T6 | `api/proto/social`、`pkg/proto/social`、numeric-allowlist 38 行 | 只為了 8 支保留 | A10 | 計畫:14,417 |
| T7 | 髒字詞典兩份 | | A6 | 計畫:123 |
| T8 | client 的聊天、評價封鎖入口改兩次 | buyer-app 2 處、seller-app 2 處（`client-inventory.md` §6、§8-1） | A6 | |
| T9 | 代行排程轉譯 `UpdateSellerPostStatus → delegation.SchedulePost` | | A10 | 計畫:347,439 |
| T10 | 經 legacy 路徑的測試與驗收 | 計畫:359-362 | 同上 | |

### 1.2 「留到 X 階段」的分類

| 留在外面的東西 | brief 標法 | 前提核對 | 分類 |
|---|---|---|---|
| im、review、crm 到 A6 | REAL | (1) RPC：Java 不呼叫 im／review／crm（`rpc-calls.csv` 的呼叫者只有 gateway、buyer-service、seller-service、social-service【量】）。它們呼叫 Java ordering 的 3 支方法（`services/{review,im}-service/internal/client/order/client.go:30,53,84`【量】），單體已有 `internal/legacy/orderservice` 可以補（`orderservice.go:55-422`【量】）。(2) **事件（v1 漏查）**：review 的事件有 Java 消費者，crm 消費 Java 事件，並呼叫 Java coupon（下表）。前例都有：`buyerconsumer` 已經在消費 `order.completed`（`internal/legacy/buyerconsumer/rabbitmq.go:13`【量】），`sellerrelay` 是 relay 的前例。(3) im 的即時通道是一個決定（ADR 09-26 D7），不是滿足不了的前提。**沒有滿足不了的前提** | **SELF-IMPOSED** |
| notification 到 A8 | REAL | 單體已經有 legacy consumer 的前例；FCM 照規則 5 走 job | **SELF-IMPOSED**。仍建議留 relay，理由見 S4。這和 wiki Case 4 不衝突：Case 4 測的是過度修正，「SELF-IMPOSED」不等於「必須提前做」（F11） |
| 代行貼文 6 條到 A10 | REAL | delegation 模組已經在單體，有 `ResolveDelegation`（`delegation.go:308`）與稽核 API（:324-336）【量】；ADR `2026-09-27-delegation-module.md`:42 寫明「A10 的編排直接加進這個模組」【量】 | **SELF-IMPOSED**。代行的商品、訂單切片依賴 Java，留在 A10 |
| 聊天與評價入口送內部編號 | REAL | 契約與 client 都是我們的 | **SELF-IMPOSED** |
| 往 Java 的橋（catalog 驗證 SD24、review→order／product、crm←order／aftersales、crm→coupon） | — | Java catalog、coupon、ordering、aftersales 在 B4、B7、B8、B9 才改寫（主計畫:256-275【量】）。A 段的任何排序都消不掉這些橋，消掉就等於把 B 段拉進來 | **REAL（相對於 A 段的任何排序）**。Java 也是我們的程式，所以嚴格說是「B 段由下而上的順序」這個 SELF-IMPOSED 決定（ADR 09-24 D8）造成的。這裡沒有人提議改它 |
| buyer、store 的 ID 是 bigint | REAL | 同上，Java 還存著（ADR 10-17 寫了觸發條件） | 同上一列 |
| 沒有買家條款 | REAL | 缺資料 | **REAL**（缺資料） |
| 賣家條款 §18.4（下架可申覆） | REAL（契約） | 系統沒上線，沒有任何賣家簽過；是我們的草稿 | **SELF-IMPOSED**：當作產品 SSOT，符合基準（YouTube 申訴、DSA 20），保留（F4） |
| Apple 1.2 | REAL | 第三方規範：過濾、檢舉並及時回應、封鎖、公開聯絡方式（[Apple 1.2](https://developer.apple.com/app-store/review/guidelines/#user-generated-content)） | **REAL**，在送審時生效。A7 到 A6 之間不會送審，S3 的缺口不違反它 |
| 10e 正在改 seller 治理 | SELF-IMPOSED | 協調成本 | **SELF-IMPOSED** |

**v1 漏查的事件（F1，我自己重跑核對過）【量】：**

指令：`awk -F, '$3~/^(im|review|crm)-service$/ || $4~/^(im|review|crm)-service$/' backend/docs/plans/2026-09-24-go-modular-monolith/events.csv`

| 事件（`events.csv` 行號） | 方向 | 消費者 | 橋活到 | 分類 |
|---|---|---|---|---|
| `review.created`（:118,:119） | review → Java | order（標記已評價＋快照）、product（評分） | B8、B4 | REAL（相對 A 段） |
| `review.hidden`（:121） | review → Java | product（評分） | B4 | REAL（相對 A 段） |
| `review.created`（:117） | review → Go | notification | A8 | SELF-IMPOSED；和上兩列共用同一個 relay，邊際成本近 0 |
| `review.created／hidden`（:120,:122） | review → 舊 seller-service | seller 評分 | 不需要 | 單體 seller 沒有評分欄位，不接（`sellerrelay/relay.go:12`【量】） |
| `chat.message.sent`（:67） | im → Go | notification | A8 | SELF-IMPOSED |
| `order.completed`（:95） | Java → crm | | B8 | REAL（相對 A 段）；照 `buyerconsumer` 前例加一個 queue |
| `aftersales.refund.completed`（:36） | Java → crm | | B9 | REAL（相對 A 段）；新的 consumer |
| `buyer.deleted`、`seller.deleted`、`seller.suspended`、`account.deleted` | 單體 → im、crm | | — | 進單體後改成 in-process 訂閱，不用橋 |
| crm 呼叫 `CouponService.ListBuyerCouponsForSeller`（`services/crm-service/internal/client/coupon/client.go:33`） | crm → Java | | B7 | REAL（相對 A 段）；新的 legacy coupon client |

反過來，A6 也會**刪掉**兩個現有的 legacy client：`internal/legacy/reviewservice`（91 行，seller、discovery 在用）與 `internal/legacy/crmservice`（55 行），改成 in-process 呼叫【量】`wc -l`、`rg -ln 'legacy/(crmservice|reviewservice)"'`。

### 1.3 邊界要不要改

業界基準：Strangler Fig 的 Transitional Architecture 承認一定要寫會被丟掉的程式。判斷標準是它換來的速度與風險降低是否大於成本，而且要容易移除（[martinfowler.com](https://martinfowler.com/articles/patterns-legacy-displacement/transitional-architecture.html)）。所以問題是：沿哪條縫切，過渡物最少。

**判斷標準（S3、S4 都套同一條，F5）：** 系統沒上線，所以本機缺口可以接受。只要「橋的成本高、而且是白做的」，就接受缺口、不寫橋；橋便宜、缺口又長，就寫橋。

#### S3：A7 不寫往舊 im／review 的橋，A6 是下一個階段

- **省下【推，依計畫步驟內容估】：**
  - A7-3 只剩 relay：從計畫的 1 個 session（實際可能 2 個，因為 T4 的停止條件會觸發）降到約 0.5 個。
  - 不在舊 im／review 新增 2–3 支 RPC（T4）。
  - 不寫「是否封鎖」RPC（T3），不動 `buyer.proto`，少一次知會 10e。
  - A6 不用拆掉這些橋（約 0.5 session）。
  - client 的聊天、評價封鎖入口只改一次（T8）。
  - 合計：A7 約 −1.5、A6 約 −0.5 session。
- **語意好處：**
  - D9 的「駁回時還原」對聊天、評價從 A6 第一天就成立。不採 S3 的話，舊服務沒有取消隱藏，會先缺。
  - 檢舉對象一開始就用 UUID＋`target_type` CHECK，不用 `target_id TEXT` 混裝 bigint 和 UUID（計畫:171-173）。
  - CRM「黑名單」直接併入店家封鎖，賣家不會同時看到兩種「封鎖」。
- **不算 S3 成本的部分（F1）：** 上表的 review relay、crm 的 Java consumer、coupon client、orderservice 的 3 支方法。A6 什麼時候做都要寫。
- **真的成本：**
  1. **本機缺口**：從 A7-4 client 改打單體封鎖 API 起，舊 im／review 讀的 Redis 封鎖集合不再有人寫。A7-5 刪掉 social-service 之後，App 聊天、評價畫面上的封鎖與檢舉按鈕打到已刪的路由會失敗，admin 也處理不了聊天、評價的檢舉。這個缺口持續到 A6 的 client 步驟完成。
  2. **排序承諾**：主計畫把 A6 排在 A7 之後（`2026-09-24-go-modular-monolith.md:205`），但 A4、A5 也還沒做。S3 等於說 A6 排在 A4、A5、A8 前面。
- **判斷：建議採 S3。** 不採的話，T3、T4 的白做成本要明列進 A7-3，而且 D9 的還原對聊天、評價會先缺。
- **缺口怎麼呈現給使用者是 Q1**：按鈕照留、本機失敗（不改 UI），或者先藏起來（UI 改動，要你同意）。

#### S1：代行貼文 6 條路由併進 A7

- **省下：**
  - T2 的代行 5 支、T9、`admin_delegated_post.go` 與 `delegation_client.go` 的 5 支 social 呼叫、T10 的代行部分。
  - 和 S3 一起採，legacy SocialService 整個不用寫，T6（`api/proto/social`、`pkg/proto/social`、allowlist 38 行）在 A7-5 就刪，不用留到 A10。
  - 代行貼文的所有寫入（不只排程）在同一個 transaction 內寫稽核，沒有 PENDING 窗口。
- **多出的成本（F6）【量＋推】：**
  - gateway 的 26 支 credential 路由共用一套 middleware：`delegation_context.go` 92 行、`delegated_routes.go` 71 行、`idempotency.go` 205 行（另有 `idempotency_store.go`）、`ratelimit_delegated`、boot 白名單斷言（`routes_delegated.go:11-30`，posts 6 條在 :107-113）【量】。
  - 單體 httpapi 沒有代行 credential 的處理，也沒有 `Idempotency-Key` 的處理（`internal/httpapi/origin.go:15-21` 只把這兩個 header 列進 CORS）【量】。要新做：
    - 讀 `X-HK-Operation-Session` → `delegation.ResolveDelegation`（模組已有，`delegation.go:308`）；
    - 能力檢查（posts.read／write／publish）；
    - 冪等：照 [AIP-155](https://google.aip.dev/155)，在 request message 放 `request_id`，對到稽核表既有的 `(session_id, idempotency_key)` 唯一鍵（`alignment-facts.md` §5）；
    - 限流沿用 httpapi 既有的機制【推】；
    - ADR `2026-09-27-delegation-module.md` 加修訂段，定第一版「代行 Connect」的形狀。
  - A10 前會有兩套代行管線並存：貼文走單體 Connect，商品與訂單的 20 條走 gateway。website 的代行工作區只連單體一個入口（還沒搬的路徑由 `gatewayproxy` 轉給 gateway），但會同時用 Connect client 和手寫 REST 兩種寫法。
  - A10 可能要修改這個形狀：只拿一個切片設計，商品、訂單的需求（例如批次、檔案）可能沒涵蓋到【推】。
- **估計：** 新增 A7-2d（1.5–2 session）；A7-3 省 0.5；A10 少做貼文切片約 1。整體 ≈ +0.5 session。不碰 10e 擁有的檔案（delegation、httpapi、新 proto、website 的 4 個代行 BFF）。
- **判斷：弱建議採。** 好處是 legacy social 契約在 A7-5 就能完全刪掉，A10 的形狀提早驗證。如果你寧可 A7 少 2 個 session，不採也合理：T2、T6、T9 照計畫留到 A10。

#### S4：A8 也提前

- 不建議。照上面同一條標準：socialrelay 只有 6 個 key，約 0.5 session，照 sellerrelay 的前例做。缺口會很長：社群推播要缺過 A7＋A6，大約 30 個 session。A6 本來就要為 Java 寫 review relay，模式可以共用。
- v1 的理由「哪個順序都有同等大小的橋」不精確，已換成這條（F5）。

#### 終點與觸發（改寫，F1）

- **A7-5（採 S1＋S3）刪除：** social-service、gateway 全部社群路由（含 `/app/blocks*` 與代行貼文 6 條）、`SocialClient`、`admin_delegated_post.go`、`api/proto/social`、`pkg/proto/social`、allowlist 38 行。
- **A6 完成時刪除：** im、review、crm 三個服務、`pkg/blocklist`、`pkg/moderation`、legacy `reviewservice` 與 `crmservice` client、gateway 的 im／review／crm 路由。
- **A6 之後還在的橋：**
  - `socialrelay`（6 key）、`chat.message.sent` relay、review relay 給 notification 的部分 → **A8**；
  - review relay 給 Java product → **B4**；給 Java order → **B8**；
  - crm 的 `order.completed` consumer、orderservice 新增的 3 支方法 → **B8**；`aftersales.refund.completed` consumer → **B9**；coupon legacy client → **B7**；
  - 商品存在驗證的 `productservice`（SD24）→ **B4**。

---

## 2. 分兩類：可以直接做的，和要你決定的

### (a) 技術與結構：建議照做（你拍板計畫時一起確認，不需要產品判斷）

| # | 建議 | 依據 | session | 碰 10e 的檔案 |
|---|---|---|---|---|
| a1 | S3：A7 不寫往舊 im／review 的橋 | §1.3 | −2（A7＋A6） | 少碰 `buyer.proto` |
| a2 | S1：代行貼文用 Connect 併進 A7（弱建議） | §1.3 | ≈ +0.5（A7 +1.5） | 否 |
| a3 | S4：留 socialrelay | §1.3 | 0 | 否 |
| a4 | D6：social 擁有排程，delegation 撤銷時取消、同步寫稽核 | §3 D6 | 0 | 否 |
| a5 | D7：認定來源改多對多，和 10e 3b 同結構 | §3 D7 | 0 | 是（seller schema；和 10e 談誰先落地） |
| a6 | D8：採 10e D′1 (c)，3d 做 seller 來源、合併層、`reviewqueue` 契約與待辦頁；A7-1e 只加 moderation 來源 | §3 D8 | −0.5 | 否（3d 由 10e 做；ADR D3 第 3 點修訂文字在 10e brief D′1） |
| a7 | D9：處分紀錄帶型別、可選的 `ends_at`、可讀的理由代碼與說明（API 先做） | §3 D9 | 0（在 A7-1d 內） | 否 |
| a8 | D11：「本週主題」改成伺服器排序；讚數、留言數同 transaction | §3 D11 | +0.5（A7-2、4a） | 否 |

### (b) 產品、政策、UI：你的決定

| # | 題 | 選項 | 我的建議 | session | 觸發或期限 |
|---|---|---|---|---|---|
| Q1 | S3 的本機缺口：接受嗎？缺口期間聊天、評價的封鎖與檢舉按鈕怎麼辦？ | (i) 接受，按鈕照留、本機會失敗；(ii) 接受，按鈕先藏（UI 改動）；(iii) 不接受，照計畫寫橋 | (i)：沒上線、不送審，而且 A6 接著做 | (i)(ii) 0；(iii) +2 | A7-0 拍板 |
| Q2 | 買家處分要不要有警告、有期限的社群參與限制？期限、次數、90 天累計怎麼訂？ | (i) 計畫：隱藏＋BAN；(ii) 加「警告」紀錄（使用者看得到）；(iii) 完整階梯：警告 → 有期限的參與限制 → 停權 | (ii) 現在做，(iii) 的參數等買家條款。只有 BAN 會讓一則垃圾留言只能用「停掉整個帳號」處理 | (ii) +0；(iii) +0.5～1 | 參數：買家條款寫好時 |
| Q3 | 一般貼文的留言要不要即時推？ | (i) 保留（A7-2 做 streaming）；(ii) 拿掉，打開或下拉時才拉（畫面行為改變） | 傾向 (ii)：非直播的貼文多半不即時推（一般知識（未查證）），扇出和 A6 的聊天一起設計。但這是產品取捨 | (ii) −0.5～1 | A7-2 開工前 |
| Q4 | 匿名瀏覽要不要計？隱私權政策要不要寫一句？ | (i) 計畫：只計登入者；(ii) 也計匿名，用每日換 salt 的雜湊去重，不存 IP 與 UA | (ii)：website 公開頁大多是匿名瀏覽，只計登入者會嚴重少算。上線時適用個資法，IP 可能算個資（一般知識（未查證）），所以政策要寫一句 | (ii) +0.5 | A7-1a 開工前 |
| Q5 | 要不要一份平台的內容政策分類，讓檢舉原因、seller 通報類別、認定類別共用？ | (i) 計畫：moderation 自己的 enum，內容下架用一個「社群」類別；(ii) 平台一份 `PolicyCategory`，各入口取子集 | (ii)：10e 的 3b、3c 還沒開工，現在改最便宜。事實（10e，2026-10-18）：現行 seller 的 7 個通報類別對不上賣家條款第十六條第 3–7 款，類別本來就要重整。但它會動到 10e 的 seller 類別與術語表，要和 10e 一起定 | (ii) +1（10e 檔） | A7-1c 開工前 |
| Q5′ | website 要不要新增封鎖名單頁（D12） | (i) 新增，重用列表元件；(ii) 不做 | (i)：不做的話，website 上的封鎖無法解除；seller-app 已經有 | (i) +0.5（4a 內） | A7-4a 前 |
| Q6 | 處分理由頁與推播 | (i) A7 只做 API，畫面等 UI 稿；(ii) A7 一起做 App 理由頁；推播都等 A8 | (i)＋推播等 A8。加推播要在舊 notification-service 加範本，違反「舊服務只做讓它繼續運作所需的修改」（backend `AGENTS.md`），這條 SELF-IMPOSED 規則符合基準，保留 | (ii) +0.5／端 | UI 稿核可時 |

---

## 3. 各題

### A7-D6 代行的排程發布由誰執行

**業界基準：**
- 排程發布是內容生命週期的狀態，由內容系統擁有。YouTube 的 `status.publishAt` 只能設在 private、沒發布過的影片上，到點等同改成 public（[YouTube Data API videos](https://developers.google.com/youtube/v3/docs/videos)）。WordPress 的 `future` 狀態加 cron 也是同一模式（一般知識（未查證））。
- 代表他人做的動作，在執行時重新驗證授權，或撤銷授權時一併取消（一般知識（未查證））。

**既有機制為什麼存在：**
- 兩段式 intent → confirm，是因為 social-service 與 account-service 不能同一個 transaction（`post_schedule_worker.go:127-246`）。
- 「只有代行能排程」是實作副作用：social-service 靠 `scheduled_by_session_id` 重建授權，賣家路徑沒有 session 可指（`gateway/internal/handler/post.go:308-310`【量】）。
- 稽核的 scheduled origin 允許 session 已經**結束**，只有**撤銷**會擋（`alignment-facts.md` §5，`audit.go:88-135`）。所以「撤銷時取消」和現行語意一致，過期不用處理。

**選項：**
- **A（計畫）**：delegation 擁有 job，social 存 `scheduled_publish_at` 供顯示。
  - 好處：依賴方向直接，到點的授權、intent、發布、confirm 都在 delegation 一個 transaction 內。
  - 缺點：排程有兩個真相（social 的欄位、delegation 的 River job），要靠 `PublishScheduled` 比對時間。哪天賣家也能排程，social 要再做一套。
- **B**：social 擁有 job，加一個反向 port 由 delegation 實作。單體沒有這種前例（現有 port 都由被依賴的一方實作，`agent/ports.go:10-15`【量】），要寫 ADR。
- **D（建議）**：
  1. social 擁有排程與 job，對所有發起者一樣，記不透明的 `scheduled_by`（店主或代行 session）。
  2. delegation 撤銷 session 時，在同一個 transaction 呼叫 `social.CancelScheduledBy(session)`。方向合法（delegation → social），`InTx` 可重入（`db.go:110-115`【量】）。
     - 撤銷目前有 3 條 SQL 路徑，沒有收斂到一個 Go 函式：`RevokeDelegationSession`、`RevokeActiveDelegationsOfActor`、`RevokeActiveDelegationsOfStore`（`internal/store/delegation_sessions.sql.go:717,767,824`【量】）。A7-1f 要先把它們收斂到一個內部撤銷點。
  3. 稽核：delegation 在同一個 transaction 內訂閱 social 的 `PostPublished{origin=scheduled, scheduled_by}`，寫一列單段的 SUCCEEDED。同步 bus 的 handler 失敗會讓發布回滾（`events/bus.go:41-77`【量】）。`SessionForAudit` 會拒絕已撤銷的 session，等於第二道防線。
  4. 發布與取消都鎖同一列貼文，所以兩者會序列化【推，要在 A7-1f 以 integration 測試驗證】。
  5. **失敗路徑（F10）**：第二道防線擋下時，delegation 回 `apperr` 的權限類錯誤；social 的 job 看到這一類就把排程清掉、轉成「已取消」、發 `PostScheduleCanceled{reason}`，不重試。其他錯誤照常重試【推】。驗收加一條：拿掉第 2 點的取消呼叫，job 要轉成已取消，貼文不發布，也不會卡在「已排程」。
  6. 兩段式只是為了跨服務。同一個 transaction 裡，PENDING 可以收掉。
- **C（發事件、不取消）**：撤銷後仍會發布。不採用，和計畫相同。

**約束帳：**

| 約束 | 分類 | 來源 |
|---|---|---|
| social 不能 import delegation（依賴表） | SELF-IMPOSED，符合基準（模組依賴單向、無環，例如 Shopify 用 packwerk 強制邊界，一般知識（未查證）），保留。D 不需要改它 | `internal/arch/modules.go:34,36` |
| 下層通知上層又要原子性時用 `events.Publish` | SELF-IMPOSED，D 照它做 | backend `AGENTS.md` 規則 4 |
| 稽核 `result` 是 PENDING→SUCCEEDED | SELF-IMPOSED（我們的 schema） | delegation `schema.sql:222-294` |
| 代行路由在 gateway 寫 POST_SCHEDULE | SELF-IMPOSED；採 S1 就消失，intent 和排程同一個 transaction | 計畫:439 |

**建議：D**（沒有 REAL 約束，照基準）。A 不是錯，D 不成立時可以當退路。賣家能不能自己排程是產品題，現在不用決定；D 讓它之後不用重做。
**終點與觸發：** 排程只有 social 一個擁有者；A7-1f 做到。

### A7-D7 「內容下架」措施的模型

> 2026-10-18 暫緩已解除：10e 審查後改回「維持兩套」（店家與商品的通報與案件留在 seller，moderation 只收 UGC，尊重 ADR 2026-09-18 D2 與 A7-D5′），共用的只做契約對齊：一份政策分類（Q5 (ii)，清單以賣家條款第十一、十六條為準）、通報人收件確認與決定的事件語意、待辦合併（D′1）、濫報規則。本題照 A7 原方向。

**業界基準：**
- YouTube 移除內容時說明移除了什麼、違反哪條政策、影響是什麼；第一次是警告，之後 strike 保留 90 天，90 天 3 次可終止頻道，可以申訴（[YouTube strikes](https://support.google.com/youtube/answer/2802032)）。
- DSA 17 要求可見性限制附「clear and specific statement of reasons」；DSA 20 要求免費的內部申訴（[Art. 17](https://www.eu-digital-services-act.com/Digital_Services_Act_Article_17.html)、[Art. 20](https://www.eu-digital-services-act.com/Digital_Services_Act_Article_20.html)）。這裡只當基準，範圍是在 EU 提供服務的 hosting provider。

**既有機制為什麼存在：** 措施只由 admin 建立（seller `schema.sql:449`）；沒有內容對象（`alignment-facts.md` §1）；`violations.source_report_id` 只能指向 `seller.reports`（`schema.sql:328`）；10e 3b 已核可改成多對多 `finding_reports`（`step10e-3-brief.md:40,277`【量】）。

**選項與取捨：**
- 同意計畫第 1、3、4、5、6、8、9、10 項。
- **第 7 項**：計畫是單欄 `moderation_report_id`。同一則內容常被多筆檢舉，單欄只掛得了一筆，也和 3b 的方向牴觸。改成多對多：seller 通報用 `finding_reports`（有 FK）；moderation 檢舉用 `finding_moderation_reports(finding_id, report_id UUID)`，不加 FK，是不透明的外部參照。或者合成一張帶 `source_kind` 的表。和 10e 談誰先落地，先落地的建表。（2026-10-18 更新：10e 3b 建 `finding_reports`；moderation 用另一張 `finding_moderation_reports`，不合成一張帶 `source_kind` 的表，因為合成會失去 `finding_reports` 對 `seller.reports` 的 FK。3c 的通報類別用明確子集常數，A7-1c 加的類別不會跑進賣家通報。）
- **第 2 項**（新增一個「社群」類別）：一個籠統的桶，理由說明不具體，升級統計也分不出原因。要不要換成平台政策分類是 Q5。

**約束帳：**

| 約束 | 分類 | 來源 |
|---|---|---|
| 下架可以申覆（賣家條款 §18.4） | SELF-IMPOSED（我們的草稿，當 SSOT），符合基準，保留 | brief:281 |
| 跨 schema 不建 FK | SELF-IMPOSED（規則 2，有測試強制），符合 modular monolith 各模組資料隔離的做法（[Kamil Grzybek, Modular Monolith: A Primer](https://www.kamilgrzybek.com/blog/posts/modular-monolith-primer)，未逐字查證），保留 | backend 規則 2 |
| 10e 擁有 seller schema、proto、術語表 | SELF-IMPOSED（協調） | 計畫:47 |

**建議：** 8 項照計畫，第 7 項改多對多，第 2 項看 Q5。
**終點與觸發：** 內容下架和其他措施共用同一條申覆、升級、理由管線；A7-1c。

### A7-D8 平台待辦佇列和 10e 第 3d 刀誰先

> 2026-10-18 修訂：原建議（A 加事件帶 `due_time`）撤回，改採 10e 3c/3d 簡報 D′1 的 (c)。全文與約束帳在 10e 的 `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3cd-brief.md` D′1，這裡只記為什麼改。

**業界基準：** 跨模組查詢有 API Composition（呼叫各擁有者、記憶體合併）與 CQRS 讀取模型兩種；工作佇列實體化是為了指派、認領、路由這類「不屬於業務狀態」的東西（ROOST Coop、Zendesk 類工具）。跨模組本身不是實體化的理由。

**原建議錯在哪：** 我否決查詢聯邦的理由是「之後要指派還得實體化，等於做兩次」。10e 的回應成立：
1. 實體化的觸發是指派；只有一位營運人員，沒有第二方，指派沒有需求（REAL，缺第二方）。
2. 投影本來就要每個來源一支全量重建查詢，那支就是 (c) 的來源查詢；之後實體化只丟合併層。
3. 現在投影要先持有事件、重建指令、開關路徑，時間觸發的項目靠 job 還有最多數分鐘的延遲；(c) 在讀取時以注入的 `now` 判定，天生一致。
4. 投影是同 transaction 的同步訂閱者，它的 bug 會 rollback `AcknowledgeReport`、`CreateMeasure`；(c) 不碰寫入路徑。

**約束帳：** ADR 2026-09-24 D3 第 3 點（跨模組篩選與分頁用讀取模型）：SELF-IMPOSED；它防的是拿別人的資料篩選或 join，(c) 的合併層不做這兩件事，但照字面是偏離，需要修訂（文字在 10e D′1）。只有一位營運人員：REAL。

**建議：(c)。** 3d 做 seller 來源、全域 keyset 合併（比較函式只有一份，放 platform）、`api/hawkr/admin/reviewqueue/v1alpha1` 與待辦頁；A7-1e 只加 moderation 的來源與種類（subject 可擴充、店家欄位選填，10e 已在契約預留）。

**終點與觸發：** 出現來源算不出的篩選或排序鍵，或只屬於合併結果的狀態（指派、認領、暫緩）時，改成讀取模型；來源查詢保留當重建查詢。

### A7-D9 被處理的內容與買家處分

> 2026-10-18 處分歸屬（10e R12）：10e 的方向是處分紀錄與申覆歸執行效果的模組，紀錄與效果同一個 transaction：買家的社群參與限制在 social，帳號停用在 identity，賣家措施在 seller；moderation 擁有通報、案件與決定，經各模組的 API 建立處分，依賴單向 moderation → social／seller／identity。照這個原則，a7 的處分紀錄從 A7-1d 移到 A7-1a 的 social。修訂版（含 R12）收到後交拍板；這個方向在 10e 改回兩套後照舊。

**業界基準（只當參考，F8）：**
- DSA 23：對「頻繁提供明顯違法內容」的使用者，在事先警告後，停權一段合理期間（[Art. 23](https://www.eu-digital-services-act.com/Digital_Services_Act_Article_23.html)）。它針對的是明顯違法內容，不是一則垃圾留言這類社群規範違規，所以只能借它的結構：先警告、有期限。
- YouTube：第一次違規通常是警告，之後的 strike 先限制一週、兩週，最後才終止（[YouTube strikes](https://support.google.com/youtube/answer/2802032)）。

**既有機制為什麼存在：** `WARN_USER` 現在對使用者沒有效果（SD11）。BAN 呼叫 `identity.SuspendAccount`，停掉整個帳號（包含購物），沒有期限（`identity.go:218`）。賣家端的措施已有 WARNING 與有期限的停權。

**選項與取捨：**
- **(a) 技術結構，建議現在做（a7）：** moderation 的處分紀錄帶型別（初始是 HIDE、BAN，已核可）、可為空的 `ends_at`、理由代碼與說明，API 可以讓作者讀到。這樣之後加警告或限制，只要加型別與規則，不用改結構。
- **(b) 產品政策，Q2：** 要不要有警告、有期限的社群參與限制、90 天累計，以及期限與次數。這些就是使用條款的處分條文，沒有條款就訂，等於 agent 發明政策（F2）。
  - 如果選 (iii)，技術上是 social 擁有「參與限制」狀態，moderation 呼叫 `social.RestrictParticipant(actor, until)`（moderation 依賴 social，方向合法）。
- **同意計畫：** 作者看得到被隱藏的內容與理由、別人看不到、作者不能編輯、駁回或重開時還原。
- **理由頁與推播是 Q6。**

**約束帳：**

| 約束 | 分類 | 來源 |
|---|---|---|
| 沒有買家條款 | **REAL（缺資料）**：階梯的參數與買家申訴都卡在這裡 | brief「還不能定的」第 1 項 |
| A8 之前 notification 的 wire 不變；舊服務只做必要修改 | SELF-IMPOSED，符合基準，保留 | 計畫:73；backend `AGENTS.md` |
| 還原聊天、評價需要新增 RPC | SELF-IMPOSED；採 S3 就消失 | §1.1 T4 |

**建議：** (a) 照做；(b) 見 Q2、Q6。D5′ 其他部分不重議。
**終點與觸發：** 買家與賣家的處分結構對稱；結構在 A7-1d，參數在買家條款寫好時。

### A7-D10 即時留言 streaming 的扇出

**前提題（產品，Q3）：** 一般貼文的留言要不要即時推。拿掉是畫面行為改變，要你同意。

**業界基準（如果保留）：**
- PostgreSQL `NOTIFY` 在 commit 時才送，中止就丟；payload 預設小於 8000 bytes，文件建議只送 key；佇列滿了會讓 commit 失敗（[NOTIFY 文件](https://www.postgresql.org/docs/current/sql-notify.html)）。
- 在 transaction 內 `NOTIFY` 時 commit 會拿一把全庫的鎖；Recall.ai 在大量並發寫入下出過事（[recall.ai](https://recall.ai/blog/postgres-listen-notify-does-not-scale)），DBOS 量到瓶頸約 2.9K 寫入／秒（[dbos.dev](https://dbos.dev/blog/postgres-listen-notify-scalability)）。修正落在哪個版本沒查證。

**既有機制為什麼存在：** 舊的留言 SSE 經 RabbitMQ 的動態 queue 扇出到 gateway pod（`events.csv` 的 `gateway.sse.<pod>` 列【量】）。

**選項與取捨：** 計畫 A：只帶 ID、以 DB 游標為準、不用環形緩衝，正是文件建議的用法。補兩點：每個實例一條專用 LISTEN 連線，不經 transaction pooling（一般知識（未查證））；採 S3 時，扇出和 A6 的聊天一起設計。

**約束帳：** RabbitMQ 在 A11 之後退場：SELF-IMPOSED，方向正確。

**建議：** 保留就照 A；拿掉就把扇出延到 A6-2 的聊天。
**終點與觸發：** 一套扇出，留言（若保留）、聊天、通知共用；A7-2 或 A6-2。

### A7-D11 計數、瀏覽數與分享數

**業界基準：**
- 熱點計數用分片計數器（[Firestore distributed counters](https://firebase.google.com/docs/firestore/solutions/counters)）；中小規模同 transaction 直接更新就夠（一般知識（未查證））。
- 不用 cookie 的訪客去重：Plausible 用 `hash(daily_salt + domain + ip + user_agent)`，salt 每 24 小時換並刪除，不存 IP 與 UA（[plausible.io/data-policy](https://plausible.io/data-policy)）。

**既有機制為什麼存在：**
- `GetPost` 不能有副作用（ADR 09-26 D3）：Connect 只對標成 side-effect free 的方法開 GET（[connectrpc.com](https://connectrpc.com/docs/go/get-requests-and-caching/)）。
- website「本週主題」在 client 把最新 30 篇依 `viewCount` 重排，註解寫明原因是後端沒有這支 API（`website/lib/api/posts.ts:104-121`【量】）。

**建議：**
- 讚數、留言數：同 transaction；熱點分片等負載測試，觸發條件寫進 ADR（a8）。
- 「本週主題」：client 只排得到拿回來的那一頁，是 client 推導的錯（wiki Case 8）。改成伺服器 `ListPosts` 依時間窗熱度排序；畫面不變（a8，A7-2 與 4a，約 +0.5 session）。
- 分享數每位買家每篇只算一次：同意。
- 匿名瀏覽：Q4。

**約束帳：** ADR 09-26 D3：SELF-IMPOSED，符合 HTTP 安全方法的語意（[RFC 9110 §9.2.1](https://www.rfc-editor.org/rfc/rfc9110#section-9.2.1)），保留。個資法：上線時適用的法律（REAL，但現在沒有資料在收）。

**終點與觸發：** 伺服器擁有所有排序與計數；A7-1a、A7-2。

### A7-D12 website 的封鎖名單頁

**業界基準：** Apple 1.2 要求有 UGC 的 App 能封鎖濫用者（REAL，送審時）；創作者後台通常有一頁管理被隱藏或封鎖的使用者，例如 YouTube Studio 的 Hidden users（一般知識（未查證））。seller-app 已有封鎖名單（`client-inventory.md` §7）。
**既有機制：** website 沒有這一頁。
**選項與取捨：** 新增（重用列表元件，和 seller-app 對等）／不做（website 上的封鎖無法解除）。
**約束帳：** 「UI 改動先取得同意」：SELF-IMPOSED（根 `AGENTS.md`），保留。
**建議：** 新增，但這是 UI 改動，是 Q5′。
**終點與觸發：** 三端都能解除封鎖；A7-4a。

### A7-D13 舊檢舉原因怎麼對到新模型

> 2026-10-18 暫緩已解除：10e 審查後改回「維持兩套」（店家與商品的通報與案件留在 seller，moderation 只收 UGC，尊重 ADR 2026-09-18 D2 與 A7-D5′），共用的只做契約對齊：一份政策分類（Q5 (ii)，清單以賣家條款第十一、十六條為準）、通報人收件確認與決定的事件語意、待辦合併（D′1）、濫報規則。本題照 A7 原方向。

**業界基準：** DSA 16 的 notice 機制；一個平台通常只有一份政策分類，例如 DSA Transparency Database 的 statement of reasons 用統一的類別清單（一般知識（未查證））。
**既有機制為什麼存在：** 舊原因 8 種（`domain-inventory.md` §3）；seller 通報與認定 7 種，偏食安與商業（seller `schema.sql:237,341`）。兩份各自演化，沒有設計上的理由。
**選項與取捨：**
- 分流同意：店家層級的檢舉走 seller 通報；使用者對象留在 moderation。
- 分類：(i) 各自維護；(ii) `api/` 定一份 `PolicyCategory`（垃圾訊息、騷擾、仇恨、詐騙、不實、食安、仿冒、其他），各入口取子集，舊的 8 種在搬遷時對映一次。
**約束帳：** 10e 擁有 seller 類別：SELF-IMPOSED（協調）。
**建議：** (ii)，但分類本身是平台政策，而且動到 10e 的檔案 → Q5。估計 +1 session（proto enum、uitext、seller 類別 migration、術語表）。
**終點與觸發：** 一份分類，理由、升級統計、報表一致；A7-1c 開工前定。

---

## 4. 修訂後的步驟表（採 S1＋S3＋D6=D 時）

每個子步驟一個 session，除非另外寫。「被擋於」可以直接抄回計畫。

| 步驟 | 內容的變動 | 被擋於 | session |
|---|---|---|---|
| A7-0 | 加：本簡報的 Q1–Q6 拍板；brief 文末更正 REAL 誤標 | 無 | 進行中 |
| A7-1a social 本體 | 加：匿名瀏覽（看 Q4）；處分理由欄位給作者讀 | Q4、a7 | 1 |
| A7-1b 封鎖 | 減：不做「是否封鎖」legacy RPC；模組 API 留給 A6 in-process 使用 | A7-1a | 1 |
| A7-1c 內容下架 | 改：認定來源多對多；類別看 Q5 | D7、Q5、A7-1a；10e 協調 | 1（Q5 選 ii 時 +1） |
| A7-1d moderation | 改：對象只有貼文、留言、討論區、使用者、店家；`target_id` 改 UUID＋`target_type` CHECK；不做 im／review port；處分紀錄帶型別與 `ends_at` | D7、Q2、Q5、A7-1a、A7-1c | 1（Q2 選 iii 時 +0.5～1） |
| A7-1e 平台佇列 | 改：只加 moderation 的來源查詢與種類，接進 10e 3d 的合併層與 `reviewqueue` 契約 | D8、A7-1d、10e 3d | 0.5 |
| A7-1f 排程發布 | 改成 D：social 擁有 job；delegation 撤銷收斂＋取消＋同步稽核；失敗轉已取消 | D6、A7-1a | 1 |
| A7-2 對外 API | 加：`ListPosts` 熱度排序；streaming 看 Q3 | Q3、A7-1a～1f | 1 |
| **A7-2d 代行貼文 Connect（新，S1）** | 代行 Connect 管線（credential、能力、`request_id`、限流）＋6 支方法；delegation ADR 修訂 | A7-1f、A7-2 | 1.5～2 |
| A7-3 relay | 只剩 `socialrelay`；不做 socialserver、`buf.gen.legacy.yaml` 不加 social、不改 im／review、不接 moderation legacy port | A7-2 | 0.5（可併進 A7-2） |
| A7-4a website | 加：代行貼文 BFF 改 Connect；封鎖名單頁看 Q5′ | A7-3、A7-2d、Q5′ | 1 |
| A7-4b admin | 不變 | A7-3 | 1 |
| A7-4c buyer-app | 聊天、評價的封鎖與檢舉按鈕看 Q1 | A7-3、Q1 | 1 |
| A7-4d seller-app | 同上 | A7-3、Q1 | 1 |
| A7-4e gateway 與清單 | 加：刪 `/app/blocks*` 3 條、代行貼文 6 條、`SocialClient` 整個、`admin_delegated_post.go`；`maxRESTOperations` 改扣 40 而不是 37（之後依 10e 期間的變動再算）；允許殘留只剩 A6 的聊天、評價入口與 CRM、catalog 的 hashtag | A7-4a～4d | 1 |
| A7-5 收尾 | 加：刪 `api/proto/social`、`pkg/proto/social`、allowlist 38 行。移到 A6：`pkg/blocklist`（im／review 還在讀 Redis） | A7-4 | 1 |
| **A6-0 盤點與計畫** | 照 A7-0 的格式；Q1 的缺口在 A6 期間持續 | A7-5 | 1 |
| A6-1 review 模組 | 加 review relay（給 Java order、product 與 notification）、orderservice `GetOrderForReview`；刪 legacy `reviewservice` | A6-0 | 1～2【推】 |
| A6-2 im 模組 | streaming（ADR 09-26 D7，沿用 D10 的扇出）；`chat.message.sent` relay；orderservice 2 支；buyer、seller、identity 事件改 in-process | A6-0（扇出看 Q3） | 2～3【推】 |
| A6-3 crm 模組 | `order.completed`、`aftersales.refund.completed` consumer；coupon legacy client；CRM 黑名單併入店家封鎖；刪 legacy `crmservice` | A6-0、A7-1b | 1～2【推】 |
| A6-4 moderation 補聊天、評價對象 | 檢舉快照、隱藏與還原改 in-process；im／review 改查 buyer、seller 封鎖 | A6-1、A6-2、A7-1d | 1 |
| A6-5 對外 API | | A6-1～4 | 1 |
| A6-6 client（四端） | 聊天、評價的封鎖與檢舉入口改送參照（缺口在這裡結束） | A6-5 | 3～4 |
| A6-7 收尾 | 刪 im、review、crm 服務、`pkg/blocklist`、`pkg/moderation` | A6-6 | 1 |

- A7 合計約 15.5 session（計畫是 14）；A6 約 12～15【推，依 A7 的 RPC 數與行數類比：A6 36 支、非測試約 9,000 行；social 34 支、9,388 行】。
- 不採 S1：拿掉 A7-2d；A7-3 加回 socialserver 的代行 5 支與 T9（+0.5）；T6 留到 A10。
- 不採 S3（Q1 選 iii）：A7-3 加回 T3、T4（+1.5，含在 im／review 新增的 RPC）；A7-1d 的 `target_id` 用 TEXT，終點寫「A6 改 UUID」；A6-0 之前可以插別的階段。

---

## 5. 沒能驗證的事

- A6 的 session 估計只用 RPC 數與非測試行數類比；沒讀 im 的 WebSocket 細節。
- S1 的限流是否能直接沿用 httpapi 現有機制、代行 credential 怎麼和 Clerk／Access 的認證並存，沒追到程式碼。
- media 的引用釋放（im、review 呼叫 `ReleaseReferences`）在 A6 走 `modules/media` 還是 `internal/legacy/mediaservice`，留給 A6-0。
- CRM `blocklist` 客戶類型除了標籤以外有沒有其他效果（例如優惠券挑選），沒追。
- D6 的「同一列鎖序列化」與「`apperr` 權限類錯誤讓 job 轉已取消」是推論，要在 A7-1f 驗證。
- 標「一般知識（未查證）」的說法：WordPress `future`、代表他人動作的授權重驗、工作佇列實體化、DSA 透明度資料庫的類別清單、YouTube Hidden users、非直播貼文不即時推、LISTEN 連線與 pooling、熱點計數的規模門檻、Shopify packwerk、IP 在個資法下的地位；Kamil Grzybek 的頁面只引用標題沒有逐字查。
- Postgres `NOTIFY` 全庫鎖的修正落在哪個版本。

---

## 6. Critique 處理對照

| # | 處理 |
|---|---|
| F1 | 修正。重跑 `events.csv` 核對：review→Java order／product、review／chat→notification、Java→crm、crm→coupon。§1.2 補表；這些橋算 A6 自己的成本，不算 S3 的；S3 的省下與成本、終點都改寫，結論是仍然值得 |
| F2 | 修正。D9 拆成 (a) 技術結構 a7 與 (b) Q2、Q6；缺買家條款標成 REAL 缺資料 |
| F3 | 修正。依賴表、規則 2、ADR D3、舊服務規則都改成 SELF-IMPOSED（符合基準，保留），附基準來源 |
| F4 | 修正。賣家條款 §18.4 改成 SELF-IMPOSED（草稿當 SSOT） |
| F5 | 修正。S3、S4 套同一條「橋的成本對缺口長度」標準，S4 改寫理由 |
| F6 | 修正。S1 列出單體要重做的 middleware、兩套管線並存、A10 可能改形狀、session 估計；降成弱建議 |
| F7 | 修正。每個改範圍的建議附 session 與是否碰 10e 檔（§2、§4） |
| F8 | 修正。DSA 只當基準，註明適用範圍（Art. 23 針對明顯違法內容）；拿掉「違反」 |
| F9 | 修正。§4 修訂步驟表，含 A6-x 與被擋於 |
| F10 | 修正。D6 的序列化標【推】；定義失敗路徑轉已取消並加驗收；另外查到撤銷有 3 條 SQL 路徑尚未收斂【量】 |
| F11 | 修正。§1.2 notification 列加一句：SELF-IMPOSED 不等於要提前，Case 4 沒有被推翻 |
