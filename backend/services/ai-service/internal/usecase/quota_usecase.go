package usecase

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QuotaUseCase 處理 Admin 賣家額度管理
type QuotaUseCase struct {
	q *gen.Queries
}

// NewQuotaUseCase 建立 QuotaUseCase
func NewQuotaUseCase(pool *pgxpool.Pool) *QuotaUseCase {
	return &QuotaUseCase{q: gen.New(pool)}
}

// GetQuota 查詢目前 store 的額度上限與本週使用量。storeID 已由 gateway 解析為現任
// store_id（account_id 命名債正名），此處不再二次解析；型別讓「拿 account_id 來查」
// 變成編譯錯誤。ai_quotas / ai_usage 都以 store_id 為鍵。
func (uc *QuotaUseCase) GetQuota(ctx context.Context, storeID identity.StoreID) (*dto.QuotaResponse, error) {
	limit := domain.DefaultWeeklyLimit
	isCustom := false
	memo := ""
	updatedAt := ""

	if quota, err := uc.q.GetAiQuotaByStoreID(ctx, int64(storeID)); err == nil {
		limit = int(quota.WeeklyLimit)
		isCustom = true
		memo = quota.Memo
		updatedAt = timeutil.FormatISO8601(quota.UpdatedAt.Time)
	}

	weekDate, parseErr := time.Parse("2006-01-02", currentWeekStart())
	if parseErr != nil {
		// currentWeekStart() 永遠回傳自己 Format("2006-01-02") 產生的字串，這個
		// parse 在正常執行下不可能失敗；失敗代表程式邏輯本身壞了，直接回傳
		// 錯誤，不吞掉繼續用零值日期查詢（跟 style_usecase.go 的
		// getUsageAndLimit 同一個理由，Wave 2 已建立的先例）。
		return nil, fmt.Errorf("parse week start: %w", parseErr)
	}

	usage := 0
	if u, err := uc.q.GetAiUsageByStoreIDAndWeek(ctx, gen.GetAiUsageByStoreIDAndWeekParams{
		StoreID:   int64(storeID),
		WeekStart: pgxutil.DateParam(&weekDate),
	}); err == nil {
		usage = int(u.UsageCount)
	}

	return &dto.QuotaResponse{
		StoreID:     strconv.FormatInt(int64(storeID), 10),
		WeeklyLimit: strconv.Itoa(limit),
		IsCustom:    isCustom,
		WeeklyUsage: strconv.Itoa(usage),
		Memo:        memo,
		UpdatedAt:   updatedAt,
	}, nil
}

// UpdateQuota 設定目前 store 的自訂額度上限。
func (uc *QuotaUseCase) UpdateQuota(ctx context.Context, storeID identity.StoreID, req dto.UpdateQuotaRequest) error {
	limit, err := strconv.Atoi(req.WeeklyLimit)
	if err != nil || limit < 0 || limit > 1000 {
		// 使用者輸入驗證，不含系統細節：用 DomainError（Code 400，無 Cause）讓
		// grpc handler 的 domainMessage() 判為已分類業務錯誤，訊息原樣透傳；
		// 不歸類的話會跟真正的 DB 錯誤一起被 handler 的 fallback 換成固定文案
		// genericFailureMessage，吃掉這則對呼叫端有意義的訊息（#3519 slice B
		// Task 4 fix round 1）。NewBusiness(400, ...) 建出的 DomainError 沒有
		// Cause，Error() 與 DomainMessage() 回傳同一個字串，跟修復前
		// err.Error() 的行為在 wire 上逐字元相同。
		return pkgerrors.NewBusiness(400, "weeklyLimit 必須為 0~1000 的整數")
	}

	return uc.q.UpsertAiQuota(ctx, gen.UpsertAiQuotaParams{
		StoreID:     int64(storeID),
		WeeklyLimit: int32(limit),
		Memo:        req.Memo,
	})
}

// DeleteQuota 刪除目前 store 的自訂額度（恢復預設）
func (uc *QuotaUseCase) DeleteQuota(ctx context.Context, storeID identity.StoreID) error {
	return uc.q.DeleteAiQuota(ctx, int64(storeID))
}
