package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type payuniRepository struct {
	pool *pgxpool.Pool
}

func NewPayuniRepository(pool *pgxpool.Pool) domain.PayuniRepository {
	return &payuniRepository{pool: pool}
}

func (r *payuniRepository) GetByStoreID(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetPayuniRegistrationByStoreID(ctx, storeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("payuni registration not found")
		}
		return nil, err
	}
	return toDomainPayuniRegistration(row)
}

func (r *payuniRepository) GetByMerID(ctx context.Context, merID string) (*domain.PayuniRegistration, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetPayuniRegistrationByMerID(ctx, nullableText(merID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("payuni registration not found")
		}
		return nil, err
	}
	return toDomainPayuniRegistration(row)
}

func (r *payuniRepository) GetPending(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetPendingPayuniRegistrations(ctx, gen.GetPendingPayuniRegistrationsParams{
		StaleBefore: nullableTimePointer(&staleBefore),
		RowLimit:    int32(limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.PayuniRegistration, 0, len(rows))
	for _, row := range rows {
		reg, err := toDomainPayuniRegistration(row)
		if err != nil {
			return nil, err
		}
		result = append(result, reg)
	}
	return result, nil
}

func (r *payuniRepository) ClaimForSync(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ClaimPayuniRegistrationForSync(ctx, gen.ClaimPayuniRegistrationForSyncParams{
		ID:          id,
		ClaimedAt:   nullableTimePointer(&claimedAt),
		StaleBefore: nullableTimePointer(&staleBefore),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *payuniRepository) Create(ctx context.Context, reg *domain.PayuniRegistration) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	_, err := q.CreatePayuniRegistration(ctx, gen.CreatePayuniRegistrationParams{
		ID:                 reg.ID,
		AccountID:          reg.AccountID,
		StoreID:            reg.StoreID,
		ApplicationID:      reg.ApplicationID,
		PayuniMerID:        nullableText(reg.PayuniMerID),
		RegistrationStatus: nullableText(reg.Status.String()),
		PaymentEnabled:     boolPointer(reg.PaymentEnabled),
		UpdatedAt:          nullableTime(time.Now().UTC()),
	})
	return err
}

func (r *payuniRepository) Update(ctx context.Context, reg *domain.PayuniRegistration) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdatePayuniRegistration(ctx, gen.UpdatePayuniRegistrationParams{
		ID:                 reg.ID,
		PayuniMerID:        nullableText(reg.PayuniMerID),
		RegistrationStatus: nullableText(reg.Status.String()),
		PaymentEnabled:     boolPointer(reg.PaymentEnabled),
		MerStatus:          nullableText(reg.MerStatus),
		MemberStatus:       nullableText(reg.MemberStatus),
		CreditStatus:       nullableText(reg.CreditStatus),
		ApplepayStatus:     nullableText(reg.ApplePayStatus),
		GooglepayStatus:    nullableText(reg.GooglePayStatus),
		AtmStatus:          nullableText(reg.ATMStatus),
		Reason:             nullableText(reg.Reason),
		ErrorMessage:       nullableText(reg.ErrorMessage),
		ErrorCode:          nullableText(reg.ErrorCode),
		RawRequest:         pgxutil.JSONBytes(reg.RawRequest),
		RawResponse:        pgxutil.JSONBytes(reg.RawResponse),
		RegisteredAt:       nullableTimePointer(reg.RegisteredAt),
		ApprovedAt:         nullableTimePointer(reg.ApprovedAt),
		LastSyncedAt:       nullableTimePointer(reg.LastSyncedAt),
		UpdatedAt:          nullableTime(reg.UpdatedAt),
	})
}

func (r *payuniRepository) Upsert(ctx context.Context, reg *domain.PayuniRegistration) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	rawRequestBytes := pgxutil.JSONBytes(reg.RawRequest)
	rawResponseBytes := pgxutil.JSONBytes(reg.RawResponse)

	logger.Info(ctx, "upserting PAYUNi registration",
		zap.Int64("account_id", reg.AccountID),
		zap.Int64("application_id", reg.ApplicationID),
		zap.Int("domain_raw_request_length", len(reg.RawRequest)),
		zap.Int("domain_raw_response_length", len(reg.RawResponse)),
		zap.Int("sqlutil_raw_request_length", len(rawRequestBytes)),
		zap.Int("sqlutil_raw_response_length", len(rawResponseBytes)),
		zap.Bool("raw_request_is_nil", rawRequestBytes == nil),
		zap.Bool("raw_response_is_nil", rawResponseBytes == nil))

	row, err := q.UpsertPayuniRegistration(ctx, gen.UpsertPayuniRegistrationParams{
		ID:                 reg.ID,
		AccountID:          reg.AccountID,
		StoreID:            reg.StoreID,
		ApplicationID:      reg.ApplicationID,
		PayuniMerID:        nullableText(reg.PayuniMerID),
		RegistrationStatus: nullableText(reg.Status.String()),
		PaymentEnabled:     boolPointer(reg.PaymentEnabled),
		MerStatus:          nullableText(reg.MerStatus),
		MemberStatus:       nullableText(reg.MemberStatus),
		CreditStatus:       nullableText(reg.CreditStatus),
		ApplepayStatus:     nullableText(reg.ApplePayStatus),
		GooglepayStatus:    nullableText(reg.GooglePayStatus),
		AtmStatus:          nullableText(reg.ATMStatus),
		Reason:             nullableText(reg.Reason),
		ErrorMessage:       nullableText(reg.ErrorMessage),
		ErrorCode:          nullableText(reg.ErrorCode),
		RawRequest:         rawRequestBytes,
		RawResponse:        rawResponseBytes,
		RegisteredAt:       nullableTimePointer(reg.RegisteredAt),
		ApprovedAt:         nullableTimePointer(reg.ApprovedAt),
		LastSyncedAt:       nullableTimePointer(reg.LastSyncedAt),
		UpdatedAt:          nullableTime(reg.UpdatedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pkgerrors.NewConflict(fmt.Sprintf(
				"payuni_mer_id is write-once: store_id=%d already has a different MerID than %q",
				reg.StoreID, reg.PayuniMerID))
		}
		return err
	}
	reg.ID = row.ID

	logger.Info(ctx, "PAYUNi registration upserted",
		zap.Int64("id", row.ID),
		zap.Int64("account_id", reg.AccountID),
		zap.Int("returned_raw_request_length", len(row.RawRequest)),
		zap.Int("returned_raw_response_length", len(row.RawResponse)))

	return nil
}

func (r *payuniRepository) UpdateMerchantProfile(ctx context.Context, storeID, expectedVersion int64, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel string) (*domain.PayuniRegistration, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.UpdatePayuniMerchantProfile(ctx, gen.UpdatePayuniMerchantProfileParams{
		StoreID:                storeID,
		MerchantProfileVersion: expectedVersion,
		ContactName:            contactName,
		ContactMobile:          contactMobile,
		ContactTel:             contactTel,
		MerEmail:               merEmail,
		MerAddr:                merAddr,
		MerAddrPost:            merAddrPost,
		MerTel:                 merTel,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, existsErr := q.GetPayuniRegistrationByStoreID(ctx, storeID); existsErr != nil {
				if errors.Is(existsErr, pgx.ErrNoRows) {
					return nil, pkgerrors.NewNotFound("payuni registration not found")
				}
				return nil, existsErr
			}
			return nil, domain.ErrMerchantProfileVersionConflict
		}
		return nil, err
	}
	return toDomainPayuniRegistration(row)
}

func (r *payuniRepository) AcquireMerchantProfileUpdateLock(ctx context.Context, storeID int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.AcquireMerchantProfileUpdateLock(ctx, storeID)
}

func (r *payuniRepository) InsertStatusHistory(ctx context.Context, h *domain.PayuniStatusHistory) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.InsertPayuniStatusHistory(ctx, gen.InsertPayuniStatusHistoryParams{
		ID:              h.ID,
		AccountID:       h.AccountID,
		StoreID:         h.StoreID,
		PayuniMerID:     h.PayuniMerID,
		Source:          h.Source,
		MerStatus:       nullableText(h.MerStatus),
		MemberStatus:    nullableText(h.MemberStatus),
		CreditStatus:    nullableText(h.CreditStatus),
		ApplepayStatus:  nullableText(h.ApplePayStatus),
		GooglepayStatus: nullableText(h.GooglePayStatus),
		AtmStatus:       nullableText(h.ATMStatus),
		Reason:          nullableText(h.Reason),
		RawResponse:     pgxutil.JSONBytes(h.RawResponse),
		RecordedAt:      timestamptzParam(h.RecordedAt),
	})
}

func (r *payuniRepository) DeleteStatusHistoryOlderThan(ctx context.Context, before time.Time) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.DeletePayuniStatusHistoryOlderThan(ctx, timestamptzParam(before))
}
