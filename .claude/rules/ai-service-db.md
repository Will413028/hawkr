---
description: ai-service session storage schema and the adk dependency
paths:
  - "backend/services/ai-service/go.mod"
  - "backend/services/ai-service/db/schema/schema.sql"
---

# AI service DB

- 升級 `google.golang.org/adk` 前先比對新舊版 session／database 的 storage model，把欄位差異補進 `db/schema/schema.sql` 再跑 `atlas migrate diff`：build 綠不代表表結構相容。
