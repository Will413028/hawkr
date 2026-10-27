package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type vendorPortalRepository struct {
	pool      *pgxpool.Pool
	txManager database.TxManager
	newID     func() int64
}

func NewVendorPortalRepository(
	pool *pgxpool.Pool, txManager database.TxManager, newID func() int64,
) domain.VendorPortalRepository {
	return &vendorPortalRepository{pool: pool, txManager: txManager, newID: newID}
}

func (r *vendorPortalRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

func newSecret() (secret, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("vendor portal: read random: %w", err)
	}
	secret = hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(secret))
	return secret, hex.EncodeToString(sum[:]), nil
}

func (r *vendorPortalRepository) EnsureInvite(
	ctx context.Context, in domain.EnsureInviteInput,
) (domain.VendorPortalInvite, string, error) {
	q := gen.New(r.pool)

	existing, err := q.GetVendorPortalInviteByRFQRecipient(ctx, in.RFQRecipientID)
	switch {
	case err == nil && existing.SecretDeliveredAt != nil:
		return domain.VendorPortalInvite{}, "", domain.ErrInviteSecretAlreadyDelivered
	case err == nil:
		rotateEmail, eErr := q.GetSellerProspectPublicEmail(ctx, in.ProspectID)
		if eErr != nil {
			return domain.VendorPortalInvite{}, "", eErr
		}
		if !domain.IsDeliverableEmail(rotateEmail) {
			return domain.VendorPortalInvite{}, "", domain.ErrProspectHasNoEmail
		}
		if _, uErr := q.UpsertVendorPortalIdentity(ctx, gen.UpsertVendorPortalIdentityParams{
			ID:           r.newID(),
			ProspectID:   in.ProspectID,
			InvitedEmail: rotateEmail,
		}); uErr != nil {
			return domain.VendorPortalInvite{}, "", fmt.Errorf("vendor portal: upsert identity: %w", uErr)
		}
		secret, hash, sErr := newSecret()
		if sErr != nil {
			return domain.VendorPortalInvite{}, "", sErr
		}
		row, rErr := q.RotateVendorPortalInviteSecret(ctx, gen.RotateVendorPortalInviteSecretParams{
			ID: existing.ID, TokenHash: hash, ExpiresAt: timestamptzParam(in.ExpiresAt),
		})
		if errors.Is(rErr, pgx.ErrNoRows) {
			return domain.VendorPortalInvite{}, "", domain.ErrInviteSecretAlreadyDelivered
		}
		if rErr != nil {
			return domain.VendorPortalInvite{}, "", rErr
		}
		invite := toVendorPortalInvite(row, in.ProspectID)
		invite.Email = rotateEmail
		return invite, secret, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return domain.VendorPortalInvite{}, "", err
	}

	email, err := q.GetSellerProspectPublicEmail(ctx, in.ProspectID)
	if err != nil {
		return domain.VendorPortalInvite{}, "", err
	}
	if !domain.IsDeliverableEmail(email) {
		return domain.VendorPortalInvite{}, "", domain.ErrProspectHasNoEmail
	}

	identity, err := q.UpsertVendorPortalIdentity(ctx, gen.UpsertVendorPortalIdentityParams{
		ID:           r.newID(),
		ProspectID:   in.ProspectID,
		InvitedEmail: email,
	})
	if err != nil {
		return domain.VendorPortalInvite{}, "", fmt.Errorf("vendor portal: upsert identity: %w", err)
	}

	secret, hash, err := newSecret()
	if err != nil {
		return domain.VendorPortalInvite{}, "", err
	}
	row, err := q.InsertVendorPortalInvite(ctx, gen.InsertVendorPortalInviteParams{
		ID: r.newID(), IdentityID: identity.ID, RfqRecipientID: in.RFQRecipientID,
		TokenHash: hash, ExpiresAt: timestamptzParam(in.ExpiresAt),
	})
	if err != nil {
		return domain.VendorPortalInvite{}, "", err
	}
	invite := toVendorPortalInvite(row, in.ProspectID)
	invite.Email = email
	return invite, secret, nil
}

func (r *vendorPortalRepository) MarkSecretDelivered(
	ctx context.Context, inviteID int64, at time.Time,
) error {
	_, err := gen.New(r.pool).MarkVendorPortalInviteSecretDelivered(ctx,
		gen.MarkVendorPortalInviteSecretDeliveredParams{ID: inviteID, SecretDeliveredAt: &at})
	return err
}

func toVendorPortalInvite(row gen.VendorPortalInvite, prospectID int64) domain.VendorPortalInvite {
	return domain.VendorPortalInvite{
		ID:                row.ID,
		IdentityID:        row.IdentityID,
		ProspectID:        prospectID,
		RFQRecipientID:    row.RfqRecipientID,
		ExpiresAt:         row.ExpiresAt.Time,
		SecretDeliveredAt: row.SecretDeliveredAt,
	}
}
