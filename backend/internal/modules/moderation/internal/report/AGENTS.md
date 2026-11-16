# moderation/internal/report

檢舉的送出與檢舉人自己的清單。

## 不變量

- 送出時依序鎖 `reporter_daily_counts`、`cases`、`case_snapshots`、`media_ref_sync`；案件處理（`internal/handle`）與刪帳訂閱者（`internal/retention`）的鎖順序要與它相容（`internal/platform/lockorder`）。
- 每日額度、去重與冪等的規則見模組 `AGENTS.md`。
