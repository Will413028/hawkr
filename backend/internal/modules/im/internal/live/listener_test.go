package live

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
)

func TestBackoffGrowsExponentiallyCapsAndResets(t *testing.T) {
	t.Parallel()
	for name, rnd := range map[string]float64{"最小 jitter": 0, "最大 jitter": 0.999999999} {
		b := &Backoff{Base: 500 * time.Millisecond, Max: 30 * time.Second, Rand: func() float64 { return rnd }}
		ceilings := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
		for i, ceil := range ceilings {
			got := b.Next()
			if got < ceil/2 || got > ceil {
				t.Errorf("%s 第 %d 次 %v，預期在 [%v, %v]", name, i+1, got, ceil/2, ceil)
			}
			if rnd == 0 && got != ceil/2 || rnd > 0.9 && got < ceil-time.Millisecond {
				t.Errorf("%s 第 %d 次 %v 不符 jitter 端點 %v", name, i+1, got, ceil)
			}
		}
		b.Reset()
		if got := b.Next(); got > 500*time.Millisecond {
			t.Errorf("%s Reset 之後 %v，預期從 Base 重來", name, got)
		}
	}
	huge := &Backoff{Base: time.Second, Max: time.Minute, Rand: func() float64 { return 0.5 }}
	for range 200 {
		if got := huge.Next(); got <= 0 || got > time.Minute {
			t.Fatalf("多次之後 %v 溢位或超過上限", got)
		}
	}
}

type fakeConn struct {
	alive time.Time
	wait  func(ctx context.Context) error
}

func (f *fakeConn) Listen(context.Context) error { return nil }
func (f *fakeConn) Wait(ctx context.Context, _ time.Duration) (string, bool, error) {
	return "", false, f.wait(ctx)
}
func (f *fakeConn) Alive(context.Context) (time.Time, error) { return f.alive, nil }
func (f *fakeConn) Close(context.Context)                    {}

// 重連迴圈：失敗以注入的 Sleep 退避（記錄每次等多久），連線成功後退避重置，重連成功後對訂閱發 epoch 並帶前一條連線的存活時間。
func TestListenLoopBacksOffResetsAndSignalsEpoch(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var sleeps []time.Duration
	attempts := 0
	t1, t2 := time.Unix(1000, 0), time.Unix(2000, 0)
	h := newTestHub(t, newFakeReader(), Config{
		Rand:  func() float64 { return 0.999999999 },
		Sleep: func(_ context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil },
		NewTicker: func(time.Duration) (<-chan time.Time, func()) {
			return make(chan time.Time), func() {}
		},
	})
	sub, err := h.Subscribe(ctx, buyerParty(5), nil)
	if err != nil {
		t.Fatal(err)
	}
	dial := func(context.Context) (Conn, error) {
		attempts++
		switch {
		case attempts <= 7:
			return nil, errors.New("down")
		case attempts == 8:
			return &fakeConn{alive: t1, wait: func(context.Context) error { return errors.New("connection reset") }}, nil
		default:
			return &fakeConn{alive: t2, wait: func(ctx context.Context) error {
				sub.mu.Lock()
				since := sub.since
				sub.mu.Unlock()
				if want := t1.Add(-model.RecoveryMargin); !since.Equal(want) {
					t.Errorf("epoch 帶的補洞起點 %v，預期前一條連線的存活時間 %v 往回放 RecoveryMargin＝%v", since, t1, want)
				}
				cancel()
				return ctx.Err()
			}}, nil
		}
	}
	h.listenLoop(ctx, dial)
	want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}
	if len(sleeps) != 8 {
		t.Fatalf("退避 %v", sleeps)
	}
	for i, w := range want {
		if sleeps[i] < w-time.Millisecond || sleeps[i] > w {
			t.Errorf("第 %d 次退避 %v，預期約 %v", i+1, sleeps[i], w)
		}
	}
	if sleeps[7] > 500*time.Millisecond {
		t.Errorf("連線成功後退避要重置：%v", sleeps[7])
	}
}

// 第一條連線的 epoch 帶的是啟動時讀到的資料庫時間：在第一次 LISTEN 成功之前就開的串流也會補洞。
func TestFirstConnectionEpochCarriesTheStartupDatabaseTime(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	start := time.Unix(777, 0)
	streams := &fakeStreams{now: start}
	h := newTestHubStreams(t, newFakeReader(), streams, Config{
		Sleep:     func(context.Context, time.Duration) error { return nil },
		NewTicker: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	})
	sub, err := h.Subscribe(ctx, buyerParty(5), nil)
	if err != nil {
		t.Fatal(err)
	}
	var since time.Time
	dial := func(context.Context) (Conn, error) {
		return &fakeConn{alive: time.Unix(900, 0), wait: func(ctx context.Context) error {
			sub.mu.Lock()
			since = sub.since
			sub.mu.Unlock()
			cancel()
			return ctx.Err()
		}}, nil
	}
	h.listenLoop(ctx, dial)
	if want := start.Add(-model.RecoveryMargin); !since.Equal(want) {
		t.Fatalf("第一次 epoch 帶 %v，預期啟動時的資料庫時間 %v 往回放 RecoveryMargin＝%v", since, start, want)
	}
}

// 讀不到資料庫時間就退避重試，成功之前不撥號。
func TestListenLoopRetriesTheStartupTimeBeforeDialing(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	streams := &fakeStreams{now: time.Unix(777, 0), nowErrs: 2}
	sleeps := 0
	h := newTestHubStreams(t, newFakeReader(), streams, Config{
		Sleep:     func(context.Context, time.Duration) error { sleeps++; return nil },
		NewTicker: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	})
	callsAtDial := -1
	dial := func(context.Context) (Conn, error) {
		streams.mu.Lock()
		callsAtDial = streams.nowCalls
		streams.mu.Unlock()
		cancel()
		return nil, errors.New("stop")
	}
	h.listenLoop(ctx, dial)
	if callsAtDial != 3 || sleeps != 2 {
		t.Fatalf("撥號時已讀 %d 次資料庫時間、退避 %d 次，預期 3 與 2", callsAtDial, sleeps)
	}
}
