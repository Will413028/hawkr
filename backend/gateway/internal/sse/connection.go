package sse

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Connection wraps a single SSE client connection.
// 設計書 §5.2。send chan buffered 32；超出視為 slow client → close。
type Connection struct {
	ID          string
	Topic       string
	send        chan []byte
	writer      http.ResponseWriter
	flusher     http.Flusher
	lastEventID string

	closeOnce sync.Once
	closed    chan struct{}

	// Per-connection metadata for cap enforcement / authorization。
	//
	// Subject is the rate-limit subject (middleware.RateLimitSubject): "v:<ip>"
	// for one human, "s:ssr" / "s:ci" for our own server-side callers. It is
	// deliberately NOT the peer address — the website's SSR worker and the CI
	// runner each present one address on behalf of an unbounded number of
	// people, so an address key lets five of them exhaust the cap for everyone
	// (#2264, 01 設計書 §3.2.8).
	Subject string // cap key for anonymous connections; empty = only the pod cap applies

	// AccountID 是這條連線的 cap 分組鍵：登入者的 account_id，0 = anonymous（走 per-subject cap path）。
	// notification stream 傳的是收件匣身分（buyer_id／store_id），那條路徑的 topic 也用同一個值；
	// 這個欄位只用來分組（hub.go 的 connsByUID），對 id 空間不敏感。
	// 要判 topic 的身分空間，看 realtime.Topic* 的呼叫點，不要看這裡。
	AccountID int64

	// Personalizer 與 Viewer：這條連線寫出前的 payload 個人化（即時事件與 replay 都經過）。
	// nil = 所有連線共用同一份 payload。事件在 Hub 內以共用的內部形狀保存，個人化只改這條連線看到的副本。
	// 同一個 topic 的連線必須用同一個 Personalizer（同一個 channel），差別只在 Viewer。
	Personalizer Personalizer
	Viewer       Viewer

	// ---- 由 Hub 在 Subscribe / teardown 期間維護，外部不得碰（issue #2040）---- //

	// subscribedAt 是入座時間，取自 Hub 的時鐘（可注入，測試用）。
	// 用途有二：挑最舊的位子回收、以及 GC 迴圈判定超過最長存活時間。
	subscribedAt time.Time

	// group 是本連線佔位的那個 cap group（per-user 或 per-subject）。
	group *connGroup

	// admitted / releasedOnce 讓「釋放位子」變成每條連線恰好一次。
	// 必要性：連線可能被 Hub 主動回收（cap 讓位或超過最長存活時間），之後
	// 服務該連線的 goroutine 仍會跑到自己的 `defer Unsubscribe`；若不擋住
	// 第二次，totalConns 會被重複扣、pod 層級的容量統計會漂移成負數。
	admitted     atomic.Bool
	releasedOnce atomic.Bool
}

// NewConnection constructs a Connection. Caller is responsible for verifying
// the writer supports http.Flusher (assert before calling).
func NewConnection(topic string, w http.ResponseWriter, flusher http.Flusher, lastEventID string) *Connection {
	return &Connection{
		ID:          uuid.NewString(),
		Topic:       topic,
		send:        make(chan []byte, 32),
		writer:      w,
		flusher:     flusher,
		lastEventID: lastEventID,
		closed:      make(chan struct{}),
	}
}

// frameFor 組出這條連線要寫出的 SSE frame；個人化失敗時回 nil，呼叫端必須丟掉這個事件（fail closed）。
// 只用在單條連線的路徑（replay）；即時事件由 Hub 對整個 topic 一次準備、依 variant 共用。
func (c *Connection) frameFor(log *zap.Logger, eventName string, data []byte, id string) []byte {
	if c.Personalizer == nil {
		return buildFrame(eventName, data, id)
	}
	prepared, err := c.Personalizer(eventName, data)
	if err != nil {
		// 只記事件名稱與 id，不記 payload。
		log.Warn("SSE.personalize.failed", zap.String("topic", c.Topic), zap.String("event", eventName), zap.String("eventId", id), zap.Error(err))
		return nil
	}
	out, err := prepared.Render(c.Viewer)
	if err != nil {
		log.Warn("SSE.personalize.render_failed", zap.String("topic", c.Topic), zap.String("event", eventName), zap.String("eventId", id), zap.Error(err))
		return nil
	}
	return buildFrame(eventName, out, id)
}

// Send queues an SSE frame for writing. Returns false if the connection's
// send buffer is full (slow consumer) or already closed; Hub treats this as
// a signal to drop the connection.
func (c *Connection) Send(frame []byte) bool {
	select {
	case <-c.closed:
		return false
	case c.send <- frame:
		return true
	default:
		return false
	}
}

// Close marks the connection as closed (idempotent).
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
}

// Closed returns the channel that signals the connection is done.
func (c *Connection) Closed() <-chan struct{} {
	return c.closed
}

// Outgoing returns the channel of frames to write to the wire.
// Consumed by stream.go's write loop.
func (c *Connection) Outgoing() <-chan []byte {
	return c.send
}

// LastEventID returns the Last-Event-ID header value (empty if not provided).
func (c *Connection) LastEventID() string {
	return c.lastEventID
}

// markReleased 回傳 true 代表這是本連線第一次（也是唯一一次）釋放。
// 從未成功入座的連線一律回傳 false——避免對「Subscribe 失敗但仍被呼叫
// Unsubscribe」的連線倒扣計數。
func (c *Connection) markReleased() bool {
	if !c.admitted.Load() {
		return false
	}
	return c.releasedOnce.CompareAndSwap(false, true)
}

// SubscribedAt 回傳入座時間（測試與診斷用；未入座為零值）。
func (c *Connection) SubscribedAt() time.Time {
	return c.subscribedAt
}
