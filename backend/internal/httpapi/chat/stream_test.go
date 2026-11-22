package chat

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 測試用的幾個時長互不相同，fakeClock 以它們辨識計時器；jitter 0.5 時壽命正好是 MaxLifetime。
func testStreamConfig() streamConfig {
	return streamConfig{
		HeartbeatEvery: 15 * time.Second, RecheckEvery: 60 * time.Second, MaxLifetime: 900 * time.Second,
		WriteTimeout: 10 * time.Second, LifetimeJitter: 0.1, MaxGuardFailures: 3,
	}
}

const (
	hbTimer   = 15 * time.Second
	chkTimer  = 60 * time.Second
	lifeTimer = 900 * time.Second
)

type streamHarness struct {
	t         *testing.T
	clk       *fakeClock
	src       *fakeSource
	frames    chan string
	log       *callLog
	guardRuns chan struct{}
	done      chan error
	cancel    context.CancelFunc
}

type callLog struct {
	mu sync.Mutex
	l  []string
}

func (c *callLog) add(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.l = append(c.l, s)
}

func (c *callLog) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.l)
}

// startStream 跑一條 runStream；guard 依序回 guards 的結果（用完回 nil），mod 可改 spec。
func startStream(t *testing.T, guards []error, mod func(*streamSpec[string], *streamHarness)) *streamHarness {
	t.Helper()
	h := &streamHarness{t: t, clk: newFakeClock(), src: newFakeSource(), frames: make(chan string, 64), log: &callLog{}, guardRuns: make(chan struct{}, 64), done: make(chan error, 1)}
	var gm sync.Mutex
	spec := streamSpec[string]{
		Config: testStreamConfig(), Clock: h.clk.clock(), Jitter: func() float64 { return 0.5 },
		Next: h.src.Next, Close: h.src.Close, Classify: classifyEnd,
		Write:     func(e string, _ streamInfo) error { h.log.add("write"); h.frames <- "event:" + e; return nil },
		Heartbeat: func() error { h.log.add("write"); h.frames <- "heartbeat"; return nil },
		Guard: func(context.Context) error {
			gm.Lock()
			defer gm.Unlock()
			defer func() { h.guardRuns <- struct{}{} }()
			if len(guards) == 0 {
				return nil
			}
			err := guards[0]
			guards = guards[1:]
			return err
		},
		SetWriteDeadline: func(d time.Time) error { h.log.add("deadline:" + d.Sub(h.clk.Now()).String()); return nil },
	}
	if mod != nil {
		mod(&spec, h)
	}
	var ctx context.Context
	ctx, h.cancel = context.WithCancel(t.Context())
	go func() { h.done <- runStream(ctx, spec) }()
	t.Cleanup(h.cancel)
	return h
}

func (h *streamHarness) push(ev string, err error) {
	h.t.Helper()
	select {
	case h.src.results <- fakeResult{ev, err}:
	case <-time.After(5 * time.Second): // 只防測試卡死，不是行為的閾值
		h.t.Fatal("來源的結果沒有被 runStream 取走")
	}
}

func (h *streamHarness) awaitGuard() {
	h.t.Helper()
	select {
	case <-h.guardRuns:
	case <-time.After(5 * time.Second): // 只防測試卡死
		h.t.Fatal("Guard 沒有被呼叫")
	}
}

func (h *streamHarness) result() error {
	h.t.Helper()
	select {
	case err := <-h.done:
		select {
		case <-h.src.closed:
		default:
			h.t.Error("runStream 返回時沒有呼叫 Close")
		}
		return err
	case <-time.After(5 * time.Second): // 只防測試卡死
		h.t.Fatal("runStream 沒有結束")
		return nil
	}
}

func TestStreamWritesHeartbeatWhenIdleAndRestartsTheIdleClockAfterEachFrame(t *testing.T) {
	h := startStream(t, nil, nil)
	h.clk.fire(hbTimer, 1)
	if got := <-h.frames; got != "heartbeat" {
		t.Fatalf("閒置到期的 frame = %q，預期 heartbeat", got)
	}
	h.clk.fire(hbTimer, 2)
	if got := <-h.frames; got != "heartbeat" {
		t.Fatalf("第二個 frame = %q，預期 heartbeat（寫出 frame 後要重新計閒置）", got)
	}
	h.cancel()
	if err := h.result(); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消後 = %v", err)
	}
}

func TestStreamDoesNotWriteHeartbeatsWhileEventsFlow(t *testing.T) {
	h := startStream(t, nil, nil)
	h.push("a", nil)
	h.push("b", nil)
	h.clk.await(hbTimer, 3) // 每個 frame 寫出後閒置計時重新武裝：1 次建立 + 2 個事件
	if got, want := len(h.frames), 2; got != want {
		t.Fatalf("frame 數 = %d，預期只有 2 個事件", got)
	}
	if a, b := <-h.frames, <-h.frames; a != "event:a" || b != "event:b" {
		t.Fatalf("frames = %q %q", a, b)
	}
	h.cancel()
	_ = h.result()
}

func TestStreamEndsNormallyAtTheLifetime(t *testing.T) {
	h := startStream(t, nil, nil)
	h.clk.fire(lifeTimer, 1)
	if err := h.result(); err != nil {
		t.Fatalf("壽命到期要正常結束（沒有錯誤），得到 %v", err)
	}
}

func TestStreamLifetimeCarriesPerStreamJitterFromTheInjectedSource(t *testing.T) {
	cfg := testStreamConfig()
	lifetimes := map[float64]time.Duration{}
	for _, j := range []float64{0, 0.5, 1} {
		h := startStream(t, nil, func(s *streamSpec[string], _ *streamHarness) { s.Jitter = func() float64 { return j } })
		h.clk.mu.Lock()
		for h.clk.timers[cfg.HeartbeatEvery] == nil || h.clk.timers[cfg.RecheckEvery] == nil || len(h.clk.timers) < 3 {
			h.clk.cond.Wait()
		}
		for d := range h.clk.timers {
			if d != cfg.HeartbeatEvery && d != cfg.RecheckEvery {
				lifetimes[j] = d
			}
		}
		h.clk.mu.Unlock()
		h.cancel()
		_ = h.result()
	}
	if lifetimes[0] != 810*time.Second || lifetimes[0.5] != 900*time.Second || lifetimes[1] != 990*time.Second {
		t.Fatalf("壽命 = %v，預期 jitter 0／0.5／1 對應 810／900／990 秒（±10%%）", lifetimes)
	}
}

func TestStreamSetsTheWriteDeadlineBeforeEveryFrame(t *testing.T) {
	h := startStream(t, nil, nil)
	h.push("a", nil)
	h.clk.fire(hbTimer, 2)
	<-h.frames
	<-h.frames
	h.cancel()
	_ = h.result()
	// 取消寫入期限（零值）相對於時鐘的差不是 10 秒：只比對「設定在寫入之前、期限是 WriteTimeout」。
	got := h.log.all()
	if len(got) < 5 || got[0] != "deadline:10s" || got[1] != "write" || got[3] != "deadline:10s" || got[4] != "write" {
		t.Fatalf("每個 frame 前都要先設 10 秒寫入期限：%v", got)
	}
}

func TestStreamEndsWhenAWriteHitsItsDeadline(t *testing.T) {
	h := startStream(t, nil, func(s *streamSpec[string], _ *streamHarness) {
		s.Write = func(string, streamInfo) error { return os.ErrDeadlineExceeded }
	})
	h.push("a", nil)
	err := h.result()
	if codeOf(err) != connect.CodeUnavailable {
		t.Fatalf("寫入逾時要以 unavailable 結束：%v", err)
	}
}

func TestStreamRechecksEveryIntervalNotOnlyAtTheStart(t *testing.T) {
	h := startStream(t, nil, nil)
	for i := 1; i <= 3; i++ {
		h.clk.fire(chkTimer, i)
		h.awaitGuard()
	}
	h.cancel()
	_ = h.result()
}

func TestStreamEndsUnauthenticatedWhenTheSessionIsGone(t *testing.T) {
	h := startStream(t, []error{apperr.Unauthenticated("session 失效")}, nil)
	h.clk.fire(chkTimer, 1)
	if err := h.result(); codeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("session 失效要以 unauthenticated 結束：%v", err)
	}
}

func TestStreamEndsUnavailableAfterThreeConsecutiveTransientGuardFailures(t *testing.T) {
	boom := errors.New("db down")
	h := startStream(t, []error{boom, boom, boom}, nil)
	for i := 1; i <= 3; i++ {
		h.clk.fire(chkTimer, i)
		h.awaitGuard()
	}
	if err := h.result(); codeOf(err) != connect.CodeUnavailable {
		t.Fatalf("連續 3 次暫時失敗要以 unavailable 結束：%v", err)
	}
}

func TestStreamTransientGuardFailureCounterResetsAfterASuccess(t *testing.T) {
	boom := errors.New("db down")
	h := startStream(t, []error{boom, boom, nil, boom, boom}, nil)
	for i := 1; i <= 5; i++ {
		h.clk.fire(chkTimer, i)
		h.awaitGuard()
	}
	h.clk.await(chkTimer, 6)
	select {
	case err := <-h.done:
		t.Fatalf("中間成功一次要歸零，不該結束：%v", err)
	default:
	}
	h.cancel()
	_ = h.result()
}

func TestStreamMapsModuleEndReasons(t *testing.T) {
	t.Run("lost eligibility 是 failed_precondition 與 STORE_CLOSED", func(t *testing.T) {
		h := startStream(t, nil, nil)
		h.push("", &im.Ended{Reason: im.EndLostEligibility})
		err := h.result()
		if codeOf(err) != connect.CodeFailedPrecondition || reasonOf(err) != "STORE_CLOSED" {
			t.Fatalf("關店 = %v（code %v、reason %q）", err, codeOf(err), reasonOf(err))
		}
	})
	t.Run("shutdown 是 unavailable", func(t *testing.T) {
		h := startStream(t, nil, nil)
		h.push("", &im.Ended{Reason: im.EndShutdown})
		if err := h.result(); codeOf(err) != connect.CodeUnavailable {
			t.Fatalf("shutdown = %v", err)
		}
	})
	t.Run("重驗時發現關店也是 failed_precondition", func(t *testing.T) {
		h := startStream(t, []error{&im.Ended{Reason: im.EndLostEligibility}}, nil)
		h.clk.fire(chkTimer, 1)
		err := h.result()
		if codeOf(err) != connect.CodeFailedPrecondition || reasonOf(err) != "STORE_CLOSED" {
			t.Fatalf("= %v", err)
		}
	})
}

func TestStreamReturnsOnContextCancelClosesTheSourceAndLeavesNoPump(t *testing.T) {
	h := startStream(t, nil, nil)
	h.cancel()
	if err := h.result(); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消 = %v", err)
	}
	select {
	case <-h.src.exited:
	default:
		t.Fatal("runStream 返回時 pump 的 Next 還沒退出（goroutine 洩漏）")
	}
}
