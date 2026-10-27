# payuni-webhook 接收流程採兩段獨立交易 ＋ status-aware replay

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18 |
| 影響範圍 | `payuni-webhook-service` 接收流程事務邊界；下游 payment-service／aftersales-service 的冪等分工 |

## 脈絡

`webhook_inbox` 的 `UNIQUE (provider, webhook_type, event_key)` 承擔一個設計層次的承諾：
**「inbox INSERT 成功 ＝ dedup commitment 完成」**，下游服務的冪等只是縱深防禦。

把 inbox INSERT、outbox INSERT、inbox 狀態更新包在單一 `@Transactional` 內時，這個承諾
**只在 commit 之後才為真**：交易進行中該 row 對其他連線不可見，被 rollback 後等同從未存在。
於是 outbox INSERT 一失敗就會連 inbox 一起 rollback ⇒ UNIQUE 約束等於沒運作 ⇒ PayUni
重送時重新進入流程，失敗持續就是無限循環。

不可動的約束：

- PayUni 只在我方**未回 200** 時重送；回 4xx 代表「不要再送」。
- 下游（payment-service）確實有自己的冪等鍵，但分工設計是「webhook-service 是 dedup 真實
  邊界」——不能靠下游把這個承諾補回來。

## 考慮過的方案

| 方案 | 機制 | 代價 | 判定 |
|---|---|---|---|
| A. inbox 拆 `REQUIRES_NEW` 先 commit，outbox ＋ status 走外層交易 | 兩段事務 | T2 失敗時 inbox 永久停在 `RECEIVED`，PayUni 重送反而被 dedup 攔下 ⇒ 事件遺失 | ❌ 製造新的 silent loss |
| B. T2 失敗時 catch 例外把 inbox 標 `FAILED`、不 rollback | 補償 | 訊息仍然遺失，仍需另一個 scheduler 掃 `FAILED` 重放，且語意比 F 模糊 | ❌ |
| C. 完全不開外層 `@Transactional`，三步各自獨立 | 無事務 | 多筆 outbox ＋ inbox 狀態更新無法原子，任一中途失敗即不一致 | ❌ |
| D. 兩段獨立交易 ＋ 專屬 scheduler 主動補掃 orphan inbox | 主動回收 | 最完備，但要新增 scheduler、query 與告警指標 | ⚠️ 當下判為過度工程 |
| **F. 兩段獨立交易 ＋ status-aware replay（PayUni 重送驅動）** | **被動回收** | **依賴 PayUni 的重送窗口** | **✓ 採用** |

E 案（單純加 outbox UNIQUE）不單獨列為候選——它是 F 案的前提，不是替代方案。

## 決策

採 F：`T1` 只做 inbox INSERT 並立即 commit（dedup 即時生效），`T2` 做 outbox INSERT ＋
inbox 狀態更新並保持 atomic；`T2` 失敗一律 rethrow 讓 controller 回 500，由 PayUni 重送
觸發 status-aware replay 路徑。

## 後果

- 接受的代價：**回收能力綁在 PayUni 的重送策略上**。若 PayUni 停止重送，卡在
  `RECEIVED` / `FAILED` 的 inbox 沒有其他回收路徑。
- 由此衍生的硬性約束：**Gateway 對 PayUni 必須回 502 而不是 424**——回 4xx 等於告訴
  PayUni 不要再送，replay 路徑就永遠等不到觸發。這一條寫在詳細設計書 §4.5.5，是規格不是背景。
- **什麼條件下應該重新評估**：坐實 PayUni 的重送窗口有上界且該上界不足以覆蓋我方的
  故障恢復時間時，D 案（`OrphanInboxRecoveryScheduler`）應作為最後防線補上。
