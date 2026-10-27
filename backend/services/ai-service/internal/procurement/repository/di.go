package repository

import "go.uber.org/fx"

var Module = fx.Module("procurement_repository",
	fx.Provide(New),
	fx.Provide(NewRankingFactsRepository),
)
