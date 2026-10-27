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
// (account-deletion and retired-incarnation cleanup, see notification_inbox.go);
// EnsureInvite/MarkInviteSecretDelivered proxy the vendor portal invite RPCs
// for the vendor portal invite email consumer (see worker/vendor_invite.go).
type SellerClient interface {
	ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error)
	EnsureInvite(ctx context.Context, prospectID, rfqRecipientID, expiresAt string) (inviteID, secret, invitedEmail string, err error)
	MarkInviteSecretDelivered(ctx context.Context, inviteID, secret string) error
}
