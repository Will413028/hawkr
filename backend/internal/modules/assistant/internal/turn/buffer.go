package turn

import (
	"strconv"
	"sync"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

type buffer struct {
	mu      sync.Mutex
	events  []model.Event
	limit   int
	done    bool
	changed chan struct{}
	endedAt time.Time
}

func newBuffer(limit int) *buffer {
	return &buffer{
		events:  make([]model.Event, 0, min(limit, 16)),
		limit:   limit,
		changed: make(chan struct{}),
	}
}

func (b *buffer) append(ev model.Event, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return false
	}
	if !ev.Kind.Terminal() && len(b.events) >= b.limit-1 {
		return false
	}
	ev.Cursor = strconv.Itoa(len(b.events) + 1)
	b.events = append(b.events, ev)
	if ev.Kind.Terminal() {
		b.done = true
		b.endedAt = now
	}
	close(b.changed)
	b.changed = make(chan struct{})
	return true
}

func (b *buffer) endTime() (time.Time, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.endedAt, b.done
}

func (b *buffer) position(cursor string) (int, error) {
	pos := 0
	if cursor != "" {
		p, err := strconv.ParseUint(cursor, 10, 31)
		if err != nil {
			return 0, model.ErrCursorAhead
		}
		pos = int(p)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if pos > len(b.events) {
		return 0, model.ErrCursorAhead
	}
	return pos, nil
}

func (b *buffer) next(pos int) (evs []model.Event, wait <-chan struct{}, finished bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if pos < len(b.events) {
		return append([]model.Event(nil), b.events[pos:]...), nil, false
	}
	if b.done {
		return nil, nil, true
	}
	return nil, b.changed, false
}
