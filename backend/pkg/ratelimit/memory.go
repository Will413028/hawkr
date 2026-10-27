package ratelimit

import (
	"container/list"
	"sync"
	"time"
)

// windowEntry is a per-key fixed-window counter kept in the fallback LRU.
type windowEntry struct {
	key   string
	count int
	start time.Time
}

// boundedWindows is a fixed-capacity, LRU-evicting set of per-key fixed-window
// counters. It replaces the old unbounded map[ip]*limiter (which let key
// cardinality — one entry per distinct client IP — grow without limit and
// exhaust memory). Capacity caps live keys; the least-recently-used key is
// evicted when a new key arrives at capacity.
type boundedWindows struct {
	mu  sync.Mutex
	cap int
	ll  *list.List               // front = most recently used
	m   map[string]*list.Element // key → element holding *windowEntry
}

func newBoundedWindows(capacity int) *boundedWindows {
	return &boundedWindows{
		cap: capacity,
		ll:  list.New(),
		m:   make(map[string]*list.Element, capacity),
	}
}

// allow increments the key's counter within the current window (resetting it
// when the window has elapsed) and reports whether the count is within limit.
func (b *boundedWindows) allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()

	if el, ok := b.m[key]; ok {
		e := el.Value.(*windowEntry)
		if now.Sub(e.start) >= window {
			e.start = now
			e.count = 0
		}
		e.count++
		b.ll.MoveToFront(el)
		return e.count <= limit
	}

	// New key: evict the LRU entry first if we are at capacity, then insert.
	if b.ll.Len() >= b.cap {
		if back := b.ll.Back(); back != nil {
			delete(b.m, back.Value.(*windowEntry).key)
			b.ll.Remove(back)
		}
	}
	b.m[key] = b.ll.PushFront(&windowEntry{key: key, count: 1, start: now})
	return 1 <= limit
}

// count reports the key's current count within its window WITHOUT incrementing
// it or refreshing its LRU position — a read must not keep an otherwise idle
// key alive. An elapsed window reads as 0, matching allow's reset semantics.
func (b *boundedWindows) count(key string, window time.Duration) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	el, ok := b.m[key]
	if !ok {
		return 0
	}
	e := el.Value.(*windowEntry)
	if time.Since(e.start) >= window {
		return 0
	}
	return int64(e.count)
}
