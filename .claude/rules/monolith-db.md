---
description: Postgres driver and sqlc rules for the Go monolith
paths:
  - "backend/internal/platform/db/db.go"
  - "backend/internal/modules/*/db/sqlc.yaml"
  - "backend/internal/modules/*/db/migrations/*"
  - "backend/internal/modules/*/internal/store/*"
---

# Monolith DB

- 改成 `QueryExecModeExec` 前，先替每個 JSONB 欄位加 sqlc override：裸 `[]byte` 會被當成 bytea，值為 NULL 時測試抓不到。
- `make sqlc` 會先 `rm -rf` 每個模組的 `internal/store` 再跑 docker：先確認 Docker 起來；誤刪時 `git restore -- backend/internal/modules/<模組>/internal/store`。
- `atlas migrate validate` 在模組的 db 目錄跑：`cd backend/internal/modules/<模組>/db && atlas migrate validate --dir file://migrations`。
