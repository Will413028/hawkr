package turn

import (
	"strconv"
	"sync"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

// buffer 是一輪的事件環形緩衝：第 s 個事件（s 從 1 起）存在 ring[(s-1)%len(ring)]，只保留最近 len(ring) 個。
// 終止事件（finished 或 error）是最後一個，不會被覆蓋。
type buffer struct {
	mu      sync.Mutex
	ring    []model.Event
	last    uint64 // 最新事件的序號，0 表示還沒有事件
	done    bool
	changed chan struct{} // 每次 append 後關閉並換新，喚醒等待者
	endedAt time.Time
}

func newBuffer(capacity int) *buffer {
	return &buffer{ring: make([]model.Event, capacity), changed: make(chan struct{})}
}

// first 回緩衝裡最舊事件的序號；呼叫者持有 mu。
func (b *buffer) first() uint64 {
	if b.last <= uint64(len(b.ring)) {
		return 1
	}
	return b.last - uint64(len(b.ring)) + 1
}

// append 寫入一個事件。終止事件之後的寫入被忽略（回 false）。
func (b *buffer) append(ev model.Event, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return false
	}
	b.last++
	ev.Cursor = strconv.FormatUint(b.last, 10)
	b.ring[(b.last-1)%uint64(len(b.ring))] = ev
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

// position 把 cursor 轉成「已收到的最後序號」，並檢查它落在可接續的範圍內。
func (b *buffer) position(cursor string) (uint64, error) {
	var pos uint64
	if cursor != "" {
		p, err := strconv.ParseUint(cursor, 10, 64)
		if err != nil {
			return 0, model.ErrCursorAhead
		}
		pos = p
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if pos > b.last {
		return 0, model.ErrCursorAhead
	}
	if pos+1 < b.first() {
		return 0, model.ErrCursorExpired
	}
	return pos, nil
}

// next 回序號 pos 之後的下一批事件；沒有時回等待用的 channel，終止後回 finished=true。
func (b *buffer) next(pos uint64) (evs []model.Event, wait <-chan struct{}, finished bool, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if pos+1 < b.first() {
		return nil, nil, false, model.ErrCursorExpired
	}
	for s := pos + 1; s <= b.last; s++ {
		evs = append(evs, b.ring[(s-1)%uint64(len(b.ring))])
	}
	if len(evs) > 0 {
		return evs, nil, false, nil
	}
	if b.done {
		return nil, nil, true, nil
	}
	return nil, b.changed, false, nil
}
