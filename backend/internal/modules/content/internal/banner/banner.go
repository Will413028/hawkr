package banner

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/images"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/validation"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

type Service struct {
	d      Deps
	images *images.Service
}

type Deps struct {
	DB    *db.DB
	Media media.Media
	Jobs  jobs.Enqueuer
	NewID func() uuid.UUID
	Now   func() time.Time
}

func New(d Deps) *Service {
	return &Service{d: d, images: images.New(d.Media)}
}

func (m *Service) q(ctx context.Context) *store.Queries {
	return store.New(m.d.DB.Conn(ctx))
}

// validate 要求行為完整：型別合法，而且帶著該型別需要的值。
//
// 修改 banner 是整筆覆寫，空的或不完整的行為必須在寫入端拒絕。url 只接受 http、https，
// 避免後台對 App 觸發任意 scheme。
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

// normalized 去掉 popup 不該有的值（表單殘留）。
func normalizeAction(a model.BannerAction) model.BannerAction {
	if a.Type == model.BannerActionPopup {
		return model.BannerAction{Type: a.Type}
	}
	return a
}

// maxButtonTextRunes 是 banner 按鈕文字的上限（字元數），與後台 banner 編輯器的輸入上限相同。
const maxButtonTextRunes = 12

// banner 清單每頁筆數的預設值與上限。
const (
	defaultBannerPage = 50
	maxBannerPage     = 100
	// maxActiveBanners 是啟用中的 banner 上限，上架時檢查。遠小於前台 ListLiveBanners 的預設頁大小，client
	// 讀第一頁就拿到全部；兩個人同時上架可能超過一兩則，仍在一頁之內。
	maxActiveBanners = 20
)

func validateInput(in model.BannerInput) error {
	if err := validation.RequireText("Title", "標題", in.Title); err != nil {
		return err
	}
	if utf8.RuneCountInString(in.ButtonText) > maxButtonTextRunes {
		return apperr.InvalidInputOn([]string{"ButtonText"}, "按鈕文字不可超過 %d 個字", maxButtonTextRunes)
	}
	// 排程是 [開始, 結束)：結束的那一刻就下架，所以結束必須晚於開始。
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
	return toBannerPage(rows, size), nil
}

// toBannerPage 把多取一筆的結果切成一頁：多出來的那筆存在，就表示還有下一頁。
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
	// 先驗證再轉正：驗證失敗時不該讓圖片掛在一則永遠不會建立的 banner 上。
	if err := validateInput(in); err != nil {
		return model.Banner{}, err
	}
	if in.IsActive {
		if err := m.checkActiveBannerLimit(ctx, uuid.UUID{}); err != nil {
			return model.Banner{}, err
		}
	}
	id := m.d.NewID()
	ref := mediaref.BannerRef(id)
	resolved, err := m.images.Resolve(ctx, adminID, ref.Type,
		images.NewSlot("PreviewImageToken", "預覽圖", in.PreviewImageToken, ""), images.NewSlot("PopupImageToken", "彈窗圖", in.PopupImageToken, ""))
	if err != nil {
		return model.Banner{}, err
	}
	action := normalizeAction(in.Action)
	var row store.ContentBanner
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		if err := mediaref.Reject(ctx, q, ref, images.NewCandidates(resolved...)...); err != nil {
			return err
		}
		var err error
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
			PreviewImageAssetID: resolved[0].Asset.ID,
			PopupImageAssetID:   resolved[1].Asset.ID,
		})
		if err != nil {
			return fmt.Errorf("content：新增 banner：%w", err)
		}
		return mediaref.Changed(ctx, q, m.d.Jobs, m.d.Now().UTC(), ref)
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
	// 只有「上架」這個動作要檢查上限；已經上架的 banner 修改其他欄位不受影響。
	if input.Banner.IsActive && !cur.IsActive {
		if err := m.checkActiveBannerLimit(ctx, input.ID); err != nil {
			return model.Banner{}, err
		}
	}
	ref := mediaref.BannerRef(input.ID)
	resolved, err := m.images.Resolve(ctx, input.AdminID, ref.Type,
		images.NewSlot("PreviewImageToken", "預覽圖", input.Banner.PreviewImageToken, cur.PreviewImageKey),
		images.NewSlot("PopupImageToken", "彈窗圖", input.Banner.PopupImageToken, cur.PopupImageKey))
	if err != nil {
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
		if err := mediaref.Reject(ctx, q, ref, images.NewCandidates(resolved...)...); err != nil {
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
			PreviewImageAssetID: resolved[0].AssetPtr(),
			PopupImageAssetID:   resolved[1].AssetPtr(),
			ExpectedVersion:     input.Version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return m.bannerWriteMissed(ctx, input.ID)
		}
		if err != nil {
			return fmt.Errorf("content：修改 banner %s：%w", input.ID, err)
		}
		if len(images.NewAssetIDs(resolved...)) == 0 {
			return nil
		}
		before := []string{old.PreviewImageAssetID, old.PopupImageAssetID}
		retired := images.Retired(before, images.After(before, resolved...))
		return mediaref.Changed(ctx, q, m.d.Jobs, m.d.Now().UTC(), ref, retired...)
	})
	if err != nil {
		return model.Banner{}, err
	}
	return toBanner(row), nil
}

// DeleteBanner 刪除 banner，並把它的圖記成退場、排入引用的同步（終態：沒有列的 ref 沒有目前的資產）。version 為 0 表示
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
		retired := images.Retired([]string{old.PreviewImageAssetID, old.PopupImageAssetID}, nil)
		return mediaref.Changed(ctx, q, m.d.Jobs, m.d.Now().UTC(), mediaref.BannerRef(id), retired...)
	})
}

// bannerWriteMissed 在條件式的修改或刪除沒有碰到任何列時，分辨是 banner 不存在還是版本不符。
func (m *Service) bannerWriteMissed(ctx context.Context, id uuid.UUID) error {
	if _, err := m.Banner(ctx, id); err != nil {
		return err
	}
	return bannerConflict()
}

// checkActiveBannerLimit 在上架前確認啟用中的 banner（不含 except）還沒到上限。
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
