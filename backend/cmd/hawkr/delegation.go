package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newDelegation 建立 delegation 模組，並訂閱事件：identity 撤銷管理員角色的 transaction 內撤銷該管理員進行中的代行（並取消他發起的排程）；
// social 排程到點發布的 transaction 內寫單段的 SUCCEEDED 稽核（所以執行排程 job 的 worker 也要組裝它）。serve、worker 與 CLI 用同一個組裝，
// 各條路徑的效果相同。d.Social 是必填，組裝點要先以 delegation.NewScheduleAuthorizer 建 social，再把 social 填進 d。
func newDelegation(bus *events.Bus, d delegation.Deps) *delegation.Module {
	d.Events = bus
	m := delegation.New(d)
	events.Subscribe(bus, m.OnAdminRoleRevoked)
	events.Subscribe(bus, m.OnPostPublished)
	return m
}

// newDelegationOffline 是沒有 serve 的組裝（admin-role、delegation-eligibility 子命令）：撤銷角色或資格時 delegation 要取消代行發起的排程，
// 所以組出只做取消的 social（沒有買家、媒體與商品驗證；取消排的快取失效 job 用 insert-only 的 job client）。呼叫方負責關閉 d。
func newDelegationOffline(d *db.DB, bus *events.Bus, id *identity.Module, log *slog.Logger) (*delegation.Module, error) {
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		return nil, err
	}
	reader := seller.NewReader(d, time.Now)
	deps := delegation.Deps{DB: d, Identity: id, Seller: reader, NewID: uuid.NewV7, Now: time.Now, Logger: log}
	deps.Social = social.New(social.Deps{
		DB: d, Events: bus, Jobs: jc, Stores: reader, StoreBlocks: noStoreBlocks{},
		ScheduleAuthorizers: scheduleAuthorizers(delegation.NewScheduleAuthorizer(deps)),
		NewID:               uuid.NewV7, Now: time.Now, Logger: log,
	})
	return newDelegation(bus, deps), nil
}
