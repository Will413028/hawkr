# 代行整個成為 delegation 模組：授權、session、稽核與編排不再拆在 identity

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-27 |
| 影響範圍 | 單體的 identity 與 delegation 模組、`internal/arch` 的依賴表、`internal/legacy/delegationservice`；設計書 56 |
| 取代 | `2026-09-24-go-modular-monolith.md` 的「delegation 整包留在 identity」與其「受影響的既有 ADR」中對 `2026-09-18-adm-delegation-service-ownership.md` §1 的沿用 |

## 脈絡

遷移計畫把代行拆成兩半：資料（授權、Operation Session、稽核三張表）在 A2 搬進 identity，編排（店家狀態檢查、對被代行模組的寫入）留給 A10 的 `delegation` 模組。identity 搬完之後，6,285 行裡約 2,400 行是代行；編排之後要讀寫這些資料，都得經過 identity 的公開 API（約 19 個代行專用的方法）。

把資料留在 identity 的理由（ownership ADR §1、主計畫 ADR）逐項對照現在的約束：

- (a) 要對管理員表下 FK：單體不建跨 schema FK（主計畫 ADR D3），所有模組引用帳號都只存 ID；代行沒有特別之處。不成立。
- (b) 管理員登出與代行撤銷要原子：`events.Publish` 在 transaction 內同步執行訂閱者，identity 發事件、代行在同一個 transaction 撤銷即可。以事件達成，不需要同一個模組。
- (c) 新服務的固定成本：模組不是服務，主計畫 ADR 已註明不再存在。

約束：

- 繼承：模組之間不得形成依賴環，只能 import 對方的根套件（主計畫 ADR D2）；仍成立。
- 繼承：設計書 56 的不變量（授權、session 的狀態機、稽核的冪等鍵）；仍成立，程式搬移不改語意。
- 外部：系統還沒上線，三張表沒有正式資料，可以直接在新 schema 重建。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：一個 bounded context 一個模組；身分（你是誰）與代表他人操作的授權（impersonation、support access）分成不同的 context，後者依賴前者 | 新 schema 與搬表；identity 改發「管理員登出、移除」事件；legacy 的 DelegationService 轉接改指向新模組 | 採用 |
| 維持：資料在 identity，編排在 delegation | 同一個概念拆在兩個模組；identity 為代行多開約 19 個方法，職責變成認證加授權 | 否決 |
| 資料與編排都放 identity | 編排要依賴 seller、ordering 等模組，identity 是最底層，依賴圖出現環（主計畫 ADR 已否決） | 否決 |

## 決策

代行的授權、Operation Session、稽核與編排整個是 `delegation` 模組（schema `delegation`），依賴 identity、seller 與被代行的模組；identity 不依賴它。identity 在管理員登出、被移除時於 transaction 內發事件，delegation 訂閱並在同一個 transaction 撤銷該管理員進行中的代行。對管理員與店家只存 ID。

## 後果

- 搬移與 0.12（ADR `2026-09-27-module-internal-structure.md`）一起做：代行的程式本來要搬進 identity 的 `internal/delegation`，改成搬進新模組的 `internal/`；排在 A2 的 C10 之後。
- identity 的 `IdentityAdminService.SignOut` 改成「發事件」，撤銷由 delegation 做；`internal/legacy/delegationservice` 改接 delegation 的公開 API；依賴表 `delegation` 的說明改為整個代行。
- A10 的編排直接加進這個模組。
- 重新評估的條件：代行的資料與身分資料出現必須同一個 transaction、而事件無法表達的寫入。

## 修訂：資格歸屬、登出與憑證核發（2026-09-28）

**脈絡**：原決策 (b) 假設 identity 的管理員登出有狀態寫入，須與撤銷代行保持原子。C10 改用 Cloudflare Access 後，登出由 Access 處理，identity 沒有登出寫入路徑；只發 `AdminSignedOut` 的 transaction 實際是在轉送 delegation 命令。identity 目前也沒有移除管理員的路徑。

**決策**：

- 代行資格是 delegation 的授權事實，由 `delegation.admin_eligibilities` 保存管理員 ID，不再是 `identity.admins.delegation_enabled`。撤回資格與撤銷該管理員進行中的代行在同一個 DB transaction；建立 session 時鎖住資格列，避免與撤回交錯後留下 active session。identity 的 `Admin` 不暴露代行資格。
- 後台登出的 HTTP handler 直接呼叫 delegation 撤銷，不再經 identity 的 `SignOutAdmin`／`AdminSignedOut`。`IdentityAdminService.SignOut` 仍留在原 Connect service：對外動作是管理員登出，撤銷代行是它的直接效果，沒有必要把登入與登出拆到不同 service。只有 identity 日後出現管理員移除路徑、且它自己的狀態變更需要通知 delegation 時，才重新引入事件。
- 建立只提供 session 及 handoff code；ADM BFF 以管理員身分按 session ID 讀取完整摘要並對帳，ADM host 不接收 operation credential。兌換時以 `crypto/rand` 產生至少 256 bit 的 operation credential，資料庫只存 SHA-256，不按 session ID 以 HMAC 推導；單句 CAS 清除 code hash 並寫入 credential hash，且只接受尚無 credential 的 session。重放同一 code 依 9282 失效語意處理；同鍵重新建立在兌換前改簽 code，兌換後只回既有 session 摘要，不再提供 code 或改寫 credential hash。
