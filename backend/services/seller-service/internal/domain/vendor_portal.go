package domain

import (
	"context"
	"errors"
	"time"
)

var ErrInviteSecretAlreadyDelivered = errors.New("vendor portal: invite secret already delivered")

var ErrProspectHasNoEmail = errors.New("vendor portal: prospect has no public email")

type VendorPortalInvite struct {
	ID                int64
	IdentityID        int64
	ProspectID        int64
	RFQRecipientID    int64
	ExpiresAt         time.Time
	SecretDeliveredAt *time.Time
	Email             string
}

type EnsureInviteInput struct {
	ProspectID     int64
	RFQRecipientID int64
	ExpiresAt      time.Time
}

type VendorPortalRepository interface {
	EnsureInvite(ctx context.Context, in EnsureInviteInput) (VendorPortalInvite, string, error)

	MarkSecretDelivered(ctx context.Context, inviteID int64, at time.Time) error
}
