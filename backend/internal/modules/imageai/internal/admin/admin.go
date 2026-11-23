// Package admin 是後台對修圖風格與店家額度的維護（ImageAiAdminService 的 7 支方法）：規則、樂觀鎖（version 對外是 etag）、縮圖的 media 引用，
// 以及每個寫入在同一個 transaction 內寫的後台動作紀錄（adminaudit）。
package admin

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Queries 是本套件用到的 sqlc 查詢（*store.Queries 滿足它）。
type Queries interface {
	ListStylesForAdmin(ctx context.Context, category string) ([]store.ImageaiStyle, error)
	GetStyle(ctx context.Context, id uuid.UUID) (store.ImageaiStyle, error)
	LockStyle(ctx context.Context, id uuid.UUID) (store.ImageaiStyle, error)
	InsertStyle(ctx context.Context, arg store.InsertStyleParams) (store.ImageaiStyle, error)
	UpdateStyle(ctx context.Context, arg store.UpdateStyleParams) (store.ImageaiStyle, error)
	ClearDefaultStyle(ctx context.Context, arg store.ClearDefaultStyleParams) (int64, error)
	DeleteStyle(ctx context.Context, id uuid.UUID) (int64, error)
	GetQuota(ctx context.Context, storeID int64) (store.ImageaiQuota, error)
	LockQuota(ctx context.Context, storeID int64) (store.ImageaiQuota, error)
	InsertQuota(ctx context.Context, arg store.InsertQuotaParams) (store.ImageaiQuota, error)
	UpdateQuota(ctx context.Context, arg store.UpdateQuotaParams) (store.ImageaiQuota, error)
	DeleteQuota(ctx context.Context, storeID int64) (int64, error)
}

// QueriesOf 回傳 Deps.Q 的實作：ctx 所在連線（transaction 內是該 transaction）上的 sqlc 查詢。
func QueriesOf(d *db.DB) func(ctx context.Context) Queries {
	return func(ctx context.Context) Queries { return store.New(d.Conn(ctx)) }
}

// Audit 是後台動作紀錄（*adminaudit.Log）：Record 必須在呼叫方的 transaction 內。
type Audit interface {
	Record(ctx context.Context, e adminaudit.Entry) error
}

// Thumbnails 是風格縮圖在 media 的轉正與引用登記（internal/mediaref 的實作）。Finalize 在 transaction 外；Reject 與 Changed 在寫風格列的 transaction 內。
type Thumbnails interface {
	// Finalize 把新上傳的縮圖轉正；token 不可用回 InvalidInput（欄位 ThumbnailToken，message 是給使用者的文字）。
	Finalize(ctx context.Context, adminID int64, token, message string) (key, assetID string, err error)
	Reject(ctx context.Context, styleID uuid.UUID, assetID string) error
	Changed(ctx context.Context, now time.Time, styleID uuid.UUID, retired ...string) error
}

// Stores 是對 seller 的需求：Exists 確認店家存在（含已退場、已關閉的），LockNotRetired 在 transaction 內鎖住店家列並確認未退場；不存在都回 NotFound。
type Stores interface {
	Exists(ctx context.Context, storeID int64) error
	LockNotRetired(ctx context.Context, storeID int64) error
}

// Deps 是 Service 的依賴。
type Deps struct {
	// Tx 開 transaction（*db.DB 的 InTx）。
	Tx func(ctx context.Context, fn func(ctx context.Context) error) error
	// Q 回傳 ctx 所在連線（transaction 內是該 transaction）上的查詢。
	Q      func(ctx context.Context) Queries
	Audit  Audit
	Thumbs Thumbnails
	Stores Stores
	// Used 是店家本週已用的修圖次數。
	Used  func(ctx context.Context, storeID int64) (int32, error)
	NewID func() uuid.UUID
	Now   func() time.Time
}

// Service 是後台的風格與額度維護。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service { return &Service{d: d} }

const (
	styleTarget = "ai_style"
	quotaTarget = "ai_quota"

	nameConstraint    = "styles_category_name_key"
	defaultConstraint = "styles_default_per_category"
)

func validCategory(c model.StyleCategory) bool {
	return slices.Contains([]model.StyleCategory{model.CategoryProduct, model.CategoryPost}, c)
}

func isUnique(err error, constraint string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == constraint
}

func notFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func loc(ctx context.Context) uitext.Locale { return uitext.LocaleFrom(ctx) }

func styleStale(ctx context.Context) *apperr.Error {
	return apperr.VersionConflict("%s", text.AdminStyleStale(loc(ctx)))
}

func styleNotFound(ctx context.Context) *apperr.Error {
	return apperr.NotFound("%s", text.AdminStyleNotFound(loc(ctx)))
}

func toStyle(r store.ImageaiStyle) model.AdminStyle {
	return model.AdminStyle{
		ID: r.ID, Name: r.Name, Label: r.Label, Prompt: r.Prompt, Category: model.StyleCategory(r.Category),
		ThumbnailKey: r.ThumbnailKey, DisplayOrder: r.DisplayOrder, IsDefault: r.IsDefault, Enabled: r.Enabled,
		Version: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (s *Service) record(ctx context.Context, a model.Actor, targetType, targetID string, detail map[string]any) error {
	return s.d.Audit.Record(ctx, adminaudit.Entry{
		Actor: adminaudit.Actor{AdminID: a.AdminID, Action: a.Action}, TargetType: targetType, TargetID: targetID, Detail: detail,
	})
}
