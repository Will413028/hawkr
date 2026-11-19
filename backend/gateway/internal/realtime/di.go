package realtime

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module wires the channel Registry and the concrete channels (notification +
// seller workspace) for the first release. New channels register here。
var Module = fx.Module("realtime",
	fx.Provide(
		NewRegistry,
		NewNotificationChannel,
		NewSellerWorkspaceChannel,
	),
	fx.Invoke(registerChannels),
)

// registerChannels attaches all known channels to the registry。
// This is called during fx app construction (effectively synchronous)，
// so the registry is fully populated before MQ consumer starts。
func registerChannels(
	reg *Registry,
	notif *NotificationChannel,
	sellerWs *SellerWorkspaceChannel,
	log *zap.Logger,
) {
	reg.Register(notif)
	reg.Register(sellerWs)
	log.Info("realtime.registry.initialized",
		zap.Int("channels", len(reg.Channels())),
		zap.Strings("routingKeys", reg.AllRoutingKeys()),
		zap.Strings("exchanges", reg.AllExchanges()))
}
