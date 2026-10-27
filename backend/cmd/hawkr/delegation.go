package main

import (
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// newDelegation 建立 delegation 模組，並訂閱 identity 的事件：撤銷管理員角色的 transaction 內撤銷該管理員進行中的代行。
// serve 與撤銷角色的 CLI 用同一個組裝，兩條路徑的撤銷效果相同。
func newDelegation(bus *events.Bus, d delegation.Deps) *delegation.Module {
	d.Events = bus
	m := delegation.New(d)
	events.Subscribe(bus, m.OnAdminRoleRevoked)
	return m
}
