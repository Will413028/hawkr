package chat

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type streamConfig struct {
	HeartbeatEvery time.Duration
	RecheckEvery time.Duration
	MaxLifetime time.Duration
	LifetimeJitter float64
	WriteTimeout time.Duration
	MaxGuardFailures int
}

func defaultStreamConfig() streamConfig {
	return streamConfig{
		HeartbeatEvery: 15 * time.Second, RecheckEvery: 60 * time.Second, MaxLifetime: 15 * time.Minute,
		LifetimeJitter: 0.1, WriteTimeout: 10 * time.Second, MaxGuardFailures: 3,
	}
}

type streamTimer interface {
	C() <-chan time.Time
	Reset(d time.Duration)
	Stop()
}

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

type streamSpec[E any] struct {
	Config streamConfig
	Clock  streamClock
	Jitter func() float64
	Next func(ctx context.Context) (E, error)
	Write     func(E, streamInfo) error
	Heartbeat func() error
	Guard func(ctx context.Context) error
	SetWriteDeadline func(time.Time) error
	Close func()
	Classify func(error) (error, bool)
}

type streamInfo struct {
	HeartbeatEvery time.Duration
	MaxLifetime    time.Duration
}

type pulled[E any] struct {
	ev  E
	err error
}

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

func randFloat() float64 { return rand.Float64() } //nolint:gosec // 壽命抖動只用來錯開重連時間，不是安全用途
