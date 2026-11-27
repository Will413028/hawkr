package sellerside

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// SubmitInput 是 SubmitSellerResponse 的輸入：ExpectedPublicationVersion 與 ExpectedSellerFacingHash 把報價綁在賣家看到的那一版詢價內容（都必填），
// ExpectedResponseVersion 是賣家讀到的正式回價版本（空字串表示尚無正式回價）。
type SubmitInput struct {
	StoreID                    int64
	PublicationID              uuid.UUID
	ClientRequestID            uuid.UUID
	ExpectedPublicationVersion string
	ExpectedSellerFacingHash   string
	ExpectedResponseVersion    string
	Quote                      model.QuoteInput
}

// Submit 送出正式回價，產生不可變的新版本，hash 用 rfqhash.SellerResponse（quote.PayloadHash），與買家端列出的 payload_hash 同一個值。
// 順序：欄位與報價內容驗證（回 INVALID_ARGUMENT，指向欄位）→ 詢價是否發給這家店（NOT_FOUND）→ 冪等重送 → 詢價內容的版本與 hash（ABORTED／VERSION_CONFLICT，publication）→
// 停止寫入（FAILED_PRECONDITION／RFQ_RECIPIENT_CLOSED）→ 回價版本（ABORTED／VERSION_CONFLICT，seller_response：expected 不是目前的版本，或空字串卻已有正式回價）→ 有效期限要在未來。
// 重送同一個 ClientRequestID：報價內容與三個 expected 欄位都相同回第一次的版本（Replayed，不建新版本，先於所有版本與停止寫入的檢查，所以重送不會被判成已提交），任一不同回 INVALID_ARGUMENT（client_request_id）。
func (s Service) Submit(ctx context.Context, in SubmitInput) (model.SellerResponseVersion, error) {
	if err := s.check(); err != nil {
		return model.SellerResponseVersion{}, err
	}
	switch {
	case in.ClientRequestID == (uuid.UUID{}):
		return model.SellerResponseVersion{}, apperr.InvalidInputOn([]string{"client_request_id"}, "client_request_id 不能是空的")
	case in.ExpectedPublicationVersion == "":
		return model.SellerResponseVersion{}, apperr.InvalidInputOn([]string{"expected_publication_version"}, "expected_publication_version 不能是空的")
	case in.ExpectedSellerFacingHash == "":
		return model.SellerResponseVersion{}, apperr.InvalidInputOn([]string{"expected_seller_facing_payload_hash"}, "expected_seller_facing_payload_hash 不能是空的")
	}
	expectedPublication, err := ParseVersion("expected_publication_version", in.ExpectedPublicationVersion)
	if err != nil {
		return model.SellerResponseVersion{}, err
	}
	expectedResponse, err := ParseVersion("expected_seller_response_version", in.ExpectedResponseVersion)
	if err != nil {
		return model.SellerResponseVersion{}, err
	}
	n, err := normalizeQuote(in.Quote)
	if err != nil {
		return model.SellerResponseVersion{}, err
	}
	hash, err := quote.PayloadHash(n.Quote)
	if err != nil {
		return model.SellerResponseVersion{}, err
	}
	var out model.SellerResponseVersion
	err = s.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.guardWrite(ctx, in.StoreID); err != nil {
			return err
		}
		q := store.New(s.DB.Conn(ctx))
		v, err := loadView(ctx, q, in.StoreID, in.PublicationID)
		if err != nil {
			return err
		}
		resp, err := s.ensureAndLock(ctx, q, in.StoreID, in.PublicationID)
		if err != nil {
			return err
		}
		now := s.Now()
		first, err := q.GetSellerResponseVersionByKey(ctx, store.GetSellerResponseVersionByKeyParams{SellerResponseID: resp.ID, ClientRequestID: in.ClientRequestID})
		switch {
		case err == nil:
			out, err = replay(first, resp, v, hash, in, expectedPublication, expectedResponse, now)
			return err
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("sellerside：讀取回價的冪等鍵：%w", err)
		}
		if expectedPublication != v.pub.Version || in.ExpectedSellerFacingHash != v.hash {
			return quote.VersionConflict("這筆詢價的內容已經更新，請重新載入", quote.TargetPublication)
		}
		if err := checkWritable(ctx, q, resp, v, now); err != nil {
			return err
		}
		if expectedResponse != resp.LatestVersion {
			return quote.VersionConflict("你的報價已經變動，請重新載入", quote.TargetSellerResponse)
		}
		if !n.Quote.ValidUntil.After(now) {
			return apperr.InvalidInputOn([]string{"ValidUntil"}, "ValidUntil 必須在現在之後")
		}
		next := resp.LatestVersion + 1
		row, err := q.InsertSellerResponseVersion(ctx, store.InsertSellerResponseVersionParams{
			ID: uuid.NewV7(), SellerResponseID: resp.ID, Version: next, ResponseType: n.Quote.ResponseType, SpecNote: n.Quote.SpecNote,
			UnitBasisQuantity: n.UnitBasisNum, UnitBasisUnit: n.Quote.UnitBasisUnit, OfferedQuantity: n.OfferedNum, OfferedQuantityUnit: n.Quote.OfferedQuantityUnit,
			ConversionBasis: n.Quote.ConversionBasis, Currency: n.Quote.Currency, GoodsAmount: n.Quote.GoodsAmount, ShippingAmount: n.Quote.ShippingAmount,
			TaxAmount: n.Quote.TaxAmount, TaxInclusionBasis: n.Quote.TaxInclusionBasis, TotalAmount: n.Quote.TotalAmount, LeadTimeDays: n.Quote.LeadTimeDays,
			PromisedDeliveryAt: n.Quote.PromisedDelivery, ValidUntil: n.Quote.ValidUntil, TermsNote: n.Quote.TermsNote, DeviationDisclosure: n.Quote.DeviationDisclosure,
			PublicationVersion: v.pub.Version, ClientRequestID: in.ClientRequestID, Now: now,
		})
		if err != nil {
			return fmt.Errorf("sellerside：寫入回價版本：%w", err)
		}
		bumped, err := q.BumpSellerResponseVersion(ctx, store.BumpSellerResponseVersionParams{ID: resp.ID, Version: next, ExpectedVersion: resp.LatestVersion, Now: now})
		if err != nil {
			return fmt.Errorf("sellerside：推進回價版本：%w", err)
		}
		if bumped != 1 {
			return fmt.Errorf("sellerside：回價 %s 的版本在鎖內被改動", resp.ID)
		}
		resp.LatestVersion = next
		out, err = versionOf(resp, row, v.payload, now)
		return err
	})
	return out, err
}

func replay(first store.ProcurementSellerResponseVersion, resp store.ProcurementSellerResponse, v view, hash string, in SubmitInput, expectedPublication, expectedResponse int32, now time.Time) (model.SellerResponseVersion, error) {
	qt, err := quote.QuoteOf(first)
	if err != nil {
		return model.SellerResponseVersion{}, err
	}
	firstHash, err := quote.PayloadHash(qt)
	if err != nil {
		return model.SellerResponseVersion{}, err
	}
	if firstHash != hash || expectedPublication != first.PublicationVersion || in.ExpectedSellerFacingHash != v.hash || expectedResponse != first.Version-1 {
		return model.SellerResponseVersion{}, quote.IdempotencyMismatch()
	}
	out, err := versionOf(resp, first, v.payload, now)
	out.Replayed = true
	return out, err
}

// FormatVersion 是版本號對外的字串（不透明的十進位字串）。
func FormatVersion(v int32) string { return strconv.FormatInt(int64(v), 10) }
