package sse

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"go.uber.org/zap"
)

// HubConfig holds tunables for the Hub. All fields have defaults; pass via env.
type HubConfig struct {
	RingBufferSize    int
	RingBufferTTL     time.Duration
	BatchWindow       time.Duration
	BatchMaxEvents    int
	MaxConnPerPod     int
	MaxConnPerSubject int
	MaxConnPerUser    int
	HeartbeatInterval time.Duration
	GCInterval        time.Duration

	// MaxConnLifetime 是單條連線在 server 端的最長存活時間，逾時由 GC 迴圈
	// 主動收掉，client 的 EventSource 會自行重連（issue #2040）。
	//
	// 為什麼要有：SSE 沒有存活訊號，中間層又可能替一條早就沒人在看的連線
	// 繼續收下 heartbeat，於是連線的生命週期實際上是被中間層決定的（實測
	// 600.0 秒）。設一個比它短的上限，主導權才回到 origin 手上，殭屍佔用
	// 也有了不依賴任何外部行為的確定上界。
	//
	// 為什麼是 8 分鐘而不是更短：這個值不是拿來修 cap 的（那是 admit 的
	// 回收邏輯負責），它只是資源面的兜底。調太短會強迫所有正常使用者頻繁
	// 重連，而每次重連都要重跑 Last-Event-ID replay；環形緩衝 TTL 是 5 分鐘，
	// 重連間隔逼近它就會開始出現 bufferMiss、讓前端做整頁刷新。8 分鐘同時
	// 滿足「短於中間層的 600 秒」與「明顯長於 buffer TTL」。0 = 不限制。
	MaxConnLifetime time.Duration

	// EvictWindow 是回收額度的時間窗長度。每個 cap key 在一個窗內最多回收
	// `2 × 該 key 的 limit` 次（見 evictBudgetFor）。
	EvictWindow time.Duration
}

// DefaultConfig returns sensible defaults per 設計書 §5.3 / §5.4 / §5.5。
func DefaultConfig() HubConfig {
	return HubConfig{
		RingBufferSize:    200,
		RingBufferTTL:     5 * time.Minute,
		BatchWindow:       50 * time.Millisecond,
		BatchMaxEvents:    100,
		MaxConnPerPod:     5000,
		MaxConnPerSubject: 5,
		MaxConnPerUser:    3,
		HeartbeatInterval: 30 * time.Second,
		GCInterval:        60 * time.Second,
		MaxConnLifetime:   8 * time.Minute,
		EvictWindow:       30 * time.Second,
	}
}

// evictBudgetFor 回傳某個 cap group 在一個 EvictWindow 內可回收的位子數。
//
// 取 2×limit 的理由：最壞情況下，一次頁面重新載入會發現「每一個位子都是上一
// 個頁面留下的殭屍」，因此單次復原就可能需要 limit 次回收；抓兩倍是為了讓
// 30 秒內連續重新載入兩次仍能完整復原。再高就沒有意義了——超過這個量的回收
// 需求代表使用者真的想開超過 cap 的連線，那時候回到 429 才是對的：客戶端連
// 兩次失敗就會停止重試，整體收斂到穩定狀態，而不是無止境互踢。
func evictBudgetFor(limit int) int { return 2 * limit }

// topicState holds the set of active connections for a single topic.
type topicState struct {
	conns map[*Connection]struct{}
}

// Hub is the central SSE fan-out. It tracks topics → connections, owns the
// ring buffers, and exposes Subscribe / Unsubscribe / Publish.
//
// Hub itself is business-agnostic. Channel-to-topic mapping lives in
// realtime/registry.go; routing key → topic dispatch is the consumer's job.
type Hub struct {
	cfg HubConfig
	log *zap.Logger

	mu      sync.RWMutex
	topics  map[string]*topicState
	buffers map[string]*RingBuffer
	batches map[string]*batchAggregator // per-topic 50ms batch aggregator

	// Counters maintained outside the main mu to avoid lock contention on hot path。
	totalConns     atomic.Int64
	connsBySubject sync.Map // string (rate-limit subject) → *connGroup
	connsByUID     sync.Map // int64  → *connGroup

	seqCounter atomic.Int64

	// now 是 Hub 的時鐘。全部與時間有關的判定（入座時間、最長存活時間、
	// 回收額度視窗、環形緩衝 TTL）都走它，測試才能在不真的等 8 分鐘的情況
	// 下驗證回收行為。預設 time.Now。
	now func() time.Time

	// afterPrepareHook 只給測試：publishAtomic 鎖外準備完、進入鎖內 fanout 之前呼叫，
	// 讓測試決定性地在兩段之間插入訂閱。
	afterPrepareHook func()
}

// NewHub constructs a Hub with the given config.
func NewHub(cfg HubConfig, log *zap.Logger) *Hub {
	if log == nil {
		log = zap.NewNop()
	}
	return &Hub{
		cfg:     cfg,
		log:     log,
		topics:  make(map[string]*topicState),
		buffers: make(map[string]*RingBuffer),
		batches: make(map[string]*batchAggregator),
		now:     time.Now,
	}
}

// Start launches background goroutines (ring buffer GC). Returns a shutdown func.
func (h *Hub) Start(ctx context.Context) func() {
	s := outbox.NewScheduler(h.cfg.GCInterval,
		func(_ context.Context) error {
			h.runGC()
			return nil
		},
		func(_ context.Context, r any) {
			h.log.Error("sse hub gc loop panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		nil,
	)
	s.Start(ctx)
	return s.Stop
}

// Subscribe attaches a Connection to a topic. Enforces caps; returns error
// if any cap (per-pod / per-subject / per-user) would be exceeded.
//
// On success the caller should:
//  1. write SSE `ready` event (with bufferMiss flag from buffer.Since)
//  2. replay any returned buffered events
//  3. run the write loop (stream.go::ServeSSE)
func (h *Hub) Subscribe(conn *Connection) (replay []BufferedEvent, bufferMiss bool, err error) {
	if total := h.totalConns.Load(); total >= int64(h.cfg.MaxConnPerPod) {
		return nil, false, ErrPodCapReached
	}

	conn.subscribedAt = h.now()

	// 佔位（必要時把最舊的位子回收過來，見 capgroup.go 檔頭）。
	victim, err := h.admitToCap(conn)
	if err != nil {
		return nil, false, err
	}

	h.mu.Lock()
	st, ok := h.topics[conn.Topic]
	if !ok {
		st = &topicState{conns: make(map[*Connection]struct{})}
		h.topics[conn.Topic] = st
	}
	st.conns[conn] = struct{}{}

	buf, ok := h.buffers[conn.Topic]
	if !ok {
		buf = NewRingBuffer(h.cfg.RingBufferSize, h.cfg.RingBufferTTL)
		h.buffers[conn.Topic] = buf
	}
	h.mu.Unlock()

	h.totalConns.Add(1)
	conn.admitted.Store(true)

	// 被讓位的那條在這裡才真正拆除——admit 只在 group 鎖內把它移出座位，
	// 拆除要碰 hub 主鎖，必須等 group 鎖放掉之後才做（避免鎖序反轉）。
	if victim != nil {
		h.log.Info("SSE.connection.evicted",
			zap.String("topic", victim.Topic),
			zap.String("connId", victim.ID),
			zap.Duration("age", conn.subscribedAt.Sub(victim.subscribedAt)),
			zap.String("reason", "cap_reached_oldest_reclaimed"))
		h.teardown(victim)
	}

	events, found := buf.Since(conn.LastEventID())
	bufferMiss = conn.LastEventID() != "" && !found
	return events, bufferMiss, nil
}

// Unsubscribe removes the connection from its topic and frees its cap slot.
// 對同一條連線重複呼叫是安全的 no-op（被 Hub 主動回收的連線，其服務
// goroutine 之後仍會跑到自己的 defer Unsubscribe）。
func (h *Hub) Unsubscribe(conn *Connection) {
	h.teardown(conn)
}

// teardown 是唯一的連線拆除路徑：退出 topic → 關閉 → 扣總數 → 釋放位子。
// 由 markReleased 保證每條連線恰好執行一次。
func (h *Hub) teardown(conn *Connection) {
	if !conn.markReleased() {
		return
	}

	h.mu.Lock()
	if st, ok := h.topics[conn.Topic]; ok {
		delete(st.conns, conn)
		if len(st.conns) == 0 {
			delete(h.topics, conn.Topic)
		}
	}
	h.mu.Unlock()

	conn.Close()
	h.totalConns.Add(-1)
	h.releaseCap(conn)
}

// Publish dispatches an event to all connections on a topic。
//
// Without a batch event name (channelBatchName == "")：直接寫 ring buffer +
// fanout to subscribers，全在同一個 mu.Lock 範圍內避免與 Subscribe 競態
// （否則新訂閱者可能同時 (a) 在 buf.Since 看到剛寫入的 event 並 replay
// (b) 在 fanout 看到自己並收到推送 → double-push）。
//
// With a batch event name：傳給該 topic 的 batch aggregator；aggregator 的
// timer 觸發 flushBatch，後者才寫 buffer + fanout（同樣全在 lock 內）。
//
// 41 號設計書 §5.7。
func (h *Hub) Publish(topic, eventName string, data []byte, channelBatchName string) {
	if channelBatchName == "" {
		ev := BufferedEvent{
			ID:        h.nextEventID(),
			EventName: eventName,
			Data:      data,
			Topic:     topic,
			CreatedAt: h.now(),
		}
		h.publishAtomic(topic, ev)
		return
	}

	// Batch path: aggregator timer will call flushBatch which writes buffer + fanout。
	// 把 event 暫存於 aggregator 不寫入 ring buffer — buffer 由 flushBatch 一次寫入
	// （單筆寫原 event，多筆寫 batch wrapper）。
	ev := BufferedEvent{
		ID:        h.nextEventID(),
		EventName: eventName,
		Data:      data,
		Topic:     topic,
		CreatedAt: h.now(),
	}
	agg := h.batchAggregatorFor(topic, channelBatchName)
	agg.add(ev)
}

// publishAtomic writes buf.Push + fanout in one critical section so that
// new subscribers cannot interleave between buffer-write and fanout-send。
func (h *Hub) publishAtomic(topic string, ev BufferedEvent) {
	frame := buildFrame(ev.EventName, ev.Data, ev.ID)

	// 個人化的解析與序列化放在鎖外：先在鎖內複製這個 topic 的連線，鎖外對每個 variant 只算一次；
	// 鎖內的 fanout 只做查表。兩次上鎖之間才加入的連線由 personalizedFrames.frameFor 補算（事件還沒解析過就用該連線的 Personalizer 解析，不會漏送）。
	h.mu.RLock()
	var snapshot []*Connection
	if st, ok := h.topics[topic]; ok {
		snapshot = make([]*Connection, 0, len(st.conns))
		for c := range st.conns {
			snapshot = append(snapshot, c)
		}
	}
	h.mu.RUnlock()
	pf := h.preparePersonalized(topic, ev, snapshot)
	if h.afterPrepareHook != nil {
		h.afterPrepareHook()
	}

	h.mu.Lock()
	buf, ok := h.buffers[topic]
	if !ok {
		buf = NewRingBuffer(h.cfg.RingBufferSize, h.cfg.RingBufferTTL)
		h.buffers[topic] = buf
	}
	buf.Push(ev)

	var slow []*Connection
	if st, ok := h.topics[topic]; ok {
		for c := range st.conns {
			f := frame
			if c.Personalizer != nil {
				f = pf.frameFor(c)
				if f == nil {
					continue // 個人化失敗：這條連線不送這個事件
				}
			}
			if !c.Send(f) {
				slow = append(slow, c)
			}
		}
	}
	h.mu.Unlock()

	for _, c := range slow {
		h.log.Info("SSE.connection.timeout",
			zap.String("topic", topic),
			zap.String("connId", c.ID),
			zap.String("reason", "send_buffer_full"))
		h.Unsubscribe(c)
	}
}

// personalizedFrames 是一個事件在這個 topic 上依 variant 算好的 frame。
// 只在 Hub 的 publish 路徑上由單一 goroutine 依序使用（鎖外準備、鎖內查表）。
type personalizedFrames struct {
	h        *Hub
	topic    string
	ev       BufferedEvent
	prepared PreparedEvent // nil 且 !failed = 還沒有人準備過（拍快照時 topic 沒有個人化連線）
	failed   bool          // 解析失敗：個人化的連線一律丟掉這個事件
	frames   map[string][]byte
}

// preparePersonalized 在鎖外為快照中的連線準備這個事件。快照之後才加入的連線由 frameFor 補算。
func (h *Hub) preparePersonalized(topic string, ev BufferedEvent, conns []*Connection) *personalizedFrames {
	pf := &personalizedFrames{h: h, topic: topic, ev: ev, frames: map[string][]byte{}}
	for _, c := range conns {
		if c.Personalizer != nil {
			pf.frameFor(c)
		}
	}
	return pf
}

// prepare 用這條連線的 Personalizer 解析事件（同一個 topic 的連線共用同一個 Personalizer）。
func (pf *personalizedFrames) prepare(c *Connection) {
	prepared, err := c.Personalizer(pf.ev.EventName, pf.ev.Data)
	if err != nil {
		// 只記事件名稱與 id，不記 payload（payload 含內部身分 id）。
		pf.h.log.Warn("SSE.personalize.failed",
			zap.String("topic", pf.topic), zap.String("event", pf.ev.EventName), zap.String("eventId", pf.ev.ID), zap.Error(err))
		pf.failed = true
		return
	}
	pf.prepared = prepared
}

// frameFor 回傳這條連線的 frame；個人化失敗回 nil（fail closed，不會漏送：
// 還沒準備過的事件在這裡補算）。
func (pf *personalizedFrames) frameFor(c *Connection) []byte {
	if pf.failed {
		return nil
	}
	if pf.prepared == nil {
		pf.prepare(c)
		if pf.failed {
			return nil
		}
	}
	key := pf.prepared.Variant(c.Viewer)
	if f, ok := pf.frames[key]; ok {
		return f
	}
	data, err := pf.prepared.Render(c.Viewer)
	if err != nil {
		pf.h.log.Warn("SSE.personalize.render_failed",
			zap.String("topic", pf.topic), zap.String("event", pf.ev.EventName), zap.String("eventId", pf.ev.ID), zap.Error(err))
		pf.frames[key] = nil
		return nil
	}
	f := buildFrame(pf.ev.EventName, data, pf.ev.ID)
	pf.frames[key] = f
	return f
}

// nextEventID generates an SSE event id of the form "<epochMillis>-<seq>".
func (h *Hub) nextEventID() string {
	seq := h.seqCounter.Add(1)
	return fmt.Sprintf("%d-%d", time.Now().UnixMilli(), seq)
}

// batchAggregatorFor returns (creating if absent) the per-topic batch aggregator.
func (h *Hub) batchAggregatorFor(topic, batchEventName string) *batchAggregator {
	h.mu.RLock()
	agg, ok := h.batches[topic]
	h.mu.RUnlock()
	if ok {
		return agg
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if agg, ok = h.batches[topic]; ok {
		return agg
	}
	agg = newBatchAggregator(topic, batchEventName, h.cfg.BatchWindow, h.cfg.BatchMaxEvents, h)
	h.batches[topic] = agg
	return agg
}

// flushBatch is called by batchAggregator when a window closes。
// Single-event windows write the event itself to the ring buffer (§5.7
// 「視窗內只 1 個 event 仍走原 event name」)；multi-event windows write a
// `<channel>.batch` wrapper event with id = max member id。
//
// Buffer write + fanout 在同一個 atomic critical section（避免新訂閱者在
// buffer-write 與 fanout-send 之間 race 進來造成 replay + send 重複）。
func (h *Hub) flushBatch(topic, batchEventName string, events []BufferedEvent) {
	if len(events) == 0 {
		return
	}
	if len(events) == 1 {
		h.publishAtomic(topic, events[0])
		return
	}
	merged := buildBatchData(events)
	batchEv := BufferedEvent{
		ID:        events[len(events)-1].ID, // use largest member id (§5.7)
		EventName: batchEventName,
		Data:      merged,
		Topic:     topic,
		CreatedAt: h.now(),
	}
	h.publishAtomic(topic, batchEv)
}

func (h *Hub) runGC() {
	now := h.now()

	// 1. 收掉超過最長存活時間的連線（issue #2040）。
	//
	// 這是唯一「不需要有人來敲門也會發生」的回收路徑：cap 讓位只在有新連線
	// 進來時觸發，若使用者關掉所有分頁就此不再回來，殭屍會一路佔著位子與
	// goroutine 直到中間層自己斷線。這裡讓 origin 自己有一個確定的上界。
	var expired []*Connection
	h.mu.Lock()
	if h.cfg.MaxConnLifetime > 0 {
		for _, st := range h.topics {
			for c := range st.conns {
				if now.Sub(c.subscribedAt) >= h.cfg.MaxConnLifetime {
					expired = append(expired, c)
				}
			}
		}
	}

	// 2. 環形緩衝過期清理 + 空 topic 回收。
	for topic, buf := range h.buffers {
		remaining := buf.GC(now)
		if remaining == 0 {
			if st, ok := h.topics[topic]; !ok || len(st.conns) == 0 {
				delete(h.buffers, topic)
				delete(h.topics, topic)
				delete(h.batches, topic)
			}
		}
	}
	h.mu.Unlock()

	// teardown 要拿 h.mu，必須在鎖外做。
	for _, c := range expired {
		h.log.Info("SSE.connection.expired",
			zap.String("topic", c.Topic),
			zap.String("connId", c.ID),
			zap.Duration("age", now.Sub(c.subscribedAt)),
			zap.String("reason", "max_lifetime_exceeded"))
		h.teardown(c)
	}
}

// ------------------------------------------------------------------------- //
// Per-Subject / Per-User 佔位。Keep separate from main mu to avoid contention.//
// ------------------------------------------------------------------------- //

// capKeyOf 回傳這條連線該記在哪個 cap map 的哪個 key、上限多少、超限回什麼錯。
// 登入者用 accountId、匿名用 `Connection.Subject`（handler 填入
// middleware.RateLimitSubject 的結果，不是裸 client IP —— 網頁 SSR／CI 一個位址
// 背後是無限多個人，用位址當 key 會讓 5 條連線的額度被整個網頁共用）；兩者都沒有
// 就不受 per-key cap 管（僅受 pod cap）。
func (h *Hub) capKeyOf(conn *Connection) (m *sync.Map, key any, limit int, capErr error) {
	switch {
	case conn.AccountID > 0:
		return &h.connsByUID, conn.AccountID, h.cfg.MaxConnPerUser, ErrUserCapReached
	case conn.Subject != "":
		return &h.connsBySubject, conn.Subject, h.cfg.MaxConnPerSubject, ErrSubjectCapReached
	default:
		return nil, nil, 0, nil
	}
}

// ConnCaps 回傳 per-user 與 per-subject 的連線上限，供連線數上限的 429 填
// X-RateLimit-Limit。不開放給呼叫端寫入——上限的唯一來源仍是 HubConfig。
func (h *Hub) ConnCaps() (perUser, perSubject int) {
	return h.cfg.MaxConnPerUser, h.cfg.MaxConnPerSubject
}

// admitToCap 佔一個位子。滿了就嘗試回收最舊的位子（見 capgroup.go 檔頭的
// 完整理由）；回收成功時把被讓位的連線回傳給呼叫端拆除，額度用完或無對象可
// 收才回傳 cap 錯誤。
func (h *Hub) admitToCap(conn *Connection) (*Connection, error) {
	m, key, limit, capErr := h.capKeyOf(conn)
	if m == nil {
		return nil, nil
	}

	for {
		g := h.groupFor(m, key, limit)
		res, victim := g.admit(conn, conn.subscribedAt, evictBudgetFor(limit), h.cfg.EvictWindow)
		switch res {
		case admitStaleGroup:
			// group 在我們拿到它之後被清空刪除了，重取一個再試。
			continue
		case admitRejected:
			return nil, capErr
		default:
			conn.group = g
			return victim, nil
		}
	}
}

func (h *Hub) groupFor(m *sync.Map, key any, limit int) *connGroup {
	if v, ok := m.Load(key); ok {
		return v.(*connGroup)
	}
	actual, _ := m.LoadOrStore(key, newConnGroup(limit))
	return actual.(*connGroup)
}

// releaseCap 釋放位子；group 因此清空時把 key 一併移除，避免 map 無限成長。
func (h *Hub) releaseCap(conn *Connection) {
	g := conn.group
	if g == nil {
		return
	}
	if !g.release(conn) {
		return
	}
	if m, key, _, _ := h.capKeyOf(conn); m != nil {
		m.Delete(key)
	}
}

// CapGroupSize 回傳某個 cap key 目前佔用的位子數（0 = 沒有紀錄）。
// accountID > 0 查 per-user，否則以 subject 查 per-subject。
func (h *Hub) CapGroupSize(accountID int64, subject string) int {
	var (
		v  any
		ok bool
	)
	if accountID > 0 {
		v, ok = h.connsByUID.Load(accountID)
	} else {
		v, ok = h.connsBySubject.Load(subject)
	}
	if !ok {
		return 0
	}
	return v.(*connGroup).size()
}

// ActiveConnections returns total active SSE connections (for metrics).
func (h *Hub) ActiveConnections() int64 {
	return h.totalConns.Load()
}

// Config returns the hub configuration (read-only).
func (h *Hub) Config() HubConfig {
	return h.cfg
}
