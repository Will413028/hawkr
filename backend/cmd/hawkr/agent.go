package main

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// newAgent 組出 agent 模組：邀請的 email 快照從 seller 的 ProspectContact 讀（seller 是它的 Prospects port）。
func newAgent(d *db.DB, sellers *seller.Module) *agent.Module {
	return agent.New(agent.Deps{DB: d, Prospects: sellers, Now: time.Now})
}
