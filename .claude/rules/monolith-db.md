---
description: Postgres driver and sqlc rules for the Go monolith
paths:
  - "backend/internal/platform/db/db.go"
  - "backend/internal/modules/*/db/sqlc.yaml"
---

# Monolith DB

- 改成 `QueryExecModeExec` 前，先替每個 JSONB 欄位加 sqlc override：裸 `[]byte` 會被當成 bytea，值為 NULL 時測試抓不到。
