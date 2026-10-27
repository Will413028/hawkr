package usecase

import (
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"go.uber.org/fx"
)

var Module = fx.Module("usecase",
	fx.Provide(tsid.NewGenerator),
	fx.Provide(NewCrmUseCase),
)
