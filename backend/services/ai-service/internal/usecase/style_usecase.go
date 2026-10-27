package usecase

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	pkgtsid "github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	styleCacheKey = "ai_styles:list"
	styleCacheTTL = 5 * time.Minute
)

var fallbackDefaultPrompts = map[string]string{
	domain.CategoryProduct: "優化這張農產品圖片，增強其視覺吸引力。保持產品的自然外觀，同時提升整體品質和專業感。Enhance this produce image to make it more visually appealing while maintaining its natural appearance.",
	domain.CategoryPost:    "優化這張貼文照片，增強其視覺吸引力與社群感，保持照片原有風格與主體清晰。Enhance this post image to make it more visually engaging and social-media friendly while preserving its original character and subject clarity.",
}

// StyleUseCase 處理 AI 風格管理的業務邏輯
type StyleUseCase struct {
	q         *gen.Queries
	pool      *pgxpool.Pool
	txManager database.TxManager
	cache     *cache.MemoryCache
	genID     func() int64
}

// NewStyleUseCase 建立 StyleUseCase
func NewStyleUseCase(pool *pgxpool.Pool) *StyleUseCase {
	return &StyleUseCase{
		q:         gen.New(pool),
		pool:      pool,
		txManager: database.NewTxManager(pool),
		cache:     cache.New(styleCacheTTL),
		genID:     pkgtsid.NewGenerator(),
	}
}

// GenID 產生新的 TSID，供呼叫端在呼叫 Create 前預先取得 ID。
func (uc *StyleUseCase) GenID() int64 {
	return uc.genID()
}

// ListForApp 取得啟用中的風格列表 + 使用量（依 category 過濾）。storeID 已由 gateway
// 解析為現任 store_id（account_id 命名債正名），此處不再二次解析。
func (uc *StyleUseCase) ListForApp(ctx context.Context, storeID identity.StoreID, category string) (*dto.StyleListResponse, error) {
	styles, defaultPrompt, err := uc.getStylesFromCacheOrDB(ctx, category)
	if err != nil {
		return nil, err
	}

	usage, limit, err := uc.getUsageAndLimit(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("get usage and limit: %w", err)
	}

	return &dto.StyleListResponse{
		Styles:        styles,
		DefaultPrompt: defaultPrompt,
		WeeklyUsage:   strconv.Itoa(usage),
		WeeklyLimit:   strconv.Itoa(limit),
	}, nil
}

// ListForAdmin 取得全部風格（含 disabled、含 default）。category 為空字串代表不篩選。
func (uc *StyleUseCase) ListForAdmin(ctx context.Context, category string) ([]dto.AdminStyleItem, error) {
	rows, err := uc.q.ListAiStyles(ctx, category)
	if err != nil {
		return nil, err
	}

	items := make([]dto.AdminStyleItem, len(rows))
	for i, r := range rows {
		items[i] = dto.AdminStyleItem{
			StyleItem: dto.StyleItem{
				ID:           strconv.FormatInt(r.ID, 10),
				Name:         r.Name,
				Label:        r.Label,
				Prompt:       r.Prompt,
				ThumbnailKey: r.ThumbnailKey,
				SortOrder:    strconv.Itoa(int(r.SortOrder)),
			},
			Category:  r.Category,
			IsDefault: r.IsDefault,
			Enabled:   r.Enabled,
			CreatedAt: timeutil.FormatISO8601(r.CreatedAt.Time),
			UpdatedAt: timeutil.FormatISO8601(r.UpdatedAt.Time),
		}
	}
	return items, nil
}

// parseSortOrder 把 SortOrder 字串解析成 int32。任何解析失敗（空字串、非數字，
// 甚至超出 int64 的離譜輸入）一律退回 0，維持遷移前 strconv.Atoi(s); _ 的「解析
// 失敗就當沒填、預設 0」語意——這條路徑在正常操作下可達：gRPC
// CreateAiStyleRequest.SortOrder 沒有 has-flag，client 沒設就是空字串（見
// internal/grpc/handler.go:392 直接把 req.SortOrder 塞進 dto.CreateStyleRequest，
// 沒有前置驗證）。只有「解析成功、但數值超出 int32 範圍」才視為錯誤——這是遷移
// 前唯一有實際差異的情形：舊版讓這個值一路帶到 Postgres 的 INTEGER 欄位，由 DB
// 端的範圍檢查擋下；換成 sqlc/pgx 後 int32(...) 轉型不會報錯、只會靜默
// wrap-around，因此改成在 Go 端顯式檢查範圍，同時不吃掉單純「沒填」的情形。
func parseSortOrder(s string) (int32, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		v = 0
	}
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, fmt.Errorf("sort order out of range: %d", v)
	}
	return int32(v), nil
}

// Create 新增風格。id 須由呼叫端透過 GenID() 預先產生（TSID）。
func (uc *StyleUseCase) Create(ctx context.Context, id int64, req dto.CreateStyleRequest) error {
	sortOrder, err := parseSortOrder(req.SortOrder)
	if err != nil {
		return err
	}
	_, err = uc.q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID:           id,
		Name:         req.Name,
		Label:        req.Label,
		Prompt:       req.Prompt,
		ThumbnailKey: req.ThumbnailKey,
		Category:     req.Category,
		SortOrder:    sortOrder,
		IsDefault:    req.IsDefault,
		Enabled:      req.Enabled,
	})
	if err != nil {
		return err
	}
	uc.invalidateStyleCache()
	return nil
}

// Update 修改風格
func (uc *StyleUseCase) Update(ctx context.Context, id int64, req dto.UpdateStyleRequest) error {
	// IsDefault 只有在明確為 true 時才會被寫入（見下方 141 行的 *req.IsDefault
	// 判斷）——原本 gorm 版本的 updates map 也只有 IsDefault==true 才會塞
	// "is_default" 這個 key，nil 或明確 false 都不會讓 map 變非空。這裡的
	// early-return 必須跟那個語意完全對齊：req.IsDefault == nil 不夠，還要排除
	// 「明確傳 false」這個同樣不該觸發任何 DB 寫入的情形，否則會對一個所有欄位
	// 邏輯上都沒變的請求發出一次多餘的 UPDATE（甚至在 id 不存在時誤報
	// "style not found"，而不是原本的靜默成功）。
	setsIsDefaultTrue := req.IsDefault != nil && *req.IsDefault
	if req.Label == nil && req.Prompt == nil && req.ThumbnailKey == nil &&
		req.SortOrder == nil && req.Enabled == nil && !setsIsDefaultTrue {
		return nil
	}

	params := gen.UpdateAiStyleParams{
		ID:           id,
		Label:        req.Label,
		Prompt:       req.Prompt,
		ThumbnailKey: req.ThumbnailKey,
	}
	if req.SortOrder != nil {
		v, err := parseSortOrder(*req.SortOrder)
		if err != nil {
			// parseSortOrder 的錯誤是使用者輸入驗證，不含系統細節；只在這裡（不是
			// 在 parseSortOrder 本身，也不影響 Create() 呼叫它的那條路徑）歸類成
			// DomainError，理由是這支錯誤從這裡開始會被 grpc handler
			// UpdateAiStyle 的未歸類 fallback 換成 genericFailureMessage
			// （#3519 slice B Task 4 fix round 1）——Create() 呼叫 parseSortOrder
			// 走的是 return nil, err → ErrorInterceptor，並未受影響，不需要、也
			// 不應該跟著改（那會把一個原本的 gRPC transport error 變成 envelope
			// Code:400，是本刀 Global Constraint 之外的契約形狀改動）。
			// NewBusiness(400, ...) 沒有 Cause，Message 逐字元等於修復前的
			// err.Error()。
			return pkgerrors.NewBusiness(400, err.Error())
		}
		params.SortOrder = &v
	}
	if req.Enabled != nil {
		params.Enabled = req.Enabled
	}

	if setsIsDefaultTrue {
		// 清除範圍由「目標所屬分類」決定，與 idx_ai_styles_default 的述詞
		// (category) 同源（見 db/queries/style.sql）。ClearCategoryDefaultStyle
		// 必須先於 MarkAiStyleDefault 執行——兩句合起來是「先把該分類
		// 現有的 true 全部降成 false（不會冒出新的 true，不可能撞 partial unique
		// index），再把目標設成 true（此時分類內已無 true，不可能撞）」，順序反過來
		// 會間歇性撞 idx_ai_styles_default 的 23505（同一句 UPDATE 用 CASE 同時
		// 搬移一整個分類也一樣會撞，取決於掃描順序；細節見 style.sql 的註解）。跟
		// UpdateAiStyle 包進同一個交易：任何一句失敗——包含 UpdateAiStyle 回報 0
		// rows（id 不存在）——前面對 is_default 的搬動都要一起回滾，不留下「別的
		// 分類已經被清掉預設，但目標 id 根本不存在」的半套狀態。not-found 的判斷
		// 因此特意留在這個閉包內部（回傳 error 給 WithTx 觸發 rollback），不是等
		// WithTx 回傳後才在外面檢查——那樣的話 UpdateAiStyle 之前的兩句已經
		// commit，「任何一句失敗都會回滾」這句話對 not-found 分支就不成立。
		// gen.New(database.GetDBTX(txCtx, uc.pool)) 是跟 TransformUseCase.q(ctx)
		// 同一個既有 pattern（transform_usecase.go:65-69）。
		err := uc.txManager.WithTx(ctx, func(txCtx context.Context) error {
			q := gen.New(database.GetDBTX(txCtx, uc.pool))
			if _, err := q.ClearCategoryDefaultStyle(txCtx, id); err != nil {
				return err
			}
			if _, err := q.MarkAiStyleDefault(txCtx, id); err != nil {
				return err
			}
			affected, err := q.UpdateAiStyle(txCtx, params)
			if err != nil {
				return err
			}
			if affected == 0 {
				return pkgerrors.NewBusiness(400, "style not found")
			}
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		affected, err := uc.q.UpdateAiStyle(ctx, params)
		if err != nil {
			return err
		}
		if affected == 0 {
			return pkgerrors.NewBusiness(400, "style not found")
		}
	}

	uc.invalidateStyleCache()
	return nil
}

// Delete 刪除風格
func (uc *StyleUseCase) Delete(ctx context.Context, id int64) error {
	// 不可刪除 default
	style, err := uc.q.GetAiStyleByID(ctx, id)
	if err != nil {
		// 這裡把「查無此列」與「查詢本身失敗」(連線、逾時等真正的 DB 故障)
		// 都當成 "style not found"——wire 回應維持不變（pre-existing 行為，
		// #3519 fix round 2 明確不改），但真正的 DB 故障不能因此無聲無息：
		// 補一行 log 讓值班看得到底層原因，跟本刀在 handler.go 對「未歸類
		// fallback」做的「訊息固定、原因照記」是同一個模式。
		pkglogger.Error(ctx, "get ai style by id failed before delete", zap.Int64("id", id), zap.Error(err))
		return pkgerrors.NewBusiness(400, "style not found")
	}
	if style.IsDefault {
		return pkgerrors.NewBusiness(400, "cannot delete default style")
	}
	if _, err := uc.q.DeleteAiStyle(ctx, id); err != nil {
		return err
	}
	uc.invalidateStyleCache()
	return nil
}

// invalidateStyleCache 清掉兩個分類的快取。Update/Delete 不一定知道目標 row 的
// category（category 建立後不可變更，patch payload 不帶這欄），與其多查一次 DB
// 才能精準清單一分類，不如兩個都清——快取只是 5 分鐘 TTL 的效能優化，多清一個
// 不存在的 key 成本是零。
func (uc *StyleUseCase) invalidateStyleCache() {
	uc.cache.Delete(styleCacheKey + ":" + domain.CategoryProduct)
	uc.cache.Delete(styleCacheKey + ":" + domain.CategoryPost)
}

// getStylesFromCacheOrDB 從快取或 DB 取得指定分類的風格列表。假設 category 永遠非空——
// gateway 的 gRPC handler 會把未知／空分類正規化成 "product" 才呼叫進來（見
// internal/grpc/handler.go 的 ListAiStyles）。若日後拿掉那個正規化，"" 會落到
// ListAiStyles 的「不篩選」語意，回傳兩個分類混合的結果，且快取在
// styleCacheKey+":"（空字串）這個 key 下，invalidateStyleCache 目前只清
// product／post 兩個明確 key，不會清到它，會在任何寫入後照樣 stale 滿一個 TTL。
func (uc *StyleUseCase) getStylesFromCacheOrDB(ctx context.Context, category string) ([]dto.StyleItem, string, error) {
	type cached struct {
		DefaultPrompt string
		Styles        []dto.StyleItem
	}

	cacheKey := styleCacheKey + ":" + category
	if v, ok := uc.cache.Get(cacheKey); ok {
		c := v.(*cached)
		return c.Styles, c.DefaultPrompt, nil
	}

	rows, err := uc.q.ListAiStyles(ctx, category)
	if err != nil {
		return nil, "", err
	}

	var styles []dto.StyleItem
	defaultPrompt := fallbackDefaultPrompts[category]

	for _, r := range rows {
		if r.IsDefault {
			defaultPrompt = r.Prompt
			continue
		}
		if !r.Enabled {
			continue
		}
		styles = append(styles, dto.StyleItem{
			ID:           strconv.FormatInt(r.ID, 10),
			Name:         r.Name,
			Label:        r.Label,
			Prompt:       r.Prompt,
			ThumbnailKey: r.ThumbnailKey,
			SortOrder:    strconv.Itoa(int(r.SortOrder)),
		})
	}

	uc.cache.Set(cacheKey, &cached{Styles: styles, DefaultPrompt: defaultPrompt})
	return styles, defaultPrompt, nil
}

// getUsageAndLimit 取得目前 store 的額度上限與本週使用量。storeID 已由 gateway 解析
// 為現任 store_id，此處直接查 ai_quotas / ai_usage（見 TransformUseCase.Transform 的說明）。
func (uc *StyleUseCase) getUsageAndLimit(ctx context.Context, storeID identity.StoreID) (usage int, limit int, err error) {
	limit = domain.DefaultWeeklyLimit
	weekStart := currentWeekStart()

	weekDate, parseErr := time.Parse("2006-01-02", weekStart)
	if parseErr != nil {
		// currentWeekStart() 永遠回傳自己 Format("2006-01-02") 產生的字串，這個
		// parse 在正常執行下不可能失敗；失敗代表程式邏輯本身壞了，直接回傳
		// 錯誤，不吞掉繼續用零值日期查詢。
		return 0, limit, fmt.Errorf("parse week start: %w", parseErr)
	}

	if quota, qerr := uc.q.GetAiQuotaByStoreID(ctx, int64(storeID)); qerr == nil {
		limit = int(quota.WeeklyLimit)
	}

	if u, uerr := uc.q.GetAiUsageByStoreIDAndWeek(ctx, gen.GetAiUsageByStoreIDAndWeekParams{
		StoreID:   int64(storeID),
		WeekStart: pgxutil.DateParam(&weekDate),
	}); uerr == nil {
		usage = int(u.UsageCount)
	}

	return usage, limit, nil
}

// weekStartFor returns the Monday (Asia/Taipei) of now's week as YYYY-MM-DD.
// Taiwan-local so the seller's "this week / resets Monday" matches wall-clock.
func weekStartFor(now time.Time) string {
	t := now.In(timeutil.LocationTaipei)
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := t.AddDate(0, 0, -int(weekday-time.Monday))
	return monday.Format("2006-01-02")
}

func currentWeekStart() string { return weekStartFor(time.Now()) }
