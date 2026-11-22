package main

import (
	"github.com/Will413028/hawkr/backend/internal/legacy/bizcalendarrelay"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// subscribeBizCalendarRelay 讓後台匯入行事曆時，在匯入的 transaction 內排入推送給 invoice-service 的 job，由 worker 送出。
// 只有 serve 需要：匯入只發生在後台的請求。
func subscribeBizCalendarRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, bizcalendarrelay.OnCalendarImported(queue))
}
