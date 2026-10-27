package usecase

import (
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"go.uber.org/fx"
)

// Module provides the usecase layer dependencies.
var Module = fx.Module("usecase",
	fx.Provide(fx.Annotate(NewMessageHub, fx.As(new(domain.Hub)))),
	fx.Provide(NewMessageUseCase),
)
