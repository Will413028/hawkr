package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
)

// newImageAI 在 composition root 建唯一的 imageai 實例；serve 與 worker 各建一個（程序各自一個 pod 內的同時修圖上限）。
// sellers 同時是修圖前確認店家未退場的守門（StoreGuard）與後台確認店家存在的讀取（StoreReader）。
func newImageAI(d *db.DB, cfg imageai.Config, sellers *seller.Module, images llm.ImageGenerator, med *media.Module, log *slog.Logger) *imageai.Module {
	return imageai.New(imageai.Deps{
		DB: d, Now: time.Now, Seller: sellers, Stores: sellers, Images: images, Log: log, MaxConcurrent: cfg.MaxConcurrent,
		Media: med, NewID: uuid.NewV7,
	})
}

// subscribeImageAI 讓店家退場在同一個 transaction 刪該店的修圖額度與用量（單體規則 4 的原子狀態變更）。
// serve 與 worker 都要訂：StoreRetired 在哪個程序的 transaction 內發布，訂閱者就要在那個程序裡。
func subscribeImageAI(bus *events.Bus, m *imageai.Module) {
	events.Subscribe(bus, m.OnStoreRetired)
}
