// Package stream 是 WatchNotifications 的訊號 hub：依 (帳號, 受眾) 訂閱，每個帳號同時開著的串流有上限（上限是每個 serve 行程各自的）。
// 寫入端（worker、其他模組）在自己的 transaction 內 pg_notify，每個 serve 行程以一條 LISTEN 連線（Hub.Run）收到後扇出給本行程的訂閱者；
// 內容（未讀數、最新一則）由訂閱者的 Next 在 serve 端讀取。hub 只傳「該重讀了」的訊號，不存內容、沒有續傳：訊號可以合併，client 以列表為準。
package stream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// MaxPerAccount 是每個帳號同時開著的串流上限（估計值：同一個瀏覽器的幾個分頁加兩支 App，不是量測值）。
const MaxPerAccount = 8

// retryAfter 是超過上限時給 client 的重試建議。
const retryAfter = 5 * time.Second

// MaxNewItemIDs 是一個訂閱者在兩則訊號之間累積的新通知 ID 上限（估計值：夠一次 toast 一批，不是量測值）；超過就標 truncated、不再帶 ID。
const MaxNewItemIDs = 20

// SellerChange 是賣家工作區哪一類資料變了。
type SellerChange int

const (
	// SellerOrder 是訂單（含物流與付款狀態）。
	SellerOrder SellerChange = iota + 1
	// SellerAfterSale 是售後。
	SellerAfterSale
)

func (c SellerChange) valid() bool { return c == SellerOrder || c == SellerAfterSale }

// Key 是訂閱的身分：帳號與受眾缺一不可，兩個受眾的收件匣互不可見。
type Key struct {
	AccountID int64
	Audience  model.Audience
}

// Inbox 是收件匣有變化的訊號內容。
type Inbox struct {
	UnreadCount int32
	// LatestItemID 是最新一則通知的 ID；收件匣是空的為 nil。
	LatestItemID *uuid.UUID
	// Hints 是這次訊號合併的通知所帶的狀態重讀提示的聯集（不重複、不含空值）；連線建立時的第一則為空。
	Hints []model.StateHint
	// NewItemIDs 是上一則訊號之後新建或聚合更新的收件匣列 ID（依到達順序、不重複）；連線建立的第一則、只有已讀或刪除變化時為空。
	// 每個 ID 在一條串流上至多送達一次，斷線期間的缺口不補送。
	NewItemIDs []uuid.UUID
	// NewItemsTruncated 為 true 表示累積的新通知超過 MaxNewItemIDs：NewItemIDs 為空，client 重讀列表、不 toast。
	NewItemsTruncated bool
}

// Event 是串流的一則訊號：Inbox 與 Workspace 恰有一個有值（Workspace 為零表示是 Inbox）。
type Event struct {
	Inbox     *Inbox
	Workspace SellerChange
}

// Hub 是訊號 hub。
type Hub struct {
	read  func(ctx context.Context, key Key, storeID int64) (Inbox, error)
	limit int
	log   *slog.Logger

	backoffBase, backoffMax time.Duration
	jitter                  func() float64
	// now 與 sleep 讓測試不用真的等；sleep 為 nil 是可被 ctx 取消的真實等待；now 為 nil（模組不得呼叫 time.Now，沒有注入時鐘可用）時連線一律不算穩定，退避只遞增。
	now        func() time.Time
	sleep      func(ctx context.Context, d time.Duration)
	listening  chan struct{}
	listenOnce sync.Once

	mu        sync.Mutex
	down      bool
	byKey     map[Key]map[*Subscription]struct{}
	byStore   map[int64]map[*Subscription]struct{}
	byAccount map[int64]int
}

// NewHub 建立 hub；read 讀訊號的內容（未讀數與最新一則，由模組根以 app 用例提供）；now 是模組注入的時鐘（LISTEN 連線穩定多久的判斷用）。
func NewHub(read func(ctx context.Context, key Key, storeID int64) (Inbox, error), log *slog.Logger, now func() time.Time) *Hub {
	h := newHub(read, MaxPerAccount)
	h.log = log
	h.now = now
	return h
}

func newHub(read func(context.Context, Key, int64) (Inbox, error), limit int) *Hub {
	return &Hub{
		read: read, limit: limit, listening: make(chan struct{}),
		byKey: map[Key]map[*Subscription]struct{}{}, byStore: map[int64]map[*Subscription]struct{}{}, byAccount: map[int64]int{},
	}
}

// Subscribe 登記一條串流。STORE 受眾必須帶 storeID（賣家工作區事件與收件匣都以店為單位），BUYER 受眾 storeID 為 0；
// 帳號同時開著的串流（不分受眾）達到上限回 resource_exhausted。第一次 Next 回連線建立時的 inbox 訊號。
func (h *Hub) Subscribe(key Key, storeID int64) (*Subscription, error) {
	switch {
	case key.AccountID <= 0:
		return nil, fmt.Errorf("notification/stream：帳號 %d 不合法", key.AccountID)
	case key.Audience == model.AudienceStore && storeID <= 0:
		return nil, fmt.Errorf("notification/stream：STORE 受眾需要店家")
	case key.Audience == model.AudienceBuyer && storeID != 0:
		return nil, fmt.Errorf("notification/stream：BUYER 受眾不帶店家")
	case key.Audience != model.AudienceBuyer && key.Audience != model.AudienceStore:
		return nil, fmt.Errorf("notification/stream：受眾 %q 不合法", key.Audience)
	}
	s := &Subscription{hub: h, key: key, storeID: storeID, wake: make(chan struct{}, 1), first: true}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.down {
		return nil, ErrClosed
	}
	if h.byAccount[key.AccountID] >= h.limit {
		return nil, apperr.ResourceExhausted(retryAfter, "同時開著的通知串流太多，請關掉不用的分頁後再試")
	}
	h.byAccount[key.AccountID]++
	add(h.byKey, key, s)
	if storeID > 0 {
		add(h.byStore, storeID, s)
	}
	return s, nil
}

func add[K comparable](m map[K]map[*Subscription]struct{}, k K, s *Subscription) {
	set := m[k]
	if set == nil {
		set = map[*Subscription]struct{}{}
		m[k] = set
	}
	set[s] = struct{}{}
}

func remove[K comparable](m map[K]map[*Subscription]struct{}, k K, s *Subscription) {
	set := m[k]
	delete(set, s)
	if len(set) == 0 {
		delete(m, k)
	}
}

// InboxChanged 把收件匣通知扇出給本行程 (帳號, 受眾) 的訂閱者（由 LISTEN 收到的通知觸發，不直接由寫入端呼叫）。
// hint 是這則通知的狀態重讀提示，空值不加提示。STORE 受眾只送給訂閱同一家店的串流。
// itemID 是新建或聚合更新的收件匣列 ID，uuid.Nil() 表示沒有新通知（已讀、刪除）。
func (h *Hub) InboxChanged(key Key, storeID int64, hint model.StateHint, itemID uuid.UUID) {
	h.mu.Lock()
	subs := make([]*Subscription, 0, len(h.byKey[key]))
	for s := range h.byKey[key] {
		if s.storeID == storeID {
			subs = append(subs, s)
		}
	}
	h.mu.Unlock()
	for _, s := range subs {
		s.mark(func() {
			s.inboxDirty = true
			if hint != model.StateHintNone && !slices.Contains(s.hints, hint) {
				s.hints = append(s.hints, hint)
			}
			s.addItem(itemID)
		})
	}
}

// workspaceChanged 把賣家工作區通知扇出給本行程這家店的 STORE 受眾訂閱者。
func (h *Hub) workspaceChanged(storeID int64, change SellerChange) {
	h.mu.Lock()
	subs := make([]*Subscription, 0, len(h.byStore[storeID]))
	for s := range h.byStore[storeID] {
		subs = append(subs, s)
	}
	h.mu.Unlock()
	for _, s := range subs {
		s.mark(func() {
			if !slices.Contains(s.changes, change) {
				s.changes = append(s.changes, change)
			}
		})
	}
}

// Subscription 是一條串流的訂閱；Next 只能由一個 goroutine 呼叫。
type Subscription struct {
	hub     *Hub
	key     Key
	storeID int64
	wake    chan struct{}

	mu         sync.Mutex
	closed     bool
	first      bool
	inboxDirty bool
	hints      []model.StateHint
	newItems   []uuid.UUID
	truncated  bool
	changes    []SellerChange
}

// addItem 把新通知 ID 併進累積的集合（呼叫端持有 s.mu）：超過上限就改標 truncated 並丟掉已累積的 ID。
func (s *Subscription) addItem(id uuid.UUID) {
	if id == uuid.Nil() || s.truncated || slices.Contains(s.newItems, id) {
		return
	}
	if len(s.newItems) >= MaxNewItemIDs {
		s.truncated = true
		s.newItems = nil
		return
	}
	s.newItems = append(s.newItems, id)
}

// ErrClosed 是訂閱已關閉。
var ErrClosed = errors.New("notification/stream：訂閱已關閉")

func (s *Subscription) mark(update func()) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	update()
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Next 阻塞到下一則訊號：第一次立刻回 inbox 訊號（無提示），之後 inbox 訊號先於賣家工作區訊號；同類訊號合併。ctx 取消或訂閱關閉時返回錯誤。
func (s *Subscription) Next(ctx context.Context) (Event, error) {
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return Event{}, ErrClosed
		}
		switch {
		case s.first || s.inboxDirty:
			s.first, s.inboxDirty = false, false
			hints, items, truncated := s.hints, s.newItems, s.truncated
			s.hints, s.newItems, s.truncated = nil, nil, false
			s.mu.Unlock()
			in, err := s.hub.read(ctx, s.key, s.storeID)
			if err != nil {
				s.restoreInbox(hints, items, truncated)
				return Event{}, err
			}
			in.Hints = hints
			in.NewItemIDs, in.NewItemsTruncated = items, truncated
			return Event{Inbox: &in}, nil
		case len(s.changes) > 0:
			c := s.changes[0]
			s.changes = s.changes[1:]
			s.mu.Unlock()
			return Event{Workspace: c}, nil
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return Event{}, ctx.Err()
		case <-s.wake:
		}
	}
}

// restoreInbox 在讀取內容失敗時把已取走的收件匣標記放回去：下一次 Next 重讀，這次合併的提示與新通知 ID 不遺失（與期間新來的取聯集，合計超過上限就標 truncated）。
func (s *Subscription) restoreInbox(hints []model.StateHint, items []uuid.UUID, truncated bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inboxDirty = true
	for _, h := range hints {
		if !slices.Contains(s.hints, h) {
			s.hints = append(s.hints, h)
		}
	}
	// 取走的 ID 排在期間新來的前面，維持到達順序。
	pending := s.newItems
	s.newItems = nil
	if truncated {
		s.truncated = true
	}
	for _, id := range append(slices.Clone(items), pending...) {
		s.addItem(id)
	}
}

// Close 釋出訂閱（可重複呼叫）；之後的 Next 回 ErrClosed。
func (s *Subscription) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	h := s.hub
	h.mu.Lock()
	remove(h.byKey, s.key, s)
	if s.storeID > 0 {
		remove(h.byStore, s.storeID, s)
	}
	if h.byAccount[s.key.AccountID]--; h.byAccount[s.key.AccountID] <= 0 {
		delete(h.byAccount, s.key.AccountID)
	}
	h.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
