// Package announce 是後台公告：建立與列表（use case），以及把公告派送給受眾的 job 鏈（fanout.go）。
// 公告是資料不是事件型別：建立時寫一列公告、一個意圖列（id 與公告相同，讓每筆推播投遞紀錄有 intent_id）與兩條 job 鏈，
// 收件匣與推播分開處理；沒有 device token 的帳號也收得到收件匣那份（缺陷 5–7）。
package announce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

const (
	// IntentType 是公告的意圖列型別；它不在型別註冊表，也不排展開意圖的 job。
	IntentType = "announcement"
	// InboxType 是公告收件匣列的 type；文案取自公告列。
	InboxType = "announcement"

	// MaxTitleRunes、MaxBodyRunes 與 schema 的 announcements_title_check、announcements_body_check 一致。
	MaxTitleRunes = 100
	MaxBodyRunes  = 2000

	DefaultPageSize int32 = 20
	MaxPageSize     int32 = 50
)

// Recipient 是派送的一個收件人；Cursor 是它在受眾名單裡的排序鍵（BUYER 是 buyer_id，STORE 是 store_id）。
type Recipient struct {
	AccountID int64
	StoreID   int64
	Cursor    int64
}

// Directory 列出受眾的全部收件人：after 之後（不含）依 Cursor 由小到大最多 limit 筆，不足 limit 筆表示沒有更多。不加鎖。
type Directory interface {
	Buyers(ctx context.Context, after int64, limit int) ([]Recipient, error)
	Stores(ctx context.Context, after int64, limit int) ([]Recipient, error)
}

// Deps 是 Service 的依賴。
type Deps struct {
	DB        *db.DB
	Locks     intake.Locks
	Directory Directory
	Audit     *adminaudit.Log
	Jobs      jobs.Enqueuer
	Now       func() time.Time
	NewID     func() uuid.UUID
	Logger    *slog.Logger
	// Chunk 是一個派送 job 處理的收件人數上限；0 用 ChunkSize。
	Chunk int
}

// Service 是公告的 use case 與派送。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service {
	if d.Chunk <= 0 {
		d.Chunk = ChunkSize
	}
	return &Service{d: d}
}

// Announcement 是一則公告；Audiences 是 BUYER、STORE 的已排序子集。
type Announcement struct {
	ID         uuid.UUID
	Audiences  []string
	Title      string
	Body       string
	CreateTime time.Time
	// InboxCount 是已寫進收件匣的人數；派送進行中時是目前為止的數字。
	InboxCount int32
	// Dispatched 是收件匣與推播兩條 job 鏈都處理完了。
	Dispatched bool
}

// CreateInput 是建立公告的輸入。Actor.Action 是後台 RPC 的 procedure（稽核紀錄用）；RequestID 為 nil 表示不做冪等。
type CreateInput struct {
	Actor     adminaudit.Actor
	Audiences []string
	Title     string
	Body      string
	RequestID *uuid.UUID
}

// Cursor 是公告列表的排序鍵：(CreatedAt, ID) 由大到小。
type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// Valid 回報游標是否完整。
func (c Cursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

// Page 是一頁公告；Next 為 nil 表示沒有下一頁。
type Page struct {
	Announcements []Announcement
	Next          *Cursor
}

func normalize(in CreateInput) (CreateInput, error) {
	if len(in.Audiences) == 0 {
		return in, apperr.InvalidInputOn([]string{"Audiences"}, "請至少選擇一個受眾")
	}
	seen := make(map[string]bool, len(in.Audiences))
	for _, a := range in.Audiences {
		if !slices.Contains([]model.Audience{model.AudienceBuyer, model.AudienceStore}, model.Audience(a)) {
			return in, apperr.InvalidInputOn([]string{"Audiences"}, "受眾必須是買家或店家")
		}
		if seen[a] {
			return in, apperr.InvalidInputOn([]string{"Audiences"}, "受眾不可重複")
		}
		seen[a] = true
	}
	in.Audiences = slices.Sorted(slices.Values(in.Audiences))
	in.Title = trimtext.Trim(in.Title)
	in.Body = trimtext.Trim(in.Body)
	switch n := utf8.RuneCountInString(in.Title); {
	case n == 0:
		return in, apperr.InvalidInputOn([]string{"Title"}, "請輸入標題")
	case n > MaxTitleRunes:
		return in, apperr.InvalidInputOn([]string{"Title"}, "標題不可超過 %d 個字", MaxTitleRunes)
	}
	switch n := utf8.RuneCountInString(in.Body); {
	case n == 0:
		return in, apperr.InvalidInputOn([]string{"Body"}, "請輸入內文")
	case n > MaxBodyRunes:
		return in, apperr.InvalidInputOn([]string{"Body"}, "內文不可超過 %d 個字", MaxBodyRunes)
	}
	return in, nil
}

func fromRow(r store.NotificationAnnouncement) Announcement {
	return Announcement{
		ID: r.ID, Audiences: slices.Clone(r.Audiences), Title: r.Title, Body: r.Body, CreateTime: r.CreatedAt,
		InboxCount: r.InboxCount, Dispatched: r.InboxDoneAt != nil && r.PushDoneAt != nil,
	}
}

// Create 建立公告並排入派送。欄位不合回 InvalidInput（Audiences、Title、Body）。帶 RequestID 時，同一個管理員以同一個 RequestID 重送回第一次的那一則（不再排派送、
// 不再寫稽核），內容不同回 InvalidInput（RequestID）。公告列、意圖列、稽核紀錄與兩條 job 鏈的第一個 job 在同一個 transaction。
func (s *Service) Create(ctx context.Context, in CreateInput) (Announcement, error) {
	in, err := normalize(in)
	if err != nil {
		return Announcement{}, err
	}
	var out Announcement
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		id, now := s.d.NewID(), s.d.Now().UTC().Truncate(time.Microsecond)
		_, err := q.InsertAnnouncement(ctx, store.InsertAnnouncementParams{
			ID: id, Audiences: in.Audiences, Title: in.Title, Body: in.Body, AdminID: in.Actor.AdminID, RequestID: in.RequestID, CreatedAt: now,
		})
		if errors.Is(err, pgx.ErrNoRows) && in.RequestID != nil {
			prev, err := q.AnnouncementByRequest(ctx, store.AnnouncementByRequestParams{AdminID: in.Actor.AdminID, RequestID: in.RequestID})
			if err != nil {
				return fmt.Errorf("讀取先前的公告：%w", err)
			}
			if !slices.Equal(prev.Audiences, in.Audiences) || prev.Title != in.Title || prev.Body != in.Body {
				return apperr.InvalidInputOn([]string{"RequestID"}, "這個 request_id 已用於內容不同的公告")
			}
			out = fromRow(prev)
			return nil
		}
		if err != nil {
			return fmt.Errorf("寫入公告：%w", err)
		}
		params, err := json.Marshal(map[string]string{"announcement_id": id.String()})
		if err != nil {
			return err
		}
		if _, err := q.InsertIntent(ctx, store.InsertIntentParams{
			ID: id, Type: IntentType, IdempotencyKey: IntentType + ":" + id.String(), Params: params, OccurredAt: now, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("寫入公告的意圖列：%w", err)
		}
		if err := s.d.Audit.Record(ctx, adminaudit.Entry{
			Actor: in.Actor, TargetType: "announcement", TargetID: id.String(), Detail: map[string]any{"audiences": in.Audiences},
		}); err != nil {
			return err
		}
		first := model.Audience(in.Audiences[0])
		if err := s.d.Jobs.Enqueue(ctx, InboxArgs{AnnouncementID: id, Audience: first}, uniqueByArgs); err != nil {
			return fmt.Errorf("排入收件匣派送：%w", err)
		}
		if err := s.d.Jobs.Enqueue(ctx, PushArgs{AnnouncementID: id, Audience: first}, uniqueByArgs); err != nil {
			return fmt.Errorf("排入推播派送：%w", err)
		}
		out = Announcement{ID: id, Audiences: in.Audiences, Title: in.Title, Body: in.Body, CreateTime: now}
		return nil
	})
	if err != nil {
		return Announcement{}, err
	}
	return out, nil
}

var uniqueByArgs = &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}

// List 列出公告，新到舊（建立時間，再依 id）。size 依 pagination.PageSize（預設 20、上限 50）。
func (s *Service) List(ctx context.Context, after *Cursor, size int32) (Page, error) {
	size, err := pagination.PageSize(size, DefaultPageSize, MaxPageSize)
	if err != nil {
		return Page{}, err
	}
	p := store.ListAnnouncementsParams{PageSize: size + 1}
	if after != nil {
		p.AfterCreatedAt, p.AfterID = &after.CreatedAt, &after.ID
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).ListAnnouncements(ctx, p)
	if err != nil {
		return Page{}, fmt.Errorf("列出公告：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := Page{Announcements: make([]Announcement, len(rows))}
	for i, r := range rows {
		out.Announcements[i] = fromRow(r)
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, nil
}
