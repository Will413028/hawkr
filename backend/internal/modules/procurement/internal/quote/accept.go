package quote

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// AcceptInput 是 AcceptSellerQuote 的輸入：ExpectedVersion 是買家看到的回價版本，ExpectedSellerFacingHash 是買家畫面上那份詢價內容的 hash。
type AcceptInput struct {
	BuyerID                                int64
	RequestID, ClientRequestID, ResponseID uuid.UUID
	ExpectedVersion                        int32
	ExpectedSellerFacingHash               string
}

// Accept 接受賣家回價的指定版本並凍結成交價；同一個 transaction 鎖回價身分列、檢查、寫入接受單並排入建單的 job（OrderProvisioningArgs），任何一步失敗全部還原、不排 job。
// 版本號不是正數、hash 是空的回 INVALID_ARGUMENT。重送同一個 ClientRequestID 且內容（需求、回價、版本、hash）相同回第一次的接受單（Replayed，先於其餘檢查，不再排 job），內容不同回 INVALID_ARGUMENT（client_request_id）。
// 回價不屬於這位買家的這張需求回 NOT_FOUND；目前版本不是 ExpectedVersion 或該版已撤回回 ABORTED／VERSION_CONFLICT（seller_response）；賣家已停止寫入回 FAILED_PRECONDITION／RFQ_RECIPIENT_CLOSED；
// 版本已過有效期限回 FAILED_PRECONDITION／RFQ_RESPONSE_EXPIRED；ExpectedSellerFacingHash 與伺服器從這次發布的核准與需求版本重算的值不符回 ABORTED／VERSION_CONFLICT（publication）；
// 這張需求已有進行中的成交回 FAILED_PRECONDITION／RFQ_CHECKOUT_ALREADY_ACTIVE（有訂單時 metadata 的 orderNo 是既有訂單）。
func (s Service) Accept(ctx context.Context, in AcceptInput) (model.Acceptance, error) {
	switch {
	case in.ExpectedVersion <= 0:
		return model.Acceptance{}, apperr.InvalidInputOn([]string{"expected_seller_response_version"}, "expected_seller_response_version 必須是正整數")
	case in.ExpectedSellerFacingHash == "":
		return model.Acceptance{}, apperr.InvalidInputOn([]string{"expected_seller_facing_hash"}, "expected_seller_facing_hash 不能是空的")
	case s.Jobs == nil:
		return model.Acceptance{}, errors.New("quote：Service.Jobs 必填：接受報價要在同一個 transaction 排入建單的 job")
	}
	var out model.Acceptance
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		var err error
		var found bool
		if out, found, err = replay(ctx, q, in); err != nil || found {
			return err
		}
		resp, err := q.LockSellerResponseForBuyer(ctx, store.LockSellerResponseForBuyerParams{ID: in.ResponseID, RequestID: in.RequestID, BuyerID: &in.BuyerID})
		if err != nil {
			return owned(err, "則回價")
		}
		if resp.LatestVersion != in.ExpectedVersion || (resp.WithdrawnVersion != nil && *resp.WithdrawnVersion == in.ExpectedVersion) {
			return VersionConflict("這份報價已經改版或撤回，請重新載入", TargetSellerResponse)
		}
		if resp.WriteClosedAt != nil {
			return apperr.FailedPrecondition("賣家已停止這筆詢價的回價").WithReason(apperr.ReasonRfqRecipientClosed)
		}
		version, err := q.GetSellerResponseVersion(ctx, store.GetSellerResponseVersionParams{SellerResponseID: resp.ID, Version: in.ExpectedVersion})
		if err != nil {
			return fmt.Errorf("quote：讀取回價 %s 的版本 %d：%w", resp.ID, in.ExpectedVersion, err)
		}
		now := s.Now()
		if now.After(version.ValidUntil) {
			return apperr.FailedPrecondition("這份報價已過有效期限").WithReason(apperr.ReasonRfqResponseExpired)
		}
		pub, err := q.GetPublication(ctx, store.GetPublicationParams{ID: resp.PublicationID, BuyerID: &in.BuyerID})
		if err != nil {
			return fmt.Errorf("quote：讀取回價 %s 的發布：%w", resp.ID, err)
		}
		sellerHash, err := rfq.SellerFacingHash(ctx, q, in.BuyerID, in.RequestID, pub.ApprovalID)
		if err != nil {
			return err
		}
		if in.ExpectedSellerFacingHash != sellerHash {
			return VersionConflict("賣家報價時依據的詢價內容與你看到的不同，請重新載入", TargetPublication)
		}
		qt, err := QuoteOf(version)
		if err != nil {
			return err
		}
		payloadHash, err := PayloadHash(qt)
		if err != nil {
			return err
		}
		acceptance := store.InsertAcceptanceParams{
			ID: uuid.NewV7(), BuyerID: &in.BuyerID, RequestID: in.RequestID, PublicationID: pub.ID, SellerResponseID: resp.ID, SellerResponseVersion: version.Version,
			SellerResponsePayloadHash: payloadHash, SellerFacingHash: sellerHash, StoreID: resp.StoreID, Currency: version.Currency, GoodsAmount: version.GoodsAmount,
			ShippingAmount: version.ShippingAmount, TaxAmount: version.TaxAmount, AcceptedDealPrice: version.TotalAmount, ClientRequestID: in.ClientRequestID, Now: now,
		}
		n, err := q.InsertAcceptance(ctx, acceptance)
		if err != nil {
			return fmt.Errorf("quote：寫入接受單：%w", err)
		}
		if n == 0 { // 冪等鍵或 live 鎖擋下：先看是不是同一個鍵的另一個請求剛寫完
			if out, found, err = replay(ctx, q, in); err != nil || found {
				return err
			}
			return liveCollision(ctx, q, in)
		}
		if _, err := s.Jobs.EnqueueJob(ctx, OrderProvisioningArgs{AcceptanceID: acceptance.ID}, nil); err != nil {
			return err
		}
		out = model.Acceptance{
			ID: acceptance.ID, SellerResponseID: resp.ID, Version: version.Version, AcceptedDealPrice: version.TotalAmount, AcceptedAt: now,
			Provisioning: model.ProvisioningCreating,
		}
		return nil
	})
	return out, err
}

// replay 以冪等鍵找第一次的接受單：沒有時 found 為 false；有但內容（需求、回價、版本、hash）不同回 INVALID_ARGUMENT。
func replay(ctx context.Context, q *store.Queries, in AcceptInput) (model.Acceptance, bool, error) {
	a, err := q.GetAcceptanceByClientRequest(ctx, store.GetAcceptanceByClientRequestParams{BuyerID: &in.BuyerID, ClientRequestID: in.ClientRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Acceptance{}, false, nil
	}
	if err != nil {
		return model.Acceptance{}, false, fmt.Errorf("quote：讀取接受單的冪等鍵：%w", err)
	}
	if a.RequestID != in.RequestID || a.SellerResponseID != in.ResponseID || a.SellerResponseVersion != in.ExpectedVersion || a.SellerFacingHash != in.ExpectedSellerFacingHash {
		return model.Acceptance{}, false, IdempotencyMismatch()
	}
	return model.Acceptance{
		ID: a.ID, SellerResponseID: a.SellerResponseID, Version: a.SellerResponseVersion, AcceptedDealPrice: a.AcceptedDealPrice, AcceptedAt: a.AcceptedAt,
		Provisioning: a.OrderProvisioningState, OrderNo: a.OrderNo, PaymentDeadline: a.PaymentDeadline, Replayed: true,
	}, true, nil
}

// liveCollision 回這張需求已有進行中的成交；既有接受單已有訂單時帶 orderNo。
func liveCollision(ctx context.Context, q *store.Queries, in AcceptInput) error {
	e := apperr.FailedPrecondition("這張需求已經有進行中的成交").WithReason(apperr.ReasonRfqCheckoutAlreadyActive)
	live, err := q.GetLiveAcceptance(ctx, store.GetLiveAcceptanceParams{RequestID: in.RequestID, BuyerID: &in.BuyerID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("quote：讀取進行中的成交：%w", err)
	}
	if err == nil && live.OrderNo != nil {
		e = e.WithMetadata("orderNo", *live.OrderNo)
	}
	return e
}
