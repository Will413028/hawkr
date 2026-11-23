// Package membership 提供額度週期與預設額度。
package membership

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// 每個買家的預設權益。沒有方案表與指派入口：額度的來源是這兩個常數，值與歷史 migration
// db/migrations/20260928230628_buyer_membership_plan_seeds.sql 的 free 列相同（計費設計開工時再建方案模型）。
const (
	DefaultPlanCode                 = "free"
	DefaultMonthlyAnswerLimit int32 = 100
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB  *db.DB
	Now func() time.Time
}

// Service 此子領域的 use case 容器。
type Service struct {
	deps Deps
}

// New 建立服務，外部依賴由組裝點提供。
func New(deps Deps) *Service { return &Service{deps: deps} }

// Period 依錨點逐月滾動，月底 clamp 後不回補日期。
func Period(anchor, now time.Time) (time.Time, time.Time) {
	start := anchor.UTC()
	end := nextMonth(start)
	for !now.Before(end) {
		start = end
		end = nextMonth(start)
	}
	return start, end
}

func nextMonth(current time.Time) time.Time {
	year, month, day := current.Date()
	last := time.Date(year, month+2, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(year, month+1, min(day, last), current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), time.UTC)
}

// Entitlement 讀取有效權益：每個買家都是免費方案，額度週期以 stint 的建立時間為錨點逐月滾動。
func (service *Service) Entitlement(ctx context.Context, buyerID int64) (model.Entitlement, error) {
	if err := model.Positive(buyerID); err != nil {
		return model.Entitlement{}, err
	}
	queries := store.New(service.deps.DB.Conn(ctx))
	buyerRow, err := queries.ByID(ctx, buyerID)
	if err != nil {
		return model.Entitlement{}, model.Error(err)
	}
	if buyerRow.Status == string(model.Closed) {
		return model.Entitlement{}, apperr.NotFound("買家已關閉")
	}
	start, end := Period(buyerRow.CreatedAt, service.deps.Now().UTC())
	return model.Entitlement{PlanCode: DefaultPlanCode, MonthlyAnswerLimit: DefaultMonthlyAnswerLimit, PeriodStart: start, PeriodEnd: end}, nil
}
