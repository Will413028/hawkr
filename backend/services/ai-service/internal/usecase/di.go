package usecase

import "go.uber.org/fx"

// Module provides the usecase layer dependencies
var Module = fx.Module("usecase",
	fx.Provide(NewRecommendUseCase),
	fx.Provide(fx.Annotate(NewStyleUseCase, fx.ParamTags(`name:"ai_pool"`))),
	fx.Provide(fx.Annotate(NewTransformUseCase, fx.ParamTags(`name:"ai_pool"`, ``))),
	fx.Provide(fx.Annotate(NewQuotaUseCase, fx.ParamTags(`name:"ai_pool"`))),
	fx.Provide(
		fx.Annotate(
			NewCandidateSearchUseCase,
			fx.As(new(CandidateLeadSearcher)),
		),
	),
)
