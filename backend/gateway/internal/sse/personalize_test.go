package sse

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// stubPrepared 以觀看者的 BuyerID 是否為 7 分成兩個 variant，並計算 Render 次數。
type stubPrepared struct{ renders *int }

func (s stubPrepared) Variant(v Viewer) string {
	if v.BuyerID == 7 {
		return "mine"
	}
	return "other"
}

func (s stubPrepared) Render(v Viewer) ([]byte, error) {
	*s.renders++
	return []byte(`{"v":"` + s.Variant(v) + `"}`), nil
}

func TestHub_Personalized_RendersOncePerVariant(t *testing.T) {
	hub := NewHub(DefaultConfig(), nil)
	renders, prepares := 0, 0
	p := func(string, []byte) (PreparedEvent, error) {
		prepares++
		return stubPrepared{renders: &renders}, nil
	}
	var conns []*Connection
	for i := 0; i < 6; i++ {
		c := newConn(t, "t", "")
		c.Personalizer = p
		c.Viewer = Viewer{BuyerID: int64(7 + i%2*100)} // 7, 107, 7, 107 ...
		if _, _, err := hub.Subscribe(c); err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	hub.Publish("t", "e", []byte(`{"internal":1}`), "")

	if prepares != 1 || renders != 2 {
		t.Errorf("prepares=%d renders=%d, want 1 and 2 (one per variant, not per connection)", prepares, renders)
	}
	for i, c := range conns {
		frames := drainFrames(c, 100*time.Millisecond)
		want := `"v":"other"`
		if i%2 == 0 {
			want = `"v":"mine"`
		}
		if len(frames) != 1 || !bytes.Contains(frames[0], []byte(want)) || bytes.Contains(frames[0], []byte("internal")) {
			t.Errorf("conn %d frames = %q, want one frame with %s and no internal payload", i, frames, want)
		}
	}
}

func TestHub_Personalized_FailureDropsEventForPersonalizedConnectionsOnly(t *testing.T) {
	hub := NewHub(DefaultConfig(), nil)
	bad := newConn(t, "t", "")
	bad.Personalizer = func(string, []byte) (PreparedEvent, error) { return nil, errors.New("boom") }
	plain := newConn(t, "t", "")
	for _, c := range []*Connection{bad, plain} {
		if _, _, err := hub.Subscribe(c); err != nil {
			t.Fatal(err)
		}
	}
	hub.Publish("t", "e", []byte(`{"userId":"9"}`), "")

	if got := drainFrames(bad, 100*time.Millisecond); len(got) != 0 {
		t.Errorf("personalized connection received %q, want the event dropped", got)
	}
	if got := drainFrames(plain, 100*time.Millisecond); len(got) != 1 {
		t.Errorf("non-personalized connection frames = %d, want 1", len(got))
	}
}

// 拍快照時 topic 沒有任何連線，兩段上鎖之間才加入的連線仍要收到事件（個人化後）。
func TestHub_Personalized_ConnectionJoiningBetweenLockSectionsStillReceives(t *testing.T) {
	hub := NewHub(DefaultConfig(), nil)
	renders := 0
	late := newConn(t, "t", "")
	late.Personalizer = func(string, []byte) (PreparedEvent, error) { return stubPrepared{renders: &renders}, nil }
	late.Viewer = Viewer{BuyerID: 7}
	hub.afterPrepareHook = func() {
		hub.afterPrepareHook = nil
		if _, _, err := hub.Subscribe(late); err != nil {
			t.Error(err)
		}
	}
	hub.Publish("t", "e", []byte(`{"internal":1}`), "")

	frames := drainFrames(late, 50*time.Millisecond)
	if len(frames) != 1 || !bytes.Contains(frames[0], []byte(`"v":"mine"`)) || bytes.Contains(frames[0], []byte("internal")) {
		t.Errorf("late connection frames = %q, want one personalized frame", frames)
	}
}

// 同上，但解析失敗：晚加入的連線丟掉事件並記錄。
func TestHub_Personalized_LateConnectionParseFailureDropsAndLogs(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	hub := NewHub(DefaultConfig(), zap.New(core))
	late := newConn(t, "t", "")
	late.Personalizer = func(string, []byte) (PreparedEvent, error) { return nil, errors.New("boom") }
	hub.afterPrepareHook = func() {
		hub.afterPrepareHook = nil
		if _, _, err := hub.Subscribe(late); err != nil {
			t.Error(err)
		}
	}
	hub.Publish("t", "e", []byte(`{"userId":"9"}`), "")

	if got := drainFrames(late, 50*time.Millisecond); len(got) != 0 {
		t.Errorf("frames = %q, want dropped", got)
	}
	assertDropLog(t, logs, "SSE.personalize.failed", "e")
}

type failingRender struct{}

func (failingRender) Variant(Viewer) string         { return "x" }
func (failingRender) Render(Viewer) ([]byte, error) { return nil, errors.New("render boom") }

func TestHub_Personalized_LiveRenderFailureDropsAndLogs(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	hub := NewHub(DefaultConfig(), zap.New(core))
	c := newConn(t, "t", "")
	c.Personalizer = func(string, []byte) (PreparedEvent, error) { return failingRender{}, nil }
	if _, _, err := hub.Subscribe(c); err != nil {
		t.Fatal(err)
	}
	hub.Publish("t", "e", []byte(`{"userId":"9"}`), "")

	if got := drainFrames(c, 50*time.Millisecond); len(got) != 0 {
		t.Errorf("frames = %q, want dropped", got)
	}
	assertDropLog(t, logs, "SSE.personalize.render_failed", "e")
}

func TestConnection_ReplayPersonalizeFailuresLog(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	log := zap.New(core)
	c := newConn(t, "t", "")
	c.Personalizer = func(string, []byte) (PreparedEvent, error) { return nil, errors.New("boom") }
	if f := c.frameFor(log, "e", []byte(`{"userId":"9"}`), "1-1"); f != nil {
		t.Errorf("parse failure frame = %q, want nil", f)
	}
	assertDropLog(t, logs, "SSE.personalize.failed", "e")

	core2, logs2 := observer.New(zap.WarnLevel)
	c.Personalizer = func(string, []byte) (PreparedEvent, error) { return failingRender{}, nil }
	if f := c.frameFor(zap.New(core2), "e", []byte(`{"userId":"9"}`), "1-2"); f != nil {
		t.Errorf("render failure frame = %q, want nil", f)
	}
	assertDropLog(t, logs2, "SSE.personalize.render_failed", "e")
}

// assertDropLog 要求恰有一筆指定訊息，帶事件名稱與 id，且不含 payload。
func assertDropLog(t *testing.T, logs *observer.ObservedLogs, msg, event string) {
	t.Helper()
	entries := logs.FilterMessage(msg).All()
	if len(entries) != 1 {
		t.Fatalf("log %q count = %d, want 1 (all: %v)", msg, len(entries), logs.All())
	}
	m := entries[0].ContextMap()
	if m["event"] != event || m["eventId"] == "" || m["eventId"] == nil {
		t.Errorf("log fields = %v, want event=%s and eventId", m, event)
	}
	for k, v := range m {
		if s, ok := v.(string); ok && bytes.Contains([]byte(s), []byte("userId")) {
			t.Errorf("log field %s leaks payload: %s", k, s)
		}
	}
}
