//go:build integration

package im_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/live"
)

// guard 只是測試卡住時的失敗保險，不是效能斷言。
const guard = 30 * time.Second

// newModule 是共用同一個資料庫的另一個 im.Module：等同另一個進程，各自有 hub 與專用的 LISTEN 連線。
func (f *fixture) newModule(cfg im.LiveConfig) *im.Module {
	return im.New(im.Deps{
		DB: f.db, Events: f.bus, Jobs: f.jobs, Media: f.media, Buyers: f.buyers, Stores: f.stores, Orders: f.orders,
		NewID: uuid.NewV7, Now: f.clock, Logger: slog.New(slog.NewTextHandler(&strings.Builder{}, nil)), Live: cfg,
	})
}

// fastBackoff 讓重連的退避在測試裡幾乎不等。
func fastBackoff() im.LiveConfig {
	return im.LiveConfig{BackoffBase: time.Millisecond, BackoffMax: 5 * time.Millisecond}
}

// run 啟動 m.Run 並等到它的 LISTEN 連線已經在監聽（存活確認 SELECT now() 在 LISTEN 之後）；測試結束或呼叫 stop 時結束。
func (f *fixture) run(t *testing.T, m *im.Module) (stop func()) {
	t.Helper()
	before := f.listeners(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx) }()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			if err := <-done; err != nil {
				t.Errorf("Run 結束：%v", err)
			}
		})
	}
	t.Cleanup(stop)
	f.eventually(t, "listener 開始監聽", func() bool { return f.listeners(t) > before })
	return stop
}

func (f *fixture) listeners(t *testing.T) int64 {
	t.Helper()
	return f.scalar(t, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid() AND query LIKE 'SELECT now()%'`)
}

func (f *fixture) eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(guard)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("等不到：%s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (f *fixture) subscribe(t *testing.T, m *im.Module, p im.Party, cursors map[uuid.UUID]int64) *im.Subscription {
	t.Helper()
	s, err := m.Subscribe(context.Background(), p, cursors)
	if err != nil {
		t.Fatalf("Subscribe：%v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func next(t *testing.T, s *im.Subscription) im.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), guard)
	defer cancel()
	ev, err := s.Next(ctx)
	if err != nil {
		t.Fatalf("Next：%v", err)
	}
	return ev
}

// untilReady 取到 Ready 為止（補洞完成），回傳之前的事件與 Ready。
func untilReady(t *testing.T, s *im.Subscription) (before []im.Event, ready im.Event) {
	t.Helper()
	for {
		ev := next(t, s)
		if ev.Kind == im.EventReady {
			return before, ev
		}
		before = append(before, ev)
	}
}

// collectUntil 取事件直到 stop 成立（含觸發它的那個事件）。
func collectUntil(t *testing.T, s *im.Subscription, stop func(im.Event) bool) []im.Event {
	t.Helper()
	var out []im.Event
	for {
		ev := next(t, s)
		out = append(out, ev)
		if stop(ev) {
			return out
		}
	}
}

func messageSeq(conv uuid.UUID, seq int64) func(im.Event) bool {
	return func(e im.Event) bool {
		return e.Kind == im.EventMessage && !e.Updated && e.Message.ConversationID == conv && e.Message.Seq == seq
	}
}

func messages(evs []im.Event, updated bool) (out []im.Message) {
	for _, e := range evs {
		if e.Kind == im.EventMessage && e.Updated == updated {
			out = append(out, e.Message)
		}
	}
	return out
}

func count(evs []im.Event, k im.EventKind) (n int) {
	for _, e := range evs {
		if e.Kind == k {
			n++
		}
	}
	return n
}

// rawListener 是測試自己的 LISTEN 連線，記錄收到的 payload（獨立於被測的 hub）。
type rawListener struct {
	mu   sync.Mutex
	got  []live.Payload
	conn *pgx.Conn
}

func (f *fixture) listenRaw(t *testing.T) *rawListener {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	conn, err := pgx.ConnectConfig(ctx, f.db.Pool().Config().ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "LISTEN "+live.Channel); err != nil {
		t.Fatal(err)
	}
	r := &rawListener{conn: conn}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			n, err := conn.WaitForNotification(ctx)
			if err != nil {
				return
			}
			p, _, derr := live.Decode(n.Payload)
			if derr != nil {
				continue
			}
			r.mu.Lock()
			r.got = append(r.got, p)
			r.mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		_ = conn.Close(context.Background())
	})
	return r
}

func (r *rawListener) payloads() []live.Payload {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]live.Payload(nil), r.got...)
}

// waitFor 等到收到至少 n 則。
func (r *rawListener) waitFor(t *testing.T, f *fixture, n int) []live.Payload {
	t.Helper()
	f.eventually(t, "收到通知", func() bool { return len(r.payloads()) >= n })
	return r.payloads()
}

var errRollback = errors.New("rollback")
