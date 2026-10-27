package repository

import (
	"go.uber.org/fx"

	"github.com/Will413028/hawkr/crm-service/internal/repository/postgres"
)

var Module = fx.Module("repository",
	postgres.Module,
)
