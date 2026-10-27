package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
)

var ErrSellerResponseNotFound = errors.New("procurement: seller response not found")

var ErrSellerResponseStale = errors.New("procurement: seller response changed since it was read")

var ErrSellerResponseDraftStale = errors.New("procurement: seller response draft changed since it was read")

var ErrSellerResponseWriteClosed = errors.New("procurement: seller response write closed")

var ErrSellerResponseIdempotencyConflict = errors.New("procurement: idempotency key already used for a different seller response submission")

const (
	SellerResponseLanePlatform = "PLATFORM"
	SellerResponseLaneExternal = "EXTERNAL"
)

const (
	SellerResponseClosedDeclined          = "DECLINED"
	SellerResponseClosedPublicationClosed = "PUBLICATION_CLOSED"
	SellerResponseClosedOtherWon          = "OTHER_WON"
)

type SellerResponseIdentity struct {
	ID                int64
	Version           int32
	PublicationID     int64
	BuyerID           identity.BuyerID
	RequestID         int64
	Lane              string
	StoreID           *int64
	ProspectID        *int64
	LatestVersion     int32
	WithdrawnVersion  *int32
	WithdrawnAt       *time.Time
	WriteClosedAt     *time.Time
	WriteClosedReason *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (s SellerResponseIdentity) HasSubmission() bool { return s.LatestVersion > 0 }

func (s SellerResponseIdentity) WriteClosed() bool { return s.WriteClosedAt != nil }

const (
	SellerResponseCommandWithdraw = "WITHDRAW"
	SellerResponseCommandDecline  = "DECLINE"
)

const VerificationMethodGoogle = "GOOGLE"

// SellerResponseVerification 是站外業者的驗證紀錄；四個欄位一起存在或一起不存在。
type SellerResponseVerification struct {
	Method     string
	Email      string
	VerifiedAt time.Time
	SessionID  string
}

func (v SellerResponseVerification) Complete() bool {
	return v.Method == VerificationMethodGoogle && v.Email != "" && !v.VerifiedAt.IsZero() && v.SessionID != ""
}

// SellerResponseCommand 是撤回或婉拒的只追加指令紀錄。
type SellerResponseCommand struct {
	ID               int64
	SellerResponseID int64
	Kind             string
	WithdrawnVersion *int32
	ClientRequestID  string
	ReasonCode       string
	Verification     *SellerResponseVerification
	CreatedAt        time.Time
}

type InsertSellerResponseCommandInput struct {
	SellerResponseID int64
	Kind             string
	WithdrawnVersion *int32
	ClientRequestID  string
	ReasonCode       string
	Verification     *SellerResponseVerification
}

type SellerResponseVersion struct {
	ID                            int64
	SellerResponseID              int64
	Version                       int32
	ResponseType                  string
	OfferedQualityGrade           string
	OfferedQualityInspectionNotes string
	UnitBasisQuantity             string
	UnitBasisUnit                 string
	OfferedQuantity               string
	OfferedQuantityUnit           string
	ConversionBasis               string
	Currency                      string
	GoodsAmount                   int64
	ShippingAmount                int64
	TaxAmount                     int64
	TaxInclusionBasis             string
	TotalAmount                   int64
	LeadTimeDays                  *int32
	PromisedDeliveryAt            *time.Time
	ValidUntil                    time.Time
	TermsNote                     string
	DeviationDisclosure           string
	OutsideBuyerRequestedRange    bool
	PayloadHash                   string
	PublicationVersion            int32
	SellerFacingPayloadHash       string
	ClientRequestID               string
	SubmittedAt                   time.Time
	Verification                  *SellerResponseVerification
}

type SellerResponseDraft struct {
	SellerResponseID int64
	Payload          json.RawMessage
	Version          int32
	UpdatedAt        time.Time
}

type InsertSellerResponseVersionInput struct {
	SellerResponseID              int64
	Version                       int32
	ResponseType                  string
	OfferedQualityGrade           string
	OfferedQualityInspectionNotes string
	UnitBasisQuantity             string
	UnitBasisUnit                 string
	OfferedQuantity               string
	OfferedQuantityUnit           string
	ConversionBasis               string
	Currency                      string
	GoodsAmount                   int64
	ShippingAmount                int64
	TaxAmount                     int64
	TaxInclusionBasis             string
	TotalAmount                   int64
	LeadTimeDays                  *int32
	PromisedDeliveryAt            *time.Time
	ValidUntil                    time.Time
	TermsNote                     string
	DeviationDisclosure           string
	OutsideBuyerRequestedRange    bool
	PayloadHash                   string
	PublicationVersion            int32
	SellerFacingPayloadHash       string
	ClientRequestID               string
	SubmittedAt                   time.Time
	Verification                  *SellerResponseVerification
}

func toSellerResponseIdentity(row gen.ProcurementSellerResponse) SellerResponseIdentity {
	return SellerResponseIdentity{
		ID:                row.ID,
		Version:           row.Version,
		PublicationID:     row.PublicationID,
		BuyerID:           identity.BuyerID(row.BuyerID),
		RequestID:         row.RequestID,
		Lane:              row.Lane,
		StoreID:           row.StoreID,
		ProspectID:        row.ProspectID,
		LatestVersion:     row.LatestVersion,
		WithdrawnVersion:  row.WithdrawnVersion,
		WithdrawnAt:       row.WithdrawnAt,
		WriteClosedAt:     row.WriteClosedAt,
		WriteClosedReason: row.WriteClosedReason,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func toSellerResponseVerification(method, email *string, verifiedAt *time.Time, sessionID *string) *SellerResponseVerification {
	if method == nil || email == nil || verifiedAt == nil || sessionID == nil {
		return nil
	}
	return &SellerResponseVerification{Method: *method, Email: *email, VerifiedAt: *verifiedAt, SessionID: *sessionID}
}

func verificationParams(v *SellerResponseVerification) (method, email *string, verifiedAt *time.Time, sessionID *string) {
	if v == nil {
		return nil, nil, nil, nil
	}
	at := v.VerifiedAt.UTC()
	return &v.Method, &v.Email, &at, &v.SessionID
}

func toSellerResponseCommand(row gen.ProcurementSellerResponseCommand) SellerResponseCommand {
	return SellerResponseCommand{
		ID:               row.ID,
		SellerResponseID: row.SellerResponseID,
		Kind:             row.Kind,
		WithdrawnVersion: row.WithdrawnVersion,
		ClientRequestID:  row.ClientRequestID,
		ReasonCode:       row.ReasonCode,
		Verification:     toSellerResponseVerification(row.VerificationMethod, row.VerificationEmail, row.VerifiedAt, row.VerificationSessionID),
		CreatedAt:        row.CreatedAt.Time,
	}
}

func toSellerResponseVersion(row gen.ProcurementSellerResponseVersion) SellerResponseVersion {
	return SellerResponseVersion{
		ID:                            row.ID,
		SellerResponseID:              row.SellerResponseID,
		Version:                       row.Version,
		ResponseType:                  row.ResponseType,
		OfferedQualityGrade:           row.OfferedQualityGrade,
		OfferedQualityInspectionNotes: row.OfferedQualityInspectionNotes,
		UnitBasisQuantity:             pgxutil.NumericToCanonicalString(row.UnitBasisQuantity),
		UnitBasisUnit:                 row.UnitBasisUnit,
		OfferedQuantity:               pgxutil.NumericToCanonicalString(row.OfferedQuantity),
		OfferedQuantityUnit:           row.OfferedQuantityUnit,
		ConversionBasis:               row.ConversionBasis,
		Currency:                      row.Currency,
		GoodsAmount:                   row.GoodsAmount,
		ShippingAmount:                row.ShippingAmount,
		TaxAmount:                     row.TaxAmount,
		TaxInclusionBasis:             row.TaxInclusionBasis,
		TotalAmount:                   row.TotalAmount,
		LeadTimeDays:                  row.LeadTimeDays,
		PromisedDeliveryAt:            row.PromisedDeliveryAt,
		ValidUntil:                    row.ValidUntil.Time,
		TermsNote:                     row.TermsNote,
		DeviationDisclosure:           row.DeviationDisclosure,
		OutsideBuyerRequestedRange:    row.OutsideBuyerRequestedRange,
		PayloadHash:                   row.PayloadHash,
		PublicationVersion:            row.PublicationVersion,
		SellerFacingPayloadHash:       row.SellerFacingPayloadHash,
		ClientRequestID:               row.ClientRequestID,
		SubmittedAt:                   row.SubmittedAt.Time,
		Verification:                  toSellerResponseVerification(row.VerificationMethod, row.VerificationEmail, row.VerifiedAt, row.VerificationSessionID),
	}
}

func (r *Repo) EnsureSellerResponseIdentityForStore(
	ctx context.Context, publicationID int64, buyerID identity.BuyerID, requestID int64, storeID int64,
) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).UpsertProcurementSellerResponseIdentityPlatform(ctx, gen.UpsertProcurementSellerResponseIdentityPlatformParams{
		ID:            r.newID(),
		PublicationID: publicationID,
		BuyerID:       int64(buyerID),
		RequestID:     requestID,
		StoreID:       &storeID,
	})
	if err != nil {
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

func (r *Repo) EnsureSellerResponseIdentityForProspect(
	ctx context.Context, publicationID int64, buyerID identity.BuyerID, requestID int64, prospectID int64,
) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).UpsertProcurementSellerResponseIdentityExternal(ctx, gen.UpsertProcurementSellerResponseIdentityExternalParams{
		ID:            r.newID(),
		PublicationID: publicationID,
		BuyerID:       int64(buyerID),
		RequestID:     requestID,
		ProspectID:    &prospectID,
	})
	if err != nil {
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

func (r *Repo) FindSellerResponseByID(ctx context.Context, id int64) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).FindProcurementSellerResponseByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseIdentity{}, ErrSellerResponseNotFound
		}
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

// FindSellerResponseByIDForUpdate 鎖住 identity 列再回傳。⛔ 只能在 transaction 內
// 呼叫：`r.q(ctx)` 會取 context 上的 tx，在交易外呼叫等於鎖一開就放掉，讀到的值仍是
// 可被別人在寫入前改掉的（doc 64 §2-3）。
func (r *Repo) FindSellerResponseByIDForUpdate(ctx context.Context, id int64) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).FindProcurementSellerResponseByIDForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseIdentity{}, ErrSellerResponseNotFound
		}
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

func (r *Repo) FindSellerResponseByStoreIdentity(ctx context.Context, publicationID, storeID int64) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).FindProcurementSellerResponseByPlatformIdentity(ctx, gen.FindProcurementSellerResponseByPlatformIdentityParams{
		PublicationID: publicationID, StoreID: &storeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseIdentity{}, ErrSellerResponseNotFound
		}
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

func (r *Repo) FindSellerResponseByProspectIdentity(ctx context.Context, publicationID, prospectID int64) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).FindProcurementSellerResponseByExternalIdentity(ctx, gen.FindProcurementSellerResponseByExternalIdentityParams{
		PublicationID: publicationID, ProspectID: &prospectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseIdentity{}, ErrSellerResponseNotFound
		}
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

func (r *Repo) UpsertSellerResponseDraft(
	ctx context.Context, sellerResponseID int64, payload json.RawMessage, expectedVersion int32,
) (SellerResponseDraft, error) {
	row, err := r.q(ctx).UpsertProcurementSellerResponseDraft(ctx, gen.UpsertProcurementSellerResponseDraftParams{
		SellerResponseID: sellerResponseID,
		Payload:          payload,
		Version:          expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseDraft{}, ErrSellerResponseDraftStale
		}
		return SellerResponseDraft{}, err
	}
	return SellerResponseDraft{
		SellerResponseID: row.SellerResponseID,
		Payload:          row.Payload,
		Version:          row.Version,
		UpdatedAt:        row.UpdatedAt.Time,
	}, nil
}

func (r *Repo) FindSellerResponseDraft(ctx context.Context, sellerResponseID int64) (SellerResponseDraft, error) {
	row, err := r.q(ctx).FindProcurementSellerResponseDraft(ctx, sellerResponseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseDraft{}, ErrSellerResponseNotFound
		}
		return SellerResponseDraft{}, err
	}
	return SellerResponseDraft{
		SellerResponseID: row.SellerResponseID,
		Payload:          row.Payload,
		Version:          row.Version,
		UpdatedAt:        row.UpdatedAt.Time,
	}, nil
}

func (r *Repo) InsertSellerResponseVersion(ctx context.Context, in InsertSellerResponseVersionInput) (SellerResponseVersion, error) {
	unitBasisQty, err := pgxutil.NumericFromString(in.UnitBasisQuantity)
	if err != nil {
		return SellerResponseVersion{}, err
	}
	offeredQty, err := pgxutil.NumericFromString(in.OfferedQuantity)
	if err != nil {
		return SellerResponseVersion{}, err
	}

	id := r.newID()
	vMethod, vEmail, vAt, vSession := verificationParams(in.Verification)
	row, err := r.q(ctx).InsertProcurementSellerResponseVersion(ctx, gen.InsertProcurementSellerResponseVersionParams{
		ID:                            id,
		SellerResponseID:              in.SellerResponseID,
		Version:                       in.Version,
		ResponseType:                  in.ResponseType,
		OfferedQualityGrade:           in.OfferedQualityGrade,
		OfferedQualityInspectionNotes: in.OfferedQualityInspectionNotes,
		UnitBasisQuantity:             unitBasisQty,
		UnitBasisUnit:                 in.UnitBasisUnit,
		OfferedQuantity:               offeredQty,
		OfferedQuantityUnit:           in.OfferedQuantityUnit,
		ConversionBasis:               in.ConversionBasis,
		Currency:                      in.Currency,
		GoodsAmount:                   in.GoodsAmount,
		ShippingAmount:                in.ShippingAmount,
		TaxAmount:                     in.TaxAmount,
		TaxInclusionBasis:             in.TaxInclusionBasis,
		TotalAmount:                   in.TotalAmount,
		LeadTimeDays:                  in.LeadTimeDays,
		PromisedDeliveryAt:            in.PromisedDeliveryAt,
		ValidUntil:                    timestamptzParam(in.ValidUntil),
		TermsNote:                     in.TermsNote,
		DeviationDisclosure:           in.DeviationDisclosure,
		OutsideBuyerRequestedRange:    in.OutsideBuyerRequestedRange,
		PayloadHash:                   in.PayloadHash,
		PublicationVersion:            in.PublicationVersion,
		SellerFacingPayloadHash:       in.SellerFacingPayloadHash,
		ClientRequestID:               in.ClientRequestID,
		SubmittedAt:                   timestamptzParam(in.SubmittedAt),
		VerificationMethod:            vMethod,
		VerificationEmail:             vEmail,
		VerifiedAt:                    vAt,
		VerificationSessionID:         vSession,
	})
	if err != nil {
		return SellerResponseVersion{}, err
	}
	version := toSellerResponseVersion(row)
	if row.ID != id && version.PayloadHash != in.PayloadHash {
		return SellerResponseVersion{}, ErrSellerResponseIdempotencyConflict
	}
	return version, nil
}

func (r *Repo) FindSellerResponseVersion(ctx context.Context, sellerResponseID int64, version int32) (SellerResponseVersion, error) {
	row, err := r.q(ctx).FindProcurementSellerResponseVersion(ctx, gen.FindProcurementSellerResponseVersionParams{
		SellerResponseID: sellerResponseID, Version: version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseVersion{}, ErrSellerResponseNotFound
		}
		return SellerResponseVersion{}, err
	}
	return toSellerResponseVersion(row), nil
}

// FindSellerResponseVersionByIdempotencyKey 找出這把冪等鍵當初寫下的版本列。
// 找不到回 ErrSellerResponseNotFound——呼叫端用它區分「第一次送」與「重送」。
func (r *Repo) FindSellerResponseVersionByIdempotencyKey(
	ctx context.Context, sellerResponseID int64, clientRequestID string,
) (SellerResponseVersion, error) {
	row, err := r.q(ctx).FindProcurementSellerResponseVersionByIdempotencyKey(ctx, gen.FindProcurementSellerResponseVersionByIdempotencyKeyParams{
		SellerResponseID: sellerResponseID, ClientRequestID: clientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseVersion{}, ErrSellerResponseNotFound
		}
		return SellerResponseVersion{}, err
	}
	return toSellerResponseVersion(row), nil
}

// InsertSellerResponseCommand 追加一筆撤回或婉拒的指令紀錄。撞到 (seller_response_id,
// client_request_id) 時不寫入，回既有那列與 created=false，由呼叫端比對內容判斷是回放還是衝突。
func (r *Repo) InsertSellerResponseCommand(ctx context.Context, in InsertSellerResponseCommandInput) (cmd SellerResponseCommand, created bool, err error) {
	vMethod, vEmail, vAt, vSession := verificationParams(in.Verification)
	row, err := r.q(ctx).InsertProcurementSellerResponseCommand(ctx, gen.InsertProcurementSellerResponseCommandParams{
		ID:                    r.newID(),
		SellerResponseID:      in.SellerResponseID,
		Kind:                  in.Kind,
		WithdrawnVersion:      in.WithdrawnVersion,
		ClientRequestID:       in.ClientRequestID,
		ReasonCode:            in.ReasonCode,
		VerificationMethod:    vMethod,
		VerificationEmail:     vEmail,
		VerifiedAt:            vAt,
		VerificationSessionID: vSession,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, findErr := r.FindSellerResponseCommandByIdempotencyKey(ctx, in.SellerResponseID, in.ClientRequestID)
		return existing, false, findErr
	}
	if err != nil {
		return SellerResponseCommand{}, false, err
	}
	return toSellerResponseCommand(row), true, nil
}

// FindSellerResponseCommandByIdempotencyKey 找出這把冪等鍵當初寫下的指令紀錄；
// 找不到回 ErrSellerResponseNotFound。
func (r *Repo) FindSellerResponseCommandByIdempotencyKey(
	ctx context.Context, sellerResponseID int64, clientRequestID string,
) (SellerResponseCommand, error) {
	row, err := r.q(ctx).FindProcurementSellerResponseCommandByIdempotencyKey(ctx, gen.FindProcurementSellerResponseCommandByIdempotencyKeyParams{
		SellerResponseID: sellerResponseID, ClientRequestID: clientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseCommand{}, ErrSellerResponseNotFound
		}
		return SellerResponseCommand{}, err
	}
	return toSellerResponseCommand(row), nil
}

func (r *Repo) ListSellerResponseCommands(ctx context.Context, sellerResponseID int64) ([]SellerResponseCommand, error) {
	rows, err := r.q(ctx).ListProcurementSellerResponseCommands(ctx, sellerResponseID)
	if err != nil {
		return nil, err
	}
	out := make([]SellerResponseCommand, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSellerResponseCommand(row))
	}
	return out, nil
}

func (r *Repo) ListSellerResponseVersions(ctx context.Context, sellerResponseID int64) ([]SellerResponseVersion, error) {
	rows, err := r.q(ctx).ListProcurementSellerResponseVersions(ctx, sellerResponseID)
	if err != nil {
		return nil, err
	}
	out := make([]SellerResponseVersion, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSellerResponseVersion(row))
	}
	return out, nil
}

func (r *Repo) BumpSellerResponseLatestVersion(
	ctx context.Context, sellerResponseID int64, expectedVersion, latestVersion int32,
) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).BumpProcurementSellerResponseLatestVersion(ctx, gen.BumpProcurementSellerResponseLatestVersionParams{
		ID: sellerResponseID, Version: expectedVersion, LatestVersion: latestVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseIdentity{}, r.classifySellerResponseWriteMiss(ctx, sellerResponseID)
		}
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

func (r *Repo) WithdrawSellerResponse(
	ctx context.Context, sellerResponseID int64, expectedVersion, expectedLatestVersion int32,
) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).WithdrawProcurementSellerResponse(ctx, gen.WithdrawProcurementSellerResponseParams{
		ID: sellerResponseID, Version: expectedVersion, WithdrawnVersion: &expectedLatestVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseIdentity{}, r.classifySellerResponseWriteMiss(ctx, sellerResponseID)
		}
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

func (r *Repo) CloseSellerResponseWrite(
	ctx context.Context, sellerResponseID int64, expectedVersion int32, reason string,
) (SellerResponseIdentity, error) {
	row, err := r.q(ctx).CloseProcurementSellerResponseWrite(ctx, gen.CloseProcurementSellerResponseWriteParams{
		ID: sellerResponseID, Version: expectedVersion, WriteClosedReason: &reason,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerResponseIdentity{}, r.classifySellerResponseWriteMiss(ctx, sellerResponseID)
		}
		return SellerResponseIdentity{}, err
	}
	return toSellerResponseIdentity(row), nil
}

func (r *Repo) classifySellerResponseWriteMiss(ctx context.Context, sellerResponseID int64) error {
	current, findErr := r.FindSellerResponseByID(ctx, sellerResponseID)
	if findErr != nil {
		return findErr
	}
	if current.WriteClosed() {
		return ErrSellerResponseWriteClosed
	}
	return ErrSellerResponseStale
}

// SellerResponseVersionWithIdentity 是買家比較清單的一列：immutable 版本本體，
// 加上推導 acceptability 需要的身分列欄位（latest_version／withdrawn_version）。
type SellerResponseVersionWithIdentity struct {
	Identity SellerResponseIdentity
	Version  SellerResponseVersion
}

func (r *Repo) ListSellerResponseVersionsForRequest(
	ctx context.Context, buyerID identity.BuyerID, requestID int64,
) ([]SellerResponseVersionWithIdentity, error) {
	rows, err := r.q(ctx).ListProcurementSellerResponseVersionsForRequest(ctx, gen.ListProcurementSellerResponseVersionsForRequestParams{
		RequestID: requestID, BuyerID: int64(buyerID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]SellerResponseVersionWithIdentity, 0, len(rows))
	for _, row := range rows {
		out = append(out, SellerResponseVersionWithIdentity{
			Identity: toSellerResponseIdentity(row.ProcurementSellerResponse),
			Version:  toSellerResponseVersion(row.ProcurementSellerResponseVersion),
		})
	}
	return out, nil
}
