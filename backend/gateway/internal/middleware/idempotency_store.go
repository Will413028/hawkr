package middleware

import (
	"container/list"
	"sync"
	"time"
)

// 本檔是 `Idempotency-Key` 的**重放紀錄存放層**（58 號 §6.1.0(10) 的三態語意）。
//
// 🔴 **邊界先寫在這裡，因為它是本機制唯一擋不住的東西**（`.claude/rules/06-no-small-tails.md`
// §3.10：一道誠實標明邊界的 gate 是資產，一道被當成牆的 gate 是負債）：
//
//	① **紀錄只活在單一 gateway process 內**。prod gateway `replicas: 2`
//	   （`k8s/values/overlays/prod/gateway.yaml`）⇒ 重送若被 Service 轉到另一顆 pod，
//	   這裡查不到紀錄、請求會照常打到下游 —— 也就是**退化成本機制上線前的行為**，
//	   不會更糟，但也擋不住。dev 是 1 顆 ⇒ dev 上是完整的。
//	② 因此本層是 **best-effort**，不是耐久保證。耐久的那一份必須與寫入在同一個
//	   transaction 內，也就是 invoice-service 的 DB（該服務目前**沒有**任何以冪等鍵為
//	   索引的欄位或表：`services/invoice-service/db/queries/invoice_documents.sql` 的
//	   註解自己寫著「invoice_documents 沒有 idempotency_key 欄」）。
//	③ 不放 Redis：`.claude/rules/10-engineering-contracts.md` §28 明文禁止把冪等紀錄
//	   放進與無成長上限的快取共用同一個 `allkeys-*` 逐出預算的實例，而現況 Redis
//	   正是該結構（`docs/速查參考/Redis 快取管理.md` §5.1／§5.6 第 4 項：要隔離必須
//	   另開實例，需架構裁決）。
//
// ⚠️ **紀錄裡有 PII**：回放需要留住回應本體，而發票這組回應含載具明碼、買受人統編與
// 抬頭。它只存在**單一 process 的記憶體**——不落磁碟、不進 log、不進任何共享快取
// （`PrivateNoStore` 管的是共享快取，與這裡正交），並在 idempotencyTTL 後移除。
// 調高 TTL 等於延長這份 PII 的駐留時間，改之前先想清楚換到了什麼。
//
// **不需要租約（§29）**：紀錄與請求 goroutine 同生命週期，執行權一律由 `defer` 歸還；
// process 崩潰時記憶體一起消失 ⇒ 不存在「持有者死了但旗標還在」的卡死狀態。這是
// in-process 存放層唯一比外部儲存好的地方，不要在移到外部儲存時把這個假設一起帶走。

const (
	// idempotencyMaxEntries 是單一 process 的紀錄上限。
	// 上限存在的理由是記憶體預算而不是正確性：滿了就從最舊的開始丟，丟掉的那一把
	// 之後重送會退化成「沒有冪等」（＝本機制上線前的行為），不會回錯的結果。
	idempotencyMaxEntries = 1024

	// idempotencyMaxBodyBytes 是「可以被重放的回應大小」上限。
	// 超過就不留紀錄（下次重送重新執行），而不是留一份被截斷的 body —— 回放半截
	// JSON 比不回放危險得多。發票這組端點的回應都是單一物件（profileId／jobId／
	// documentId 等），8 KiB 是寬鬆上界。
	idempotencyMaxBodyBytes = 8 << 10

	// idempotencyTTL 是完成紀錄的存活時間。
	// 涵蓋的是「使用者連點」「App 送出後網路逾時重試」這兩種真實重送窗口。
	//
	// 🔴 **刻意取 10 分鐘而不是 1 小時**：這個值同時是兩件壞事的窗口長度，
	// 而它要涵蓋的重送行為只發生在數秒到數分鐘內 —— 拉長換不到任何東西。
	//   ① 上面那段標明的 PII 駐留時間（回放需要留住含載具明碼與買受人資料的回應本體）
	//   ② **授權過期窗口**：回放發生在賣家 capability guard 之前，所以在這段時間內，
	//      一個剛被停權的賣家仍可用同一把鍵取回停權前那次的回應。
	//      沒有新的副作用、拿到的也是他自己的資料，但那是一段真實的陳舊授權。
	// 兩者都與 TTL 等長 ⇒ 調高它之前，先想清楚換到了什麼。
	idempotencyTTL = 10 * time.Minute
)

// 記憶體預算：1024 × (8 KiB body ＋ 約 300 B 鍵與後設) ≈ 8.3 MiB 上界，
// 對 gateway 的 `limits.memory: 512Mi`（`k8s/values/gateway.yaml`）是 1.6%。

// idempotencyOutcome 是 Begin 的四態。
type idempotencyOutcome int

const (
	// idemProceed：本次請求取得執行權，跑完必須 Commit 或 Release。
	idemProceed idempotencyOutcome = iota
	// idemReplay：同鍵同內容且已完成 ⇒ 回放上次的回應。
	idemReplay
	// idemMismatch：同鍵但請求內容不同 ⇒ 9350，且**不得**執行。
	idemMismatch
	// idemInFlight：同鍵同內容但上一次還在處理中 ⇒ 9351。
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
	lru     *list.List // front = 最新
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

// defaultIdempotencyStore 是 RequireIdempotencyKey() 不帶參數時用的那一份。
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
		// 執行期間被 LRU 擠掉了 —— 下次重送重新執行，與沒有本機制時相同。
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
		// 進行中的紀錄不看 TTL：它的歸還由持有者的 defer 負責，時間到就搶走等於
		// 讓兩個請求同時在跑，正是本機制要防的事。
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
				continue // 進行中的不能丟，理由同上
			}
			s.removeLocked(rec)
			evicted = true
			break
		}
		if !evicted {
			// 全部都在進行中 ⇒ 由併發數自然封頂，不強制丟。
			return
		}
	}
}
