---
description: Go module, lint and service-retirement checks for the backend
paths:
  - "backend/**/*.go"
  - "backend/go.mod"
  - "backend/services/**"
---

# Backend Go

- 改到共用模組（`backend/pkg/`、`backend/api/proto` 生成的 Go）時，本機跑 go.work 每個成員的 gofmt、vet、test，不只單體與 gateway：CI 的 backend-go-checks 動到 pkg 就測全部。
- `make lint-monolith`（`backend/.golangci-version` 與 `.go-version` 經 `go run`）基準是 0 issues：有任何發現就是新問題，推之前修掉。
- 改到 `backend/go.mod`（含 subagent 跑過 `go get`）後，commit 前跑 `GOWORK=off go mod tidy` 確認沒有 diff（CI backend monolith 有 tidy drift 檢查）。
- 刪除一個舊服務時，同一次變更在 `backend/docs/plans/2026-09-24-go-modular-monolith/rpc-servers.csv` 補上它每個 RPC 的 disposition，跑 `python3 backend/scripts/check-migration-inventory.py`（CI `backend migration inventory`）。
- 推之前照 workflow 的範圍跑：backend-monolith 的 gofmt 只掃 `cmd`、`internal`；`backend/pkg` 與各服務的 gofmt、vet、test 在 backend-go-checks。
