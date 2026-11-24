---
description: assistant module adk session storage schema and the adk dependency
paths:
  - "backend/go.mod"
  - "backend/internal/modules/assistant/db/schema.sql"
  - "backend/internal/modules/assistant/internal/adksession/**"
---

# Assistant session DB

- 升級 `google.golang.org/adk/v2` 前先比對新舊版 session／database 的 storage model，把欄位差異補進 `backend/internal/modules/assistant/db/schema.sql` 與 `internal/adksession` 的 SQL，再跑 `atlas migrate diff`：build 綠不代表表結構相容（ADK 的 `RunServiceTests` 是 CI 的驗收）。
