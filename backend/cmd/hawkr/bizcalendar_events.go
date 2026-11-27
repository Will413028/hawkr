package main

import (
	"github.com/Will413028/hawkr/backend/internal/legacy/bizcalendarrelay"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func subscribeBizCalendarRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, bizcalendarrelay.OnCalendarImported(queue))
}
