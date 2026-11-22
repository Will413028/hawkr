package chat

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeTimer 是測試用的計時器：只有測試呼叫 fire 才會到期，Reset 的次數（arms）讓測試等到 loop 已重新武裝它。
type fakeTimer struct {
	clk   *fakeClock
	c     chan time.Time
	d     time.Duration
	arms  int
	armed bool
}

func (t *fakeTimer) C() <-chan time.Time { return t.c }

func (t *fakeTimer) Reset(d time.Duration) {
	t.clk.mu.Lock()
	defer t.clk.mu.Unlock()
	select {
	case <-t.c:
	default:
	}
	t.d, t.armed = d, true
	t.arms++
	t.clk.cond.Broadcast()
}

func (t *fakeTimer) Stop() {
	t.clk.mu.Lock()
	defer t.clk.mu.Unlock()
	t.armed = false
	t.clk.stopped = true
	t.clk.cond.Broadcast()
}

// fakeClock 以建立時的時長辨識計時器（測試把三個時長設成互不相同）。runStream 只在返回時 Stop 計時器，所以 stopped 為真表示它已返回，
// 這時還在等計時器被武裝的 fire、await 立刻讓測試失敗，而不是永遠等下去。
type fakeClock struct {
	t       testing.TB
	stopped bool
	mu      sync.Mutex
	cond    *sync.Cond
	now     time.Time
	timers  map[time.Duration]*fakeTimer
}

func newFakeClock(t testing.TB) *fakeClock {
	c := &fakeClock{t: t, now: time.Now(), timers: map[time.Duration]*fakeTimer{}}
	c.cond = sync.NewCond(&c.mu)
	return c
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) NewTimer(d time.Duration) streamTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{clk: c, c: make(chan time.Time, 1), d: d, arms: 1, armed: true}
	c.timers[d] = t
	c.cond.Broadcast()
	return t
}

func (c *fakeClock) clock() streamClock { return streamClock{Now: c.Now, NewTimer: c.NewTimer} }

// fire 等到名為 name 的計時器（以最初的時長辨識）至少被武裝過 arms 次，再讓它到期。
func (c *fakeClock) fire(name time.Duration, arms int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.timers[name] == nil || c.timers[name].arms < arms || !c.timers[name].armed {
		if c.stopped {
			c.t.Fatalf("runStream 提早結束，計時器 %v 不會再被武裝（第 %d 次）", name, arms)
		}
		c.cond.Wait()
	}
	t := c.timers[name]
	t.armed = false
	c.now = c.now.Add(t.d)
	t.c <- c.now
}

// await 等到計時器至少被武裝過 arms 次（不讓它到期）。
func (c *fakeClock) await(name time.Duration, arms int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.timers[name] == nil || c.timers[name].arms < arms {
		if c.stopped {
			c.t.Fatalf("runStream 提早結束，計時器 %v 不會再被武裝（第 %d 次）", name, arms)
		}
		c.cond.Wait()
	}
}

// fakeSource 是測試的拉取來源：Next 阻塞到測試推進一個結果，ctx 取消時返回並記下自己已退出。
type fakeSource struct {
	results chan fakeResult
	exited  chan struct{}
	closed  chan struct{}
	once    sync.Once
}

type fakeResult struct {
	ev  string
	err error
}

func newFakeSource() *fakeSource {
	return &fakeSource{results: make(chan fakeResult), exited: make(chan struct{}), closed: make(chan struct{})}
}

func (s *fakeSource) Next(ctx context.Context) (string, error) {
	select {
	case r := <-s.results:
		return r.ev, r.err
	case <-ctx.Done():
		s.once.Do(func() { close(s.exited) })
		return "", ctx.Err()
	}
}

func (s *fakeSource) Close() { close(s.closed) }
