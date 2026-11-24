# 規則 4 鎖順序 vs media：設計簡報（v2，已依審查改寫）

## 結論先講
建議 **F＋G（擁有者 transaction 對 media 只做 append，不鎖 media 既有的列）**：`MarkDeleted` 改成 INSERT 刪除請求列、由 media 的 job 套用；`Confirm` 改成 INSERT 確認列（唯一鍵＝upload_id）宣告 token，資產狀態由 media 自己推進。這樣擁有者 transaction 裡 media 不再出現在鎖圖上，任何呼叫順序（之前、之後、夾在 Publish 前後）都不產生邊，規則 4 不必改、不必加呼叫方規則、不必豁免。v1 推薦的 A（media 當 leaf 排最後）**是錯的**：現有程式已有「先 MarkDeleted 再 Publish、訂閱者鎖別的模組」的路徑，A 會把它們判成違規（見下）。（推論，依下列程式證據；F＋G 未實作、未量測。）

## 前提問題：這段程式是否排定重寫、搬移或刪除？
- **否。** `platform/lockorder` 與 media 是目標架構。擁有者會增加：目前 19 個檔、26 處呼叫（`rg -n "\.(Confirm|ConfirmField|ConfirmFields|MarkDeleted|CopyPublic)\(" internal/modules -g '!*_test.go' -g '!**/media/**' | wc -l` → 26），B2–B10 還會加。
- 所以「改呼叫方」類方案（A'、B）的成本隨擁有者數成長；「改 media 內部」類（F、G）只做一次。沒有 REAL 的「之後會丟掉」排程。

## 現況證據：v1 漏掉的兩件事
1. **MarkDeleted 之後同 transaction 還 Publish，訂閱者鎖別的模組**
   - `seller/internal/purge/purge.go:170` MarkDeleted → `:174` Publish `StoreRetired` → 同 transaction 訂閱者 moderation `OnStoreRetired`（`moderation/moderation.go:214`，接線 `cmd/hawkr/moderation.go:37`），另有 im、social（`cmd/hawkr/im.go:24`、`social.go:96`）。
   - `buyer/internal/deletion/deletion.go:136` 先呼叫 `retireAvatar`（`:146` MarkDeleted）→ `:133` Publish `BuyerDeleted` → moderation `:219`、im、social、review。
   - 所以順序是「擁有者列 → media 列 → moderation 列」。A 把 media 排在所有模組之後，這兩條路徑就是 media→moderation 的倒退。**「擁有者路徑 0 改動」不成立。**
2. **moderation 自己在鎖 moderation 列之後呼叫 media**：`report.go:193` CopyPublic、`:220` Confirm（建快照的 transaction 內）；`retention.go:124` MarkDeleted（刪快照後）。
   - 模組層級因此有環：moderation → media（report/retention）且 media → moderation（seller purge、buyer deletion 經事件）。v1 的「沒有環」是錯的。
   - 列層級：要真死鎖，需同時有 (a) 店家退場 transaction 持有該店的資產列、等 moderation 該店當檢舉人的列，和 (b) moderation transaction 持有那些檢舉列、等**同一批**資產列。moderation 只碰自己 Confirm 的複本資產（B1-D20），店家退場只 MarkDeleted 店家 logo/cover，兩者資產集合不相交 → 目前**推論**不會撞同一列；未量測、無測試覆蓋。檢查器以模組（schema）為單位，仍會判違規，這是正確的保守行為。
- Confirm 後同 transaction Publish 的路徑（`rg` 逐檔比對行號）：social post `:132→176`、`:463→487`（`PostPublished`）、im send `:84→96`（`MessageSent`）、seller profile `:460→513`（`StoreProfileChanged`）、review write `:142→158`、seller devseed `:474→491`；review purge `:170→174` 是 MarkDeleted→Publish。目前這些事件的正式訂閱者只有 relay（只 Enqueue/INSERT）或沒有（`rg -n "events.Subscribe\(" cmd/hawkr`），所以今天不產生跨模組鎖邊；但任何人加一個會鎖列的訂閱者就會出現。

## 業界基準（精確說法）
- **Lock hierarchy**（Sutter "Use Lock Hierarchies"、Linux lockdep）：全域一個偏序，持有高層鎖只能再取低層鎖。**同步呼叫**時被呼叫方的鎖在後（呼叫方高、被呼叫方低）；**向上的事件通知**（本 repo ADR D4(c) 的 identity→buyer→seller）是下層先鎖、上層後鎖。兩種方向在同一 transaction 混用時，模組無法排成單一層級——同一個模組（moderation）既是 media 的同步呼叫方、又是經 media 之後才通知到的訂閱者。只能 (i) 限制呼叫位置，或 (ii) 讓其中一方不取鎖。
- **PostgreSQL**（§13.3.4）：只要求一致順序；**單純 INSERT 不鎖既有列**，只在唯一鍵和未 commit 的 INSERT 撞到時等待。`internal/platform/lockorder/classify.go:21-26` 也這樣分類：只算 FOR …、UPDATE、DELETE、MERGE、`INSERT … ON CONFLICT DO UPDATE`；`ON CONFLICT DO NOTHING` 不算。
- **Rails Active Storage**：attach 在 record 的 transaction 內 INSERT `attachments` 列（不 UPDATE blob），`purge_later` 交給 job。**Stripe／Shopify 式 append-only ledger**：狀態變更寫成新列，由背景程序或讀取時推導，避免熱點列鎖。兩者都是 (ii)：被呼叫的共用服務在呼叫方 transaction 內只做 append。

## 現有機制為何存在
- ADR D4(c)（`docs/adr/2026-09-24-go-modular-monolith.md:340-351`）：當時跨模組寫入只有向上事件，由下往上＝實際等待順序；條文已寫「上層呼叫下層 API 要在鎖自己的列之前」（`lockorder.go:28-29`）。media 是第一個在擁有者 transaction 內被同步呼叫寫入的模組，條文沒預見它和事件混用。

## 選項
| 選項 | 內容 | 失效條件 |
|---|---|---|
| A | media 當 leaf 排最後，只改順序表 | **已失效**：seller purge、buyer deletion 先 MarkDeleted 再 Publish，訂閱者 moderation 等在 media 之後 |
| A' | A＋呼叫方規則「media 呼叫一律在 Publish 之後」 | moderation 自己（report、retention）是 media 呼叫方又是訂閱者，環仍在；要靠呼叫方紀律，26 處逐一改並守住；只能用執行期檢查器在整合測試抓，沒跑到的路徑漏掉 |
| B | 擁有者在鎖自己列之前先 Confirm/MarkDeleted | MarkDeleted 的資產 ID 常要先讀（鎖）擁有者列才知道（purge 讀 `row.LogoAssetID`）；Confirm 前沒有版本檢查會先消耗 token。多數路徑做不到 |
| F | MarkDeleted 改 INSERT `media.asset_deletions`，job 套用；Confirm 不變 | Confirm 仍是 `UPDATE media.staged_uploads`（`uploads.sql.go:428-433`）＋`ActivateAsset`，Confirm→Publish 的 6 條路徑在訂閱者加鎖時仍成邊；moderation report 的環仍在。**單獨不夠** |
| G | Confirm 改 INSERT `media.upload_claims(upload_id PK, asset_id, claimed_xid)` `ON CONFLICT DO NOTHING`，資產啟用由 media 推導或 job 推進 | GC 回收與 Confirm 的仲裁改到同一唯一鍵（GC 也 INSERT claim，輸的一方沒有列）；`ActivateAsset` 的 UPDATE 必須移出擁有者 transaction，讀取端改以 claim 存在判斷 ACTIVE。若某讀取路徑必須在同 transaction 看到 `assets.state='ACTIVE'`，G 失效 |
| **F＋G** | 兩者合併 | 見 G；另需 job 套用刪除的延遲可接受（擁有者已不引用，公開讀取靠擁有者參照，推論可接受） |

### G 如何保住 B1-D23（只在同 transaction 內冪等、跨 transaction 回 FailedPrecondition）
- `INSERT … ON CONFLICT (upload_id) DO NOTHING RETURNING asset_id`：有列＝本次宣告成功。無列時讀既有 claim，`claimed_xid = pg_current_xact_id()` → 同 transaction 重入，回同一資產；否則 → FailedPrecondition。
- 兩個 transaction 同時宣告同一 token：後者在唯一索引上等前者 commit（PostgreSQL 的 INSERT 語意），前者 commit 後後者拿到衝突 → FailedPrecondition；前者 rollback 後後者成功。這是唯一的等待，只發生在兩個擁有者搶同一 token，而 D23 本來就判其中一方失敗；它不是對既有列的鎖，classify.go 不算（`:22`）。（推論，PostgreSQL 文件語意；未寫測試。）
- 過期檢查 `expires_at > now` 改成 INSERT … SELECT 帶條件；GC 先 INSERT 同一 claim（type=RECLAIMED）再刪，兩方由唯一鍵互斥，不再靠 `FOR UPDATE SKIP LOCKED` 搶 staged_uploads。

## 推翻建議的事實
- 存在必須在擁有者 transaction 內同步看到 `assets.state` 變化的讀取（如同 transaction 內 `ReadSecure` 剛確認的資產）。
- 刪除請求延遲套用違反某個合規要求（例如 KYC 文件須同步失效）。
- 量測顯示 claim 唯一索引的等待在實際流量下成為瓶頸（目前沒有量測）。

## 可逆性與成本
- F＋G：改 media 內部（新 2 表、Confirm/MarkDeleted/GC/讀取推導、migration）；擁有者 API 簽章不變，26 處呼叫不動（推論，介面不需改）。反悔＝回到 UPDATE，同樣只改 media。中等成本、一次性。
- A'：每個現有與未來擁有者都要守呼叫位置，越晚越貴；且 moderation 環無解。
- B：多數路徑不可行。

## 限制帳本
| 限制 | 判定 | 來源 |
|---|---|---|
| Confirm/MarkDeleted 在擁有者 transaction 內呼叫 | REAL（已決 Will 2026-11-21） | plan B1-D1、B1-D9 |
| Confirm 只在同 transaction 冪等 | REAL（已決 2026-11-23） | plan B1-D23 :277 |
| 不用豁免清單 | REAL（Will 2026-11-06） | ADR 單體 :353 |
| moderation 快照時複製並 Confirm（B1-D20） | REAL（已決） | plan B1-D20 |
| Confirm 必須以 UPDATE staged_uploads 實作 | SELF-IMPOSED（現有實作） | `uploads.sql.go:428` |
| MarkDeleted 必須同步改 assets.state | SELF-IMPOSED（D9 只要求「在擁有者 transaction 內標記」，INSERT 請求列也是標記） | `assets.sql.go:225`、plan B1-D9 |
| 鎖順序由依賴表推導 | SELF-IMPOSED（自己的 arch 測試） | `internal/arch/lockorder_test.go:10-18` |
| 改動量、和現有 26 處寫法一致 | SELF-IMPOSED | 上方 rg |
- 需確認：B1-D9 原文是否要求「同步」狀態轉移；若是，F 要 Will 修訂 D9。

## 建議
- 採 **F＋G**：擁有者 transaction 對 media 只 INSERT；所有改既有列的狀態轉移（ACTIVE、DELETED、GC 回收）在 media 自己的 transaction（job）裡做，media 的 transaction 照舊不碰他人列。
- 規則 4 與檢查器不改、`Modules` 順序不改、無豁免；media 不需 leaf 特例。
- **強制方式**（回應「只檢查 media 自身擋不住呼叫方」）：(1) 靜態：arch 測試用 `lockorder` 的 classify 掃 media 對擁有者公開方法（Confirm、ConfirmField(s)、MarkDeleted、CopyPublic）所執行的 SQL，任何 lock kind 即失敗——這條保證與呼叫方位置無關，所以不必檢查呼叫方。(2) 執行期：既有 lockorder 檢查器在整合測試中照常跑，覆蓋 moderation report/retention、seller purge、buyer deletion 這四條已知混序路徑當回歸。
- 落地前在本機 compose 不跑；寫整合測試（CI）證明：兩 transaction 搶同 token 一方 FailedPrecondition、同 transaction 重入回同資產、GC 與 Confirm 互斥。

## 終局與觸發
- 終局：media 在任何他人 transaction 內只 append；規則 4 維持依賴表由下往上一條規則；arch 測試保證 media 對外寫入方法無 lock kind。
- 重新評估觸發：出現第二個「被同步呼叫寫入」的模組（如 B 系列的 payment、inventory）——同樣要求它走 append-only；或 G 的推翻事實之一成立。
