# procurement 模組行為規格

本文是單體 `procurement` 模組行為的唯一來源。目前只有骨架：買家採購（RFQ）與賣家報價；採購的另一半（賣家回報價）也在這裡。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：ordering、catalog、buyer、seller；不依賴 notification（notification 依賴 procurement，方向見 `internal/arch/modules.go`）。
- 公開介面與行為隨 A9-2 到 A9-4 的實作補上。
