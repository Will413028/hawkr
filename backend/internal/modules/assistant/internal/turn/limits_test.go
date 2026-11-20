package turn

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/adk/v2/session"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)} }

func wantLimit(t *testing.T, err error, sentinel error) {
	t.Helper()
	var le *model.LimitError
	if !errors.Is(err, sentinel) || !errors.As(err, &le) || le.RetryAfter <= 0 {
		t.Fatalf("err = %v，要 %v 且帶大於零的 RetryAfter", err, sentinel)
	}
}

// 同一位買家同時進行中的輪次（跨對話）最多 2 輪；第 3 輪被擋、別的買家不受影響，輪次結束後釋放名額。
func TestAtMostTwoConcurrentTurnsPerBuyerAcrossSessions(t *testing.T) {
	g := newGatedLLM()
	e := newExecutor(t, g, &fakeOrders{}, func(c *Config) { c.MaxSubmitsPerMinute = 100 })
	var refs []Ref
	for range 2 {
		ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "你好"})
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}
	_, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "第三輪"})
	wantLimit(t, err, model.ErrTooManyTurns)
	other, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer + 1, Topic: model.TopicChat, Message: "別人"})
	if err != nil {
		t.Fatalf("別的買家被擋：%v", err)
	}
	close(g.release)
	collect(t, e, buyer, refs[0].TurnID, "")
	collect(t, e, buyer, refs[1].TurnID, "")
	collect(t, e, buyer+1, other.TurnID, "")
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "結束後再送"})
	if err != nil {
		t.Fatalf("輪次結束後名額沒有釋放：%v", err)
	}
	collect(t, e, buyer, ref.TurnID, "")
}

// 每分鐘最多 10 次：第 11 次被擋，窗口過去後恢復；被擋的送出不佔名額。
func TestAtMostTenSubmitsPerMinutePerBuyer(t *testing.T) {
	clock := newClock()
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, func(c *Config) {
		c.Now = clock.now
		c.MaxConcurrentTurns = 100
	})
	for i := range DefaultMaxSubmitsPerMinute {
		ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "你好"})
		if err != nil {
			t.Fatalf("第 %d 次：%v", i+1, err)
		}
		collect(t, e, buyer, ref.TurnID, "")
		clock.advance(time.Second)
	}
	_, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "第十一次"})
	wantLimit(t, err, model.ErrRateLimited)
	_, err = e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicReturnPolicy})
	wantLimit(t, err, model.ErrRateLimited)
	if _, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer + 1, Topic: model.TopicChat, Message: "別人"}); err != nil {
		t.Fatalf("別的買家被擋：%v", err)
	}
	clock.advance(time.Minute)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "窗口過後"})
	if err != nil {
		t.Fatalf("窗口過後仍被擋：%v", err)
	}
	collect(t, e, buyer, ref.TurnID, "")
}

// 已受理請求的冪等重送不計入也不受上限影響：次數用完、同時輪次已滿時仍回原本的 Ref。
func TestIdempotentReplayIsExemptFromBothLimits(t *testing.T) {
	g := newGatedLLM()
	e := newExecutor(t, g, &fakeOrders{}, func(c *Config) {
		c.MaxConcurrentTurns = 1
		c.MaxSubmitsPerMinute = 1
	})
	first, err := submitKeyed(e, buyer, requestA, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		again, err := submitKeyed(e, buyer, requestA, "", "你好")
		if err != nil || again != first {
			t.Fatalf("第 %d 次重送 = %+v, %v；要等於 %+v", i+1, again, err, first)
		}
	}
	if _, err := submitKeyed(e, buyer, "22222222-2222-4222-8222-222222222222", "", "新的請求"); !errors.Is(err, model.ErrRateLimited) && !errors.Is(err, model.ErrTooManyTurns) {
		t.Fatalf("新請求 = %v，要被上限擋下", err)
	}
	close(g.release)
	collect(t, e, buyer, first.TurnID, "")
}

// 失敗路徑都釋放同時輪次的名額：建立對話失敗多次之後，仍不會撞上同時輪次上限。
func TestFailedSubmitsReleaseTheConcurrencySlot(t *testing.T) {
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, func(c *Config) {
		c.Sessions = failingSessions{c.Sessions}
		c.MaxConcurrentTurns = 1
	})
	for i := range 2 * DefaultMaxSubmitsPerMinute {
		_, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "你好"})
		if !errors.Is(err, model.ErrSessionCreate) {
			t.Fatalf("第 %d 次 = %v，要 ErrSessionCreate（名額沒釋放會變成 ErrTooManyTurns）", i+1, err)
		}
	}
	_, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, SessionID: "不存在", Message: "你好"})
	if !errors.Is(err, model.ErrSessionNotFound) {
		t.Fatalf("= %v，要 ErrSessionNotFound", err)
	}
}

// 受理前失敗的送出（建立對話失敗、對話不存在）不留紀錄，不佔每分鐘的名額：失敗次數超過上限之後，成功的送出仍被受理。
func TestFailedSubmitsDoNotCountTowardsThePerMinuteLimit(t *testing.T) {
	clock := newClock()
	var failing atomic.Bool
	failing.Store(true)
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, func(c *Config) {
		c.Now = clock.now
		c.Sessions = toggledFailingSessions{Service: c.Sessions, failing: &failing}
	})
	for i := range 3 * DefaultMaxSubmitsPerMinute {
		if _, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "你好"}); !errors.Is(err, model.ErrSessionCreate) {
			t.Fatalf("第 %d 次 = %v，要 ErrSessionCreate", i+1, err)
		}
	}
	for range DefaultMaxSubmitsPerMinute {
		if _, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, SessionID: "不存在", Message: "你好"}); !errors.Is(err, model.ErrSessionNotFound) {
			t.Fatalf("= %v，要 ErrSessionNotFound", err)
		}
	}
	failing.Store(false)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "終於成功"})
	if err != nil {
		t.Fatalf("失敗的送出佔了每分鐘的名額：%v", err)
	}
	collect(t, e, buyer, ref.TurnID, "")
}

type toggledFailingSessions struct {
	session.Service
	failing *atomic.Bool
}

func (s toggledFailingSessions) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	if s.failing.Load() {
		return nil, errors.New("session store down")
	}
	return s.Service.Create(ctx, req)
}

// 兩個上限都超過時，RetryAfter 取較長的那個，而且被擋的送出不記進每分鐘的次數。
func TestBothLimitsExceededReturnsTheLongerRetryAfter(t *testing.T) {
	clock := newClock()
	g := newGatedLLM()
	e := newExecutor(t, g, &fakeOrders{}, func(c *Config) {
		c.Now = clock.now
		c.MaxConcurrentTurns = 1
		c.MaxSubmitsPerMinute = 1
	})
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "你好"})
	if err != nil {
		t.Fatal(err)
	}
	clock.advance(10 * time.Second)
	_, err = e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "又一則"})
	var le *model.LimitError
	if !errors.As(err, &le) || le.RetryAfter != 50*time.Second {
		t.Fatalf("err = %v，RetryAfter 要是每分鐘上限的 50 秒（比同時輪次的 %v 長）", err, concurrentRetryAfter)
	}
	close(g.release)
	collect(t, e, buyer, ref.TurnID, "")
}

// Abort 讓進行中的一輪以失敗結束：終止事件是 error，名額釋放，同一個對話可以再送。
func TestAbortEndsTheTurnAsFailedAndReleasesTheSession(t *testing.T) {
	g := newGatedLLM()
	e := newExecutor(t, g, &fakeOrders{}, nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, Message: "你好"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, SessionID: ref.SessionID, Message: "再一則"}); !errors.Is(err, model.ErrTurnInProgress) {
		t.Fatalf("進行中的對話 = %v，要 ErrTurnInProgress", err)
	}
	e.Abort(buyer+1, ref.TurnID)
	e.Abort(buyer, ref.TurnID)
	evs := collect(t, e, buyer, ref.TurnID, "")
	if last := evs[len(evs)-1]; last.Kind != model.EventError {
		t.Fatalf("最後一個事件 = %+v，要是 error", last)
	}
	again, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, SessionID: ref.SessionID, Message: "再一則"})
	if err != nil {
		t.Fatalf("Abort 之後同一個對話仍被擋：%v", err)
	}
	close(g.release)
	collect(t, e, buyer, again.TurnID, "")
}

// failOnceCreate 的第一次 Create 等到 release 才失敗，之後交給底層。
type failOnceCreate struct {
	session.Service
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (f *failOnceCreate) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	first := false
	f.once.Do(func() { first = true })
	if first {
		f.entered <- struct{}{}
		<-f.release
		return nil, errors.New("session store down")
	}
	return f.Service.Create(ctx, req)
}

// 第一次受理還在進行時，同一個鍵、不同內容的重送要等第一次的結果；第一次失敗不留紀錄，所以這個重送當新請求受理，不是內容不符。
func TestMismatchedRetryWaitsForAFailingFirstAcceptance(t *testing.T) {
	f := &failOnceCreate{Service: session.InMemoryService(), entered: make(chan struct{}, 1), release: make(chan struct{})}
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, func(c *Config) { c.Sessions = f })
	firstErr := make(chan error, 1)
	go func() {
		_, err := submitKeyed(e, buyer, requestA, "", "你好")
		firstErr <- err
	}()
	<-f.entered
	type result struct {
		ref Ref
		err error
	}
	second := make(chan result, 1)
	go func() {
		ref, err := submitKeyed(e, buyer, requestA, "", "不一樣的內容")
		second <- result{ref, err}
	}()
	select {
	case r := <-second:
		t.Fatalf("第一次還沒結束，重送就回了 %+v, %v", r.ref, r.err)
	case <-time.After(100 * time.Millisecond):
	}
	close(f.release)
	if err := <-firstErr; !errors.Is(err, model.ErrSessionCreate) {
		t.Fatalf("第一次 = %v", err)
	}
	r := <-second
	if r.err != nil {
		t.Fatalf("重送 = %v，要當新請求受理", r.err)
	}
	collect(t, e, buyer, r.ref.TurnID, "")
}
