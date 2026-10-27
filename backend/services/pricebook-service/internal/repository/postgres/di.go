package postgres

import (
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"go.uber.org/fx"
)

var Module = fx.Module("repository",
	fx.Provide(NewMarketWatchCleanupRepository),
	fx.Provide(fx.Annotate(NewForecastRepository, fx.As(new(domain.ForecastRepository)))),
)
