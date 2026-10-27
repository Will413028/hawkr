package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

type stubCancelReader struct {
	status string
	err    error
	calls  int
}

func (s *stubCancelReader) GetRun(_ context.Context, runID int64, buyerID identity.BuyerID) (repository.Run, error) {
	s.calls++
	if s.err != nil {
		return repository.Run{}, s.err
	}
	return repository.Run{ID: runID, BuyerID: buyerID, Status: s.status}, nil
}

// newTestObserver 給一個時間可控的觀察器：測試推進 clock，不 sleep
// （CLAUDE.md §21）。
func newTestObserver(store runCancelStatusReader, now *time.Time) *runCancelObserver {
	o := newRunCancelObserver(store, 901, 42)
	o.now = func() time.Time { return *now }
	return o
}

func TestRunCancelObserver_FirstCallAlwaysQueries(t *testing.T) {
	now := time.Date(2026, 9, 23, 1, 0, 0, 0, time.UTC)
	store := &stubCancelReader{status: domain.RunStatusCancelReq()}
	o := newTestObserver(store, &now)

	require.True(t, o.cancelRequested(t.Context()),
		"買家在 turn 一開始就按停止是正常操作，第一次呼叫不得被節流掉")
	require.Equal(t, 1, store.calls)
}

func TestRunCancelObserver_ThrottlesWithinInterval(t *testing.T) {
	now := time.Date(2026, 9, 23, 1, 0, 0, 0, time.UTC)
	store := &stubCancelReader{status: domain.RunStatusRunning()}
	o := newTestObserver(store, &now)

	require.False(t, o.cancelRequested(t.Context()))
	require.Equal(t, 1, store.calls)

	// 同一個節流窗內連打 50 次（一次 turn 的事件量級），一次都不該再查 DB。
	for range 50 {
		now = now.Add(10 * time.Millisecond)
		require.False(t, o.cancelRequested(t.Context()))
	}
	require.Equal(t, 1, store.calls, "節流窗內不得重複查 DB")

	// 跨過窗口就要再查一次——節流是省成本，不是永遠不看。
	now = now.Add(runCancelCheckInterval)
	require.False(t, o.cancelRequested(t.Context()))
	require.Equal(t, 2, store.calls)
}

func TestRunCancelObserver_RemembersCancelWithoutRequerying(t *testing.T) {
	now := time.Date(2026, 9, 23, 1, 0, 0, 0, time.UTC)
	store := &stubCancelReader{status: domain.RunStatusCancelReq()}
	o := newTestObserver(store, &now)

	require.True(t, o.cancelRequested(t.Context()))
	now = now.Add(time.Hour)
	require.True(t, o.cancelRequested(t.Context()))
	require.Equal(t, 1, store.calls,
		"CANCEL_REQUESTED 對執行中的 run 是單向的，看到一次就不必再問")
}

// TestRunCancelObserver_QueryFailureDoesNotCancel 守的是失敗方向。讀不到狀態
// 時如果回 true，一次暫時性的 DB 抖動就會把買家正在等、而且本來會成功的結果
// 丟掉——那是把可恢復的基礎設施問題升級成資料損失。漏掉一次取消的代價小得多：
// 下一次檢查就會看到。
func TestRunCancelObserver_QueryFailureDoesNotCancel(t *testing.T) {
	now := time.Date(2026, 9, 23, 1, 0, 0, 0, time.UTC)
	store := &stubCancelReader{err: errors.New("connection reset")}
	o := newTestObserver(store, &now)

	require.False(t, o.cancelRequested(t.Context()),
		"查詢失敗必須偏向繼續跑完，不得中止")
	require.Equal(t, 1, store.calls)
}

func TestRunCancelObserver_NilObserverIsSafe(t *testing.T) {
	var o *runCancelObserver
	require.False(t, o.cancelRequested(t.Context()))
}
