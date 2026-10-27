# 刪除帳號是 accountdeletion 模組擁有的流程，不放在傳輸層

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-30 |
| 影響範圍 | 單體新增 `accountdeletion` 模組、`internal/arch` 的依賴表、buyer 模組的刪除 API 與 `deletion_audits`、`internal/httpapi/identity` 的 `DeleteAccount`、遷移計畫 A3 步驟 4 的 `order.completed` consumer 與步驟 8 的後台刪帳號 |
| 取代 | 遷移計畫 `2026-09-29-a3-buyer-seller.md` 決定 4a「刪帳號是 `internal/httpapi` 的一個 use case」的放置；`2026-09-24-go-modular-monolith.md` D2「反方向的需要」第 3 點中「buyer／seller 刪除帳號需要確認沒有未結訂單，由 ordering 實作」的 port 擁有者 |

## 脈絡

刪除帳號要先確認沒有未完成的訂單，有的話等最後一筆結束再刪；刪除時在同一個 transaction 內關閉買家（A3b 起還有賣家）並刪除帳號。App Store 審核準則 5.1.1(v) 要求 App 內能刪帳號。

這個流程有三個入口：App 的 `IdentityAppService.DeleteAccount`、後台刪帳號（A3 步驟 8）、訂單結束時完成等待中的刪除（步驟 4 的 `order.completed` consumer，在 `internal/legacy`）。決定 4a 把它放在 `internal/httpapi`；步驟 3 實作後，第二個入口只能經 `cmd` 注入 httpapi 的型別，而 `backend/AGENTS.md` 規定 httpapi 只做傳輸與讀取組裝，兩份唯一來源互相牴觸（步驟 3 的設計審查）。

約束：

- 繼承：模組只能依賴依賴表允許的模組，不得成環（主計畫 ADR D2）。buyer、seller 依賴 identity，所以 identity 不能呼叫它們。
- 繼承：跨模組共用 transaction（主計畫 ADR D4）；外部呼叫不在 transaction 內（ordering 在 Java 期間，未結訂單是遠端查詢）。
- 外部：系統還沒上線，`buyer.deletion_audits` 沒有正式資料。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：流程管理者（Hohpe & Woolf《Enterprise Integration Patterns》的 Process Manager）擁有長時間流程的狀態，依賴參與者、以 port 查詢前置條件；repo 內的先例是 delegation 模組（`2026-09-27-delegation-module.md`） | 依賴表多一個模組；刪除稽核從 buyer 移過去 | 採用 |
| 維持在 `internal/httpapi`（決定 4a） | 第二、三個入口要經 `cmd` 注入傳輸層的型別；httpapi 的職責變成可以承載跨模組寫入 | 否決 |
| identity 當編排者，buyer、seller 以依賴反轉註冊關閉的 hook | identity 會知道訂單這類跨領域規則；流程藏在註冊的 hook 裡，正是計畫要從 identity 拿掉的 `RoleState` 那一類耦合 | 否決 |
| 事件編舞：identity 發「申請刪除」，各模組各自關閉 | 「等訂單結束」與「全部參與者關閉後才刪帳號」需要一個地方彙總狀態，最後仍要有流程管理者；流程分散，難以追查 | 否決 |

## 決策

- `accountdeletion` 模組（schema `accountdeletion`）擁有「刪除帳號的申請」：申請者（本人或管理員）、原因、申請與完成時間、狀態（等待訂單結束、已完成）。每個帳號同時最多一筆未完成的申請。
- 依賴 identity、buyer；A3b 加上 seller。沒有模組依賴它。
- 未結訂單由它定義的 port 查詢：ordering 在 Java 期間由 `internal/legacy` 實作，在 transaction 之前查；ordering 搬進單體後改由 ordering 的公開 API 實作，並移進同一個 transaction。
- buyer（與之後的 seller）只提供自己的動作：進入等待刪除（期間不能新增收藏、追蹤）、關閉。刪除的稽核屬於帳號層級，由 `accountdeletion` 保存；buyer 不再有 `deletion_audits`。
- 三個入口都呼叫它的公開 API；`internal/httpapi` 只做傳輸。

## 後果

- 主計畫 ADR D2 的依賴表加上 `accountdeletion`；「反方向的需要」第 3 點的刪除帳號改成由本模組定義 port。
- 步驟 4 的 consumer 呼叫本模組完成等待中的申請；步驟 8 的後台刪帳號是本模組以管理員為申請者的同一個流程，seller 成為參與者。
- 關閉買家只有本模組這一條路徑：在同一個 transaction 內先以 `identity.LockAccount` 鎖帳號列、再呼叫 buyer 的 `CloseForDeletion`、再刪帳號（2026-10-01 修訂）。

## 修訂：關閉買家只由本模組編排（2026-10-01，W8）

**脈絡**：原後果寫「identity 發出 `AccountDeleted` 時，buyer 仍以自己的訂閱關閉買家」，預設有一條不經申請、從 Clerk 那一側刪帳的路徑。實際上 identity 的 `DeleteAccount` 在正式程式裡只有本模組呼叫；buyer 的訂閱成了第二條關閉路徑，`CloseForDeletion` 也另外自己鎖帳號列（W5 之前移除 BUYER 角色的遺留）。兩條路徑會互相掩蓋：本模組漏掉關閉買家時，訂閱會默默補上。

**決策**：刪除帳號與關閉買家都只經本模組（process manager 明確編排，不用事件編舞）。buyer 刪除 `AccountDeleted` 訂閱與自己的帳號列鎖；它的 `BeginDeletion`／`CloseForDeletion` 只在本模組已鎖住帳號列的 transaction 內呼叫，鎖順序仍是帳號列先於買家列。新的刪帳入口（A3b 步驟 8 的後台）一樣呼叫本模組的公開 API。

**後果**：刪帳測試同時檢查帳號已刪與買家已關，拿掉 `CloseForDeletion` 的呼叫就失敗。本模組自己對 `AccountDeleted` 的訂閱（帳號在申請之外被刪時收斂 pending 申請）一併刪除：identity 的 `DeleteAccount` 只由本模組呼叫，沒有申請之外的刪帳路徑；A3b 步驟 8 的後台刪帳也是以管理員為申請者走本模組。

## 修訂：等待刪除是 identity 保存的帳號狀態（2026-10-01，Will 決定）

**脈絡**：申請進入等待未結訂單之後，帳號仍可登入，也仍可以開店（授予 SELLER）。等待期間開的店不在申請裡，完成申請時就卡住（A3b 7a 的審查 CR-F1）；同樣的缺口也出在之後任何會授予角色的入口。要擋下它，「這個帳號正在刪除」必須是授予角色的那一方在同一把帳號列鎖下看得到的事實。

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：生命週期狀態由擁有實體的模組保存（帳號 ACTIVE → PENDING_DELETION → DELETED 在 identity），流程管理者只改變它；所有寫入在同一把帳號列鎖下檢查狀態（狀態機屬於實體、流程屬於 process manager，Hohpe & Woolf 的 Process Manager） | identity 多一個狀態欄與一支「進入等待刪除」；accountdeletion 在受理會等待的申請時呼叫它 | 採用 |
| identity 當編排者，參與者以 hook 註冊 | 本 ADR 的原決策已否決：identity 會知道訂單這類跨領域規則，流程藏在 hook 裡 | 否決 |
| 每個授予角色的入口自己查 accountdeletion 有沒有等待中的申請 | 依賴方向反了（seller、identity 不能依賴 accountdeletion）；漏一個入口就漏擋 | 否決 |

**決策**：identity 的帳號有 `deletion_pending_at`，生命週期是 ACTIVE → PENDING_DELETION → DELETED（`identity.Account.Lifecycle`）。accountdeletion 在受理會等待未結訂單的申請的 transaction 內呼叫 `identity.BeginDeletion`；identity 的 `GrantRole` 在 PENDING_DELETION 時回 FailedPrecondition，`DeleteAccount` 清掉這個狀態。流程（等待、完成、對帳）仍由本模組編排，identity 只保存狀態、不知道訂單。目前沒有撤回申請的路徑，所以沒有「回到 ACTIVE」的轉移；之後加上撤回時，由本模組在同一個 transaction 清掉它。

**後果**：開店（seller 的 Apply）已經在同一個 transaction 內鎖帳號再授予 SELLER，所以等待刪除的帳號開店會整筆回滾；等待期間才開的店不會出現。`TestStoreCannotOpenWhileAccountPendingDeletion`（accountdeletion）、`TestDeletionPendingRefusesGrantRole`（identity）驗證。
