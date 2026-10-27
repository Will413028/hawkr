package repository

import (
	"go.uber.org/fx"

	"github.com/Will413028/hawkr/notification-service/internal/repository/postgres"
)

// Module aggregates all repository sub-modules.
var Module = fx.Module("repository",
	postgres.Module,
)
