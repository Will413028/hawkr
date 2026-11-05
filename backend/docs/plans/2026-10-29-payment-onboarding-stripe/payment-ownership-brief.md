# 收款帳戶與 webhook inbox 的擁有者：決定簡報

| 項目 | 值 |
|---|---|
| 狀態 | 草稿，待 Will 決定（2026-11-05） |
| 來源 | 單體架構 review 的第 2、3 點 |
| 程式的事實 | `payment-onboarding` 分支 `f834407`（步驟 4–7b 完成，7c、7d、8、9 未完成）；`dev` 是 `fe7641a` |
| 編號 | 決定時由計畫給 D 編號（分支已用到 D25），本檔用 P1、P2 |

證據等級：〔量測〕＝以指令或讀檔確認；〔推論〕＝由程式與文件推得；〔推測〕＝業界做法憑公開資料記憶，未逐條查證原文。路徑相對 `backend/`。

## 0. 前提題：要改的程式之後會不會被重寫、搬走或刪掉

| 會被改到的程式 | 之後的命運 | REAL / SELF-IMPOSED |
|---|---|---|
| `seller/internal/payment`（port、Stripe 與 fake adapter、建立與同步 job、inbox、輪詢）、`seller/payment.go`、`model/payment.go` | 步驟 8 不刪；B2 要在同一個 port 上加付款、退款，會變大〔量測：主計畫 B2「付款建立、webhook（先寫 inbox、event id 去重）都走 port」〕 | SELF-IMPOSED（目前的切法） |
| `seller.payment_accounts`、`seller.payment_provider_events` 與 4 支 migration | 只在分支，沒進 `main`（D21）；階段 C 第一次部署前會把各模組的 migration 壓成 baseline〔量測：主計畫階段 C〕 | SELF-IMPOSED |
| `httpapi/seller/payment_webhook.go`（`POST /webhooks/stripe` 只轉給 seller） | B2 的付款事件也會打到 Stripe 的 webhook | SELF-IMPOSED |
| `platform/stripe`（client、驗簽） | 不動；P1 的任何選項都共用它 | — |

沒有「因為之後要刪所以現在將就」的理由；分支還沒合進 `main`，搬動不牽涉已部署的資料。

## 1. 問題

- B2 的 port 現在 import 不到。主計畫的 B2 寫「付款經 payment port」，可是 port 在 `seller/internal/payment`，規則 1 不准別的模組 import 子套件〔量測：`internal/arch/modules.go` 的規則；`seller/internal/payment/port.go`〕。B2 只能自己另寫一份 port、adapter 包裝與 inbox，或者讓付款事件也經過 seller。
- 依賴表裡本來就有 payment，而且是葉節點：「PayUni 收款、退款、撥款 API」〔量測：ADR `2026-09-24-go-modular-monolith.md` D2；`modules.go` 的 `"payment": {}`〕。收款開通放進 seller，是這次計畫沒有提出來決定的照搬：舊 seller-service 擁有 PayUni 子商店，計畫裡也找不到「收款帳戶歸誰」的討論〔量測：分支計畫檔搜尋 `payment 模組`、`internal/modules/payment` 都是 0 筆〕。
- ADR D2 的分開判準：「改變的原因不同（法規、外部 provider、演算法）」。收款帳戶的狀態模型（D13）、建立冪等（D19）、事件格式（E1）都跟著 provider 改；店家的規則（入駐審核、措施、曝光）不跟。
- 收款帳戶的消費者不只 seller：B2 的 destination charges 要目的帳戶（D20），B10 finance 的撥款要 `payouts_active`〔推論〕。

## P1：收款帳戶的擁有者

### 選項

- **A（建議）新建 payment 模組，現階段只有收款帳戶；seller 依賴 payment。**
  - payment 擁有：帳戶列（不透明的擁有者參照，現在只有店家；一個擁有者一列，D19 不變）、建立與同步 job、輪詢、inbox（依 P2）、port 與 Stripe／fake adapter、後台的讀帳戶、重新整理、重試建立。
  - seller 留下：平台審核、開通閘門（`store_requirement_flags` 改讀 seller 自己存的 `transfers_active` 副本）、曝光公告、fingerprint 防詐 hash（`kyc.Fingerprints`）。
  - 介面：
    - seller 核准時呼叫 `payment.RequestAccount(owner, contactEmail)`。聯絡信箱照現在的做法由 seller 傳入、存在 payment 的列上（D23 不變）。
    - payment 同步寫快取的 transaction 內發 `PayoutAccountChanged`（擁有者、`transfers_active`、`payouts_active`、狀態、fingerprint）。seller 以同 transaction 的訂閱者更新閘門副本、寫防詐 hash、排曝光公告。規則 4 允許訂閱者「把擁有者的事實複製進自己的表」。
  - 鎖順序的限制〔推論〕：payment 在 seller 之下，所以先鎖 payment 再鎖 seller。同步 job 照這個方向走（payment 列 → 事件 → seller 列）；反方向的只有核准。分支上的 `InsertPaymentAccount` 是 `ON CONFLICT (store_id) DO UPDATE`，重新核准時會鎖既有的帳戶列，在核准的 transaction 裡排在 seller 的列之後，和同步 job 的方向相反。改法二選一：seller 的 transaction 對 payment 只做不鎖既有列的動作（排唯一 job，或在申請表 INSERT 一列、不 upsert），帳戶列只由 payment 自己的 job 寫；或者讓核准先呼叫 payment 再鎖 seller 的列。這一點要在設計時用鎖順序檢查（review 第 4 點的原型）驗證。分支上兩者都在 seller 裡，有沒有同樣的模組內倒序，本檔沒有驗證。
  - 待辦的來源：`PAYMENT_ACCOUNT_CREATE_FAILED` 由 payment 發布 `pub_*`，`reviewqueue` 的依賴加上 payment。
  - 優點：B2、B10 直接擴充同一個模組；provider 的變動不進 seller；seller 回到「店家」的職責（review 第 1 點）。
  - 成本〔量測：分支上的行數〕：要搬的非測試程式約 1,900 行（`seller/internal/payment` 1,218、`seller/payment.go` 171、`model/payment.go` 90、`queries/payment.sql` 118、migration 129、`httpapi/seller/payment*.go` 199）；測試約 1,450 行跟著改（`payment_integration_test.go` 640、`stripe_test.go` 374、`payment_test.go` 154＋203、`payment_webhook_test.go` 83）。`platform/stripe` 不動。帳戶列失去對 `seller.stores` 的 FK（D3 本來就不建跨 schema FK）。
- **B 維持現狀，B2 另建付款的 port 與 inbox。**
  - ordering 向 seller 取目的帳戶 ID，傳給 payment；`/webhooks/stripe` 依事件種類分派給 seller 與 payment 兩個 inbox，或者分成兩個 Stripe event destination。
  - 成本：provider 的包裝、驗簽後的去重、補掃各有兩份；finance 要經 seller 讀 `payouts_active`；`seller/internal/payment` 這個名字會一直誤導。
- **C 維持現狀，到 B2 再搬。**
  - 搬的範圍和 A 一樣，但那時已經進了 `main`，`docs/modules/seller.md`、admin、website、seller-app 的引用都更多；和根目錄的「不留技術債」不合。

### 業界基準〔推測〕

市集型平台通常把金流商的 connected account 放在 payments 的邊界，店家側只存「可以收款」的事實。未逐條查證。

### 約束帳

- 外部呼叫不在 transaction 內（D5）：REAL（正確性）。A 照舊滿足。
- 一個擁有者一個外部帳戶、關店再開沿用（D19）：產品決定，A 照舊。
- fingerprint 在寫快取的同一個 transaction 寫成 hash（D12、分支的 `payment/AGENTS.md`）：REAL（防詐不漏）。A 由同 transaction 的訂閱者滿足。
- payment 是葉節點：SELF-IMPOSED（ADR D2），但它是正確的分層，A 沿用。
- 「收款開通屬於 seller」：SELF-IMPOSED（舊 seller-service 的切法）。

### 時機

在步驟 8 合進 `main` 之前做（D21）。建議排成步驟 9「獨立設計審查」之前的一個子步驟，或交給步驟 9 的審查一併評估。

## P2：webhook inbox 的機制

### 事實

- ADR D5：「外部進來的 webhook 一律先寫 `webhook_inbox`、先 commit，再交給負責的模組處理。這是 `platform` 提供的單一機制，模組不得自己實作。」〔量測：ADR `2026-09-24-go-modular-monolith.md` D5〕
- `internal/platform` 在 `dev` 與分支上都沒有 inbox 的套件〔量測：`ls internal/platform`〕。
- 現有的實作都是模組自己做的：seller 的 PayUni 用 `payuni_observations` 加 River 以 `event_key` 去重的 `ObserveArgs`〔量測：`seller/internal/payuni/jobs.go`〕；分支的 Stripe 用 `seller.payment_provider_events`（`(provider, event_id)` 主鍵）加補掃〔量測：分支 migration `20261102122552_payment_accounts.sql`〕。
- 接下來還有：B2 的付款事件（Stripe）、B3 的貨態（綠界），可能還有 A5（AMEGO）〔推論：主計畫 B2、B3〕。

### 選項

- **A（建議）由 platform 擁有 inbox。**
  - schema `inbox` 由 platform 的 migration 建立，前例是 River 的 `river` schema〔量測：`platform/jobs` 的 `Schema`、`Migrate`〕。
  - 表的欄位：`(provider, event_id)` 唯一、收到時間、處理時間、處理者、原始 payload（依 provider 設保存期）。
  - 入口驗簽後，在同一個 transaction 寫 inbox，並以 `InsertTx` 排一個派送 job，commit 之後才回 2xx。
  - 派送 job 依 provider 與事件種類，呼叫組裝點登記的模組 handler。handler 在自己的 transaction 寫業務列，同一個 transaction 經 platform 的 API 標記已處理。
  - 補掃與 payload 清理是 platform 的週期 job。
  - 優點：符合 D5 的字面，一張表看全部 provider 的積壓；分支在 `payment/AGENTS.md` 寫的「inbox 是不漏的來源、補掃超過 1 分鐘未處理」，變成每個 provider 都有的保證。
  - 成本：platform 第一次擁有 River 以外的表；收件的業務語意（例如「重讀帳戶」）仍在模組。
- **B platform 提供套件，表由各模組依同一個形狀自建，arch test 檢查形狀。**
  - 優點：資料留在擁有者，保存期與清理跟著模組。
  - 成本：每個模組一份表；platform 的程式要對模組的表下 SQL，規則 2 與 `querycheck` 需要例外。
- **C 各模組自己做（現況）。** 違反 D5 的字面，而且每個 provider 都要重新實作一次補掃。

### D5 另一半的字句（intent 列）

D5 也說 intent 列由 platform 提供，但 intent 列其實是模組的業務列（PayUni 的 `intent_seq`／`synced_seq`、分支帳戶列的「建立中」與 `create_idempotency_key`），兩處都已經是模組自己的做法〔量測〕。建議把 D5 改成：platform 提供 inbox；intent 列是模組的業務列，固定的形狀是唯一 job、有 deadline 的外部呼叫、在新的 transaction 寫結果。

### 時機

B2 之前一定要有；分支的 Stripe inbox 可以在 P1 搬遷時一起換成 platform 的。

## 2. 採 P1-A、P2-A 時要改的地方

- `internal/arch/modules.go`：seller 加 payment；reviewqueue 加 payment；`InternalDependencies` 加 payment 的內部圖。`cmd/hawkr/modules.go` 登記 payment 的 migration。
- ADR `2026-09-24-go-modular-monolith.md`：加修訂段（payment 列先只有收款帳戶、seller 依賴 payment、D5 的字句）；ADR `2026-10-19-seller-kyc-payuni-consolidated.md`：收款帳戶的擁有者改成 payment。
- Stripe 計畫：步驟 9 之前加一個搬遷子步驟；B2 的「payment port」改指 payment 模組的公開 API。
- 文件：`docs/modules/seller.md` 的收款開通章節移到新的 `docs/modules/payment.md`；seller 與 payment 的 `AGENTS.md`；`backend/AGENTS.md` 規則 4 的全域順序加上 payment（在 identity 之後、buyer 之前）。
- 本機種子：`cmd/hawkr/dev_seed.go` 改由 payment 的 fake provider 寫「可以收款」的帳戶，再經事件更新 seller 的副本。

## 3. 本檔沒有驗證的

- 沒有在分支上跑測試。P1-A 的鎖順序限制是〔推論〕；分支上 seller 模組內的核准與同步有沒有倒序，沒有逐條核對。
- Stripe event destination 能不能在同一個端點混送 thin 與 snapshot 事件，沒有查文件；P2 的選項不依賴這一點。
- 搬遷的行數是分支 `f834407` 的檔案行數，不含生成的 `store/payment.sql.go`（478 行，由 sqlc 重新生成）。
