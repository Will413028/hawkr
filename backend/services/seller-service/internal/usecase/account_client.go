package usecase

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/rolestate"
)

// AccountInfo holds the account fields needed by seller-service.
type AccountInfo struct {
	Email       string
	Phone       string
	AccountType int32
}

// AccountClient defines the interface for account-service operations.
type AccountClient interface {
	GetEmail(ctx context.Context, accountID int64) (string, error)
	GetAccountInfo(ctx context.Context, accountID int64) (*AccountInfo, error)
	AccountRoleGranter
}

// AccountRoleGranter grants the seller role, attributing it to the stint that
// justifies it (issue #3189).
//
// storeID is not decoration: account-service records it on the grant, and a
// later deletion may only revoke a grant carrying its own storeID. Callers must
// pass the account's CURRENT stint — the one that exists at the moment of the
// call — never a surrogate snapshotted when a job was enqueued. Passing a
// retired stint would re-point the grant backwards and re-arm the very
// revocation the fence exists to reject.
type AccountRoleGranter interface {
	BindSellerRole(ctx context.Context, accountID, storeID int64) error
}

// AccountRoleRemover removes a role from the account via account-service.
// If the account has no other active roles AND the caller confirms the other
// role is rolestate.OtherRoleAbsent, account-service performs full
// soft-deletion (Apple revoke + Redis blacklist + MQ).
// rolestate.OtherRoleUnknown never triggers cascade.
//
// storeID names the stint whose closure authorises the removal. account-service
// applies it only if the account's current seller grant came from that same
// stint, so a removal that has been retrying for hours can no longer strip a
// role the account earned by re-onboarding in the meantime (issue #3189).
type AccountRoleRemover interface {
	RemoveSellerRole(ctx context.Context, accountID int64, other rolestate.OtherRoleState, storeID int64) error
}
