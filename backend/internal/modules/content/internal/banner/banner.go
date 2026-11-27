package banner

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/images"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/validation"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

type Service struct {
	d      Deps
	images *images.Service
}

// Social 是 banner 對 social 的需求：一次問多篇貼文對匿名觀看者是不是公開可讀（social 的公開可讀規則，不在這裡重寫）。
type Social interface {
	PubliclyReadablePosts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

var _ Social = (*social.Module)(nil)

// Products 是 banner 對 catalog 的需求：商品目標要是公開可見的商品（存在、沒刪除、不是草稿、店家此刻可曝光；規則在 catalog）。
type Products interface {
	PublicProduct(ctx context.Context, productID uuid.UUID, at time.Time) (catalog.Product, error)
}

var _ Products = (*catalog.Module)(nil)

type Deps struct {
	DB       *db.DB
	Social   Social
	Products Products
	Media    model.Media
	NewID    func() uuid.UUID
	Now      func() time.Time
}

func New(d Deps) *Service {
	return &Service{d: d, images: images.New(d.Media)}
}

func (m *Service) q(ctx context.Context) *store.Queries {
	return store.New(m.d.DB.Conn(ctx))
}

func validateAction(a model.BannerAction) error {
	switch a.Type {
	case model.BannerActionPopup:
		return nil
	case model.BannerActionURL:
		u, err := url.Parse(a.Target)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return apperr.InvalidInputOn([]string{"Action.Target"}, "網址必須是 http:// 或 https:// 開頭")
		}
		return nil
	case model.BannerActionProduct, model.BannerActionPost:
		if strings.TrimSpace(a.Target) == "" {
			what := "商品"
			if a.Type == model.BannerActionPost {
				what = "貼文"
			}
			return apperr.InvalidInputOn([]string{"Action.Target"}, "請指定要連到的%s", what)
		}
		return nil
	case "":
		return apperr.InvalidInputOn([]string{"Action.Type"}, "請選擇點擊行為")
	default:
		return apperr.InvalidInputOn([]string{"Action.Type"}, "不認得的點擊行為")
	}
}

func hasEntityTarget(a model.BannerAction) bool {
	return a.Type == model.BannerActionPost || a.Type == model.BannerActionProduct
}

func canonicalEntityTarget(a model.BannerAction) (model.BannerAction, uuid.UUID, error) {
	if !hasEntityTarget(a) {
		return a, uuid.UUID{}, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(a.Target))
	if err != nil {
		what := "商品"
		if a.Type == model.BannerActionPost {
			what = "貼文"
		}
		return a, uuid.UUID{}, apperr.InvalidInputOn([]string{"Action.Target"}, "%s ID 格式不正確", what)
	}
	a.Target = id.String()
	return a, id, nil
}

func (m *Service) checkEntityTarget(ctx context.Context, a model.BannerAction, id uuid.UUID) error {
	if a.Type == model.BannerActionProduct {
		return m.checkProductTarget(ctx, id)
	}
	return m.checkPostTarget(ctx, id)
}

func (m *Service) checkProductTarget(ctx context.Context, id uuid.UUID) error {
	if _, err := m.d.Products.PublicProduct(ctx, id, m.d.Now()); err != nil {
		if errors.Is(err, catalog.ErrProductNotFound) {
			return apperr.InvalidInputOn([]string{"Action.Target"}, "找不到這個商品，或商品目前不公開")
		}
		return fmt.Errorf("content：確認 banner 的商品目標 %s：%w", id, err)
	}
	return nil
}

func (m *Service) checkPostTarget(ctx context.Context, id uuid.UUID) error {
	readable, err := m.d.Social.PubliclyReadablePosts(ctx, []uuid.UUID{id})
	if err != nil {
		return fmt.Errorf("content：確認 banner 的貼文目標 %s：%w", id, err)
	}
	if !readable[id] {
		return apperr.InvalidInputOn([]string{"Action.Target"}, "找不到這則貼文，或貼文目前不公開")
	}
	return nil
}

func normalizeAction(a model.BannerAction) model.BannerAction {
	if a.Type == model.BannerActionPopup {
		return model.BannerAction{Type: a.Type}
	}
	return a
}

const maxButtonTextRunes = 12

const (
	defaultBannerPage = 50
	maxBannerPage     = 100
	maxActiveBanners  = 20
)

func validateInput(in model.BannerInput) error {
	if err := validation.RequireText("Title", "標題", in.Title); err != nil {
		return err
	}
	if utf8.RuneCountInString(in.ButtonText) > maxButtonTextRunes {
		return apperr.InvalidInputOn([]string{"ButtonText"}, "按鈕文字不可超過 %d 個字", maxButtonTextRunes)
	}
	if in.StartsAt != nil && in.EndsAt != nil && !in.EndsAt.After(*in.StartsAt) {
		return apperr.InvalidInputOn([]string{"EndsAt"}, "結束時間必須晚於開始時間")
	}
	return validateAction(in.Action)
}

// ListBanners 回傳一頁 banner（後台，含停用的），依顯示順序。after 為 nil 表示第一頁。
func (m *Service) ListBanners(ctx context.Context, after *model.BannerCursor, size int32) (model.BannerPage, error) {
	size, err := pagination.PageSize(size, defaultBannerPage, maxBannerPage)
	if err != nil {
		return model.BannerPage{}, err
	}
	p := store.ListBannersParams{MaxRows: size + 1}
	if after != nil {
		p.HasCursor, p.AfterOrder, p.AfterCreated, p.AfterID = true, after.DisplayOrder, after.CreatedAt, after.ID
	}
	rows, err := m.q(ctx).ListBanners(ctx, p)
	if err != nil {
		return model.BannerPage{}, fmt.Errorf("content：列出 banner：%w", err)
	}
	return toBannerPage(rows, size), nil
}

// ListLiveBanners 回傳前台此刻要顯示的一頁 banner：啟用中，而且在顯示期間內。after 為 nil 表示第一頁。
func (m *Service) ListLiveBanners(ctx context.Context, after *model.BannerCursor, size int32) (model.BannerPage, error) {
	size, err := pagination.PageSize(size, defaultBannerPage, maxBannerPage)
	if err != nil {
		return model.BannerPage{}, err
	}
	p := store.ListLiveBannersParams{Now: m.d.Now(), MaxRows: size + 1}
	if after != nil {
		p.HasCursor, p.AfterOrder, p.AfterCreated, p.AfterID = true, after.DisplayOrder, after.CreatedAt, after.ID
	}
	rows, err := m.q(ctx).ListLiveBanners(ctx, p)
	if err != nil {
		return model.BannerPage{}, fmt.Errorf("content：列出顯示中的 banner：%w", err)
	}
	page := toBannerPage(rows, size)
	page.Banners, err = m.dropUnreadablePostTargets(ctx, page.Banners)
	if err != nil {
		return model.BannerPage{}, err
	}
	return page, nil
}

func (m *Service) dropUnreadablePostTargets(ctx context.Context, banners []model.Banner) ([]model.Banner, error) {
	if !slices.ContainsFunc(banners, isPostBanner) {
		return banners, nil
	}
	var ids []uuid.UUID
	for _, b := range banners {
		if isPostBanner(b) {
			ids = append(ids, postBannerTarget(b))
		}
	}
	readable, err := m.d.Social.PubliclyReadablePosts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("content：確認 banner 的貼文目標：%w", err)
	}
	out := make([]model.Banner, 0, len(banners))
	for _, b := range banners {
		if isPostBanner(b) {
			if !readable[postBannerTarget(b)] {
				continue
			}
		}
		out = append(out, b)
	}
	return out, nil
}

func postBannerTarget(b model.Banner) uuid.UUID { return uuid.MustParse(b.Action.Target) }

func isPostBanner(b model.Banner) bool { return b.Action.Type == model.BannerActionPost }

func toBannerPage(rows []store.ContentBanner, size int32) model.BannerPage {
	var next *model.BannerCursor
	if len(rows) > int(size) {
		rows = rows[:size]
		last := rows[len(rows)-1]
		next = &model.BannerCursor{DisplayOrder: last.DisplayOrder, CreatedAt: last.CreatedAt, ID: last.ID}
	}
	banners := make([]model.Banner, len(rows))
	for i, r := range rows {
		banners[i] = toBanner(r)
	}
	return model.BannerPage{Banners: banners, Next: next}
}

// Banner 回傳一則 banner。
func (m *Service) Banner(ctx context.Context, id uuid.UUID) (model.Banner, error) {
	row, err := m.q(ctx).GetBanner(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Banner{}, apperr.NotFound("找不到這則 banner")
	}
	if err != nil {
		return model.Banner{}, fmt.Errorf("content：讀取 banner %s：%w", id, err)
	}
	return toBanner(row), nil
}

// CreateBanner 新增 banner。預覽圖與彈窗圖都必須是新上傳的圖。
func (m *Service) CreateBanner(ctx context.Context, adminID int64, in model.BannerInput) (model.Banner, error) {
	if in.PreviewImageToken == "" || in.PopupImageToken == "" {
		var fields []string
		if in.PreviewImageToken == "" {
			fields = append(fields, "PreviewImageToken")
		}
		if in.PopupImageToken == "" {
			fields = append(fields, "PopupImageToken")
		}
		return model.Banner{}, apperr.InvalidInputOn(fields, "新增 banner 必須上傳預覽圖與彈窗圖")
	}
	if err := validateInput(in); err != nil {
		return model.Banner{}, err
	}
	canonical, targetID, err := canonicalEntityTarget(in.Action)
	if err != nil {
		return model.Banner{}, err
	}
	in.Action = canonical
	if hasEntityTarget(canonical) {
		if err := m.checkEntityTarget(ctx, canonical, targetID); err != nil {
			return model.Banner{}, err
		}
	}
	if in.IsActive {
		if err := m.checkActiveBannerLimit(ctx, uuid.UUID{}); err != nil {
			return model.Banner{}, err
		}
	}
	id := m.d.NewID()
	action := normalizeAction(in.Action)
	var row store.ContentBanner
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		resolved, err := m.images.Confirm(ctx, adminID,
			images.NewSlot("PreviewImageToken", in.PreviewImageToken, ""), images.NewSlot("PopupImageToken", in.PopupImageToken, ""))
		if err != nil {
			return err
		}
		row, err = q.InsertBanner(ctx, store.InsertBannerParams{
			ID:                  id,
			Title:               in.Title,
			ButtonText:          in.ButtonText,
			PreviewImageKey:     resolved[0].Key,
			PopupImageKey:       resolved[1].Key,
			CanvasJson:          in.CanvasJSON,
			ActionType:          string(action.Type),
			ActionTarget:        action.Target,
			DisplayOrder:        in.DisplayOrder,
			IsActive:            in.IsActive,
			StartDate:           in.StartsAt,
			EndDate:             in.EndsAt,
			PreviewImageAssetID: *resolved[0].AssetID,
			PopupImageAssetID:   *resolved[1].AssetID,
		})
		if err != nil {
			return fmt.Errorf("content：新增 banner：%w", err)
		}
		return nil
	})
	if err != nil {
		return model.Banner{}, err
	}
	return toBanner(row), nil
}

// UpdateBanner 整筆覆寫 banner。每張圖可以換新（staged token）或沿用現在的圖（空字串）。
//
// version 是讀取時拿到的版本，與目前的不符就回 VersionConflict、不寫入。轉正新圖之前先比對一次，
// 少一些注定被拒的轉正；最後以條件式 UPDATE 為準。
func (m *Service) UpdateBanner(ctx context.Context, input model.UpdateBannerInput) (model.Banner, error) {
	if err := validation.RequireVersion(" banner ", input.Version); err != nil {
		return model.Banner{}, err
	}
	if err := validateInput(input.Banner); err != nil {
		return model.Banner{}, err
	}
	cur, err := m.Banner(ctx, input.ID)
	if err != nil {
		return model.Banner{}, err
	}
	if cur.Version != input.Version {
		return model.Banner{}, bannerConflict()
	}
	if input.Banner.Action != cur.Action {
		canonical, targetID, err := canonicalEntityTarget(input.Banner.Action)
		if err != nil {
			return model.Banner{}, err
		}
		input.Banner.Action = canonical
		if canonical != cur.Action && hasEntityTarget(canonical) {
			if err := m.checkEntityTarget(ctx, canonical, targetID); err != nil {
				return model.Banner{}, err
			}
		}
	}
	if input.Banner.IsActive && !cur.IsActive {
		if err := m.checkActiveBannerLimit(ctx, input.ID); err != nil {
			return model.Banner{}, err
		}
	}
	slots := []images.Slot{
		images.NewSlot("PreviewImageToken", input.Banner.PreviewImageToken, cur.PreviewImageKey),
		images.NewSlot("PopupImageToken", input.Banner.PopupImageToken, cur.PopupImageKey),
	}
	if err := images.Require(ctx, slots...); err != nil {
		return model.Banner{}, err
	}
	action := normalizeAction(input.Banner.Action)
	var row store.ContentBanner
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		old, err := q.LockBannerAssets(ctx, input.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return m.bannerWriteMissed(ctx, input.ID)
		}
		if err != nil {
			return fmt.Errorf("content：鎖定 banner %s：%w", input.ID, err)
		}
		resolved, err := m.images.Confirm(ctx, input.AdminID, slots...)
		if err != nil {
			return err
		}
		row, err = q.UpdateBanner(ctx, store.UpdateBannerParams{
			ID:                  input.ID,
			Title:               input.Banner.Title,
			ButtonText:          input.Banner.ButtonText,
			PreviewImageKey:     resolved[0].Key,
			PopupImageKey:       resolved[1].Key,
			CanvasJson:          input.Banner.CanvasJSON,
			ActionType:          string(action.Type),
			ActionTarget:        action.Target,
			DisplayOrder:        input.Banner.DisplayOrder,
			IsActive:            input.Banner.IsActive,
			StartDate:           input.Banner.StartsAt,
			EndDate:             input.Banner.EndsAt,
			PreviewImageAssetID: resolved[0].AssetID,
			PopupImageAssetID:   resolved[1].AssetID,
			ExpectedVersion:     input.Version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return m.bannerWriteMissed(ctx, input.ID)
		}
		if err != nil {
			return fmt.Errorf("content：修改 banner %s：%w", input.ID, err)
		}
		if !images.HasNew(resolved...) {
			return nil
		}
		before := []uuid.UUID{old.PreviewImageAssetID, old.PopupImageAssetID}
		retired := media.RetiredAssets(before, images.After(before, resolved...))
		return m.d.Media.MarkDeleted(ctx, retired)
	})
	if err != nil {
		return model.Banner{}, err
	}
	return toBanner(row), nil
}

// DeleteBanner 刪除 banner，並在同一個 transaction 內把它的圖標記刪除（終態：沒有列就沒有目前的資產）。version 為 0 表示
// 不檢查版本；不為 0 時必須與目前的相符。
func (m *Service) DeleteBanner(ctx context.Context, id uuid.UUID, version int64) error {
	return m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		old, err := q.LockBannerAssets(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return m.bannerWriteMissed(ctx, id)
		}
		if err != nil {
			return fmt.Errorf("content：鎖定 banner %s：%w", id, err)
		}
		n, err := q.DeleteBanner(ctx, store.DeleteBannerParams{ID: id, ExpectedVersion: version})
		if err != nil {
			return fmt.Errorf("content：刪除 banner %s：%w", id, err)
		}
		if n == 0 {
			return m.bannerWriteMissed(ctx, id)
		}
		return m.d.Media.MarkDeleted(ctx, media.RetiredAssets([]uuid.UUID{old.PreviewImageAssetID, old.PopupImageAssetID}, nil))
	})
}

func (m *Service) bannerWriteMissed(ctx context.Context, id uuid.UUID) error {
	if _, err := m.Banner(ctx, id); err != nil {
		return err
	}
	return bannerConflict()
}

func (m *Service) checkActiveBannerLimit(ctx context.Context, except uuid.UUID) error {
	n, err := m.q(ctx).CountActiveBanners(ctx, except)
	if err != nil {
		return fmt.Errorf("content：計算上架中的 banner：%w", err)
	}
	if n >= maxActiveBanners {
		return apperr.FailedPrecondition("上架中的 banner 最多 %d 則，請先下架其他 banner", maxActiveBanners)
	}
	return nil
}

func bannerConflict() error {
	return apperr.VersionConflict("這則 banner 已被其他人修改，請對照最新的內容後再送出")
}

func toBanner(r store.ContentBanner) model.Banner {
	return model.Banner{
		ID:              r.ID,
		Title:           r.Title,
		ButtonText:      r.ButtonText,
		PreviewImageKey: r.PreviewImageKey,
		PopupImageKey:   r.PopupImageKey,
		CanvasJSON:      r.CanvasJson,
		Action:          model.BannerAction{Type: model.BannerActionType(r.ActionType), Target: r.ActionTarget},
		DisplayOrder:    r.DisplayOrder,
		IsActive:        r.IsActive,
		StartsAt:        r.StartDate,
		EndsAt:          r.EndDate,
		Version:         r.Version,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}
