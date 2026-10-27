package main

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// subscribeAgent 讓 prospect 的封存與合併撤銷它的 vendor portal 邀請與 session：事件在 seller 的 transaction 內同步發布，
// handler 在同一個 transaction 內撤銷，所以 prospect 封存的那一刻，它的邀請連結與登入就都失效。seller 與 agent 的接線只在這裡。
func subscribeAgent(bus *events.Bus, module *agent.Module) {
	events.Subscribe(bus, func(ctx context.Context, e seller.ProspectArchived) error { return module.OnProspectArchived(ctx, e) })
	events.Subscribe(bus, func(ctx context.Context, e seller.ProspectMerged) error { return module.OnProspectMerged(ctx, e) })
}
