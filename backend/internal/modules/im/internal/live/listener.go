package live

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Conn 是專用的 LISTEN 連線。實作不需要並行安全：listenLoop 是唯一的使用者。
type Conn interface {
	// Listen 開始 LISTEN Channel。
	Listen(ctx context.Context) error
	// Wait 等一則通知，最多 timeout；逾時回 (空, false, nil)。
	Wait(ctx context.Context, timeout time.Duration) (payload string, ok bool, err error)
	// Alive 在同一條連線上確認存活並回傳資料庫目前的時間。
	Alive(ctx context.Context) (time.Time, error)
	Close(ctx context.Context)
}

// Dial 建立一條新的專用連線。
type Dial func(ctx context.Context) (Conn, error)

// DialPG 以 cfg 建立 *pgx.Conn 形式的專用連線：不從連線池借，所以不影響池的容量，也不被池的回收影響。
// cfg 必須指向直連或 session 模式的 Postgres，不能經 transaction pooling（LISTEN 要 session 層級的連線）。
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

func (p *pgConn) Wait(ctx context.Context, timeout time.Duration) (string, bool, error) {
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	n, err := p.c.WaitForNotification(wctx)
	switch {
	case err == nil:
		return n.Payload, true, nil
	case ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded):
		return "", false, nil
	}
	return "", false, err
}

func (p *pgConn) Alive(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := p.c.QueryRow(ctx, "SELECT now()").Scan(&t)
	return t, err
}

func (p *pgConn) Close(ctx context.Context) { _ = p.c.Close(ctx) }

// listenLoop：連線 → LISTEN → 等通知並每 AliveEvery 確認存活；任何錯誤都關連線，以指數退避加 jitter 重連。
// 連線成功後對所有訂閱發 epoch，帶前一條連線最後確認存活的資料庫時間；第一條連線帶的是啟動時（第一次撥號之前）讀到的資料庫時間，
// 所以在第一次 LISTEN 成功之前就開的串流也會補洞。斷線區間內的通知不會補送，所以訂閱者依游標從資料庫補洞。
func (h *Hub) listenLoop(ctx context.Context, dial Dial) {
	bo := &Backoff{Base: h.cfg.BackoffBase, Max: h.cfg.BackoffMax, Rand: h.cfg.Rand}
	lastOK, ok := h.startTime(ctx, bo)
	if !ok {
		return
	}
	bo.Reset()
	for ctx.Err() == nil {
		c, err := h.connect(ctx, dial)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			d := bo.Next()
			h.cfg.Logger.Warn("im：監聽連線失敗，稍後重連", "error", err, "retry_in", d)
			if h.cfg.Sleep(ctx, d) != nil {
				return
			}
			continue
		}
		bo.Reset()
		if !lastOK.IsZero() {
			h.epoch(lastOK)
		}
		err = h.serve(ctx, c, &lastOK)
		c.Close(context.WithoutCancel(ctx))
		if ctx.Err() != nil {
			return
		}
		d := bo.Next()
		h.cfg.Logger.Warn("im：監聽連線中斷，稍後重連", "error", err, "retry_in", d)
		if h.cfg.Sleep(ctx, d) != nil {
			return
		}
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

func (h *Hub) serve(ctx context.Context, c Conn, lastOK *time.Time) error {
	tick, stop := h.cfg.NewTicker(h.cfg.AliveEvery)
	defer stop()
	alive := func() error {
		t, err := c.Alive(ctx)
		if err == nil {
			*lastOK = t
		}
		return err
	}
	if err := alive(); err != nil {
		return err
	}
	for {
		payload, ok, err := c.Wait(ctx, h.cfg.AliveEvery)
		if err != nil {
			return err
		}
		if ok {
			h.dispatch(payload)
		}
		select {
		case <-tick:
			if err := alive(); err != nil {
				return err
			}
		default:
		}
	}
}

// startTime 讀一次資料庫時間當作第一個補洞起點；讀不到就以同一組退避重試，直到成功或 ctx 結束（回傳 false）。
func (h *Hub) startTime(ctx context.Context, bo *Backoff) (time.Time, bool) {
	for ctx.Err() == nil {
		t, err := h.streams.DatabaseNow(ctx)
		if err == nil {
			return t, true
		}
		if ctx.Err() != nil {
			break
		}
		d := bo.Next()
		h.cfg.Logger.Warn("im：讀取資料庫時間失敗，稍後重試", "error", err, "retry_in", d)
		if h.cfg.Sleep(ctx, d) != nil {
			break
		}
	}
	return time.Time{}, false
}
