package quote

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// PutShortlist 把賣家回價的一個版本放進入圍清單；同一則回價已在清單時換成這個版本（A9-D31 Q12，唯一鍵 (request_id, seller_response_id)），回掛著的版本。
// 版本號不是正數回 INVALID_ARGUMENT（seller_response_version）；回價不屬於這位買家的這張需求、或沒有這個版本回 NOT_FOUND。
func (s Service) PutShortlist(ctx context.Context, buyerID int64, requestID, sellerResponseID uuid.UUID, version int32) (model.ShortlistEntry, error) {
	if version <= 0 {
		return model.ShortlistEntry{}, apperr.InvalidInputOn([]string{"seller_response_version"}, "seller_response_version 必須是正整數")
	}
	var out model.ShortlistEntry
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		if _, err := q.GetSellerResponseForBuyer(ctx, store.GetSellerResponseForBuyerParams{ID: sellerResponseID, RequestID: requestID, BuyerID: &buyerID}); err != nil {
			return owned(err, "則回價")
		}
		if _, err := q.GetSellerResponseVersion(ctx, store.GetSellerResponseVersionParams{SellerResponseID: sellerResponseID, Version: version}); err != nil {
			return owned(err, "個回價版本")
		}
		e, err := q.PutShortlistEntry(ctx, store.PutShortlistEntryParams{
			ID: uuid.NewV7(), RequestID: requestID, SellerResponseID: sellerResponseID, SellerResponseVersion: version, Now: s.Now(),
		})
		if err != nil {
			return fmt.Errorf("quote：寫入入圍清單：%w", err)
		}
		out = model.ShortlistEntry{SellerResponseID: e.SellerResponseID, Version: e.SellerResponseVersion, Present: true}
		return nil
	})
	return out, err
}

// DeleteShortlist 把賣家回價從入圍清單移除（目標狀態的方法，不在清單時結果相同，Present 為 false）；Version 是被移除那一列掛的版本，不在清單時是請求帶的版本。
// 需求不屬於這位買家回 NOT_FOUND；版本號不是正數回 INVALID_ARGUMENT（seller_response_version）。
func (s Service) DeleteShortlist(ctx context.Context, buyerID int64, requestID, sellerResponseID uuid.UUID, version int32) (model.ShortlistEntry, error) {
	if version <= 0 {
		return model.ShortlistEntry{}, apperr.InvalidInputOn([]string{"seller_response_version"}, "seller_response_version 必須是正整數")
	}
	out := model.ShortlistEntry{SellerResponseID: sellerResponseID, Version: version, Present: false}
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		if _, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: requestID, BuyerID: &buyerID}); err != nil {
			return owned(err, "張需求")
		}
		removed, err := q.DeleteShortlistEntry(ctx, store.DeleteShortlistEntryParams{RequestID: requestID, SellerResponseID: sellerResponseID})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil
		case err != nil:
			return fmt.Errorf("quote：移出入圍清單：%w", err)
		}
		out.Version = removed
		return nil
	})
	if err != nil {
		return model.ShortlistEntry{}, err
	}
	return out, nil
}
