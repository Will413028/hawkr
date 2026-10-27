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
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

var (
	ErrPriceBasisUnavailable           = errors.New("procurement: currency/quantity/unit/conversion basis cannot be canonicalized")
	ErrStalePricePreviewRevision       = errors.New("procurement: expectedRevisionId doesn't match current active revision")
	ErrMappingUnavailableForGuidedMode = errors.New("procurement: PRICEBOOK_GUIDED requested but pricebook mapping/market data is unavailable; resubmit as BUYER_ENTERED_NO_REFERENCE")
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

type ReferencePriceRange struct {
	Lower         *float64 `json:"lower"`
	Midpoint      float64  `json:"midpoint"`
	Upper         *float64 `json:"upper"`
	BasisQuantity string   `json:"basisQuantity"`
	BasisUnit     string   `json:"basisUnit"`
}

const (
	MarketPriceScopeRegion   = "REGION"
	MarketPriceScopeNational = "NATIONAL"
)

func (r BuyerRequestedRange) IsUnset() bool {
	return r.Currency == "" && r.PriceComponent == "" &&
		r.LowerBoundAmount == "" && r.UpperBoundAmount == ""
}

type MarketPriceBasis struct {
	Scope                 string  `json:"scope"`
	PriceLayer            string  `json:"priceLayer"`
	RegionName            string  `json:"regionName"`
	SupportingMarketCount int32   `json:"supportingMarketCount"`
	SourceDataAsOf        *string `json:"sourceDataAsOf"`
	FreshnessStatus       string  `json:"freshnessStatus"`
}

type QuoteTotalRange struct {
	Currency         string `json:"currency"`
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
	Quantity         string `json:"quantity"`
	QuantityUnit     string `json:"quantityUnit"`
}

type DealTemperature struct {
	Status                   string `json:"status"`
	ReasonCode               string `json:"reasonCode"`
	ProbabilityLowerBps      string `json:"probabilityLowerBps,omitempty"`
	ProbabilityUpperBps      string `json:"probabilityUpperBps,omitempty"`
	Band                     string `json:"band,omitempty"`
	OutcomeDefinition        string `json:"outcomeDefinition,omitempty"`
	OutcomeDefinitionVersion string `json:"outcomeDefinitionVersion,omitempty"`
	PolicyVersion            string `json:"policyVersion,omitempty"`
	BandVersion              string `json:"bandVersion,omitempty"`
}

type PreviewRfqPriceInput struct {
	RequestID           int64
	BuyerID             identity.BuyerID
	ClientRequestID     string
	ExpectedRevisionID  int64
	PricingMode         string
	DeliveryCity        string
	BuyerRequestedRange *BuyerRequestedRange
}

type PricePreviewResult struct {
	PricePreviewID        int64
	Version               int64
	PricingMode           string
	QualityKnowledgeState string
	ReferenceAvailability string
	ReferencePriceRange   *ReferencePriceRange
	BuyerRequestedRange   BuyerRequestedRange
	DealTemperature       DealTemperature
	PayloadHash           string
	RequestID             int64
	RequestRevisionID     int64
	ExpiresAt             time.Time
	MarketPriceBasis      *MarketPriceBasis
	QuoteTotalRange       *QuoteTotalRange
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
	pricebookClient pbusecase.PricebookClient
	repo            PricePreviewRepo
	revisions       RevisionReader
}

func NewPreviewRfqPriceUseCase(pc pbusecase.PricebookClient, repo PricePreviewRepo, revisions RevisionReader) *PreviewRfqPriceUseCase {
	return &PreviewRfqPriceUseCase{pricebookClient: pc, repo: repo, revisions: revisions}
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

	result := PricePreviewResult{PricingMode: in.PricingMode}
	var unavailableReason *string

	if in.PricingMode == "BUYER_ENTERED_NO_REFERENCE" {
		if err := validateBuyerRequestedRange(in.BuyerRequestedRange); err != nil {
			return PricePreviewResult{}, err
		}
		result.ReferenceAvailability = "UNAVAILABLE"
		result.DealTemperature = DealTemperature{Status: "UNKNOWN", ReasonCode: codemeta.ProcurementDealTemperatureReasonMappingMissing.Value}
		reason := codemeta.ProcurementDealTemperatureReasonMappingMissing.Value
		unavailableReason = &reason
		result.BuyerRequestedRange = *in.BuyerRequestedRange
		if slots.Unit != nil {
			result.BuyerRequestedRange.BasisQuantity, result.BuyerRequestedRange.BasisUnit = referenceBasis(*slots.Unit)
		}
	} else {
		if commodityCode := slots.ItemResolution.PricebookCommodityCode(); commodityCode != nil {
			if err := uc.fillCommodityPrice(ctx, in, slots, &result, *commodityCode); err != nil {
				return PricePreviewResult{}, err
			}
		} else {
			cropCode, groupCode := slots.ItemResolution.PricebookQueryCodes()

			var region *pbusecase.RegionalComparisonRegion
			var sourceDataAsOf *string
			if cropCode != nil || groupCode != nil {
				comparison, err := uc.pricebookClient.GetRegionalComparison(ctx, pbusecase.RegionalComparisonInput{
					CropCode: cropCode, GroupCode: groupCode,
				}, 1)
				if err != nil {
					return PricePreviewResult{}, err
				}
				regions := comparison.Regions
				sourceDataAsOf = comparison.SourceDataAsOf
				deliveryCity := ""
				if slots.DeliveryCity != nil {
					deliveryCity = *slots.DeliveryCity
				}
				normalizedDeliveryCity := domain.NormalizeCityName(deliveryCity)
				for i := range regions {
					if domain.NormalizeCityName(regions[i].RegionName) == normalizedDeliveryCity {
						region = &regions[i]
						break
					}
				}
			}

			mappingReliable := region != nil && region.Status == "ready" &&
				region.P10 != nil && region.P50 != nil && region.P90 != nil
			if !mappingReliable {
				return PricePreviewResult{}, ErrMappingUnavailableForGuidedMode
			}
			unit := ""
			if slots.Unit != nil {
				unit = *slots.Unit
			}
			lower, err := domain.CompileReferencePrice(*region.P10, unit, domain.PriceComponentGoodsOnly)
			if err != nil {
				return PricePreviewResult{}, ErrPriceBasisUnavailable
			}
			mid, err := domain.CompileReferencePrice(*region.P50, unit, domain.PriceComponentGoodsOnly)
			if err != nil {
				return PricePreviewResult{}, ErrPriceBasisUnavailable
			}
			upper, err := domain.CompileReferencePrice(*region.P90, unit, domain.PriceComponentGoodsOnly)
			if err != nil {
				return PricePreviewResult{}, ErrPriceBasisUnavailable
			}
			basisQuantity, basisUnit := referenceBasis(unit)
			result.ReferenceAvailability = "AVAILABLE"
			result.ReferencePriceRange = &ReferencePriceRange{
				Lower: &lower, Midpoint: mid, Upper: &upper,
				BasisQuantity: basisQuantity, BasisUnit: basisUnit,
			}
			result.MarketPriceBasis = &MarketPriceBasis{
				Scope:                 MarketPriceScopeRegion,
				RegionName:            region.RegionName,
				SupportingMarketCount: region.SupportingMarketCount,
				SourceDataAsOf:        sourceDataAsOf,
				FreshnessStatus:       region.FreshnessStatus,
			}
			if in.BuyerRequestedRange != nil {
				result.BuyerRequestedRange = *in.BuyerRequestedRange
				result.BuyerRequestedRange.BasisQuantity = basisQuantity
				result.BuyerRequestedRange.BasisUnit = basisUnit
			} else {
				result.BuyerRequestedRange = BuyerRequestedRange{
					Currency: "TWD", PriceComponent: domain.PriceComponentGoodsOnly,
					BasisQuantity: basisQuantity, BasisUnit: basisUnit,
					LowerBoundAmount: formatAmount(lower), UpperBoundAmount: formatAmount(upper),
				}
			}

			result.DealTemperature = dealTemperatureFor(result.ReferencePriceRange, in.BuyerRequestedRange)
		}
	}

	result.QuoteTotalRange = quoteTotalRange(result.BuyerRequestedRange, slots.Quantity)

	result.PayloadHash = HashJSON(result)

	var referencePayloadJSON json.RawMessage
	if result.ReferencePriceRange != nil {
		referencePayloadJSON, _ = json.Marshal(result.ReferencePriceRange)
	}
	var marketPriceBasisJSON json.RawMessage
	if result.MarketPriceBasis != nil {
		marketPriceBasisJSON, _ = json.Marshal(result.MarketPriceBasis)
	}
	var quoteTotalRangeJSON json.RawMessage
	if result.QuoteTotalRange != nil {
		quoteTotalRangeJSON, _ = json.Marshal(result.QuoteTotalRange)
	}
	buyerRangeJSON, _ := json.Marshal(result.BuyerRequestedRange)
	dealTempJSON, _ := json.Marshal(result.DealTemperature)

	inserted, err := uc.repo.InsertPricePreview(ctx, repository.InsertPricePreviewInput{
		BuyerID: in.BuyerID, RequestID: in.RequestID, RequestRevisionID: activeRevisionID,
		PricingMode: in.PricingMode, QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		ReferenceAvailability: result.ReferenceAvailability, ReferencePayloadJSON: referencePayloadJSON,
		UnavailableReason: unavailableReason, BuyerRequestedRangeJSON: buyerRangeJSON,
		MarketPriceBasisJSON: marketPriceBasisJSON, QuoteTotalRangeJSON: quoteTotalRangeJSON,
		DealTemperatureJSON: dealTempJSON, ClientRequestID: in.ClientRequestID,
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
		PricePreviewID: row.ID, Version: row.Version, PricingMode: row.PricingMode,
		QualityKnowledgeState: row.QualityKnowledgeState,
		ReferenceAvailability: row.ReferenceAvailability, PayloadHash: row.PayloadHash,
		RequestID: row.RequestID, RequestRevisionID: row.RequestRevisionID, ExpiresAt: row.ExpiresAt,
	}
	if len(row.ReferencePayloadJSON) > 0 {
		r.ReferencePriceRange = &ReferencePriceRange{}
		_ = json.Unmarshal(row.ReferencePayloadJSON, r.ReferencePriceRange)
	}
	if len(row.MarketPriceBasisJSON) > 0 {
		r.MarketPriceBasis = &MarketPriceBasis{}
		_ = json.Unmarshal(row.MarketPriceBasisJSON, r.MarketPriceBasis)
	}
	if len(row.QuoteTotalRangeJSON) > 0 {
		r.QuoteTotalRange = &QuoteTotalRange{}
		_ = json.Unmarshal(row.QuoteTotalRangeJSON, r.QuoteTotalRange)
	}
	_ = json.Unmarshal(row.BuyerRequestedRangeJSON, &r.BuyerRequestedRange)
	_ = json.Unmarshal(row.DealTemperatureJSON, &r.DealTemperature)
	return r
}

func validateBuyerRequestedRange(r *BuyerRequestedRange) error {
	if r == nil || r.Currency != "TWD" {
		return ErrPriceBasisUnavailable
	}
	lower, err := strconv.ParseInt(r.LowerBoundAmount, 10, 64)
	if err != nil || lower <= 0 {
		return ErrPriceBasisUnavailable
	}
	upper, err := strconv.ParseInt(r.UpperBoundAmount, 10, 64)
	if err != nil || upper <= 0 {
		return ErrPriceBasisUnavailable
	}
	if lower > upper {
		return ErrPriceBasisUnavailable
	}
	return nil
}

func referenceBasis(unit string) (basisQuantity, basisUnit string) {
	if unit == "GRAM" {
		return "1000", "GRAM"
	}
	return "1", unit
}

func dealTemperatureFor(ref *ReferencePriceRange, buyer *BuyerRequestedRange) DealTemperature {
	if ref == nil {
		return DealTemperature{
			Status:     domain.DealTemperatureStatusUnknown,
			ReasonCode: codemeta.ProcurementDealTemperatureReasonMappingMissing.Value,
		}
	}
	if buyer == nil {
		return DealTemperature{
			Status:     domain.DealTemperatureStatusUnknown,
			ReasonCode: domain.DealTemperatureReasonBuyerRangeNotSet,
		}
	}
	lo, err := strconv.ParseInt(buyer.LowerBoundAmount, 10, 64)
	if err != nil {
		return DealTemperature{
			Status:     domain.DealTemperatureStatusUnknown,
			ReasonCode: domain.DealTemperatureReasonBuyerRangeNotSet,
		}
	}
	hi, err := strconv.ParseInt(buyer.UpperBoundAmount, 10, 64)
	if err != nil {
		return DealTemperature{
			Status:     domain.DealTemperatureStatusUnknown,
			ReasonCode: domain.DealTemperatureReasonBuyerRangeNotSet,
		}
	}
	var cov domain.DealTemperatureCoverage
	var reason string
	if ref.Lower == nil || ref.Upper == nil {
		cov, reason = domain.ComputeDealTemperatureFromPoint(ref.Midpoint, lo, hi)
	} else {
		cov, reason = domain.ComputeDealTemperatureCoverage(*ref.Lower, ref.Midpoint, *ref.Upper, lo, hi)
	}
	if reason != "" {
		return DealTemperature{Status: domain.DealTemperatureStatusUnknown, ReasonCode: reason}
	}
	return DealTemperature{
		Status:                   domain.DealTemperatureStatusAvailable,
		ProbabilityLowerBps:      strconv.FormatInt(cov.WithinRangeBps, 10),
		ProbabilityUpperBps:      strconv.FormatInt(cov.AtOrBelowUpperBps, 10),
		Band:                     cov.Band,
		OutcomeDefinition:        "BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE",
		OutcomeDefinitionVersion: "1",
		PolicyVersion:            "DEAL_TEMPERATURE_COVERAGE_V1",
		BandVersion:              "BANDS_V2",
	}
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

func formatAmount(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func HashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "hash-error"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (uc *PreviewRfqPriceUseCase) fillCommodityPrice(
	ctx context.Context,
	in PreviewRfqPriceInput,
	slots domain.Slots,
	result *PricePreviewResult,
	commodityCode string,
) error {
	cp, err := uc.pricebookClient.GetCommodityPrice(ctx, commodityCode)
	if err != nil {
		return err
	}
	if cp.P50 == nil {
		return ErrMappingUnavailableForGuidedMode
	}

	unit := ""
	if slots.Unit != nil {
		unit = *slots.Unit
	}
	mid, err := domain.CompileReferencePrice(*cp.P50, unit, domain.PriceComponentGoodsOnly)
	if err != nil {
		return ErrPriceBasisUnavailable
	}
	basisQuantity, basisUnit := referenceBasis(unit)
	result.ReferenceAvailability = "AVAILABLE"
	result.ReferencePriceRange = &ReferencePriceRange{
		Midpoint:      mid,
		BasisQuantity: basisQuantity,
		BasisUnit:     basisUnit,
	}
	var sourceDataAsOf *string
	if cp.SourceDataAsOf != "" {
		v := cp.SourceDataAsOf
		sourceDataAsOf = &v
	}
	result.MarketPriceBasis = &MarketPriceBasis{
		Scope:                 MarketPriceScopeNational,
		PriceLayer:            cp.PriceLayer,
		SupportingMarketCount: parseInt32(cp.SupportingMarketCount),
		SourceDataAsOf:        sourceDataAsOf,
	}
	if in.BuyerRequestedRange != nil {
		result.BuyerRequestedRange = *in.BuyerRequestedRange
		result.BuyerRequestedRange.BasisQuantity = basisQuantity
		result.BuyerRequestedRange.BasisUnit = basisUnit
	}
	result.DealTemperature = dealTemperatureFor(result.ReferencePriceRange, in.BuyerRequestedRange)
	return nil
}

func parseInt32(s string) int32 {
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int32(v)
}
