package pricing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

var (
	ErrBuyerRequestedRangeRequired = errors.New("procurement: buyerRequestedRange is missing or invalid")
	ErrQuantityUnitUnconvertible   = errors.New("procurement: quantity and unit cannot be converted to a quote total range")
	ErrStalePricePreviewRevision   = errors.New("procurement: expectedRevisionId doesn't match current active revision")
)

const pricePreviewTTL = 15 * time.Minute

type BuyerRequestedRange struct {
	Currency         string `json:"currency"`
	PriceComponent   string `json:"priceComponent"`
	BasisQuantity    string `json:"basisQuantity"`
	BasisUnit        string `json:"basisUnit"`
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
}

func (r BuyerRequestedRange) IsUnset() bool {
	return r.Currency == "" && r.PriceComponent == "" &&
		r.LowerBoundAmount == "" && r.UpperBoundAmount == ""
}

type QuoteTotalRange struct {
	Currency         string `json:"currency"`
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
	Quantity         string `json:"quantity"`
	QuantityUnit     string `json:"quantityUnit"`
}

type PreviewRfqPriceInput struct {
	RequestID           int64
	BuyerID             identity.BuyerID
	ClientRequestID     string
	ExpectedRevisionID  int64
	DeliveryCity        string
	BuyerRequestedRange *BuyerRequestedRange
}

type PricePreviewResult struct {
	PricePreviewID      int64
	Version             int64
	BuyerRequestedRange BuyerRequestedRange
	PayloadHash         string
	RequestID           int64
	RequestRevisionID   int64
	ExpiresAt           time.Time
	QuoteTotalRange     *QuoteTotalRange
}

type PricePreviewRepo interface {
	InsertPricePreview(ctx context.Context, in repository.InsertPricePreviewInput) (repository.PricePreview, error)
	FindPricePreviewByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.PricePreview, error)
}

type RevisionReader interface {
	GetSlots(ctx context.Context, requestID int64, buyerID identity.BuyerID) (domain.Slots, error)
	GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error)
}

type PreviewRfqPriceUseCase struct {
	repo      PricePreviewRepo
	revisions RevisionReader
}

func NewPreviewRfqPriceUseCase(repo PricePreviewRepo, revisions RevisionReader) *PreviewRfqPriceUseCase {
	return &PreviewRfqPriceUseCase{repo: repo, revisions: revisions}
}

func (uc *PreviewRfqPriceUseCase) PreviewRfqPrice(ctx context.Context, in PreviewRfqPriceInput) (PricePreviewResult, error) {
	if existing, err := uc.repo.FindPricePreviewByIdempotencyKey(ctx, in.BuyerID, in.ClientRequestID); err == nil {
		if HashJSON(in) != existing.PreviewInputHash {
			return PricePreviewResult{}, domain.ErrIdempotencyKeyConflict
		}
		return ToPricePreviewResult(existing), nil
	} else if !errors.Is(err, repository.ErrPricePreviewNotFound) {
		return PricePreviewResult{}, err
	}

	_, rev, err := uc.revisions.GetSnapshot(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return PricePreviewResult{}, err
	}
	if rev.ID != in.ExpectedRevisionID {
		return PricePreviewResult{}, ErrStalePricePreviewRevision
	}
	activeRevisionID := rev.ID

	slots, err := uc.revisions.GetSlots(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return PricePreviewResult{}, err
	}

	if err := validateBuyerRequestedRange(in.BuyerRequestedRange); err != nil {
		return PricePreviewResult{}, err
	}
	result := PricePreviewResult{BuyerRequestedRange: *in.BuyerRequestedRange}
	if slots.Unit != nil {
		result.BuyerRequestedRange.BasisQuantity, result.BuyerRequestedRange.BasisUnit = referenceBasis(*slots.Unit)
	}

	result.QuoteTotalRange = quoteTotalRange(result.BuyerRequestedRange, slots.Quantity)
	if slots.Unit != nil && slots.Quantity != nil && *slots.Quantity > 0 && result.QuoteTotalRange == nil {
		return PricePreviewResult{}, ErrQuantityUnitUnconvertible
	}

	result.PayloadHash = HashJSON(result)

	var quoteTotalRangeJSON json.RawMessage
	if result.QuoteTotalRange != nil {
		quoteTotalRangeJSON, _ = json.Marshal(result.QuoteTotalRange)
	}
	buyerRangeJSON, _ := json.Marshal(result.BuyerRequestedRange)

	inserted, err := uc.repo.InsertPricePreview(ctx, repository.InsertPricePreviewInput{
		BuyerID: in.BuyerID, RequestID: in.RequestID, RequestRevisionID: activeRevisionID,
		BuyerRequestedRangeJSON: buyerRangeJSON, QuoteTotalRangeJSON: quoteTotalRangeJSON,
		ClientRequestID:  in.ClientRequestID,
		PreviewInputHash: HashJSON(in), PayloadHash: result.PayloadHash,
		ExpiresAt: time.Now().Add(pricePreviewTTL),
	})
	if err != nil {
		if errors.Is(err, repository.ErrPricePreviewIdempotencyConflict) {
			return PricePreviewResult{}, domain.ErrIdempotencyKeyConflict
		}
		return PricePreviewResult{}, err
	}
	return ToPricePreviewResult(inserted), nil
}

func ToPricePreviewResult(row repository.PricePreview) PricePreviewResult {
	r := PricePreviewResult{
		PricePreviewID: row.ID, Version: row.Version,
		PayloadHash: row.PayloadHash,
		RequestID:   row.RequestID, RequestRevisionID: row.RequestRevisionID, ExpiresAt: row.ExpiresAt,
	}
	if len(row.QuoteTotalRangeJSON) > 0 {
		r.QuoteTotalRange = &QuoteTotalRange{}
		_ = json.Unmarshal(row.QuoteTotalRangeJSON, r.QuoteTotalRange)
	}
	_ = json.Unmarshal(row.BuyerRequestedRangeJSON, &r.BuyerRequestedRange)
	return r
}

func validateBuyerRequestedRange(r *BuyerRequestedRange) error {
	if r == nil || r.Currency != "TWD" {
		return ErrBuyerRequestedRangeRequired
	}
	lower, err := strconv.ParseInt(r.LowerBoundAmount, 10, 64)
	if err != nil || lower <= 0 {
		return ErrBuyerRequestedRangeRequired
	}
	upper, err := strconv.ParseInt(r.UpperBoundAmount, 10, 64)
	if err != nil || upper <= 0 {
		return ErrBuyerRequestedRangeRequired
	}
	if lower > upper {
		return ErrBuyerRequestedRangeRequired
	}
	return nil
}

func referenceBasis(unit string) (basisQuantity, basisUnit string) {
	if unit == "GRAM" {
		return "1000", "GRAM"
	}
	return "1", unit
}

func quoteTotalRange(r BuyerRequestedRange, quantity *int) *QuoteTotalRange {
	if quantity == nil || *quantity <= 0 {
		return nil
	}
	basis, err := strconv.ParseInt(r.BasisQuantity, 10, 64)
	if err != nil || basis <= 0 {
		return nil
	}
	lower, err := strconv.ParseInt(r.LowerBoundAmount, 10, 64)
	if err != nil || lower <= 0 {
		return nil
	}
	upper, err := strconv.ParseInt(r.UpperBoundAmount, 10, 64)
	if err != nil || upper <= 0 {
		return nil
	}
	qty := int64(*quantity)
	lowerTotal, ok := scaleByQuantity(lower, qty, basis)
	if !ok {
		return nil
	}
	upperTotal, ok := scaleByQuantity(upper, qty, basis)
	if !ok {
		return nil
	}
	return &QuoteTotalRange{
		Currency:         r.Currency,
		LowerBoundAmount: strconv.FormatInt(lowerTotal, 10),
		UpperBoundAmount: strconv.FormatInt(upperTotal, 10),
		Quantity:         strconv.FormatInt(qty, 10),
		QuantityUnit:     r.BasisUnit,
	}
}

func scaleByQuantity(unitAmount, quantity, basis int64) (int64, bool) {
	if unitAmount > math.MaxInt64/quantity {
		return 0, false
	}
	product := unitAmount * quantity
	if product > math.MaxInt64-basis/2 {
		return 0, false
	}
	return (product + basis/2) / basis, true
}

func HashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "hash-error"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
