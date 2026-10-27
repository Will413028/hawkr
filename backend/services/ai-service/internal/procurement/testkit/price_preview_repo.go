package testkit

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// FakePricePreviewRepo 的方法簽章必須逐字對齊 repository.Repo.InsertPricePreview／
// FindPricePreviewByIdempotencyKey——呼叫端用的是 repository 套件的真實輸入/輸出
// 型別，不是自訂窄型別，這樣才能保證真的能接上 repository.Repo。
type FakePricePreviewRepo struct {
	InsertCalls      int
	ByIdempotencyKey map[string]repository.PricePreview
}

func (f *FakePricePreviewRepo) InsertPricePreview(ctx context.Context, in repository.InsertPricePreviewInput) (repository.PricePreview, error) {
	f.InsertCalls++
	row := repository.PricePreview{
		ID: int64(f.InsertCalls), BuyerID: in.BuyerID, RequestID: in.RequestID,
		RequestRevisionID: in.RequestRevisionID, PricingMode: in.PricingMode,
		QualityKnowledgeState: in.QualityKnowledgeState, ReferenceAvailability: in.ReferenceAvailability,
		ReferenceFactsHash: in.ReferenceFactsHash, ReferencePayloadJSON: in.ReferencePayloadJSON,
		UnavailableReason: in.UnavailableReason, BuyerRequestedRangeJSON: in.BuyerRequestedRangeJSON,
		MarketPriceBasisJSON: in.MarketPriceBasisJSON, QuoteTotalRangeJSON: in.QuoteTotalRangeJSON,
		DealTemperatureJSON: in.DealTemperatureJSON, ClientRequestID: in.ClientRequestID,
		PreviewInputHash: in.PreviewInputHash, PayloadHash: in.PayloadHash,
		Version: 1, ExpiresAt: in.ExpiresAt,
	}
	if f.ByIdempotencyKey == nil {
		f.ByIdempotencyKey = map[string]repository.PricePreview{}
	}
	f.ByIdempotencyKey[in.ClientRequestID] = row
	return row, nil
}

func (f *FakePricePreviewRepo) FindPricePreviewByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.PricePreview, error) {
	if existing, ok := f.ByIdempotencyKey[clientRequestID]; ok {
		return existing, nil
	}
	return repository.PricePreview{}, repository.ErrPricePreviewNotFound
}
