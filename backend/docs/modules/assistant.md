# assistant 模組行為規格

本文是單體 `assistant` 模組行為的唯一來源。目前只有骨架：買家助理的對話：買家與助理的跨輪對話、串流回應與續傳。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：ordering、catalog、buyer。
- 公開介面與行為隨 A9-2 到 A9-4 的實作補上。
