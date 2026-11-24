package sellerside

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

// List 回傳發給這家店的詢價，依 (published_at DESC, id DESC) keyset 分頁。不建任何列。
// pageSize 為 0 是 DefaultPageSize、超過 MaxPageSize 以上限計，負數回 INVALID_ARGUMENT（PageSize）；pageToken 不是這個方法發出的 token 回 INVALID_ARGUMENT（page_token）。
// 店家沒有賣家能力回 seller.Authorize 的錯。不過濾投遞失敗、已成交與買家已刪的詢價。
func (s Service) List(ctx context.Context, storeID int64, pageSize int32, pageToken string) (model.SellerPublicationPage, error) {
	if err := s.check(); err != nil {
		return model.SellerPublicationPage{}, err
	}
	pageSize, err := pagination.PageSize(pageSize, DefaultPageSize, MaxPageSize)
	if err != nil {
		return model.SellerPublicationPage{}, err
	}
	params := store.ListSellerPublicationViewsParams{StoreID: storeID, PageSize: pageSize + 1}
	if pageToken != "" {
		t, ok := quote.DecodeToken(pageToken)
		if !ok {
			return model.SellerPublicationPage{}, apperr.InvalidInputOn([]string{"page_token"}, "page_token 不是有效的分頁 token")
		}
		after, id := time.UnixMicro(t.CreatedAt).UTC(), t.ID
		params.AfterPublishedAt, params.AfterID = &after, &id
	}
	var out model.SellerPublicationPage
	err = s.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		if err := s.authorize(ctx, storeID); err != nil {
			return err
		}
		q := store.New(s.DB.Conn(ctx))
		rows, err := q.ListSellerPublicationViews(ctx, params)
		if err != nil {
			return fmt.Errorf("sellerside：列出詢價：%w", err)
		}
		more := len(rows) > int(pageSize)
		if more {
			rows = rows[:pageSize]
		}
		ids := make([]uuid.UUID, len(rows))
		for i, r := range rows {
			ids[i] = r.ProcurementRfqPublication.ID
		}
		states, err := q.ListSellerResponseStates(ctx, store.ListSellerResponseStatesParams{StoreID: storeID, PublicationIds: ids})
		if err != nil {
			return fmt.Errorf("sellerside：讀取回價狀態：%w", err)
		}
		byPublication := make(map[uuid.UUID]store.ListSellerResponseStatesRow, len(states))
		for _, st := range states {
			byPublication[st.PublicationID] = st
		}
		now := s.Now()
		for _, r := range rows {
			v, err := newView(r.ProcurementRfqPublication, r.ProcurementRfqPublicationApproval, r.ProcurementRequestRevision, r.DeliveryState)
			if err != nil {
				return err
			}
			item := model.SellerPublicationItem{Summary: v.summary(), Payload: v.payload}
			if st, ok := byPublication[v.pub.ID]; ok {
				item.HasDraft = st.HasDraft
				if st.LatestVersion > 0 && st.ValidUntil != nil {
					status := quote.VersionStatus(st.LatestVersion, st.LatestVersion, st.WithdrawnVersion, *st.ValidUntil, now)
					item.LatestResponseStatus = &status
				}
			}
			out.Items = append(out.Items, item)
		}
		if more {
			last := rows[len(rows)-1].ProcurementRfqPublication
			out.NextPageToken = quote.EncodeToken(last.PublishedAt, last.ID)
		}
		return nil
	})
	return out, err
}

// Get 回傳一筆發給這家店的詢價，加上這家店自己的草稿與目前的正式回價（沒有時為 nil）。不建任何列。
// 詢價不存在、不是發給這家店的（包含沒被邀請）回 NOT_FOUND；買家已刪除的詢價照常讀得到。
func (s Service) Get(ctx context.Context, storeID int64, publicationID uuid.UUID) (model.SellerPublicationDetail, error) {
	if err := s.check(); err != nil {
		return model.SellerPublicationDetail{}, err
	}
	var out model.SellerPublicationDetail
	err := s.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		if err := s.authorize(ctx, storeID); err != nil {
			return err
		}
		q := store.New(s.DB.Conn(ctx))
		v, err := loadView(ctx, q, storeID, publicationID)
		if err != nil {
			return err
		}
		out = model.SellerPublicationDetail{Summary: v.summary(), Payload: v.payload}
		resp, err := q.GetSellerResponse(ctx, store.GetSellerResponseParams{PublicationID: publicationID, StoreID: storeID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("sellerside：讀取回價身分列：%w", err)
		}
		draft, err := q.GetSellerDraft(ctx, resp.ID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return fmt.Errorf("sellerside：讀取草稿：%w", err)
		default:
			if out.Draft, err = draftOf(draft); err != nil {
				return err
			}
		}
		if resp.LatestVersion == 0 {
			return nil
		}
		latest, err := q.GetSellerResponseVersion(ctx, store.GetSellerResponseVersionParams{SellerResponseID: resp.ID, Version: resp.LatestVersion})
		if err != nil {
			return fmt.Errorf("sellerside：讀取回價 %s 的版本 %d：%w", resp.ID, resp.LatestVersion, err)
		}
		lv, err := versionOf(resp, latest, v.payload, s.Now())
		if err != nil {
			return err
		}
		out.Latest = &lv
		return nil
	})
	return out, err
}
