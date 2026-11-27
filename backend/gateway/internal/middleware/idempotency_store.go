package middleware

import (
	"container/list"
	"sync"
	"time"
)


const (
	idempotencyMaxEntries = 1024

	idempotencyMaxBodyBytes = 8 << 10

	idempotencyTTL = 10 * time.Minute
)


type idempotencyOutcome int

const (
	idemProceed idempotencyOutcome = iota
	idemReplay
	idemMismatch
	idemInFlight
)

type idempotencyRecord struct {
	scope       string
	fingerprint string
	done        bool
	status      int
	contentType string
	body        []byte
	expiresAt   time.Time
	elem        *list.Element
}

// IdempotencyStore 是有界的 in-process 冪等紀錄表（LRU ＋ TTL）。
// 零值不可用，一律走 NewIdempotencyStore。
type IdempotencyStore struct {
	mu      sync.Mutex
	entries map[string]*idempotencyRecord
	lru     *list.List
	max     int
	ttl     time.Duration
	now     func() time.Time
}

// NewIdempotencyStore 建立一個存放層。max/ttl 給 0 時套用預設值。
func NewIdempotencyStore(max int, ttl time.Duration) *IdempotencyStore {
	if max <= 0 {
		max = idempotencyMaxEntries
	}
	if ttl <= 0 {
		ttl = idempotencyTTL
	}
	return &IdempotencyStore{
		entries: make(map[string]*idempotencyRecord, max/4+1),
		lru:     list.New(),
		max:     max,
		ttl:     ttl,
		now:     time.Now,
	}
}

var defaultIdempotencyStore = NewIdempotencyStore(0, 0)

// Begin 嘗試取得 scope 的執行權。
//
// 判定順序刻意是「指紋不符 > 進行中 > 回放」：同一把鍵被綁到兩份不同內容時，
// 不論前一次跑完了沒有，正確答案都是拒絕（9350），不是叫對方稍後再試。
func (s *IdempotencyStore) Begin(scope, fingerprint string) (idempotencyOutcome, *idempotencyRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked()

	if rec, ok := s.entries[scope]; ok {
		if rec.fingerprint != fingerprint {
			return idemMismatch, nil
		}
		if !rec.done {
			return idemInFlight, nil
		}
		s.lru.MoveToFront(rec.elem)
		return idemReplay, rec
	}

	rec := &idempotencyRecord{
		scope:       scope,
		fingerprint: fingerprint,
		expiresAt:   s.now().Add(s.ttl),
	}
	rec.elem = s.lru.PushFront(rec)
	s.entries[scope] = rec
	s.evictOverflowLocked()
	return idemProceed, nil
}

// Commit 把回應存起來供之後回放。body 超過上限就當作沒跑過（刪紀錄）。
func (s *IdempotencyStore) Commit(scope string, status int, contentType string, body []byte) {
	if len(body) > idempotencyMaxBodyBytes {
		s.Release(scope)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.entries[scope]
	if !ok {
		return
	}
	stored := make([]byte, len(body))
	copy(stored, body)
	rec.done = true
	rec.status = status
	rec.contentType = contentType
	rec.body = stored
	rec.expiresAt = s.now().Add(s.ttl)
	s.lru.MoveToFront(rec.elem)
}

// Release 放掉執行權且不留紀錄：用在「這次沒有成功」——沒有留下值得回放的結果，
// 同一把鍵重送必須真的重跑，否則就是把一次失敗永久釘在那把鍵上。
func (s *IdempotencyStore) Release(scope string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.entries[scope]; ok {
		s.removeLocked(rec)
	}
}

// Len 回傳目前紀錄數，供測試斷言容量行為。
func (s *IdempotencyStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *IdempotencyStore) removeLocked(rec *idempotencyRecord) {
	s.lru.Remove(rec.elem)
	delete(s.entries, rec.scope)
}

func (s *IdempotencyStore) evictExpiredLocked() {
	now := s.now()
	for e := s.lru.Back(); e != nil; {
		prev := e.Prev()
		rec := e.Value.(*idempotencyRecord)
		if rec.done && now.After(rec.expiresAt) {
			s.removeLocked(rec)
		}
		e = prev
	}
}

func (s *IdempotencyStore) evictOverflowLocked() {
	for len(s.entries) > s.max {
		evicted := false
		for e := s.lru.Back(); e != nil; e = e.Prev() {
			rec := e.Value.(*idempotencyRecord)
			if !rec.done {
				continue
			}
			s.removeLocked(rec)
			evicted = true
			break
		}
		if !evicted {
			return
		}
	}
}
