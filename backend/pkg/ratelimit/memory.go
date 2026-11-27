package ratelimit

import (
	"container/list"
	"sync"
	"time"
)

type windowEntry struct {
	key   string
	count int
	start time.Time
}

type boundedWindows struct {
	mu  sync.Mutex
	cap int
	ll  *list.List
	m   map[string]*list.Element
}

func newBoundedWindows(capacity int) *boundedWindows {
	return &boundedWindows{
		cap: capacity,
		ll:  list.New(),
		m:   make(map[string]*list.Element, capacity),
	}
}

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

	if b.ll.Len() >= b.cap {
		if back := b.ll.Back(); back != nil {
			delete(b.m, back.Value.(*windowEntry).key)
			b.ll.Remove(back)
		}
	}
	b.m[key] = b.ll.PushFront(&windowEntry{key: key, count: 1, start: now})
	return 1 <= limit
}

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
