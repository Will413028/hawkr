# 代行營運：工作階段與稽核的服務歸屬、落庫形狀與保留期

| 項目 | 值 |
|---|---|
| 狀態 | accepted；§1 superseded by `2026-09-27-delegation-module.md`（代行整個成為 delegation 模組）；§2 superseded by `2026-09-24-go-modular-monolith.md`（D4：intent、寫入、稽核同一個 transaction）。遷移計畫 A2（2026-09-27）起：§1 的 Operation Session 與稽核落在單體的 identity 模組，舊的 DelegationService 由單體的 legacy listener 提供；§4 的 exchange 照舊，事件在確認的 transaction 內排入 River job 送出，不再經 outbox；§3、§5 仍有效 |
| 日期 | 2026-09-18（決策內容成形於 2026-08-09） |
| 影響範圍 | account-service、seller-service、Gateway、MQ topology |

## 1. Operation Session 與稽核落在哪個服務

**考慮過**：① 新建 `delegation-service` ② account-service ③ seller-service ④ Gateway 自持（Redis）

**決策：② account-service。** 三個決定性理由：

- **(a)** 稽核的 `actor_admin_id` 指向 `admin_accounts(id)`，而該表就在 account-service 同一個庫
  ——只有落在這裡才下得了 FK，其餘任一選項都只能靠應用層維持參照完整性。
- **(b)** 它已是 `web_sessions` ＋ `pkg/redis/session.go` 的持有者，「有時效、可撤銷的 session」
  是它既有的領域語彙。
- **(c)** 新建服務要付 Node ID／部署單元／NetworkPolicy／MQ topology／observability 五份固定成本，
  而本功能的資料量與 QPS 連一個服務的門檻都構不上。

**否決 ③**：seller-service 是「被代行的對象」，把「誰能代行它」放進去等於讓受管對象持有自己的授權紀錄。
**否決 ④**：Redis 沒有稽核所需的持久性與可查詢性，且硬上限依賴治理規範明令
「不能失敗的狀態不得放在無預算數字的依賴上」。

## 2. 稽核與領域寫入的一致性

**決策：兩階段 intent → 領域寫入 → confirm。** 詳細規格見 56 號 §8.4。
理由見 `2026-09-18-adm-delegation-authz-propagation.md`（跨服務時兩者不在同一個交易裡，
而「稽核寧可多一列 PENDING，不可少一列」是本案的不變式方向）。

## 3. session 狀態如何落庫

**考慮過**：① 三個持久狀態 `active`／`expired`／`revoked`，用排程器把過期的改成 `expired`
② 兩個持久狀態 `active`／`revoked`，`expired` 由 `expires_at` 推導

**決策：② 兩態 ＋ 推導。**

**否決 ①** 的理由是它會憑空生出一個排程器，而那個排程器的唯一工作是把資料改成
它本來就能被算出來的樣子——這正是後端服務責任邊界規範要擋的「資料完整性監測」型基礎設施。
② 另外讓「結束代行」有了乾淨語意：結束 ＝ 立刻到期
（`SET ended_at = now(), expires_at = LEAST(expires_at, now())`），而不是第三種狀態轉換。
代價是每次讀取都要跑一次推導（三行判斷），這比一個排程器便宜得多。

## 4. 代行事件走哪個 exchange

**考慮過**：① 塞進既有 `hawkr.governance` ② 新增 `hawkr.delegation`
③ 沿用 producer 服務自己的既有 exchange `hawkr.account`

**決策：② 新增 `hawkr.delegation`。**

`governance` 承載的是**平台對賣家的治理動作**（違規、食安通報、停權、申訴），
代行是**平台替賣家做事**，兩者的 consumer 群、風險等級與通知語氣都不同。
混用會讓 consumer 必須解 payload 才知道要不要處理，而 routing key 分流本來就是 exchange 的職責。

**否決 ③**（本 repo 的 exchange 慣例確實是依 producer 服務命名，而代行事件的 producer
正是 account-service，所以這條必須明確否決）：`hawkr.account` 上的既有 routing key
全部是**帳號生命週期**，consumer 綁的是「這個帳號還在不在」；代行事件是**對某家賣家資源的操作**，
收件人是賣家、語意與帳號存亡無關。混進去之後帳號類 consumer 必須開始解 payload 判斷要不要處理
——那正是否決 ① 的同一條理由，換一個 exchange 不改變它。

## 5. 保留期與清理一律走維運 SQL，不寫排程任務

**考慮過**：① 三張表各自加清理排程 ② 一支統一的保留期清理 worker ③ 全部走維運 SQL

**決策：③ 維運 SQL。**

**否決 ①②**：清理的目的是**資料完整性維護**不是業務正確性，屬後端服務責任邊界規範
明列拒絕寫進 application code 的那一類。

**這條必須是 ADR 而不只是一張保留期表的理由**：`session` 那條「永不硬刪」
**不是保留策略而是正確性前提**——`DELETE .../current` 回 200 ＋ tombstone 的終止語意，
成立的條件就是「查不到」永遠不會發生。有人為了讓三張表的清理策略「一致」而順手把 session 也清掉，
**前端的終止語意會當場退化成 404**。
⇒ 清理稽核列時**不得**連帶刪 session 列（稽核列有 FK 指向 session，順序也反了）。

## 後果

- 代行功能不引入任何新的部署單元，也不引入任何排程器。
- 三段保留期與執行步驟屬維運動作，歸 Runbook 與維運 SQL，不進 application code。
