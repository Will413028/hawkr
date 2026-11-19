# imageai 模組行為規格

本文是單體 `imageai` 模組行為的唯一來源。目前只有骨架：賣家的 AI 修圖、修圖配額與後台風格管理。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：seller、media。
- 公開介面與行為隨 A9-2 到 A9-4 的實作補上。
