package usecase

import (
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// provideRevalidateClient builds the hawkr-website /api/revalidate caller
// from env vars. Returns a Client even when env is blank — Client.BustAsync
// no-ops when disabled, so local dev doesn't blow up.
func provideRevalidateClient(cfg *config.Config, logger *zap.Logger) *revalidate.Client {
	rcfg := revalidate.Config{
		BaseURL: cfg.WebRevalidate.BaseURL,
		Secret:  cfg.WebRevalidate.Secret,
	}
	// 部署漏設 WEB_REVALIDATE_* 時 client 靜默 no-op → 前端快取永不失效卻無訊號。
	// 啟動時發一次 WARN（不 fail startup，本地無此 config 仍可開發）。§9.5
	if !rcfg.Enabled() {
		logger.Warn("revalidate disabled: WEB_REVALIDATE_BASE_URL / WEB_REVALIDATE_SECRET 未設定，前端快取將不失效")
	}
	return revalidate.New(rcfg, logger)
}

var Module = fx.Module("usecase",
	fx.Provide(tsid.NewGenerator),
	fx.Provide(provideRevalidateClient),
	fx.Provide(
		NewPostUseCase,
		NewCommentUseCase,
		NewForumUseCase,
		NewBlockUseCase,
		NewReportUseCase,
		NewHashtagUseCase,
		NewPostHider,
		fx.Annotate(NewFanoutHideDispatcher, fx.As(new(HideTargetDispatcher)), fx.As(new(domain.TargetOwnerResolver))),
		fx.Annotate(NewAccountBanDispatcher, fx.As(new(BanUserDispatcher))),
	),
)
