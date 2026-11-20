package turn

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/adk/v2/session"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

const requestA = "11111111-1111-4111-8111-111111111111"

func submitKeyed(e *Executor, buyerID int64, requestID, sessionID, msg string) (Ref, error) {
	return e.Submit(context.Background(), SubmitInput{BuyerID: buyerID, Topic: model.TopicChat, SessionID: sessionID, Message: msg, RequestID: requestID})
}

// 同一位買家以同一個鍵重送：原樣回第一次的 Ref，LLM 只被呼叫一次、只有一個輪次；即使第一輪還在進行
// （對話被佔用）也是回放，不是 ErrTurnInProgress。
func TestIdempotentResendReturnsTheFirstResponseWithoutANewTurn(t *testing.T) {
	g := newGatedLLM()
	e := newExecutor(t, g, &fakeOrders{}, nil)
	first, err := submitKeyed(e, buyer, requestA, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	<-g.started
	again, err := submitKeyed(e, buyer, requestA, "", "你好")
	if err != nil || again != first {
		t.Fatalf("重送 = %+v, %v；要等於第一次 %+v", again, err, first)
	}
	close(g.release)
	collect(t, e, buyer, first.TurnID, "")
	if n := g.calls.Load(); n != 1 {
		t.Fatalf("LLM 被呼叫 %d 次，要 1 次（重送不開新的一輪）", n)
	}
	e.mu.Lock()
	turns := len(e.turns)
	e.mu.Unlock()
	if turns != 1 {
		t.Fatalf("輪次數 = %d，要 1", turns)
	}
	// 輪次結束後重送仍然回放。
	if again, err := submitKeyed(e, buyer, requestA, "", "你好"); err != nil || again != first {
		t.Fatalf("結束後重送 = %+v, %v", again, err)
	}
}

func TestIdempotencyKeyWithDifferentContentIsRejected(t *testing.T) {
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, nil)
	first, err := submitKeyed(e, buyer, requestA, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, first.TurnID, "")
	if _, err := submitKeyed(e, buyer, requestA, "", "不一樣的內容"); !errors.Is(err, model.ErrIdempotencyMismatch) {
		t.Fatalf("同鍵不同訊息 = %v，要 ErrIdempotencyMismatch", err)
	}
	if _, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "你好", RequestID: requestA}); !errors.Is(err, model.ErrIdempotencyMismatch) {
		t.Fatalf("同鍵不同主題 = %v，要 ErrIdempotencyMismatch", err)
	}
}

// 鍵的範圍是買家：別人用同一個值是另一個請求，不會讀到我的 Ref。
func TestIdempotencyKeyIsScopedToTheBuyer(t *testing.T) {
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, nil)
	mine, err := submitKeyed(e, buyer, requestA, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := submitKeyed(e, buyer+1, requestA, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	if theirs.TurnID == mine.TurnID || theirs.SessionID == mine.SessionID {
		t.Fatalf("別的買家拿到我的 Ref：%+v vs %+v", theirs, mine)
	}
	collect(t, e, buyer, mine.TurnID, "")
	collect(t, e, buyer+1, theirs.TurnID, "")
}

// 第一次受理失敗不留紀錄：同一個鍵之後可以用任何內容重送。
func TestAFailedFirstSubmitLeavesNoIdempotencyRecord(t *testing.T) {
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, nil)
	if _, err := submitKeyed(e, buyer, requestA, "不存在的對話", "你好"); !errors.Is(err, model.ErrSessionNotFound) {
		t.Fatalf("第一次 = %v", err)
	}
	ref, err := submitKeyed(e, buyer, requestA, "", "換個內容")
	if err != nil {
		t.Fatalf("失敗後同鍵重送被擋：%v", err)
	}
	collect(t, e, buyer, ref.TurnID, "")
}

// 同時送出同一個鍵：只有一個受理，另一個等它並拿到同樣的 Ref。
func TestConcurrentSendsWithTheSameKeyShareOneTurn(t *testing.T) {
	g := newGatedLLM()
	e := newExecutor(t, g, &fakeOrders{}, nil)
	const n = 8
	refs := make([]Ref, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			refs[i], errs[i] = submitKeyed(e, buyer, requestA, "", "你好")
		}()
	}
	<-g.started
	wg.Wait()
	close(g.release)
	for i := range n {
		if errs[i] != nil || refs[i] != refs[0] {
			t.Fatalf("第 %d 個 = %+v, %v；要等於 %+v", i, refs[i], errs[i], refs[0])
		}
	}
	collect(t, e, buyer, refs[0].TurnID, "")
	if c := g.calls.Load(); c != 1 {
		t.Fatalf("LLM 被呼叫 %d 次，要 1 次", c)
	}
}

// 紀錄與輪次的保留期一起清除：輪次被清掉之後，同一個鍵是新的請求。
func TestIdempotencyRecordExpiresWithTheTurn(t *testing.T) {
	var mu sync.Mutex
	now := time.Now()
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, func(c *Config) {
		c.Now = clock
		c.Retention = time.Minute
		c.Sessions = session.InMemoryService()
	})
	first, err := submitKeyed(e, buyer, requestA, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, first.TurnID, "")
	mu.Lock()
	now = now.Add(2 * time.Minute)
	mu.Unlock()
	second, err := submitKeyed(e, buyer, requestA, "", "完全不同的內容")
	if err != nil {
		t.Fatalf("保留期過後同鍵不同內容被擋：%v", err)
	}
	if second.TurnID == first.TurnID {
		t.Fatalf("保留期過後應開新的一輪，拿到同一個輪次 %s", second.TurnID)
	}
	collect(t, e, buyer, second.TurnID, "")
	e.mu.Lock()
	left := len(e.idem)
	e.mu.Unlock()
	if left != 1 {
		t.Fatalf("冪等紀錄數 = %d，過期的要一起清掉，只剩 1", left)
	}
}

func TestEmptyMessageIsRejectedByTheRunnerToo(t *testing.T) {
	e := newExecutor(t, textLLM("x"), &fakeOrders{}, nil)
	for _, topic := range []model.Topic{model.TopicOrders, model.TopicTracking, model.TopicChat} {
		if _, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: topic}); !errors.Is(err, model.ErrMessageRequired) {
			t.Fatalf("主題 %d 空訊息 = %v，要 ErrMessageRequired（不再有預設訊息）", topic, err)
		}
	}
}

// 輪次結束超過保留期就被清掉：訂閱回 ErrTurnNotFound（不是 cursor 過期，也不是成功；A9-D17 修訂）。
func TestWatchOfATurnPastRetentionIsNotFound(t *testing.T) {
	var mu sync.Mutex
	now := time.Now()
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, func(c *Config) { c.Now = clock; c.Retention = time.Minute })
	ref, err := submitKeyed(e, buyer, requestA, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, ref.TurnID, "")
	mu.Lock()
	now = now.Add(30 * time.Second)
	mu.Unlock()
	if _, err := e.Watch(context.Background(), buyer, ref.TurnID, ""); err != nil {
		t.Fatalf("保留期內訂閱 = %v", err)
	}
	mu.Lock()
	now = now.Add(2 * time.Minute)
	mu.Unlock()
	if _, err := e.Watch(context.Background(), buyer, ref.TurnID, ""); !errors.Is(err, model.ErrTurnNotFound) {
		t.Fatalf("超過保留期的輪次 = %v，要 ErrTurnNotFound", err)
	}
	if _, err := e.Watch(context.Background(), buyer, ref.TurnID, "5"); !errors.Is(err, model.ErrTurnNotFound) {
		t.Fatalf("超過保留期的輪次帶 cursor = %v，要 ErrTurnNotFound", err)
	}
}
