package usecase

import "context"

// BuyerClient enumerates a buyer-service account's historical buyer_id
// surrogates, consumed by RedactChatNotificationsByAccount (see
// notification_inbox.go) to build the full historical id set to redact.
type BuyerClient interface {
	ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error)
}

// SellerClient is notification-service's seller-service gRPC client
// abstraction: ListIDsByAccount enumerates historical store_id surrogates
// (account-deletion and retired-incarnation cleanup, see notification_inbox.go).
type SellerClient interface {
	ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error)
}
