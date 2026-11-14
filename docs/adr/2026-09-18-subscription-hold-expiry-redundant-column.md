# 定期配送 hold 壽命：用冗餘欄 ＋ 單表 CHECK，而不是代數消去或 trigger

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18 |
| 影響範圍 | 67 號 `recurring_inventory_hold`／`recurring_capacity_hold`／`recurring_cycle` |

## 脈絡

不變式 INV-10 要求 hold 的租約活過「RESERVE 與 CHARGE 之間的空窗 ＋ 扣款後的 materialize 重試窗」：

```
expires_at ≥ charge_cutoff_at + 24h
```

兩個運算元原本分屬兩張表（`*_hold` 與 `recurring_cycle`），
而 **PostgreSQL 的 table CHECK 不能引用另一張表** ⇒ 照字面施工只會留下一條
**永遠紅不了的不變式**——比沒有更糟，因為假的保證會排擠掉真的警覺。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| **代數消去 hold 側**，得 `charge_cutoff_at ≤ scheduled_for_at − 12h`，併進 `chk_rcy_time_order` | 式子沒錯，但**把約束推到錯的一側**：publish 側只要求 `charge_cutoff ≥ lead_time + prep_time`，而 `delivery_prep_time_hours` 與 `seller_lead_time_hours` 都沒有下界 CHECK、且賣家可覆寫 ⇒ 賣家設 `prep=4／lead=0／cutoff=4` 時 publish gate 全過，之後**每一個 cycle 都 23514**，整個方案的週期永遠生不出來，而錯誤發生在排程器裡、賣家端零訊號 | ❌ |
| **trigger** | trigger 的觀測窗是「有沒有人繞過它」 | ❌ |
| **應用層算** | 應用層口頭約定；lock 步驟算錯時沒有任何東西會紅 | ❌ |
| **兩張 hold 表各加一欄 `charge_cutoff_at`，於 lock 步驟自同一個 cycle 列複製，掛單表 CHECK** | 一個冗餘欄 | ✅ **採用** |

⚠️ 第一個方案被否決的通則是：**用一條 DB 約束去限制一個賣家有權設定的商業參數，本身就是訊號**
（gate 擋住時可以動的東西有優先序：放寬門檻 ＞ 修真正的問題 ＞ 改產品行為，最後一項一律要功能擁有者點頭）。

## 決策

`recurring_inventory_hold` 與 `recurring_capacity_hold` 各加一欄
`charge_cutoff_at timestamptz NOT NULL`，掛單表 CHECK
`expires_at >= charge_cutoff_at + INTERVAL '24 hours'`。

`chk_rcy_time_order` 維持 `edit_cutoff_at < charge_cutoff_at < scheduled_for_at`，
**不碰賣家的設定空間**。

## 後果

- 接受一個冗餘欄。**它沒有漂移面**：`cycle.charge_cutoff_at` 與 hold 列都在 lock 那一刻定案
  且此後不可變（cycle 進 `LOCKED` 後不再改時間軸，hold 只會被釋放不會被改期）⇒ 沒有第二個寫入者。
- INV-10 因此是**真的 DB 強制**，而不是宣告式的口頭約定。

## 修訂：設計書 67 已刪除（2026-10-29）

定期配送訂閱不做，設計書 67 已刪除，本文討論的 `recurring_inventory_hold`／`recurring_capacity_hold`／`recurring_cycle` 不會建立。決策對這三張表不再適用；若日後重做定期配送，要從零重新設計 hold 壽命，不能把本文當作既定結論。
