package turn

import (
	"strconv"
	"sync"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

// buffer 是一輪的事件緩衝：只增不減，事件依序存入，第 s 個事件（s 從 1 起）的序號就是它的 cursor；
// 切片隨事件增長，不預先配置。上限是 limit 個事件：前 limit-1 個位置給一般事件，最後一個位置保留給終止事件，
// 所以一般事件寫滿時 append 回 false（事件不寫入），終止事件永遠寫得進去。舊事件不會被覆蓋，沒有「cursor 過期」。
type buffer struct {
	mu      sync.Mutex
	events  []model.Event
	limit   int
	done    bool
	changed chan struct{} // 每次 append 後關閉並換新，喚醒等待者
	endedAt time.Time
}

func newBuffer(limit int) *buffer {
	return &buffer{
		events:  make([]model.Event, 0, min(limit, 16)),
		limit:   limit,
		changed: make(chan struct{}),
	}
}

// append 寫入一個事件。回 false 表示沒有寫入：終止事件之後的寫入，或一般事件已寫滿（保留最後一格給終止事件）。
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

// position 把 cursor 轉成「已收到的事件數」；空字串是 0。不是伺服器發的值，或超前於最新事件，回 ErrCursorAhead。
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

// next 回已收到 pos 個之後的事件；沒有時回等待用的 channel，終止後回 finished=true。
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
