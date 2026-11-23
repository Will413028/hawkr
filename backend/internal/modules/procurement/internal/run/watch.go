package run

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const watchPage = 200

// Watcher 回放並跟隨一個 run 的事件。
type Watcher struct {
	DB  *db.DB
	Hub *Hub
	// Window 是一條串流的最長時間，到了就正常結束，client 以最後的 cursor 接續。
	Window time.Duration
}

// Watch 從 cursor 之後依序送出事件，直到送出結尾事件、Window 到期或 ctx 結束。cursor 早於保留範圍回
// REPLAY_CURSOR_EXPIRED，超前於最新事件回 REPLAY_CURSOR_AHEAD（皆 FAILED_PRECONDITION）。
func (w Watcher) Watch(ctx context.Context, buyerID int64, runID uuid.UUID, cursor string, send func(Event) error) error {
	after, err := parseCursor(cursor)
	if err != nil {
		return err
	}
	q := store.New(w.DB.Conn(ctx))
	r, err := q.GetRunForBuyer(ctx, store.GetRunForBuyerParams{ID: runID, BuyerID: buyerID})
	if err != nil {
		return readRunError(err)
	}
	if after+1 < r.MinAvailableSeq {
		return apperr.FailedPrecondition("這個位置的事件已過保留期").WithReason(apperr.ReasonReplayCursorExpired)
	}
	if after > r.LastEventSeq {
		return apperr.FailedPrecondition("這個位置超前於最新的事件").WithReason(apperr.ReasonReplayCursorAhead)
	}
	sub, err := w.Hub.Subscribe(runID)
	if err != nil {
		return err
	}
	defer sub.Close()

	ctx, cancel := context.WithTimeout(ctx, w.Window)
	defer cancel()
	for {
		r, err := q.GetRunForBuyer(ctx, store.GetRunForBuyerParams{ID: runID, BuyerID: buyerID})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return readRunError(err)
		}
		ended := IsTerminal(r.Status)
		events, err := q.ListRunEventsAfter(ctx, store.ListRunEventsAfterParams{RunID: runID, AfterSeq: after, PageSize: watchPage})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		for _, e := range events {
			if err := send(Event{Seq: e.Seq, Type: e.EventType, Payload: e.Payload, OccurredAt: e.OccurredAt}); err != nil {
				return err
			}
			after = e.Seq
			if IsTerminalEvent(e.EventType) {
				return nil
			}
		}
		if len(events) == watchPage {
			continue
		}
		if drainedEndedRun(ended, len(events)) {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-sub.C:
		}
	}
}

// readRunError 把讀 run 的錯誤分類：run 不存在（串流中途被刪除也一樣）是 NOT_FOUND。
func readRunError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到這個 run")
	}
	return err
}

func parseCursor(c string) (int64, error) {
	if c == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(c, 10, 64)
	if err != nil || n < 1 {
		return 0, apperr.InvalidInputOn([]string{"cursor"}, "cursor 不正確")
	}
	return n, nil
}

// drainedEndedRun：在列事件之前讀到 run 已結束（狀態與結尾事件同一個 transaction 可見），且這一頁沒有填滿，
// 代表該送的都送完了；之後再等通知只會等到串流視窗結束。
func drainedEndedRun(endedBeforeList bool, listed int) bool {
	return endedBeforeList && listed < watchPage
}
