# 建立帳號時同步建立買家身分

| 項目 | 值 |
|---|---|
| 狀態 | accepted（2026-09-30、2026-10-01 修訂，見文末；部分取代見下列） |
| 日期 | 2026-09-30 |
| 影響範圍 | identity、buyer、accountdeletion、HTTP 入口、gateway proxy 與 `api/` 的 identity 契約 |
| 部分取代 | W5 修訂的 `Account.perspective` 由 `2026-10-07-connect-without-perspective.md` 取代 |

## 脈絡

入口按 RPC 路徑或角色 hint 補建買家 profile，會讓同一個帳號的身分取決於第一個請求；刪除後的在途請求也可能重建買家。系統尚未上線，沒有舊版 client 或正式帳號資料要相容。identity 不能依賴 buyer；跨模組原子變更使用同 transaction 的 in-process event（主計畫 ADR D4）。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：建帳就是買家，identity 發 `AccountCreated`，buyer 在同交易建立 profile 並授予 BUYER | 僅當賣家的帳號也有買家資料；建立帳號時多一項原子寫入 | 採用 |
| 登入時 JIT 建立 | 首次登入必須判斷與重試；與刪除競態 | 否決 |
| 明確加入買家 RPC | client 需多一步；未加入前帳號不能使用一般買家功能 | 否決 |
| 每請求按路徑或 hint 隱式建立 | 入口含業務判斷；容易誤建，且請求增加讀寫 | 否決 |

## 決策

identity 只發布 `AccountCreated`；buyer 訂閱，在建立帳號的 transaction 內建立帳號唯一的買家，不授予角色：「是買家」由「帳號活著」推導，identity 的角色只剩要另外申請的 SELLER。

帳號活著 ⇔ 買家活著。買家只由 `accountdeletion` 在刪除帳號的 transaction 內關閉（先鎖帳號列、再關閉買家、同交易刪帳號），buyer 不訂閱 `AccountDeleted`，沒有第二條關閉路徑。

buyer 的 Connect handler 與轉給 gateway 的請求在需要時唯讀解析現任 buyer_id，不補建。管理員或未登入呼叫買家的 App 方法回 `permission_denied`；存活的帳號查不到現任買家是不變量被破壞，回 `internal` 並記 error log。舊系統需要的買家位元（`account_type`、`X-Account-Type`、`X-Login-Role`）由 `internal/legacy/accounttype` 在邊界合成。

## 後果

- 本機既有帳號不會收到歷史事件。系統尚未上線，開發環境重建本機資料庫，再以 `./dev.sh seed` 匯入測試帳號（`infra/seed-data/01-account.sql` 為每個帳號建立買家，buyer_id 與 account_id 刻意不同）；W5 的 migration `buyer_is_not_a_role` 刪掉既有的 BUYER 列；不為可丟棄的本機資料加入正式 migration。若將來已有需保存的帳號資料，另以一次性資料回填處理，不用請求時 JIT。只當賣家的帳號同樣持有買家 profile。
- **不變量：帳號活著 ⇔ 買家活著**（W5 之後就是「存活的帳號有現任買家」，沒有另外的角色）。Will 2026-09-30 決定後台不提供「只刪買家」，只能刪賣家角色或整個帳號（遷移計畫 A3b 步驟 8），所以買家只會隨帳號刪除、由 `accountdeletion` 關閉，不會有「帳號還在、買家已關」或「帳號在、沒有買家」的狀態。
- 一個帳號一生只有一個買家：刪除的帳號留墓碑，同一個 Clerk 使用者不會再建帳號，所以不會重新成為買家。buyer 以 `UNIQUE (account_id)` 保證，建帳事件內單純建立；重送或重建違反約束時回錯誤讓建帳回滾，不重讀、不補建。登入時的衝突重讀、CLOSED 重試與相應的測試已刪除。
- 業務身分只在需要的地方解析：buyer 的 Connect handler 與轉給 gateway 的請求各查一次 buyer_id（每個帳號都查，store_id 只在帳號有 SELLER 時查），入口不查，其他請求（賣家的 Connect 方法、GetSession）不受 buyer 查詢失敗影響。管理員呼叫 buyer 的 App 方法回 `permission_denied`；存活的帳號查不到現任買家是不變量被破壞，回 internal 並記 error log，不當成「沒有買家」。
- identity 的 `source_id` 柵欄只留給 seller-service 對 SELLER 的非同步移除，A3b 步驟 11 拿掉。
- W5 的契約變更是破壞性的（`ROLE_BUYER` 與 `active_role` 刪除），上線前允許（根目錄 `AGENTS.md`「上線前不受相容性約束」）；刪除的號碼與名稱都 reserved，`buf breaking`（WIRE_JSON）不報錯。四個消費端在同一次變更內重新生成並跟著契約調整（website 的工作區一定有買家；後台會員頁不帶角色篩選就是全部買家）。

## 修訂：後台不再「只刪買家」（2026-09-30）

原決策寫「buyer 授予 BUYER；沒有現任 stint 的買家 RPC 回 `failed_precondition`」。Will 決定後台不提供「只刪買家」後：帳號活著 ⇔ 買家活著，「帳號在、沒有現任買家」不會發生。沒有 BUYER 的呼叫者回 `permission_denied`；有 BUYER 卻沒有現任買家是不變量被破壞，回 `internal`。入口不解析 buyer_id，改由 buyer handler 與 gateway 轉發在需要時解析（見後果）。

## 修訂：BUYER 不再存成角色（2026-10-01，W5）

既然帳號活著 ⇔ 買家活著，「是買家」由「帳號活著」推導，不另外存成角色；存成角色只多一個會與帳號狀態漂移的來源。identity 的角色只剩要另外申請的 SELLER，管理員是另一種 principal。buyer 建帳時只建立買家、不授予角色；last-role cascade、`RoleState`、BUYER 的來源柵欄與授予、移除 BUYER 的程式刪除，移除 SELLER 永遠不刪帳號。對外契約 `identity.proto` 把原本同時表示「擁有的角色」與「這次的視角」的 `Role` 拆開：`Account.roles` 只列 `ROLE_SELLER`（`ROLE_BUYER` 移除、號碼保留），視角改用 `Perspective { PERSPECTIVE_BUYER, PERSPECTIVE_SELLER }` 的 `Account.perspective`（取代 `active_role`）。舊系統需要的買家位元（`account_type`、`X-Account-Type`、`X-Login-Role`）由 `internal/legacy/accounttype` 在邊界合成（anti-corruption layer），不回頭污染 identity。

## 修訂：關閉買家只有一條路徑（2026-10-01，W8）

**脈絡**：買家原本有兩條關閉路徑：`accountdeletion` 在刪帳的 transaction 內呼叫 `CloseForDeletion`，以及 buyer 自己訂閱 identity 的 `AccountDeleted`；`CloseForDeletion` 另外自己鎖帳號列，是 W5 之前移除 BUYER 角色時留下的。identity 的 `DeleteAccount` 在正式程式裡只有 `accountdeletion` 呼叫（沒有 Clerk webhook 之類的第二個刪帳入口），第二條路徑只會掩蓋「刪了帳號卻沒關買家」的錯誤。

**決策**：關閉買家只由 `accountdeletion` 明確編排（process manager，與 `2026-09-30-account-deletion-module.md` 一致）。buyer 的 `AccountDeleted` 訂閱與自己的帳號列鎖刪除；`BeginDeletion`／`CloseForDeletion` 的前置條件是在 `accountdeletion` 已以 `identity.LockAccount` 鎖住帳號列的 transaction 內呼叫，鎖順序仍是帳號列先於買家列。

**後果**：`accountdeletion` 的刪帳測試（`TestRequestClosedDeletesAccountAndCompletes`、`TestCompletePendingChecksOrdersAgain`）與 Connect 的 `TestConnectDeleteAccount` 同時檢查帳號已刪、買家已關；拿掉 `CloseForDeletion` 的呼叫它們就失敗，不再被第二條路徑補上。之後新增刪帳入口（A3b 步驟 8 的後台）也必須經 `accountdeletion`。
