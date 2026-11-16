package main

import (
	"github.com/Will413028/hawkr/backend/internal/legacy/socialrelay"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// subscribeSocialRelay 把 social 的互動事件與 seller 的內容下架措施轉給還在 RabbitMQ 上的 notification-service：每個事件在發布的
// transaction 內排入一個 relay job，worker 組出舊 publisher 的 payload 再送出。serve 與 worker 兩個角色都要訂閱：排程到點的發布
// 從 worker 的 job 發出。對應與不轉送的事件見 socialrelay 的套件說明。
func subscribeSocialRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, socialrelay.OnPostPublished(queue))
	events.Subscribe(bus, socialrelay.OnCommentCreated(queue))
	events.Subscribe(bus, socialrelay.OnCommentReplied(queue))
	events.Subscribe(bus, socialrelay.OnPostLiked(queue))
	events.Subscribe(bus, socialrelay.OnCommentLiked(queue))
	events.Subscribe(bus, socialrelay.OnMeasureCreated(queue))
}
