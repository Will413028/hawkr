package notification

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/announce"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// Announcement 是一則後台公告；Audiences 是 "BUYER"、"STORE" 的已排序子集。
type Announcement = announce.Announcement

// CreateAnnouncementInput 是建立公告的輸入（Actor.Action 是後台 RPC 的 procedure）。
type CreateAnnouncementInput = announce.CreateInput

// AnnouncementCursor 是公告列表的排序鍵。
type AnnouncementCursor = announce.Cursor

// AnnouncementPage 是一頁公告。
type AnnouncementPage = announce.Page

// CreateAnnouncement 建立後台公告並排入派送：收件匣與推播是兩條分開的 job 鏈，逐受眾、逐頁展開，沒有 device token 的帳號也收得到收件匣。
// 帶 RequestID 時重送回第一次的結果，內容不同回 InvalidInput（RequestID）；寫稽核紀錄。
func (m *Module) CreateAnnouncement(ctx context.Context, in CreateAnnouncementInput) (Announcement, error) {
	return m.announce.Create(ctx, in)
}

// ListAnnouncements 列出公告，新到舊（建立時間，再依 id），預設 20 筆、上限 50。
func (m *Module) ListAnnouncements(ctx context.Context, after *AnnouncementCursor, size int32) (AnnouncementPage, error) {
	return m.announce.List(ctx, after, size)
}

type directory struct {
	buyers *buyer.Module
	stores *seller.Module
}

// NewDirectory 把 buyer 與 seller 組成公告的收件人名單。
func NewDirectory(b *buyer.Module, s *seller.Module) announce.Directory {
	return directory{buyers: b, stores: s}
}

func (d directory) Buyers(ctx context.Context, after int64, limit int) ([]announce.Recipient, error) {
	rows, err := d.buyers.ListLiveAccounts(ctx, after, limit)
	if err != nil {
		return nil, err
	}
	out := make([]announce.Recipient, len(rows))
	for i, r := range rows {
		out[i] = announce.Recipient{AccountID: r.AccountID, Cursor: r.BuyerID}
	}
	return out, nil
}

func (d directory) Stores(ctx context.Context, after int64, limit int) ([]announce.Recipient, error) {
	rows, err := d.stores.ListLiveStoreAccounts(ctx, after, limit)
	if err != nil {
		return nil, err
	}
	out := make([]announce.Recipient, len(rows))
	for i, r := range rows {
		out[i] = announce.Recipient{AccountID: r.AccountID, StoreID: r.StoreID, Cursor: r.StoreID}
	}
	return out, nil
}
