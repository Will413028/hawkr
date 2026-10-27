package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

var ErrBuyerQuoteAcceptanceNotFound = errors.New("procurement: buyer quote acceptance not found")

var ErrBuyerQuoteAcceptanceStale = errors.New("procurement: buyer quote acceptance changed since it was read")

var ErrBuyerQuoteAcceptanceIdempotencyConflict = errors.New("procurement: idempotency key already used for a different acceptance request")

var ErrRequestAlreadyHasLiveAcceptance = errors.New("procurement: request already has a live acceptance")

const (
	AcceptanceStateActive           = "ACTIVE"
	AcceptanceStatePaymentFinalized = "PAYMENT_FINALIZED"
	AcceptanceStateCancelled        = "CANCELLED"
	AcceptanceStateExpired          = "EXPIRED"
)

const (
	OrderProvisioningCreating = "CREATING_ORDER"
	OrderProvisioningCreated  = "ORDER_CREATED"
	OrderProvisioningFailed   = "ORDER_FAILED"
)

func isUniqueViolation(err error) (constraintName string, ok bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName, true
	}
	return "", false
}

type BuyerShortlistEntry struct {
	ID                    int64
	BuyerID               identity.BuyerID
	RequestID             int64
	SellerResponseID      int64
	SellerResponseVersion int32
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type BuyerQuoteAcceptance struct {
	ID                        int64
	Version                   int32
	BuyerID                   identity.BuyerID
	RequestID                 int64
	PublicationID             int64
	SellerResponseID          int64
	SellerResponseVersion     int32
	SellerResponsePayloadHash string
	ConfirmedRfqSnapshotID    int64
	ConfirmedRfqPayloadHash   string
	Lane                      string
	StoreID                   *int64
	ProspectID                *int64
	AcceptedDealPrice         int64
	Currency                  string
	GoodsAmount               int64
	ShippingAmount            int64
	TaxAmount                 int64
	State                     string
	OrderProvisioningState    string
	// OrderFailureReason 只有在 OrderProvisioningState == ORDER_FAILED 時有值
	// （doc 64 §6.4.14 的五個粗粒度成因）。細節在
	// procurement_order_provisioning_outbox.last_error，不在這裡。
	OrderFailureReason *string
	OrderNo            *string
	PaymentDeadline    *time.Time
	ClientRequestID    string
	RequestHash        string
	AcceptedAt         time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (a BuyerQuoteAcceptance) Live() bool {
	return a.State == AcceptanceStateActive || a.State == AcceptanceStatePaymentFinalized
}

type InsertBuyerQuoteAcceptanceInput struct {
	BuyerID                   identity.BuyerID
	RequestID                 int64
	PublicationID             int64
	SellerResponseID          int64
	SellerResponseVersion     int32
	SellerResponsePayloadHash string
	ConfirmedRfqSnapshotID    int64
	ConfirmedRfqPayloadHash   string
	Lane                      string
	StoreID                   *int64
	ProspectID                *int64
	AcceptedDealPrice         int64
	Currency                  string
	GoodsAmount               int64
	ShippingAmount            int64
	TaxAmount                 int64
	ClientRequestID           string
	RequestHash               string
	AcceptedAt                time.Time
}

func toBuyerShortlistEntry(row gen.ProcurementBuyerShortlistEntry) BuyerShortlistEntry {
	return BuyerShortlistEntry{
		ID:                    row.ID,
		BuyerID:               identity.BuyerID(row.BuyerID),
		RequestID:             row.RequestID,
		SellerResponseID:      row.SellerResponseID,
		SellerResponseVersion: row.SellerResponseVersion,
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
	}
}

func toBuyerQuoteAcceptance(row gen.ProcurementBuyerQuoteAcceptance) BuyerQuoteAcceptance {
	return BuyerQuoteAcceptance{
		ID:                        row.ID,
		Version:                   row.Version,
		BuyerID:                   identity.BuyerID(row.BuyerID),
		RequestID:                 row.RequestID,
		PublicationID:             row.PublicationID,
		SellerResponseID:          row.SellerResponseID,
		SellerResponseVersion:     row.SellerResponseVersion,
		SellerResponsePayloadHash: row.SellerResponsePayloadHash,
		ConfirmedRfqSnapshotID:    row.ConfirmedRfqSnapshotID,
		ConfirmedRfqPayloadHash:   row.ConfirmedRfqPayloadHash,
		Lane:                      row.Lane,
		StoreID:                   row.StoreID,
		ProspectID:                row.ProspectID,
		AcceptedDealPrice:         row.AcceptedDealPrice,
		Currency:                  row.Currency,
		GoodsAmount:               row.GoodsAmount,
		ShippingAmount:            row.ShippingAmount,
		TaxAmount:                 row.TaxAmount,
		State:                     row.State,
		OrderProvisioningState:    row.OrderProvisioningState,
		OrderFailureReason:        row.OrderFailureReason,
		OrderNo:                   row.OrderNo,
		PaymentDeadline:           row.PaymentDeadline,
		ClientRequestID:           row.ClientRequestID,
		RequestHash:               row.RequestHash,
		AcceptedAt:                row.AcceptedAt.Time,
		CreatedAt:                 row.CreatedAt.Time,
		UpdatedAt:                 row.UpdatedAt.Time,
	}
}

func (r *Repo) PutShortlistEntry(
	ctx context.Context, buyerID identity.BuyerID, requestID, sellerResponseID int64, sellerResponseVersion int32,
) (BuyerShortlistEntry, error) {
	row, err := r.q(ctx).UpsertProcurementBuyerShortlistEntry(ctx, gen.UpsertProcurementBuyerShortlistEntryParams{
		ID:                    r.newID(),
		BuyerID:               int64(buyerID),
		RequestID:             requestID,
		SellerResponseID:      sellerResponseID,
		SellerResponseVersion: sellerResponseVersion,
	})
	if err != nil {
		return BuyerShortlistEntry{}, err
	}
	return toBuyerShortlistEntry(row), nil
}

func (r *Repo) DeleteShortlistEntry(
	ctx context.Context, buyerID identity.BuyerID, requestID, sellerResponseID int64,
) (bool, error) {
	affected, err := r.q(ctx).DeleteProcurementBuyerShortlistEntry(ctx, gen.DeleteProcurementBuyerShortlistEntryParams{
		RequestID: requestID, SellerResponseID: sellerResponseID, BuyerID: int64(buyerID),
	})
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *Repo) ListShortlistEntries(ctx context.Context, buyerID identity.BuyerID, requestID int64) ([]BuyerShortlistEntry, error) {
	rows, err := r.q(ctx).ListProcurementBuyerShortlistEntries(ctx, gen.ListProcurementBuyerShortlistEntriesParams{
		RequestID: requestID, BuyerID: int64(buyerID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]BuyerShortlistEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBuyerShortlistEntry(row))
	}
	return out, nil
}

func (r *Repo) InsertBuyerQuoteAcceptance(ctx context.Context, in InsertBuyerQuoteAcceptanceInput) (BuyerQuoteAcceptance, error) {
	id := r.newID()
	row, err := r.q(ctx).InsertProcurementBuyerQuoteAcceptance(ctx, gen.InsertProcurementBuyerQuoteAcceptanceParams{
		ID:                        id,
		BuyerID:                   int64(in.BuyerID),
		RequestID:                 in.RequestID,
		PublicationID:             in.PublicationID,
		SellerResponseID:          in.SellerResponseID,
		SellerResponseVersion:     in.SellerResponseVersion,
		SellerResponsePayloadHash: in.SellerResponsePayloadHash,
		ConfirmedRfqSnapshotID:    in.ConfirmedRfqSnapshotID,
		ConfirmedRfqPayloadHash:   in.ConfirmedRfqPayloadHash,
		Lane:                      in.Lane,
		StoreID:                   in.StoreID,
		ProspectID:                in.ProspectID,
		AcceptedDealPrice:         in.AcceptedDealPrice,
		Currency:                  in.Currency,
		GoodsAmount:               in.GoodsAmount,
		ShippingAmount:            in.ShippingAmount,
		TaxAmount:                 in.TaxAmount,
		ClientRequestID:           in.ClientRequestID,
		RequestHash:               in.RequestHash,
		AcceptedAt:                timestamptzParam(in.AcceptedAt),
	})
	if err != nil {
		if name, ok := isUniqueViolation(err); ok && name == "uq_prbqa_request_live" {
			return BuyerQuoteAcceptance{}, ErrRequestAlreadyHasLiveAcceptance
		}
		return BuyerQuoteAcceptance{}, err
	}
	acceptance := toBuyerQuoteAcceptance(row)
	if row.ID != id && acceptance.RequestHash != in.RequestHash {
		return BuyerQuoteAcceptance{}, ErrBuyerQuoteAcceptanceIdempotencyConflict
	}
	return acceptance, nil
}

func (r *Repo) FindBuyerQuoteAcceptanceByIdempotencyKey(
	ctx context.Context, buyerID identity.BuyerID, clientRequestID string,
) (BuyerQuoteAcceptance, error) {
	row, err := r.q(ctx).FindProcurementBuyerQuoteAcceptanceByIdempotencyKey(ctx, gen.FindProcurementBuyerQuoteAcceptanceByIdempotencyKeyParams{
		BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyerQuoteAcceptance{}, ErrBuyerQuoteAcceptanceNotFound
		}
		return BuyerQuoteAcceptance{}, err
	}
	return toBuyerQuoteAcceptance(row), nil
}

func (r *Repo) FindBuyerQuoteAcceptanceByIDAndBuyer(
	ctx context.Context, id int64, buyerID identity.BuyerID,
) (BuyerQuoteAcceptance, error) {
	row, err := r.q(ctx).FindProcurementBuyerQuoteAcceptanceByIDAndBuyer(ctx, gen.FindProcurementBuyerQuoteAcceptanceByIDAndBuyerParams{
		ID: id, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyerQuoteAcceptance{}, ErrBuyerQuoteAcceptanceNotFound
		}
		return BuyerQuoteAcceptance{}, err
	}
	return toBuyerQuoteAcceptance(row), nil
}

func (r *Repo) FindLiveBuyerQuoteAcceptanceForRequest(
	ctx context.Context, buyerID identity.BuyerID, requestID int64,
) (BuyerQuoteAcceptance, error) {
	row, err := r.q(ctx).FindLiveProcurementBuyerQuoteAcceptanceForRequest(ctx, gen.FindLiveProcurementBuyerQuoteAcceptanceForRequestParams{
		RequestID: requestID, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyerQuoteAcceptance{}, ErrBuyerQuoteAcceptanceNotFound
		}
		return BuyerQuoteAcceptance{}, err
	}
	return toBuyerQuoteAcceptance(row), nil
}

func (r *Repo) AttachOrderToAcceptance(
	ctx context.Context, id int64, expectedVersion int32, orderNo string, paymentDeadline time.Time,
) (BuyerQuoteAcceptance, error) {
	row, err := r.q(ctx).AttachOrderToProcurementBuyerQuoteAcceptance(ctx, gen.AttachOrderToProcurementBuyerQuoteAcceptanceParams{
		ID: id, Version: expectedVersion, OrderNo: &orderNo, PaymentDeadline: &paymentDeadline,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyerQuoteAcceptance{}, ErrBuyerQuoteAcceptanceStale
		}
		return BuyerQuoteAcceptance{}, err
	}
	return toBuyerQuoteAcceptance(row), nil
}

// MarkAcceptanceOrderFailed 在**知道 version** 的路徑上把 acceptance 轉 ORDER_FAILED。
// reason 是 doc 64 §6.4.14 的粗粒度成因，⛔ 不得傳空字串——分不出成因正是本欄要解的問題。
func (r *Repo) MarkAcceptanceOrderFailed(
	ctx context.Context, id int64, expectedVersion int32, reason string,
) (BuyerQuoteAcceptance, error) {
	if reason == "" {
		return BuyerQuoteAcceptance{}, errors.New("procurement: order failure reason must not be empty")
	}
	row, err := r.q(ctx).MarkProcurementBuyerQuoteAcceptanceOrderFailed(ctx, gen.MarkProcurementBuyerQuoteAcceptanceOrderFailedParams{
		ID: id, Version: expectedVersion, OrderFailureReason: &reason,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyerQuoteAcceptance{}, ErrBuyerQuoteAcceptanceStale
		}
		return BuyerQuoteAcceptance{}, err
	}
	return toBuyerQuoteAcceptance(row), nil
}

// MarkAcceptanceOrderFailedByOutboxID 供 outbox worker 的終態路徑使用：呼叫端手上只有
// outbox 列的 id，CAS 述詞改用 order_provisioning_state（CREATING_ORDER 是唯一的非終態
// ⇒ 述詞本身就表達「還沒有人寫過」）。影響 0 列不是錯誤——代表別的副本先寫了。
//
// ⚠️ 它必須與「outbox 列轉 failed」在**同一個交易**內執行，呼叫端負責開交易；本方法的
// r.q(ctx) 走 database.GetDBTX，ctx 上有交易就用交易。
func (r *Repo) MarkAcceptanceOrderFailedByOutboxID(ctx context.Context, outboxID int64, reason string) error {
	if reason == "" {
		return errors.New("procurement: order failure reason must not be empty")
	}
	return r.q(ctx).MarkProcurementBuyerQuoteAcceptanceOrderFailedByOutboxID(ctx,
		gen.MarkProcurementBuyerQuoteAcceptanceOrderFailedByOutboxIDParams{
			ID: outboxID, OrderFailureReason: &reason,
		})
}

func (r *Repo) ReleaseAcceptance(ctx context.Context, id int64, expectedVersion int32, state string) (BuyerQuoteAcceptance, error) {
	if state != AcceptanceStateCancelled && state != AcceptanceStateExpired {
		return BuyerQuoteAcceptance{}, errors.New("procurement: release state must be CANCELLED or EXPIRED, got " + state)
	}
	row, err := r.q(ctx).ReleaseProcurementBuyerQuoteAcceptance(ctx, gen.ReleaseProcurementBuyerQuoteAcceptanceParams{
		ID: id, Version: expectedVersion, State: state,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyerQuoteAcceptance{}, ErrBuyerQuoteAcceptanceStale
		}
		return BuyerQuoteAcceptance{}, err
	}
	return toBuyerQuoteAcceptance(row), nil
}

func (r *Repo) MarkAcceptancePaymentFinalized(ctx context.Context, id int64, expectedVersion int32) (BuyerQuoteAcceptance, error) {
	row, err := r.q(ctx).MarkProcurementBuyerQuoteAcceptancePaymentFinalized(ctx, gen.MarkProcurementBuyerQuoteAcceptancePaymentFinalizedParams{
		ID: id, Version: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyerQuoteAcceptance{}, ErrBuyerQuoteAcceptanceStale
		}
		return BuyerQuoteAcceptance{}, err
	}
	return toBuyerQuoteAcceptance(row), nil
}
