# `account:blocked:<id>` 的寫入責任在 use case，不在呼叫端

| 項目 | 值 |
|---|---|
| 狀態 | superseded by `2026-09-27-identity-opaque-sessions.md`（單體沒有 `account:blocked:*`：停權在同一個 transaction 內刪掉帳號的 session，解析 session 時也檢查帳號狀態）。遷移計畫 A2 落地前，現行程式仍依本檔運作 |
| 日期 | 2026-09-18 |
| 影響範圍 | account-service（`SuspendAccountUseCase`）、social-service（moderation dispatcher）、gateway |

## 脈絡

`account:blocked:<id>` 是 `accounts.status` 的**衍生快取**，不是第二份狀態：
gateway 在 Redis 答不出來時會自己從 `accounts.status` 重新推導同一個判決。

原本的分工是 `SuspendAccountUseCase` 寫事實（`UpdateStatus → StatusSuspended`），
而 social-service 的 moderation dispatcher 在 RPC 回來之後才寫衍生值。
admin 會員管理新增停權入口時，這個分工第一次出現第二個呼叫端。

規格本身見 `docs/詳細設計書/17-登入與帳號建立流程（Login & Account）詳細設計書.md`
§3「帳號停權與解除停權」。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 維持「立即生效層由呼叫端負責」 | 那是一份**沒有任何機制強制**的每呼叫端義務。新呼叫端（admin 停權入口）會漏寫，而且它根本不該碰 `account:blocked:*`——那是 account-service 的 key namespace | 否決 |
| 兩邊都寫 | 留著第二個 writer 等於保留要消除的漂移來源 | 否決 |
| **收斂進 `SuspendAccountUseCase`，dispatcher 不再寫** | 推導值與被推導的事實寫在同一處 | **採用** |

維持原分工的實際風險：dispatcher 的 `SetAccountBlocked` 若失敗，`SuspendAccount` RPC 早已
commit——SSOT 說 suspended、快取說乾淨。而穩態下快取贏（`AccountNotBlocked` 是定論，
只有 `AccountBlockUnknown` 才回頭問 SSOT），停權對該帳號手上的 mobile JWT 完全不生效，
直到它自己過期。

## 被推翻的舊斷言

`suspend_account_test.go` 曾有一條刻意的反向斷言「Suspend must not write `account:blocked`
— that layer belongs to the moderation dispatcher」，理由是「悄悄在這裡取得這一層，會改變
每個呼叫端（含 mobile JWT surface）的語意」。

把呼叫端集合枚舉出來之後這個理由不成立：`SuspendAccount` 在生產只有一個呼叫端
（social-service dispatcher），而它在 RPC 之後用**更長**的 TTL 寫同一把 key，`SET` 覆蓋，
終態逐位元組相同。唯一的行為差落在「dispatcher 那次寫入失敗」的窗口：帳號從「完全沒被擋」
變成「被擋滿覆蓋期」——嚴格更正確，不是語意變更。舊註解真正要防的是「悄悄地做」。

## 後果

- 停權的兩個寫入（SSOT ＋ 快取）同屬一個 use case，可以一起 fail-closed。
- dispatcher 只保留 principal 解析與 `SuspendAccount` 呼叫。
- 重新評估條件：若未來出現「不經 account-service 就要封鎖帳號」的需求，這個收斂會擋路，
  屆時要先問那個需求是不是應該改走 account-service。

## 修訂（2026-10-28）

設計書 17（登入與帳號建立流程）已刪除，登入、帳號與 session 見 `docs/modules/identity.md`，買家建立見 `docs/modules/buyer.md`。本文中「設計書 17」與它的章節編號指向已刪除的文件，內容見 git 歷史。
