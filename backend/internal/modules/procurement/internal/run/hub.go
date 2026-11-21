package run

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// MaxSubscriptions 是每個程序同時訂閱的上限（A9-D20 R5）。所有訂閱共用一條專用 LISTEN 連線，連線數與訂閱數無關，
// 上限擋的是記憶體與每次通知的扇出成本：每條訂閱是一個 goroutine、一個容量 1 的 channel 與一次事件查詢。
// 推論值，不是量測：一個 buyer 同時開幾條 watch 的上限遠小於 500，500 條在單程序的 goroutine 與查詢量上沒有壓力。
const MaxSubscriptions = 500

const (
	reconnectMin = time.Second
	reconnectMax = 30 * time.Second
)

// Hub 用一條專用連線 LISTEN，把通知扇出給程序內的訂閱者。通知只是「這個 run 可能有新事件」的提示，事件的內容與順序以資料庫為準。
type Hub struct {
	db     *db.DB
	max    int
	logger *slog.Logger

	mu    sync.Mutex
	subs  map[uuid.UUID]map[*Subscription]struct{}
	count int
}

// NewHub 建立 Hub；max 是訂閱數上限。
func NewHub(d *db.DB, max int, logger *slog.Logger) *Hub {
	return &Hub{db: d, max: max, logger: logger, subs: map[uuid.UUID]map[*Subscription]struct{}{}}
}

// Subscription 是一個 run 的喚醒訂閱；C 的容量是 1，多次通知合併成一次。
type Subscription struct {
	C     chan struct{}
	runID uuid.UUID
	hub   *Hub
}

// Subscribe 訂閱 run 的喚醒；超過上限回 RESOURCE_EXHAUSTED。呼叫端要在讀取事件之前訂閱，之後才不會漏掉中間的通知。
func (h *Hub) Subscribe(runID uuid.UUID) (*Subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count >= h.max {
		return nil, apperr.ResourceExhausted(5*time.Second, "目前觀看的人數太多，請稍後再試")
	}
	s := &Subscription{C: make(chan struct{}, 1), runID: runID, hub: h}
	if h.subs[runID] == nil {
		h.subs[runID] = map[*Subscription]struct{}{}
	}
	h.subs[runID][s] = struct{}{}
	h.count++
	return s, nil
}

// Close 取消訂閱；可以重複呼叫。
func (s *Subscription) Close() {
	h := s.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.subs[s.runID]
	if _, ok := set[s]; !ok {
		return
	}
	delete(set, s)
	if len(set) == 0 {
		delete(h.subs, s.runID)
	}
	h.count--
}

// Count 是目前的訂閱數。
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

func wake(s *Subscription) {
	select {
	case s.C <- struct{}{}:
	default:
	}
}

func (h *Hub) notify(runID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs[runID] {
		wake(s)
	}
}

func (h *Hub) notifyAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, set := range h.subs {
		for s := range set {
			wake(s)
		}
	}
}

// Run 維持 LISTEN 連線直到 ctx 結束（回傳 ctx.Err()）；連線中斷時退避重連，重連後喚醒全部訂閱者重新查詢（斷線期間的通知不會補送）。
func (h *Hub) Run(ctx context.Context) error {
	backoff := reconnectMin
	for {
		err := h.listen(ctx)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		h.logger.WarnContext(ctx, "procurement：run 的 LISTEN 連線中斷，稍後重連", "error", err, "retryIn", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, reconnectMax)
	}
}

func (h *Hub) listen(ctx context.Context) error {
	conn, err := pgx.ConnectConfig(ctx, h.db.Pool().Config().ConnConfig.Copy())
	if err != nil {
		return fmt.Errorf("連線：%w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		return fmt.Errorf("LISTEN：%w", err)
	}
	h.notifyAll()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("等待通知：%w", err)
		}
		id, err := uuid.Parse(n.Payload)
		if err != nil {
			h.notifyAll()
			continue
		}
		h.notify(id)
	}
}
