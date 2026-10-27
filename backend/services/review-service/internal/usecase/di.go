package usecase

import (
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// provideRevalidateClient builds the hawkr-website /api/revalidate caller from env vars.
// Blank config → Client.BustAsync no-ops (safe for local dev without web wiring).
func provideRevalidateClient(cfg *config.Config, logger *zap.Logger) *revalidate.Client {
	return revalidate.New(revalidate.Config{
		BaseURL: cfg.WebRevalidate.BaseURL,
		Secret:  cfg.WebRevalidate.Secret,
	}, logger)
}

var Module = fx.Module("usecase",
	fx.Provide(tsid.NewGenerator),
	fx.Provide(provideRevalidateClient),
	fx.Provide(NewReviewUseCase),
)
