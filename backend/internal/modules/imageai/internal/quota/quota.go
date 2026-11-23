// Package quota 是每家店每週的修圖額度：一列計數器加條件 UPDATE（A9-D9）。
package quota

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

const refundTimeout = 5 * time.Second

// Service 扣與退修圖額度。
type Service struct {
	db  *db.DB
	now func() time.Time
}

// New 建立 Service。
func New(d *db.DB, now func() time.Time) *Service { return &Service{db: d, now: now} }

// WeekStart 是 t 所在那週的起點：週一 00:00 Asia/Taipei。
func WeekStart(t time.Time) time.Time {
	day := twtime.Midnight(t)
	back := (int(day.Weekday()) + 6) % 7
	return time.Date(day.Year(), day.Month(), day.Day()-back, 0, 0, 0, 0, twtime.Taipei)
}

// NextWeekStart 是 periodStart 的下一週起點。
func NextWeekStart(periodStart time.Time) time.Time {
	p := periodStart.In(twtime.Taipei)
	return time.Date(p.Year(), p.Month(), p.Day()+7, 0, 0, 0, 0, twtime.Taipei)
}

// Limit 是一家店現在的每週上限：個別設定優先，否則預設。
func (s *Service) Limit(ctx context.Context, storeID int64) (int32, error) {
	limit, err := store.New(s.db.Conn(ctx)).GetQuotaLimit(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DefaultWeeklyLimit, nil
	}
	if err != nil {
		return 0, fmt.Errorf("quota：讀店家 %d 的額度上限：%w", storeID, err)
	}
	return limit, nil
}

// Consume 扣本週一次；額度用完回 model.ErrQuotaExhausted（Usage 仍帶上限與重置時間）。單一語句，不開 transaction。
func (s *Service) Consume(ctx context.Context, storeID int64) (model.Usage, error) {
	now := s.now()
	start := WeekStart(now)
	u := model.Usage{PeriodStart: start, ResetTime: NextWeekStart(start)}
	limit, err := s.Limit(ctx, storeID)
	if err != nil {
		return u, err
	}
	u.Limit = limit
	used, err := store.New(s.db.Conn(ctx)).ConsumeUsage(ctx, store.ConsumeUsageParams{StoreID: storeID, PeriodStart: start, Now: now, QuotaLimit: limit})
	if errors.Is(err, pgx.ErrNoRows) {
		u.Used = limit
		return u, model.ErrQuotaExhausted
	}
	if err != nil {
		return u, fmt.Errorf("quota：扣店家 %d 的額度：%w", storeID, err)
	}
	u.Used = used
	return u, nil
}

// Refund 退回 Consume 扣的那一次。呼叫端的 ctx 取消後（請求被中斷）仍要退，所以不跟著取消，另設 deadline。
func (s *Service) Refund(ctx context.Context, storeID int64, periodStart time.Time) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), refundTimeout)
	defer cancel()
	if _, err := store.New(s.db.Conn(ctx)).RefundUsage(ctx, store.RefundUsageParams{StoreID: storeID, PeriodStart: periodStart, Now: s.now()}); err != nil {
		return fmt.Errorf("quota：退店家 %d 的額度：%w", storeID, err)
	}
	return nil
}

// Used 是店家在 periodStart 那週已用的次數，沒有紀錄是 0。
func (s *Service) Used(ctx context.Context, storeID int64, periodStart time.Time) (int32, error) {
	used, err := store.New(s.db.Conn(ctx)).GetUsage(ctx, store.GetUsageParams{StoreID: storeID, PeriodStart: periodStart})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("quota：讀店家 %d 的用量：%w", storeID, err)
	}
	return used, nil
}

// OnStoreRetired 在 StoreRetired 的 transaction 內執行（單體規則 4：同 transaction 的訂閱者只做原子的狀態變更）：刪該店的個別額度與用量。冪等。
func (s *Service) OnStoreRetired(ctx context.Context, storeID int64) error {
	q := store.New(s.db.Conn(ctx))
	if _, err := q.DeleteStoreUsage(ctx, storeID); err != nil {
		return fmt.Errorf("quota：清除店家 %d 的用量：%w", storeID, err)
	}
	if _, err := q.DeleteStoreQuota(ctx, storeID); err != nil {
		return fmt.Errorf("quota：清除店家 %d 的個別額度：%w", storeID, err)
	}
	return nil
}
