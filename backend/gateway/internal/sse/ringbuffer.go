// Package sse implements the Server-Sent Events push platform for Gateway.
// 41 號詳細設計書 §五。本套件為通用 Hub，無業務語意；channel 註冊與業務對應由
// gateway/internal/realtime 套件負責。
package sse

import (
	"sync"
	"time"
)

// BufferedEvent is an event persisted in the ring buffer for Last-Event-ID replay.
type BufferedEvent struct {
	ID        string // server-assigned event id, format: "<epochMillis>-<seq>"
	EventName string // SSE event field, e.g. "comment.created"
	Data      []byte // pre-marshaled SSE data payload (raw JSON)
	Topic     string
	CreatedAt time.Time
}

// RingBuffer is a per-topic ring buffer storing recent events for replay.
// Events beyond size cap are dropped from the head; events older than TTL
// are removed on read or via scheduled GC (see Hub.Start's GC scheduler).
//
// 設計書 §5.3：
//   - 大小預設 200
//   - TTL 預設 5 分鐘
//   - 寫入時機：Hub.Publish 同步寫入
//   - 過期清理：scheduled goroutine 每 60s（避免 lazy clean 在低活動 topic 留 stale）
type RingBuffer struct {
	mu    sync.RWMutex
	items []BufferedEvent
	size  int
	ttl   time.Duration
}

// NewRingBuffer creates a ring buffer with given capacity and TTL.
func NewRingBuffer(size int, ttl time.Duration) *RingBuffer {
	if size <= 0 {
		size = 200
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &RingBuffer{
		items: make([]BufferedEvent, 0, size),
		size:  size,
		ttl:   ttl,
	}
}

// Push appends an event, dropping the oldest if at capacity.
func (b *RingBuffer) Push(ev BufferedEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) >= b.size {
		b.items = b.items[1:]
	}
	b.items = append(b.items, ev)
}

// Since returns all events with ID greater than lastEventID.
// If lastEventID is empty, returns nothing (client did not request replay).
// Returns (events, found) — found=false means lastEventID was provided but not
// found in buffer (client must perform one-shot refresh; signal via bufferMiss=true).
func (b *RingBuffer) Since(lastEventID string) ([]BufferedEvent, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if lastEventID == "" {
		return nil, true
	}

	idx := -1
	for i, e := range b.items {
		if e.ID == lastEventID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, false
	}
	out := make([]BufferedEvent, 0, len(b.items)-idx-1)
	out = append(out, b.items[idx+1:]...)
	return out, true
}

// GC removes events older than TTL. Returns the number of items remaining.
func (b *RingBuffer) GC(now time.Time) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	cutoff := now.Add(-b.ttl)
	cut := 0
	for cut < len(b.items) && b.items[cut].CreatedAt.Before(cutoff) {
		cut++
	}
	if cut > 0 {
		b.items = b.items[cut:]
	}
	return len(b.items)
}

// Len returns current buffer size (for metrics).
func (b *RingBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.items)
}
