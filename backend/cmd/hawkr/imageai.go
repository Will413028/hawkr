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

func newImageAI(d *db.DB, cfg imageai.Config, sellers *seller.Module, images llm.ImageGenerator, med *media.Module, log *slog.Logger) *imageai.Module {
	return imageai.New(imageai.Deps{
		DB: d, Now: time.Now, Seller: sellers, Stores: sellers, Images: images, Log: log, MaxConcurrent: cfg.MaxConcurrent,
		Media: med, NewID: uuid.NewV7,
	})
}

func subscribeImageAI(bus *events.Bus, m *imageai.Module) {
	events.Subscribe(bus, m.OnStoreRetired)
}
