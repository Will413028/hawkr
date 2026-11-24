package purge

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// StaleTokenDays 是裝置 token 的有效天數：App 每次啟動都會重新註冊（更新 last_registered_at），超過這麼久沒有註冊的 token 視為已不使用。
const StaleTokenDays = 60

// SweepStaleTokens 刪除 last_registered_at 超過 StaleTokenDays 天的裝置 token，每批一個 transaction，回傳刪除的列數。
func (s *Service) SweepStaleTokens(ctx context.Context) (int64, error) {
	cutoff := s.d.Now().UTC().Add(-StaleTokenDays * 24 * time.Hour)
	n, err := s.batches(ctx, func(q *store.Queries) (int64, error) {
		return q.DeleteStaleDeviceTokens(ctx, store.DeleteStaleDeviceTokensParams{Cutoff: cutoff, MaxRows: batchSize})
	})
	if err != nil {
		return n, fmt.Errorf("清除過期的裝置 token：%w", err)
	}
	return n, nil
}

// TokenSweepArgs 是每日清除過期裝置 token 的 job。
type TokenSweepArgs struct{}

// Kind 是 River 的 job 種類。
func (TokenSweepArgs) Kind() string { return "notification.token_sweep" }

// InsertOpts 以 SweepInterval 為唯一性的時間窗，同保留期清除。
func (TokenSweepArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(SweepInterval)
	o.MaxAttempts = 3
	return o
}

type tokenSweepWorker struct {
	river.WorkerDefaults[TokenSweepArgs]
	svc *Service
}

// Work 執行清除；失敗由 River 重試（冪等）。
func (w *tokenSweepWorker) Work(ctx context.Context, _ *river.Job[TokenSweepArgs]) error {
	n, err := w.svc.SweepStaleTokens(ctx)
	if err != nil {
		return err
	}
	w.svc.log().InfoContext(ctx, "過期裝置 token 清除完成", "tokens", n)
	return nil
}

// Timeout 是一次清除的時間上限。
func (w *tokenSweepWorker) Timeout(*river.Job[TokenSweepArgs]) time.Duration { return 5 * time.Minute }
