package client

import (
	"github.com/Will413028/hawkr/notification-service/internal/client/buyer"
	"github.com/Will413028/hawkr/notification-service/internal/client/seller"
	"go.uber.org/fx"
)

// Module provides the buyer-service/seller-service gRPC clients used to
// enumerate historical buyer_id/store_id surrogates before chat-notification
// PII redaction (individually erroring — RedactChatNotificationsByAccount's
// enumeration; see internal/usecase/notification_inbox.go).
var Module = fx.Module("client",
	fx.Provide(buyer.NewClient),
	fx.Provide(seller.NewClient),
)
