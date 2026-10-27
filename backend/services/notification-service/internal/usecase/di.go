package usecase

import "go.uber.org/fx"

// Module provides all UseCase implementations.
var Module = fx.Module("usecase",
	fx.Provide(
		NewRegisterTokenUseCase,
		NewRemoveTokenUseCase,
		NewSendPushUseCase,
		NewNotificationPreferenceUseCase,
		NewNotificationInboxUseCase,
	),
)
