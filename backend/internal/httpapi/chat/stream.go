package chat

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// streamConfig 是串流傳輸的常數。全部是估計值，不是量測值（step-2b-design.md 的常數段）；經 web host 反代與 Cloudflare 的實際行為需另行實測。
type streamConfig struct {
	// HeartbeatEvery：來源閒置這麼久就寫一個 Heartbeat（每次寫出任何 frame 後重新計時）。與三端 client 舊 WebSocket 的 15 秒相同，
	// client 的「30 秒沒有任何 frame 判定半開」容忍錯過一個心跳（step-2b-design.md §5.2）。
	HeartbeatEvery time.Duration
	// RecheckEvery：重新驗證 session 與身分的間隔（step-5-design.md §5.3）。
	RecheckEvery time.Duration
	// MaxLifetime：串流的最長壽命（step-5-design.md §5.3），到期正常結束，client 用最後的 seq 重連。
	MaxLifetime time.Duration
	// LifetimeJitter：壽命的抖動比例（±10%），每條串流各取一次，避免同時開的串流同時重連。
	LifetimeJitter float64
	// WriteTimeout：每次寫 frame 的期限；逾時結束串流，慢 client 不佔著連線。server 不設全域 WriteTimeout（會切斷所有串流）。
	WriteTimeout time.Duration
	// MaxGuardFailures：重新驗證連續暫時失敗幾次就以 unavailable 結束。
	MaxGuardFailures int
}

func defaultStreamConfig() streamConfig {
	return streamConfig{
		HeartbeatEvery: 15 * time.Second, RecheckEvery: 60 * time.Second, MaxLifetime: 15 * time.Minute,
		LifetimeJitter: 0.1, WriteTimeout: 10 * time.Second, MaxGuardFailures: 3,
	}
}

// streamTimer 是 time.Timer 的最小介面，測試換成手動到期的假計時器。
type streamTimer interface {
	C() <-chan time.Time
	Reset(d time.Duration)
	Stop()
}

// streamClock 是串流用到的時間來源。
type streamClock struct {
	Now      func() time.Time
	NewTimer func(d time.Duration) streamTimer
}

type realTimer struct{ t *time.Timer }

func (r realTimer) C() <-chan time.Time   { return r.t.C }
func (r realTimer) Reset(d time.Duration) { r.t.Reset(d) }
func (r realTimer) Stop()                 { r.t.Stop() }

func realClock() streamClock {
	return streamClock{Now: time.Now, NewTimer: func(d time.Duration) streamTimer { return realTimer{time.NewTimer(d)} }}
}

// streamSpec 是一條串流的輸入，與 im 無關：事件型別 E 由來源決定。
type streamSpec[E any] struct {
	Config streamConfig
	Clock  streamClock
	// Jitter 回傳 [0, 1) 的亂數，壽命以它取抖動。
	Jitter func() float64
	// Next 阻塞到下一個事件；ctx 取消要返回，且取消時不得丟掉待處理的事件（Subscription.Next 的語意）。只由 runStream 的單一 goroutine 呼叫。
	Next func(ctx context.Context) (E, error)
	// Write 寫出一個事件的 frame（info 是這一條串流的實際設定，Ready 要帶）；Heartbeat 寫出心跳 frame。
	Write     func(E, streamInfo) error
	Heartbeat func() error
	// Guard 重新驗證呼叫者（session、身分、店家），成功後自己把新身分交給來源；回 unauthenticated 結束串流，其他錯誤算暫時失敗。
	Guard func(ctx context.Context) error
	// SetWriteDeadline 設定底層連線的寫入期限（零值是取消）。
	SetWriteDeadline func(time.Time) error
	// Close 釋出來源（一定會呼叫，且早於 runStream 返回）。
	Close func()
	// Classify 判斷錯誤是不是來源自己的結束（例如模組的 Ended），是的話回對外的錯誤與 true。
	Classify func(error) (error, bool)
}

// streamInfo 是這一條串流的實際值：壽命已含抖動。
type streamInfo struct {
	HeartbeatEvery time.Duration
	MaxLifetime    time.Duration
}

type pulled[E any] struct {
	ev  E
	err error
}

// runStream 跑一條串流直到結束；回 nil 表示壽命到期的正常結束。
//
// 並行結構：一個 pump goroutine 專門呼叫 Next（Next 只能單一 goroutine 呼叫）並經不緩衝的 channel 交給主迴圈；主迴圈以 select
// 同時等事件、閒置計時、重驗計時與壽命，寫 frame 與 Guard 都在主迴圈（不與 Next 並行）。Guard 與 Close 可以和 Next 並行，
// 所以 Guard 期間 pump 照常拉取。計時不靠「Next 帶短 timeout 輪詢」：Next 在 ctx 取消時待處理事件留在訂閱裡（take 之後處理失敗會 requeue、
// 已備好的 out 不會被取走），所以只在結束時取消 pump 的 ctx，不會丟事件。
func runStream[E any](ctx context.Context, s streamSpec[E]) error {
	cfg := s.Config
	pumpCtx, stop := context.WithCancel(ctx)
	events := make(chan pulled[E])
	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)
		for {
			ev, err := s.Next(pumpCtx)
			select {
			case events <- pulled[E]{ev, err}:
			case <-pumpCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() {
		stop()
		s.Close()
		<-pumpDone
	}()

	lifetime := time.Duration(float64(cfg.MaxLifetime) * (1 + cfg.LifetimeJitter*(2*s.Jitter()-1)))
	info := streamInfo{HeartbeatEvery: cfg.HeartbeatEvery, MaxLifetime: lifetime}
	idle := s.Clock.NewTimer(cfg.HeartbeatEvery)
	recheck := s.Clock.NewTimer(cfg.RecheckEvery)
	life := s.Clock.NewTimer(lifetime)
	defer idle.Stop()
	defer recheck.Stop()
	defer life.Stop()

	write := func(f func() error) error {
		if err := s.SetWriteDeadline(s.Clock.Now().Add(cfg.WriteTimeout)); err != nil {
			return apperr.WithCause(apperr.Unavailable("串流無法設定寫入期限"), err)
		}
		if err := f(); err != nil {
			return apperr.WithCause(apperr.Unavailable("串流寫入失敗或逾時"), err)
		}
		_ = s.SetWriteDeadline(time.Time{})
		idle.Reset(cfg.HeartbeatEvery)
		return nil
	}

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-life.C():
			return nil
		case <-idle.C():
			if err := write(s.Heartbeat); err != nil {
				return err
			}
		case <-recheck.C():
			err := s.Guard(ctx)
			recheck.Reset(cfg.RecheckEvery)
			switch {
			case err == nil:
				failures = 0
			case ctx.Err() != nil:
				return ctx.Err()
			default:
				if end, ok := s.Classify(err); ok {
					return end
				}
				if apperr.KindOf(err) == apperr.KindUnauthenticated {
					return err
				}
				failures++
				if failures >= cfg.MaxGuardFailures {
					return apperr.WithCause(apperr.Unavailable("串流重新驗證連續失敗"), fmt.Errorf("連續 %d 次：%w", failures, err))
				}
			}
		case r := <-events:
			if r.err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if end, ok := s.Classify(r.err); ok {
					return end
				}
				return apperr.WithCause(apperr.Unavailable("串流中斷"), r.err)
			}
			if err := write(func() error { return s.Write(r.ev, info) }); err != nil {
				return err
			}
		}
	}
}

// 壽命的亂數來源（正式用 math/rand/v2）。
func randFloat() float64 { return rand.Float64() } //nolint:gosec // 壽命抖動只用來錯開重連時間，不是安全用途
