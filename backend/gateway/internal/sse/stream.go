package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Sentinel errors returned by Subscribe so callers can map to HTTP status。
var (
	ErrPodCapReached     = errors.New("sse: pod connection cap reached")
	ErrSubjectCapReached = errors.New("sse: per-subject connection cap reached")
	ErrUserCapReached    = errors.New("sse: per-user connection cap reached")
)

// ReadyPayload is the data of the initial `ready` event sent to clients。
type ReadyPayload struct {
	Topic      string    `json:"topic"`
	ServerTime time.Time `json:"serverTime"`
	BufferMiss bool      `json:"bufferMiss"`
	Replayed   int       `json:"replayed"`
}

// ServeOptions controls per-request behaviour of ServeSSE。
type ServeOptions struct {
	// HeartbeatInterval overrides the hub default for this connection (0 = use hub default)。
	HeartbeatInterval time.Duration
}

// ServeSSE writes SSE headers, sends `ready`, replays buffered events, then
// runs the write loop (frames from conn.Outgoing + heartbeat ticker) until
// either the client disconnects or the request context is cancelled.
//
// Caller responsibilities BEFORE invoking this:
//   - h.Subscribe(conn) succeeded
//   - any authorization (AuthorizeSubscribe) already passed
//
// On return the connection is Unsubscribed (cleanup is the caller's responsibility
// via Hub.Unsubscribe — defer it before ServeSSE)。
//
// 设计书 §5.4 sequence。
func ServeSSE(
	ctx context.Context,
	hub *Hub,
	conn *Connection,
	w http.ResponseWriter,
	replay []BufferedEvent,
	bufferMiss bool,
	opts ServeOptions,
	log *zap.Logger,
) {
	if log == nil {
		log = zap.NewNop()
	}

	hbInterval := opts.HeartbeatInterval
	if hbInterval <= 0 {
		hbInterval = hub.cfg.HeartbeatInterval
	}
	writeDeadline := 2 * hbInterval
	rc := http.NewResponseController(w)
	refreshWriteDeadline := func() {
		_ = rc.SetWriteDeadline(time.Now().Add(writeDeadline))
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	conn.flusher.Flush()

	// 1. ready event。
	readyPayload, _ := json.Marshal(ReadyPayload{
		Topic:      conn.Topic,
		ServerTime: time.Now().UTC(),
		BufferMiss: bufferMiss,
		Replayed:   len(replay),
	})
	readyFrame := buildFrame("ready", readyPayload, hub.nextEventID())
	refreshWriteDeadline()
	if _, err := w.Write(readyFrame); err != nil {
		log.Debug("SSE.ready.write_failed", zap.Error(err))
		return
	}
	conn.flusher.Flush()

	// 2. replay buffered events (Last-Event-ID > clientLastSeen)。
	for _, ev := range replay {
		frame := conn.frameFor(log, ev.EventName, ev.Data, ev.ID)
		if frame == nil {
			continue // 個人化失敗：丟掉這個事件，不送內部形狀
		}
		refreshWriteDeadline()
		if _, err := w.Write(frame); err != nil {
			return
		}
	}
	if len(replay) > 0 {
		conn.flusher.Flush()
	}

	// 3. write loop。
	heartbeat := time.NewTicker(hbInterval)
	defer heartbeat.Stop()

	heartbeatFrame := buildFrame("heartbeat", []byte("{}"), "")

	clientGone := ctx.Done()

	for {
		select {
		case <-clientGone:
			return
		case <-conn.Closed():
			return
		case frame, ok := <-conn.Outgoing():
			if !ok {
				return
			}
			refreshWriteDeadline()
			if _, err := w.Write(frame); err != nil {
				return
			}
			conn.flusher.Flush()
		case <-heartbeat.C:
			refreshWriteDeadline()
			if _, err := w.Write(heartbeatFrame); err != nil {
				return
			}
			conn.flusher.Flush()
		}
	}
}

// buildFrame constructs a single SSE event frame ending with the required \n\n。
// id may be empty (e.g. heartbeat) — in that case the id: line is omitted。
func buildFrame(eventName string, data []byte, id string) []byte {
	var buf bytes.Buffer
	buf.Grow(len(data) + len(eventName) + len(id) + 32)
	buf.WriteString("event: ")
	buf.WriteString(eventName)
	buf.WriteByte('\n')
	buf.WriteString("data: ")
	buf.Write(data)
	buf.WriteByte('\n')
	if id != "" {
		buf.WriteString("id: ")
		buf.WriteString(id)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}
