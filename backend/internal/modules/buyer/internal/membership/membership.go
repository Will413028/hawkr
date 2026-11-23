// Package membership 提供會員方案與額度週期。
package membership

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
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

func entitlement(planRow store.BuyerMembershipPlan, start, end time.Time) model.Entitlement {
	return model.Entitlement{
		PlanCode:           planRow.PlanCode,
		MonthlyAnswerLimit: planRow.MonthlyAnswerLimit,
		PeriodStart:        start,
		PeriodEnd:          end,
	}
}

func plan(planRow store.BuyerMembershipPlan) model.Plan {
	return model.Plan{
		PlanCode:           planRow.PlanCode,
		Version:            planRow.Version,
		DisplayName:        planRow.DisplayName,
		MonthlyAnswerLimit: planRow.MonthlyAnswerLimit,
		MonthlyPriceTWD:    planRow.MonthlyPriceTwd,
		IsPurchasable:      planRow.IsPurchasable,
		CreatedAt:          planRow.CreatedAt,
		UpdatedAt:          planRow.UpdatedAt,
	}
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
	planRow, err := queries.Plan(ctx, "free")
	if err != nil {
		return model.Entitlement{}, planError(err)
	}
	return entitlement(planRow, start, end), nil
}

// ListPlans 列出全部會員方案。
func (service *Service) ListPlans(ctx context.Context) ([]model.Plan, error) {
	rows, err := store.New(service.deps.DB.Conn(ctx)).Plans(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.Plan, 0, len(rows))
	for _, row := range rows {
		result = append(result, plan(row))
	}
	return result, err
}

// Plan 依代碼讀取單一會員方案。
func (service *Service) Plan(ctx context.Context, planCode string) (model.Plan, error) {
	row, err := store.New(service.deps.DB.Conn(ctx)).Plan(ctx, planCode)
	if err != nil {
		return model.Plan{}, planError(err)
	}
	return plan(row), nil
}

// UpdatePlan 修改提供的欄位；不存在的 plan_code 回 NotFound。
func (service *Service) UpdatePlan(ctx context.Context, input model.UpdatePlanInput) (model.Plan, error) {
	if input.MonthlyAnswerLimit == nil && input.MonthlyPriceTWD == nil && input.IsPurchasable == nil {
		return model.Plan{}, apperr.InvalidInput("必須提供更新欄位")
	}
	if input.MonthlyAnswerLimit != nil && *input.MonthlyAnswerLimit <= 0 {
		return model.Plan{}, apperr.InvalidInputOn([]string{"MonthlyAnswerLimit"}, "額度須為正數")
	}
	if input.MonthlyPriceTWD != nil && *input.MonthlyPriceTWD < 0 {
		return model.Plan{}, apperr.InvalidInputOn([]string{"MonthlyPriceTWD"}, "價格不可為負")
	}
	var result store.BuyerMembershipPlan
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		current, err := queries.Plan(ctx, input.PlanCode)
		if err != nil {
			return planError(err)
		}
		version := input.ExpectedVersion
		if version == 0 {
			version = current.Version
		}
		result, err = queries.UpdatePlan(ctx, store.UpdatePlanParams{
			PlanCode:        input.PlanCode,
			ExpectedVersion: version,
			AnswerLimit:     input.MonthlyAnswerLimit,
			Price:           input.MonthlyPriceTWD,
			Purchasable:     input.IsPurchasable,
			NowAt:           service.deps.Now().UTC(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.VersionConflict("會員方案已被修改")
		}
		return err
	})
	if err != nil {
		return model.Plan{}, err
	}
	return plan(result), nil
}

func planError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到會員方案")
	}
	return err
}
