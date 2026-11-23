package stream

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
)

// Conn 是專用的 LISTEN 連線；listenLoop 是唯一的使用者，實作不需要並行安全。
type Conn interface {
	Listen(ctx context.Context) error
	// Wait 等一則通知；ctx 結束或連線壞掉回錯誤。
	Wait(ctx context.Context) (payload string, err error)
	Close(ctx context.Context)
}

// Dial 建立一條新的專用連線。
type Dial func(ctx context.Context) (Conn, error)

// DialPG 以 cfg 建立專用連線：不從連線池借。cfg 必須指向直連或 session 模式的 Postgres，不能經 transaction pooling。
func DialPG(cfg *pgx.ConnConfig) Dial {
	return func(ctx context.Context) (Conn, error) {
		c, err := pgx.ConnectConfig(ctx, cfg.Copy())
		if err != nil {
			return nil, err
		}
		return &pgConn{c: c}, nil
	}
}

type pgConn struct{ c *pgx.Conn }

func (p *pgConn) Listen(ctx context.Context) error {
	_, err := p.c.Exec(ctx, "LISTEN "+Channel)
	return err
}

func (p *pgConn) Wait(ctx context.Context) (string, error) {
	n, err := p.c.WaitForNotification(ctx)
	if err != nil {
		return "", err
	}
	return n.Payload, nil
}

func (p *pgConn) Close(ctx context.Context) { _ = p.c.Close(ctx) }

// stableAfter 是連線要維持多久才算穩定（估計值，不是量測值）：連上就斷的連線不重設退避，否則壞掉的連線每次重連都從頭等，
// 每次重連的完整重讀訊號（resync）會讓所有訂閱者各讀一次資料庫，約每秒數次。
const stableAfter = 10 * time.Second

// Run 持有這個行程唯一的 LISTEN 連線，把通知扇出給本行程的訂閱者，直到 ctx 結束（回 nil 或 ctx 的錯誤）。
// 只有 serve 角色跑它（串流在那裡）；worker 只發通知。連線斷掉以指數退避重連（連線維持 stableAfter 以上才重設退避，連上就斷的連續遞增到 backoffMax）；每次連線成功（含第一次）對所有本地訂閱者發一次完整重讀訊號：
// 斷線期間的通知不會補送，訂閱者以重讀列表為準。
func (h *Hub) Run(ctx context.Context, dial Dial) error {
	defer h.shutdown()
	base, maxWait := h.backoffBase, h.backoffMax
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	if maxWait <= 0 {
		maxWait = 30 * time.Second
	}
	jitter := h.jitter
	if jitter == nil {
		jitter = rand.Float64
	}
	now, sleep := h.now, h.sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	wait := base
	for ctx.Err() == nil {
		c, err := h.connect(ctx, dial)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			h.logger().Warn("notification：監聽連線失敗，稍後重連", "error", err, "retry_in", wait)
		} else {
			h.resync()
			h.markListening()
			upSince := now()
			err = h.pump(ctx, c)
			c.Close(context.WithoutCancel(ctx))
			if ctx.Err() != nil {
				break
			}
			if now().Sub(upSince) >= stableAfter {
				wait = base
			}
			h.logger().Warn("notification：監聽連線中斷，稍後重連", "error", err, "retry_in", wait)
		}
		sleep(ctx, time.Duration(float64(wait)*(0.5+0.5*jitter())))
		wait = min(wait*2, maxWait)
	}
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (h *Hub) connect(ctx context.Context, dial Dial) (Conn, error) {
	c, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.Listen(ctx); err != nil {
		c.Close(context.WithoutCancel(ctx))
		return nil, err
	}
	return c, nil
}

func (h *Hub) pump(ctx context.Context, c Conn) error {
	for {
		raw, err := c.Wait(ctx)
		if err != nil {
			return err
		}
		h.dispatch(raw)
	}
}

// Listening 在第一次 LISTEN 成功之後關閉（測試與啟動順序用）。
func (h *Hub) Listening() <-chan struct{} { return h.listening }

func (h *Hub) markListening() {
	h.listenOnce.Do(func() { close(h.listening) })
}

func (h *Hub) logger() *slog.Logger {
	if h.log != nil {
		return h.log
	}
	return slog.Default()
}

// dispatch 把一則通知交給本地訂閱者；不合法的通知記錄後丟棄。
func (h *Hub) dispatch(raw string) {
	p, err := decode(raw)
	if err != nil {
		h.logger().Warn("notification：丟棄不合法的通知", "error", err)
		return
	}
	switch p.Kind {
	case kindInbox:
		h.InboxChanged(Key{AccountID: p.Account, Audience: model.Audience(p.Audience)}, p.Store, model.StateHint(p.Hint))
	case kindWorkspace:
		h.workspaceChanged(p.Store, SellerChange(p.Change))
	}
}

// resync 對本行程所有訂閱者發完整重讀訊號：收件匣，以及店家訂閱者的各類賣家工作區變更。
func (h *Hub) resync() {
	h.mu.Lock()
	subs := make([]*Subscription, 0, len(h.byAccount))
	for _, set := range h.byKey {
		for s := range set {
			subs = append(subs, s)
		}
	}
	h.mu.Unlock()
	for _, s := range subs {
		s.mark(func() {
			s.inboxDirty = true
			if s.storeID > 0 {
				for _, c := range []SellerChange{SellerOrder, SellerAfterSale} {
					if !containsChange(s.changes, c) {
						s.changes = append(s.changes, c)
					}
				}
			}
		})
	}
}

func containsChange(cs []SellerChange, c SellerChange) bool {
	for _, x := range cs {
		if x == c {
			return true
		}
	}
	return false
}

// shutdown 在 Run 結束時關閉全部訂閱（開著的串流以 ErrClosed 結束，server 的 Shutdown 才不會被它們卡住），之後的 Subscribe 回 ErrClosed。
func (h *Hub) shutdown() {
	h.mu.Lock()
	h.down = true
	var subs []*Subscription
	for _, set := range h.byKey {
		for s := range set {
			subs = append(subs, s)
		}
	}
	h.mu.Unlock()
	for _, s := range subs {
		s.Close()
	}
}
