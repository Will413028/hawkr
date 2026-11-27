package live

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Reader 是訂閱者讀資料的唯一路徑：與 unary 讀取同一個授權與內容規則。
type Reader interface {
	// MessagesAfter 是對話裡 seq 大於 afterSeq 的訊息（舊的在前）；對話不屬於 p 回 NotFound。
	MessagesAfter(ctx context.Context, conversationID uuid.UUID, p model.Party, afterSeq int64, size int32) (model.MessagePage, error)
	UnreadTotal(ctx context.Context, p model.Party) (int64, error)
	// ActiveSince 是 p 這一方在 since（資料庫時間）之後有新訊息的對話，最多 limit 個。
	ActiveSince(ctx context.Context, p model.Party, since time.Time, limit int) ([]uuid.UUID, error)
}

// Config 是即時通道的設定；零值欄位用預設。這些常數都是估計值，不是量測值。
type Config struct {
	MaxStreamsPerParty int           // 每個 Party 全域並行訂閱上限，預設 10
	MaxCursors         int           // Subscribe 的游標上限，預設 50
	MaxBacklog         int           // 每個對話一次最多交出的訊息數，預設 200
	MaxPending         int           // 待處理訊號的對話數上限，預設 500
	MaxDelivered       int           // 每個訂閱記住的游標數上限，預設 1000
	PresenceTTL        time.Duration // 存活標記的有效期，預設 60 秒
	PresenceBeat       time.Duration // 延長批次的間隔，預設 20 秒
	AliveEvery         time.Duration // 監聽連線的存活確認間隔，預設 10 秒
	BackoffBase        time.Duration // 重連退避起點，預設 0.5 秒
	BackoffMax         time.Duration // 重連退避上限，預設 30 秒

	// NewTicker 建立週期訊號；測試注入以不靠真的時間。預設 time.NewTicker。
	NewTicker func(d time.Duration) (<-chan time.Time, func())
	// Sleep 等待 d 或 ctx 結束；測試注入。
	Sleep func(ctx context.Context, d time.Duration) error
	// Rand 回傳 [0,1)；測試注入。
	Rand func() float64
	// NewID 產生串流 ID（UUIDv7）。
	NewID  func() uuid.UUID
	Logger *slog.Logger
}

func (c Config) withDefaults() Config {
	def := func(v *int, d int) {
		if *v <= 0 {
			*v = d
		}
	}
	defD := func(v *time.Duration, d time.Duration) {
		if *v <= 0 {
			*v = d
		}
	}
	def(&c.MaxStreamsPerParty, 10)
	def(&c.MaxCursors, 50)
	def(&c.MaxBacklog, 200)
	def(&c.MaxPending, 500)
	def(&c.MaxDelivered, 1000)
	defD(&c.PresenceTTL, 60*time.Second)
	defD(&c.PresenceBeat, 20*time.Second)
	defD(&c.AliveEvery, 10*time.Second)
	defD(&c.BackoffBase, 500*time.Millisecond)
	defD(&c.BackoffMax, 30*time.Second)
	if c.NewTicker == nil {
		c.NewTicker = func(d time.Duration) (<-chan time.Time, func()) {
			t := time.NewTicker(d)
			return t.C, t.Stop
		}
	}
	if c.Sleep == nil {
		c.Sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	if c.Rand == nil {
		c.Rand = rand.Float64
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return c
}

type partyKey struct {
	kind model.PartyKind
	id   int64
}

// Hub 是進程內「Party → 訂閱者」的表與扇出。
type Hub struct {
	reader  Reader
	streams Streams
	cfg     Config

	mu   sync.RWMutex
	subs map[partyKey]map[*Subscription]struct{}
	down bool

	dropped  atomic.Int64
	received atomic.Int64

	// AfterRegister、AfterBackfill 是測試縫：前者在 Subscribe 登記之後、補洞讀取之前，後者在第一輪補洞讀取之後、Ready 之前。
	AfterRegister func()
	AfterBackfill func()
}

// NewHub 建立 Hub。
func NewHub(r Reader, s Streams, cfg Config) *Hub {
	return &Hub{reader: r, streams: s, cfg: cfg.withDefaults(), subs: map[partyKey]map[*Subscription]struct{}{}}
}

// Received 是收到的通知總數（含被丟棄的）。
func (h *Hub) Received() int64 { return h.received.Load() }

// Dropped 是被丟棄（非 JSON、版本不認得、缺欄位）的通知數。
func (h *Hub) Dropped() int64 { return h.dropped.Load() }

// Subscribe 為 p 開一條訂閱。順序固定：寫存活標記（同時執行全域並行上限）→ 登記進 Hub → 補洞。登記先於補洞，
// 兩者之間 commit 的訊息要嘛被補洞讀到，要嘛以通知處理，不漏也不重複。
func (h *Hub) Subscribe(ctx context.Context, p model.Party, cursors map[uuid.UUID]int64) (*Subscription, error) {
	if err := p.Authorize(); err != nil {
		return nil, err
	}
	if len(cursors) > h.cfg.MaxCursors {
		return nil, apperr.InvalidInputOn([]string{"Cursors"}, "%s", text.ImTooManyCursors(uitext.LocaleFrom(ctx)))
	}
	h.mu.RLock()
	down := h.down
	h.mu.RUnlock()
	if down {
		return nil, &Ended{Reason: EndShutdown}
	}
	id := h.cfg.NewID()
	if err := h.streams.Open(ctx, p.Kind, p.ID(), id, h.cfg.MaxStreamsPerParty, h.cfg.PresenceTTL); err != nil {
		return nil, err
	}
	s := &Subscription{
		h: h, id: id, key: partyKey{p.Kind, p.ID()}, party: p,
		delivered: map[uuid.UUID]int64{}, pending: map[uuid.UUID]*pend{}, peer: map[uuid.UUID]int64{},
		wake: make(chan struct{}, 1),
	}
	for c, q := range cursors {
		q = max(q, 0)
		s.delivered[c] = q
		s.pending[c] = &pend{msg: true, minQ: q + 1, maxQ: maxSeq}
	}
	h.mu.Lock()
	if h.down {
		h.mu.Unlock()
		h.deleteRows(context.WithoutCancel(ctx), []uuid.UUID{id})
		return nil, &Ended{Reason: EndShutdown}
	}
	s.mu.Lock()
	s.stopCtx = context.AfterFunc(ctx, s.Close)
	s.mu.Unlock()
	if h.subs[s.key] == nil {
		h.subs[s.key] = map[*Subscription]struct{}{}
	}
	h.subs[s.key][s] = struct{}{}
	h.mu.Unlock()
	if h.AfterRegister != nil {
		h.AfterRegister()
	}
	return s, nil
}

func (h *Hub) remove(s *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.subs[s.key]; m != nil {
		delete(m, s)
		if len(m) == 0 {
			delete(h.subs, s.key)
		}
	}
}

func (h *Hub) deleteRows(ctx context.Context, ids []uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := h.streams.Delete(ctx, ids); err != nil {
		h.cfg.Logger.Error("im：刪除串流存活標記失敗", "error", err, "streams", len(ids))
	}
}

func (h *Hub) all() []*Subscription {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*Subscription
	for _, m := range h.subs {
		for s := range m {
			out = append(out, s)
		}
	}
	return out
}

func (h *Hub) endAll(reason EndReason) {
	subs := h.all()
	if len(subs) == 0 {
		return
	}
	ids := make([]uuid.UUID, len(subs))
	for i, s := range subs {
		ids[i] = s.id
	}
	h.deleteRows(context.Background(), ids)
	for _, s := range subs {
		s.finish(reason, false)
	}
}

func (h *Hub) shutdown() {
	h.mu.Lock()
	h.down = true
	h.mu.Unlock()
	h.endAll(EndShutdown)
}

func (h *Hub) dispatch(raw string) {
	h.received.Add(1)
	p, conv, err := Decode(raw)
	if err != nil {
		h.dropped.Add(1)
		h.cfg.Logger.Warn("im：丟棄不合法的通知", "error", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	each := func(k partyKey, f func(*Subscription)) {
		for s := range h.subs[k] {
			f(s)
		}
	}
	buyer, store := partyKey{model.PartyBuyer, p.Buyer}, partyKey{model.PartyStore, p.Store}
	switch p.Kind {
	case KindMessage, KindVisible:
		visible := p.Kind == KindVisible
		f := func(s *Subscription) { s.markMessage(conv, p.Seq, visible) }
		each(buyer, f)
		each(store, f)
	case KindRead:
		own, peer := buyer, store
		if p.Side == SideStore {
			own, peer = store, buyer
		}
		each(own, func(s *Subscription) { s.markUnread() })
		each(peer, func(s *Subscription) { s.markPeerRead(conv, p.Seq) })
	case KindReadBatch:
		k := buyer
		if p.Buyer <= 0 {
			k = store
		}
		each(k, func(s *Subscription) { s.markUnread() })
	}
}

func (h *Hub) epoch(lastOK time.Time) {
	since := lastOK.Add(-model.RecoveryMargin)
	for _, s := range h.all() {
		s.epoch(since)
	}
}

// Run 持有監聽連線與存活標記的延長批次，直到 ctx 結束；結束時所有訂閱以 Shutdown 結束。
func (h *Hub) Run(ctx context.Context, dial Dial) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.beatLoop(ctx)
	}()
	h.listenLoop(ctx, dial)
	wg.Wait()
	h.shutdown()
	return nil
}

func (h *Hub) beatLoop(ctx context.Context) {
	tick, stop := h.cfg.NewTicker(h.cfg.PresenceBeat)
	defer stop()
	fails := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
		}
		subs := h.all()
		if len(subs) == 0 {
			fails = 0
			continue
		}
		streams := make([]Stream, len(subs))
		for i, s := range subs {
			streams[i] = Stream{ID: s.id, Kind: s.key.kind, PartyID: s.key.id}
		}
		if err := h.streams.Extend(ctx, streams, h.cfg.PresenceTTL); err != nil {
			if ctx.Err() != nil {
				return
			}
			fails++
			h.cfg.Logger.Error("im：延長串流存活標記失敗", "error", err, "consecutive", fails)
			if fails >= 2 {
				h.endAll(EndShutdown)
				fails = 0
			}
			continue
		}
		fails = 0
	}
}
