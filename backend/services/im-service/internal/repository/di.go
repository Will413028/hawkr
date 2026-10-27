package repository

import (
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres"
	"go.uber.org/fx"
)

// Module Repository Layer
var Module = fx.Module("repository",
	postgres.Module,
)
