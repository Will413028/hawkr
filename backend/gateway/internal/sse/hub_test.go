package sse

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeFlusher provides a minimal http.Flusher for tests.
type fakeFlusher struct{}

func (fakeFlusher) Flush() {}

func newConn(t *testing.T, topic, lastEventID string) *Connection {
	t.Helper()
	rec := httptest.NewRecorder()
	return NewConnection(topic, rec, fakeFlusher{}, lastEventID)
}

// drainFrames pulls all currently-queued frames from a connection without blocking.
func drainFrames(c *Connection, deadline time.Duration) [][]byte {
	var out [][]byte
	timeout := time.After(deadline)
	for {
		select {
		case f := <-c.Outgoing():
			out = append(out, f)
		case <-timeout:
			return out
		}
	}
}

func TestHub_Publish_FanOutToSubscribers(t *testing.T) {
	hub := NewHub(DefaultConfig(), nil)
	conn := newConn(t, "comment:post:1", "")
	if _, _, err := hub.Subscribe(conn); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer hub.Unsubscribe(conn)

	hub.Publish("comment:post:1", "comment.created", []byte(`{"id":"1"}`), "")

	frames := drainFrames(conn, 100*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("want 1 frame, got %d", len(frames))
	}
	if !bytes.Contains(frames[0], []byte("event: comment.created")) {
		t.Errorf("frame missing event name: %s", frames[0])
	}
}

func TestHub_Subscribe_DoesNotDoublePushOnConcurrentPublish(t *testing.T) {
	// Regression test for the race fix: a subscriber that arrives concurrently
	// with a publish must NOT receive both a replay of the in-flight event AND
	// a fan-out send of the same event.
	hub := NewHub(DefaultConfig(), nil)

	// Existing event so ring buffer is non-empty
	hub.Publish("t", "e", []byte(`{"v":0}`), "")

	// Tight loop: subscribe + publish concurrently, ensure each new subscriber
	// receives every fanned-out event exactly once (no duplicates).
	for i := 0; i < 50; i++ {
		conn := newConn(t, "t", "")
		if _, _, err := hub.Subscribe(conn); err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		// Subscribe is now in topic — publish 1 event, expect exactly 1 frame.
		hub.Publish("t", "e", []byte(`{"v":1}`), "")
		frames := drainFrames(conn, 50*time.Millisecond)
		if len(frames) != 1 {
			t.Errorf("iter %d: want 1 frame, got %d", i, len(frames))
		}
		hub.Unsubscribe(conn)
	}
}

func TestHub_Replay_FromLastEventID(t *testing.T) {
	hub := NewHub(DefaultConfig(), nil)

	hub.Publish("t", "e", []byte(`{"v":1}`), "")
	// Capture the id of the first publish via a subscriber
	c1 := newConn(t, "t", "")
	hub.Subscribe(c1)
	hub.Publish("t", "e", []byte(`{"v":2}`), "")
	frames := drainFrames(c1, 100*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("c1 want 1 frame after subscribe+publish, got %d", len(frames))
	}
	// Extract id from frame v=2
	id := extractID(frames[0])
	hub.Unsubscribe(c1)

	// New subscriber comes back with Last-Event-ID == v=2 id; expects no replay.
	hub.Publish("t", "e", []byte(`{"v":3}`), "")
	c2 := newConn(t, "t", id)
	replay, miss, err := hub.Subscribe(c2)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer hub.Unsubscribe(c2)
	if miss {
		t.Errorf("unexpected bufferMiss")
	}
	if len(replay) != 1 {
		t.Errorf("want 1 replayed event (v=3), got %d", len(replay))
	}
}

func TestHub_BufferMiss_UnknownLastEventID(t *testing.T) {
	hub := NewHub(DefaultConfig(), nil)
	hub.Publish("t", "e", []byte(`{"v":1}`), "")

	c := newConn(t, "t", "9999999999999-9999")
	_, miss, err := hub.Subscribe(c)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer hub.Unsubscribe(c)
	if !miss {
		t.Errorf("want bufferMiss=true for unknown Last-Event-ID")
	}
}

func TestHub_Batch_SingleEventBypassesBatchWrapper(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BatchWindow = 20 * time.Millisecond
	hub := NewHub(cfg, nil)
	conn := newConn(t, "t", "")
	hub.Subscribe(conn)
	defer hub.Unsubscribe(conn)

	hub.Publish("t", "comment.created", []byte(`{"v":1}`), "comment.batch")

	frames := drainFrames(conn, 60*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("want 1 frame, got %d", len(frames))
	}
	if !bytes.Contains(frames[0], []byte("event: comment.created")) {
		t.Errorf("single-event window should NOT use batch wrapper; got %s", frames[0])
	}
}

func TestHub_Batch_MultiEventMerges(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BatchWindow = 20 * time.Millisecond
	hub := NewHub(cfg, nil)
	conn := newConn(t, "t", "")
	hub.Subscribe(conn)
	defer hub.Unsubscribe(conn)

	for i := 0; i < 3; i++ {
		hub.Publish("t", "comment.created", []byte(`{"v":1}`), "comment.batch")
	}

	frames := drainFrames(conn, 60*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("want 1 batch frame, got %d", len(frames))
	}
	if !bytes.Contains(frames[0], []byte("event: comment.batch")) {
		t.Errorf("multi-event window should use batch wrapper; got %s", frames[0])
	}
	if !bytes.Contains(frames[0], []byte(`"events":`)) {
		t.Errorf("batch payload should contain events array; got %s", frames[0])
	}
}

// TestHub_Caps_PerSubject 驗證 per-subject 上限。
//
// 契約自 issue #2040 起改變：滿座時不再無條件拒絕，而是先嘗試把最舊的位子
// 回收過來（SSE 沒有存活訊號，被中間層養著的殭屍會永久佔位——理由見
// capgroup.go 檔頭）。硬上限本身沒有放寬：同時存活數永遠 ≤ MaxConnPerSubject，
// 且每個視窗的回收額度用完後仍然回 ErrSubjectCapReached。
func TestHub_Caps_PerSubject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConnPerSubject = 2
	hub := NewHub(cfg, nil)

	mkAnon := func() *Connection {
		c := newConn(t, "t", "")
		c.Subject = "v:1.2.3.4"
		return c
	}

	c1 := mkAnon()
	if _, _, err := hub.Subscribe(c1); err != nil {
		t.Fatalf("c1: %v", err)
	}
	c2 := mkAnon()
	if _, _, err := hub.Subscribe(c2); err != nil {
		t.Fatalf("c2: %v", err)
	}

	// 滿座 → 回收最舊的（c1）換位子給 c3。
	c3 := mkAnon()
	if _, _, err := hub.Subscribe(c3); err != nil {
		t.Fatalf("c3 應可回收最舊位子入座，got %v", err)
	}
	if !isClosed(c1) {
		t.Error("c1（最舊）應被回收並關閉")
	}
	if got := hub.CapGroupSize(0, "v:1.2.3.4"); got != cfg.MaxConnPerSubject {
		t.Errorf("佔位數 = %d，want %d（硬上限不得被回收機制放寬）", got, cfg.MaxConnPerSubject)
	}

	// 把回收額度耗盡後，必須回到原本的 429 行為。
	remaining := evictBudgetFor(cfg.MaxConnPerSubject) - 1
	last := c3
	for i := 0; i < remaining; i++ {
		c := mkAnon()
		if _, _, err := hub.Subscribe(c); err != nil {
			t.Fatalf("額度內第 %d 次回收應成功，got %v", i+2, err)
		}
		last = c
	}
	if _, _, err := hub.Subscribe(mkAnon()); !errors.Is(err, ErrSubjectCapReached) {
		t.Errorf("額度用完後 want ErrSubjectCapReached, got %v", err)
	}

	// 正常釋放（連線自己收工）騰出的位子不吃回收額度，額度已耗盡也照樣可用。
	hub.Unsubscribe(last)
	if _, _, err := hub.Subscribe(mkAnon()); err != nil {
		t.Errorf("正常釋放後入座 want nil, got %v", err)
	}
}

// TestHub_Caps_PerUser 驗證 per-user 上限，契約同 TestHub_Caps_PerSubject。
func TestHub_Caps_PerUser(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConnPerUser = 1
	hub := NewHub(cfg, nil)

	mkUser := func() *Connection {
		c := newConn(t, "t", "")
		c.AccountID = 42
		return c
	}

	c1 := mkUser()
	if _, _, err := hub.Subscribe(c1); err != nil {
		t.Fatalf("c1: %v", err)
	}

	// 滿座 → 回收 c1 的位子。
	c2 := mkUser()
	if _, _, err := hub.Subscribe(c2); err != nil {
		t.Fatalf("c2 應可回收最舊位子入座，got %v", err)
	}
	if !isClosed(c1) {
		t.Error("c1 應被回收並關閉")
	}
	if got := hub.CapGroupSize(42, ""); got != cfg.MaxConnPerUser {
		t.Errorf("佔位數 = %d，want %d", got, cfg.MaxConnPerUser)
	}

	// 額度耗盡後回到 429。
	remaining := evictBudgetFor(cfg.MaxConnPerUser) - 1
	for i := 0; i < remaining; i++ {
		if _, _, err := hub.Subscribe(mkUser()); err != nil {
			t.Fatalf("額度內第 %d 次回收應成功，got %v", i+2, err)
		}
	}
	if _, _, err := hub.Subscribe(mkUser()); !errors.Is(err, ErrUserCapReached) {
		t.Errorf("額度用完後 want ErrUserCapReached, got %v", err)
	}
}

func TestHub_RingBuffer_GC(t *testing.T) {
	buf := NewRingBuffer(10, 1*time.Millisecond)
	buf.Push(BufferedEvent{ID: "x", CreatedAt: time.Now().Add(-1 * time.Second)})
	buf.Push(BufferedEvent{ID: "y", CreatedAt: time.Now()})
	remaining := buf.GC(time.Now())
	if remaining != 1 {
		t.Errorf("want 1 remaining after GC, got %d", remaining)
	}
}

// extractID parses `id: <value>` out of an SSE frame.
func extractID(frame []byte) string {
	for _, line := range strings.Split(string(frame), "\n") {
		if strings.HasPrefix(line, "id: ") {
			return strings.TrimPrefix(line, "id: ")
		}
	}
	return ""
}
