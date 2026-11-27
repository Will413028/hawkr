package live

import (
	"context"
	"errors"
	"maps"
	"math"
	"slices"
	"sync"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const maxSeq = math.MaxInt64

const messagePageSize = model.MaxPageSize

type pend struct {
	msg        bool
	minQ, maxQ int64
	vis        map[int64]struct{}
}

func (p *pend) merge(o *pend) {
	if o.msg {
		if p.msg {
			p.minQ, p.maxQ = min(p.minQ, o.minQ), max(p.maxQ, o.maxQ)
		} else {
			p.msg, p.minQ, p.maxQ = true, o.minQ, o.maxQ
		}
	}
	for q := range o.vis {
		if p.vis == nil {
			p.vis = map[int64]struct{}{}
		}
		p.vis[q] = struct{}{}
	}
}

// Subscription 是一個 Party 的一條拉取式事件流。Next 只能由單一 goroutine 呼叫；Revalidate、Close 可以並行。
type Subscription struct {
	h   *Hub
	id  uuid.UUID
	key partyKey

	mu        sync.Mutex
	party     model.Party
	delivered map[uuid.UUID]int64
	pending   map[uuid.UUID]*pend
	peer      map[uuid.UUID]int64
	overflow  bool
	dirty     bool
	since     time.Time
	ready     bool
	err       error
	stopCtx   func() bool
	wake      chan struct{}

	out []Event
}

func (s *Subscription) size() int { return len(s.pending) + len(s.peer) }

func (s *Subscription) wakeLocked() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Subscription) overflowLocked() {
	s.overflow = true
	s.pending, s.peer = map[uuid.UUID]*pend{}, map[uuid.UUID]int64{}
	s.delivered = map[uuid.UUID]int64{}
	s.wakeLocked()
}

func (s *Subscription) markMessage(c uuid.UUID, q int64, visible bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || s.overflow {
		return
	}
	p := s.pending[c]
	if p == nil {
		if s.size() >= s.h.cfg.MaxPending {
			s.overflowLocked()
			return
		}
		p = &pend{}
		s.pending[c] = p
	}
	if visible {
		if p.vis == nil {
			p.vis = map[int64]struct{}{}
		}
		p.vis[q] = struct{}{}
		if len(p.vis) > s.h.cfg.MaxPending {
			s.overflowLocked()
			return
		}
	} else {
		p.merge(&pend{msg: true, minQ: q, maxQ: q})
	}
	s.wakeLocked()
}

func (s *Subscription) markPeerRead(c uuid.UUID, q int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || s.overflow {
		return
	}
	if _, ok := s.peer[c]; !ok && s.size() >= s.h.cfg.MaxPending {
		s.overflowLocked()
		return
	}
	s.peer[c] = max(s.peer[c], q)
	s.wakeLocked()
}

func (s *Subscription) markUnread() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return
	}
	s.dirty = true
	s.wakeLocked()
}

func (s *Subscription) epoch(since time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || s.overflow {
		return
	}
	if s.since.IsZero() || since.Before(s.since) {
		s.since = since
	}
	for c, d := range s.delivered {
		p := s.pending[c]
		if p == nil {
			if s.size() >= s.h.cfg.MaxPending {
				s.overflowLocked()
				return
			}
			p = &pend{}
			s.pending[c] = p
		}
		p.merge(&pend{msg: true, minQ: d + 1, maxQ: maxSeq})
	}
	s.dirty = true
	s.wakeLocked()
}

type work struct {
	pending   map[uuid.UUID]*pend
	peer      map[uuid.UUID]int64
	overflow  bool
	dirty     bool
	since     time.Time
	needReady bool
	party     model.Party
}

func (w work) any() bool {
	return len(w.pending) > 0 || len(w.peer) > 0 || w.overflow || w.dirty || !w.since.IsZero() || w.needReady
}

func (s *Subscription) take() (work, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return work{}, s.err
	}
	w := work{pending: s.pending, peer: s.peer, overflow: s.overflow, dirty: s.dirty, since: s.since, needReady: !s.ready, party: s.party}
	s.pending, s.peer = map[uuid.UUID]*pend{}, map[uuid.UUID]int64{}
	s.overflow, s.dirty, s.since = false, false, time.Time{}
	return w, nil
}

func (s *Subscription) requeue(w work) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overflow || w.overflow {
		s.overflowLocked()
		return
	}
	for c, p := range w.pending {
		if cur := s.pending[c]; cur != nil {
			cur.merge(p)
		} else {
			s.pending[c] = p
		}
	}
	for c, q := range w.peer {
		s.peer[c] = max(s.peer[c], q)
	}
	s.dirty = s.dirty || w.dirty
	if !w.since.IsZero() && (s.since.IsZero() || w.since.Before(s.since)) {
		s.since = w.since
	}
	if s.size() > s.h.cfg.MaxPending {
		s.overflowLocked()
		return
	}
	s.wakeLocked()
}

// Next 阻塞到下一個業務事件；串流結束回 *Ended（模組結束）或 ErrClosed（自己關閉），ctx 取消回 ctx 的錯誤。
func (s *Subscription) Next(ctx context.Context) (Event, error) {
	for {
		ev, ok, err := s.poll(ctx)
		if err != nil || ok {
			return ev, err
		}
		select {
		case <-s.wake:
		case <-ctx.Done():
			return Event{}, ctx.Err()
		}
	}
}

func (s *Subscription) poll(ctx context.Context) (ev Event, ok bool, err error) {
	for {
		s.mu.Lock()
		err := s.err
		s.mu.Unlock()
		if err != nil {
			return Event{}, false, err
		}
		if len(s.out) > 0 {
			ev := s.out[0]
			s.out = s.out[1:]
			return ev, true, nil
		}
		if err := ctx.Err(); err != nil {
			return Event{}, false, err
		}
		w, err := s.take()
		if err != nil {
			return Event{}, false, err
		}
		if !w.any() {
			return Event{}, false, nil
		}
		if err := s.process(ctx, w); err != nil {
			s.requeue(w)
			return Event{}, false, err
		}
	}
}

type result struct {
	evs       []Event
	delivered map[uuid.UUID]int64
	forget    map[uuid.UUID]struct{}
	ready     bool
}

func (s *Subscription) deliveredOf(c uuid.UUID) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.delivered[c]
	return d, ok
}

func sortedConvs[V any](m map[uuid.UUID]V) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(m))
	for c := range m {
		ids = append(ids, c)
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	return ids
}

func (s *Subscription) process(ctx context.Context, w work) error {
	r := &result{delivered: map[uuid.UUID]int64{}, forget: map[uuid.UUID]struct{}{}}
	dirty := w.dirty
	if w.overflow {
		r.evs = append(r.evs, Event{Kind: EventResync, All: true})
		dirty = true
		w.pending, w.peer = nil, nil
	}
	if !w.since.IsZero() && !w.overflow {
		resync, err := s.activeResync(ctx, w, r)
		if err != nil {
			return err
		}
		dirty = dirty || resync
	}
	for _, c := range sortedConvs(w.pending) {
		fromOthers, err := s.readConversation(ctx, w.party, c, w.pending[c], r)
		if err != nil {
			return err
		}
		dirty = dirty || fromOthers
	}
	for _, c := range sortedConvs(w.peer) {
		r.evs = append(r.evs, Event{Kind: EventPeerRead, ConversationID: c, PeerReadSeq: w.peer[c]})
	}
	if w.needReady {
		if hook := s.h.AfterBackfill; hook != nil {
			hook()
		}
	}
	if w.needReady || dirty {
		total, err := s.h.reader.UnreadTotal(ctx, w.party)
		if err != nil {
			return err
		}
		kind := EventUnread
		if w.needReady {
			kind, r.ready = EventReady, true
		}
		r.evs = append(r.evs, Event{Kind: kind, Unread: total})
	}
	s.commit(r)
	return nil
}

func (s *Subscription) activeResync(ctx context.Context, w work, r *result) (bool, error) {
	limit := s.h.cfg.MaxPending
	ids, err := s.h.reader.ActiveSince(ctx, w.party, w.since, limit+1)
	if err != nil {
		return false, err
	}
	if len(ids) > limit {
		r.evs = append(r.evs, Event{Kind: EventResync, All: true})
		return true, nil
	}
	known := s.deliveredSnapshot()
	for _, c := range sortedConvs(known) {
		r.evs = append(r.evs, Event{Kind: EventResync, ConversationID: c})
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	for _, c := range ids {
		if _, ok := known[c]; ok {
			continue
		}
		r.evs = append(r.evs, Event{Kind: EventResync, ConversationID: c})
	}
	return false, nil
}

func (s *Subscription) deliveredSnapshot() map[uuid.UUID]int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.delivered)
}

func (s *Subscription) readConversation(ctx context.Context, party model.Party, c uuid.UUID, p *pend, r *result) (bool, error) {
	fromOthers := false
	d, inDelivered := s.deliveredOf(c)
	if p.msg && (!inDelivered || p.maxQ > d) {
		base := p.minQ - 1
		if inDelivered {
			base = d
		}
		count := 0
		for {
			page, err := s.h.reader.MessagesAfter(ctx, c, party, base, messagePageSize)
			if err != nil {
				if apperr.KindOf(err) == apperr.KindNotFound {
					r.forget[c] = struct{}{}
					return fromOthers, nil
				}
				return false, err
			}
			for _, m := range page.Items {
				r.evs = append(r.evs, Event{Kind: EventMessage, Message: m})
				base = m.Seq
				count++
				fromOthers = fromOthers || m.SenderKind != party.Kind
			}
			if len(page.Items) > 0 {
				r.delivered[c] = base
			}
			if !page.HasMore {
				break
			}
			if count >= s.h.cfg.MaxBacklog {
				delete(r.delivered, c)
				r.forget[c] = struct{}{}
				r.evs = append(r.evs, Event{Kind: EventResync, ConversationID: c})
				return fromOthers, nil
			}
		}
	}
	seqs := make([]int64, 0, len(p.vis))
	for q := range p.vis {
		seqs = append(seqs, q)
	}
	slices.Sort(seqs)
	for _, q := range seqs {
		page, err := s.h.reader.MessagesAfter(ctx, c, party, q-1, 1)
		if err != nil {
			if apperr.KindOf(err) == apperr.KindNotFound {
				return fromOthers, nil
			}
			return false, err
		}
		if len(page.Items) > 0 && page.Items[0].Seq == q {
			r.evs = append(r.evs, Event{Kind: EventMessage, Message: page.Items[0], Updated: true})
		}
	}
	return fromOthers, nil
}

func (s *Subscription) commit(r *result) {
	s.mu.Lock()
	for c := range r.forget {
		delete(s.delivered, c)
	}
	for c, q := range r.delivered {
		s.delivered[c] = max(s.delivered[c], q)
	}
	for c := range s.delivered {
		if len(s.delivered) <= s.h.cfg.MaxDelivered {
			break
		}
		delete(s.delivered, c)
	}
	s.ready = s.ready || r.ready
	s.mu.Unlock()
	s.out = append(s.out, r.evs...)
}

// Revalidate 由串流傳輸 helper 在重新驗證 session、重讀店家之後呼叫：以新的 Party 取代舊快照並重跑 Authorize。
// 失去讀取資格（關店）時訂閱以 LostEligibility 結束，並回傳那個 *Ended；Party 不是這條訂閱的身分回一般錯誤。
func (s *Subscription) Revalidate(ctx context.Context, p model.Party) error {
	if (partyKey{p.Kind, p.ID()}) != s.key {
		return errors.New("im：Revalidate 的 Party 與訂閱的身分不符")
	}
	s.mu.Lock()
	if s.err != nil {
		err := s.err
		s.mu.Unlock()
		return err
	}
	if p.Authorize() == nil {
		s.party = p
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	s.finish(EndLostEligibility, true)
	return &Ended{Reason: EndLostEligibility}
}

func (s *Subscription) finish(reason EndReason, deleteRow bool) {
	s.mu.Lock()
	if s.err != nil {
		s.mu.Unlock()
		return
	}
	s.err = &Ended{Reason: reason}
	s.wakeLocked()
	stop := s.stopCtx
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
	s.h.remove(s)
	if deleteRow {
		s.h.deleteRows(context.Background(), []uuid.UUID{s.id})
	}
}

// Close 結束訂閱（冪等）：移出 Hub、刪存活標記並釋出並行名額。ctx 取消與 handler 返回都會呼叫它。
func (s *Subscription) Close() {
	s.mu.Lock()
	if s.err != nil {
		s.mu.Unlock()
		return
	}
	s.err = ErrClosed
	s.wakeLocked()
	stop := s.stopCtx
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
	s.h.remove(s)
	s.h.deleteRows(context.Background(), []uuid.UUID{s.id})
}
