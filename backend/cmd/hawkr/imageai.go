package main

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// newImageAI 在 composition root 建唯一的 imageai 實例；serve 與 worker 各建一個（程序各自一個 pod 內的同時修圖上限）。
func newImageAI(d *db.DB, cfg imageai.Config) *imageai.Module {
	return imageai.New(imageai.Deps{DB: d, Now: time.Now, MaxConcurrent: cfg.MaxConcurrent})
}

// subscribeImageAI 讓店家退場在同一個 transaction 刪該店的修圖額度與用量（單體規則 4 的原子狀態變更）。
// serve 與 worker 都要訂：StoreRetired 在哪個程序的 transaction 內發布，訂閱者就要在那個程序裡。
func subscribeImageAI(bus *events.Bus, m *imageai.Module) {
	events.Subscribe(bus, m.OnStoreRetired)
}
