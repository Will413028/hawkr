package postgres

import (
	"github.com/Will413028/hawkr/review-service/internal/domain"
	"go.uber.org/fx"
)

var Module = fx.Module("repository",
	fx.Provide(fx.Annotate(NewReviewRepository, fx.As(new(domain.ReviewRepository)))),
)
