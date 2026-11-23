package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// MaxBatchDelete 是一次刪除通知的上限（契約：BatchDeleteInboxItems）。
const MaxBatchDelete = 100

// Cursor 是收件匣列表的排序鍵：上一頁最後一列的 (updated_at, id)。
type Cursor struct {
	UpdatedAt time.Time `json:"u"`
	ID        uuid.UUID `json:"i"`
}

// Valid 回報游標是不是有內容（解得開但是空的 token 不合法）。
func (c Cursor) Valid() bool { return !c.UpdatedAt.IsZero() && c.ID != (uuid.UUID{}) }

// Item 是收件匣的一則通知，文字已依語系組好。Time 是最後一次合併的時間（聚合列）或建立時間。
type Item struct {
	ID           uuid.UUID
	Category     model.PrefCategory
	Presentation model.Presentation
	Title        string
	Body         string
	DeepLink     string
	ImageKey     string
	ReadAt       *time.Time
	Time         time.Time
}

// Page 是收件匣的一頁；Total 是同一個受眾與分類篩選下、型別已註冊的全部筆數（含組不出文字而被略過的列）。
type Page struct {
	Items []Item
	Next  *Cursor
	Total int64
}

// ListInbox 列出收件人的收件匣，新到舊。category 不為 nil 時只列該分類，分類不屬於受眾回 invalid_argument。size 已由呼叫端正規化。
// 只顯示註冊表裡有型別的列；文字依 l 組，型別自己指定的語系（例如公告只有英文）以型別為準；參數只存 ID 的型別（公告）在組字前由型別的 Load 讀取內容。
// Total 與未讀數只依型別（受眾、分類、註冊表）篩選，不看文字組不組得出來：組不出文字的列會被記錄並略過，所以 Total 可能比所有頁加起來多。
func (s *Service) ListInbox(ctx context.Context, r Recipient, category *model.PrefCategory, l uitext.Locale, after *Cursor, size int32) (Page, error) {
	if err := r.validate(); err != nil {
		return Page{}, err
	}
	types, err := s.types(ctx, r, category)
	if err != nil {
		return Page{}, err
	}
	arg := store.ListInboxItemsParams{
		AccountID: r.AccountID, Audience: string(r.Audience), StoreID: r.storeID(), Types: types, MaxRows: size + 1,
	}
	if after != nil {
		arg.AfterUpdatedAt = &after.UpdatedAt
		arg.AfterID = &after.ID
	}
	var rows []store.ListInboxItemsRow
	var page Page
	// 列與總筆數在同一份快照讀，兩者之間提交的新通知不會讓 total 與列對不上。
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		var err error
		if rows, err = q.ListInboxItems(ctx, arg); err != nil {
			return fmt.Errorf("讀取收件匣：%w", err)
		}
		if page.Total, err = q.CountInboxItems(ctx, store.CountInboxItemsParams{AccountID: r.AccountID, Audience: string(r.Audience), StoreID: r.storeID(), Types: types}); err != nil {
			return fmt.Errorf("計算收件匣筆數：%w", err)
		}
		return nil
	})
	if err != nil {
		return Page{}, err
	}
	if len(rows) > int(size) {
		last := rows[size-1]
		page.Next = &Cursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
		rows = rows[:size]
	}
	for _, row := range rows {
		item, err := s.render(ctx, l, row)
		if err != nil {
			// 一列組不出文字（參數快照壞掉）不拖垮整份收件匣：記錄後略過這一列。
			s.d.Logger.ErrorContext(ctx, "收件匣列組不出文字，略過", "inbox_item_id", row.ID, "type", row.Type, "error", err)
			continue
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

func (s *Service) render(ctx context.Context, l uitext.Locale, row store.ListInboxItemsRow) (Item, error) {
	e, ok := s.d.Registry.Lookup(row.Type)
	if !ok {
		// types 只含註冊表的型別，查詢之後型別不會消失：走到這裡是註冊表不一致。
		return Item{}, fmt.Errorf("收件匣列 %s 的型別 %q 沒有註冊", row.ID, row.Type)
	}
	params := json.RawMessage(row.Params)
	loaded, err := e.Load(ctx, s, params)
	if err != nil {
		return Item{}, fmt.Errorf("解析收件匣列 %s（%s）的內容：%w", row.ID, row.Type, err)
	}
	out, err := e.RenderLoaded(l, params, loaded)
	if err != nil {
		return Item{}, fmt.Errorf("組收件匣列 %s（%s）的文字：%w", row.ID, row.Type, err)
	}
	return Item{
		ID: row.ID, Category: e.Spec().Category, Presentation: out.Presentation, Title: out.Title, Body: out.Body,
		DeepLink: out.DeepLink, ImageKey: out.ImageKey, ReadAt: row.ReadAt, Time: row.UpdatedAt,
	}, nil
}

// types 回傳收件匣查詢要用的型別名稱；category 不屬於受眾回 invalid_argument。
func (s *Service) types(ctx context.Context, r Recipient, category *model.PrefCategory) ([]string, error) {
	if category != nil && !model.ValidCategoryFor(r.Audience, *category) {
		return nil, apperr.InvalidInputOn([]string{"Category"}, "%s", text.PreferenceNotAvailable(uitext.LocaleFrom(ctx)))
	}
	return registry.TypeNames(s.d.Registry.All(), r.Audience, category), nil
}

// UnreadCount 回傳收件人的未讀數（不分分類）。
func (s *Service) UnreadCount(ctx context.Context, r Recipient) (int64, error) {
	if err := r.validate(); err != nil {
		return 0, err
	}
	n, err := store.New(s.d.DB.Conn(ctx)).CountUnreadInboxItems(ctx, store.CountUnreadInboxItemsParams{
		AccountID: r.AccountID, Audience: string(r.Audience), StoreID: r.storeID(), Types: registry.TypeNames(s.d.Registry.All(), r.Audience, nil),
	})
	if err != nil {
		return 0, fmt.Errorf("計算未讀數：%w", err)
	}
	return n, nil
}

// MarkRead 把帳號的一則通知標成已讀；已讀的不改已讀時間。不存在與不是帳號的回同一個 not_found。
func (s *Service) MarkRead(ctx context.Context, accountID int64, id uuid.UUID) error {
	now := s.d.Now()
	_, err := store.New(s.d.DB.Conn(ctx)).MarkInboxItemRead(ctx, store.MarkInboxItemReadParams{Now: now, ID: id, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound(ctx)
	}
	if err != nil {
		return fmt.Errorf("標記已讀：%w", err)
	}
	return nil
}

// MarkAllRead 把收件人的全部未讀通知標成已讀，回傳這次改成已讀的筆數。
func (s *Service) MarkAllRead(ctx context.Context, r Recipient) (int64, error) {
	if err := r.validate(); err != nil {
		return 0, err
	}
	n, err := store.New(s.d.DB.Conn(ctx)).MarkAllInboxItemsRead(ctx, store.MarkAllInboxItemsReadParams{
		Now: s.d.Now(), AccountID: r.AccountID, Audience: string(r.Audience), StoreID: r.storeID(),
		Types: registry.TypeNames(s.d.Registry.All(), r.Audience, nil),
	})
	if err != nil {
		return 0, fmt.Errorf("全部標成已讀：%w", err)
	}
	return n, nil
}

// BatchDelete 刪除帳號的通知，全有或全無：ids 為空、重複或超過 MaxBatchDelete 回 invalid_argument；任何一個不存在或不是帳號的，
// 整批回滾並回 not_found（兩種情況同一個回應）。
func (s *Service) BatchDelete(ctx context.Context, accountID int64, ids []uuid.UUID) error {
	if len(ids) == 0 || len(ids) > MaxBatchDelete {
		return apperr.InvalidInputOn([]string{"Ids"}, "%s", text.InboxItemBatchInvalid(uitext.LocaleFrom(ctx)))
	}
	distinct := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		distinct[id] = struct{}{}
	}
	if len(distinct) != len(ids) {
		return apperr.InvalidInputOn([]string{"Ids"}, "%s", text.InboxItemBatchInvalid(uitext.LocaleFrom(ctx)))
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		n, err := store.New(s.d.DB.Conn(ctx)).DeleteInboxItems(ctx, store.DeleteInboxItemsParams{Ids: ids, AccountID: accountID})
		if err != nil {
			return fmt.Errorf("刪除收件匣：%w", err)
		}
		if n != int64(len(ids)) {
			return notFound(ctx)
		}
		return nil
	})
}

// Announcement 讀取公告的標題與內文（registry.Sources）：公告型別的收件匣列只存公告 ID，讀取時在這裡解析。
func (s *Service) Announcement(ctx context.Context, id uuid.UUID) (registry.AnnouncementText, error) {
	a, err := store.New(s.d.DB.Conn(ctx)).AnnouncementByID(ctx, id)
	if err != nil {
		return registry.AnnouncementText{}, fmt.Errorf("讀取公告 %s：%w", id, err)
	}
	return registry.AnnouncementText{Title: a.Title, Body: a.Body}, nil
}
