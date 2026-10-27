package postgres

import (
	"go.uber.org/fx"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
)

var Module = fx.Module("postgres_repository",
	fx.Provide(
		fx.Annotate(NewCrmRepository, fx.As(new(domain.CrmRepository))),
	),
)
