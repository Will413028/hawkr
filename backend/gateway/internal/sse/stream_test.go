package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// deadlineTrackingWriter records every SetWriteDeadline call so tests can
// assert ServeSSE refreshes the deadline before each write.
type deadlineTrackingWriter struct {
	http.ResponseWriter
	deadlines []time.Time
}

func (w *deadlineTrackingWriter) SetWriteDeadline(t time.Time) error {
	w.deadlines = append(w.deadlines, t)
	return nil
}

func TestServeSSE_RefreshesWriteDeadlineBeforeEachWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	dw := &deadlineTrackingWriter{ResponseWriter: rec}
	conn := NewConnection("topic", dw, fakeFlusher{}, "")
	hub := NewHub(DefaultConfig(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ServeSSE(ctx, hub, conn, dw, nil, false, ServeOptions{}, nil)
		close(done)
	}()

	if !conn.Send([]byte("event: test\ndata: {}\n\n")) {
		t.Fatal("failed to queue test frame")
	}
	time.Sleep(50 * time.Millisecond) // let the write loop drain the queued frame
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ServeSSE did not return after context cancellation")
	}

	// ready event (1) + the queued frame (1) = at least 2 deadline refreshes.
	if len(dw.deadlines) < 2 {
		t.Fatalf("expected at least 2 SetWriteDeadline calls (ready + queued frame), got %d", len(dw.deadlines))
	}
	want := 2 * hub.cfg.HeartbeatInterval
	for i, d := range dw.deadlines {
		if got := time.Until(d); got <= 0 || got > want {
			t.Fatalf("deadline #%d not within (0, %v] of now: got %v ahead", i, want, got)
		}
	}
}

// TestServeSSE_RefreshesWriteDeadlineDuringReplay covers the replay loop
// (Last-Event-ID catch-up) specifically — the write-deadline test above never
// exercises it because it passes replay: nil.
func TestServeSSE_RefreshesWriteDeadlineDuringReplay(t *testing.T) {
	rec := httptest.NewRecorder()
	dw := &deadlineTrackingWriter{ResponseWriter: rec}
	conn := NewConnection("topic", dw, fakeFlusher{}, "")
	hub := NewHub(DefaultConfig(), nil) // default 30s heartbeat: won't fire in this short test.

	replay := []BufferedEvent{
		{ID: "1-1", EventName: "test.replay", Data: []byte(`{"n":1}`)},
		{ID: "1-2", EventName: "test.replay", Data: []byte(`{"n":2}`)},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ServeSSE(ctx, hub, conn, dw, replay, false, ServeOptions{}, nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // let ready + replay drain before cancelling
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ServeSSE did not return after context cancellation")
	}

	// ready event (1) + each replayed event (2) = exactly 3 deadline refreshes;
	// the (long) default heartbeat interval cannot have fired in this window.
	want := 1 + len(replay)
	if got := len(dw.deadlines); got != want {
		t.Fatalf("expected exactly %d SetWriteDeadline calls (ready + %d replayed events), got %d", want, len(replay), got)
	}
}

// TestServeSSE_RefreshesWriteDeadlineOnHeartbeat covers the heartbeat-ticker
// write case specifically — the write-deadline test above never exercises it
// because it relies on the real ~30s default heartbeat interval, which never
// fires within the test's short runtime.
func TestServeSSE_RefreshesWriteDeadlineOnHeartbeat(t *testing.T) {
	rec := httptest.NewRecorder()
	dw := &deadlineTrackingWriter{ResponseWriter: rec}
	conn := NewConnection("topic", dw, fakeFlusher{}, "")
	hub := NewHub(DefaultConfig(), nil)

	const heartbeatInterval = 15 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ServeSSE(ctx, hub, conn, dw, nil, false, ServeOptions{HeartbeatInterval: heartbeatInterval}, nil)
		close(done)
	}()

	time.Sleep(8 * heartbeatInterval) // guarantee at least one heartbeat tick fires
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ServeSSE did not return after context cancellation")
	}

	// ready event (1) + at least one heartbeat write = more than just the ready refresh.
	if got := len(dw.deadlines); got <= 1 {
		t.Fatalf("expected additional SetWriteDeadline calls from heartbeat writes, got %d (only the ready event)", got)
	}
}
