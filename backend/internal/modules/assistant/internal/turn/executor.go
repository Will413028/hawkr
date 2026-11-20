// Package turn 執行買家助理的一輪對話：受理後立刻回輪次 ID，agent 在背景跑，事件放記憶體的環形緩衝（A9-D12）；
// 同一個對話同時只允許一輪進行中（A9-D14）。
//
// 這個套件不開 transaction，也不接受已在 transaction 內的 ctx：agent 執行包含 LLM 呼叫（backend/AGENTS.md 規則 5），
// session 的寫入由 session.Service 自己在最上層開 transaction（adksession 在外層 transaction 內回 ErrInOuterTx）。
package turn

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/adk/v2/agent"
	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/platform"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	// DefaultBufferCapacity 是每輪事件數的上限（含終止事件）。緩衝只增不減，不覆蓋舊事件；寫滿時這一輪以失敗結束。
	// 一則回覆的 text 增量通常是逐 token 一個事件，4096 留給長回覆的餘裕。
	DefaultBufferCapacity = 4096
	// DefaultRetention 是一輪結束後，輪次與它的事件還能被訂閱的時間。
	DefaultRetention = 10 * time.Minute
	// DefaultTimeout 是一輪從開始到結束的上限；到了就以失敗結束，釋放對話的佔用。
	DefaultTimeout = 2 * time.Minute

	minBufferCapacity = 2
)

// Config 是執行器的依賴與參數；Now、Sessions、Model、Orders 必填，其餘零值取預設。
type Config struct {
	Sessions session.Service
	Model    adkmodel.LLM
	Orders   buyeragent.Orders

	Now            func() time.Time
	BufferCapacity int
	Retention      time.Duration
	Timeout        time.Duration

	// ToolObserver 在每次 tool 呼叫結束時被呼叫（含模型送的參數的拷貝），可為 nil。參數不進對外事件（A9-D13 修訂）；
	// 這個觀察點給測試的 characterization 比對與日後的指標。可能從多個 goroutine 同時呼叫。
	ToolObserver func(buyeragent.ToolCall)
}

// SubmitInput 是送出一則訊息。
type SubmitInput struct {
	BuyerID   int64
	Topic     model.Topic
	SessionID string // 空字串表示開新的對話
	Message   string
	ImageURL  string // 空字串表示沒有圖
	// RequestID 是冪等鍵（client_request_id）；空字串表示不做冪等。範圍是同一位買家，見 Submit。
	RequestID string
}

// Ref 是受理後的結果。
type Ref struct {
	TurnID    string
	SessionID string
}

type turnState struct {
	buyerID int64
	buf     *buffer
}

// Executor 是輪次執行器。
type Executor struct {
	cfg Config

	base   context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	closed bool
	turns  map[string]*turnState
	active map[string]struct{} // A9-D14：進行中的對話，鍵是 buyerID/sessionID
	idem   map[string]*idemEntry
}

// idemEntry 是一個冪等鍵的紀錄：請求內容的雜湊與第一次受理的結果。ready 關閉後 ok、ref 才可讀；
// ok 為 false 表示第一次受理失敗（紀錄已移除，等待者自己重試）。
type idemEntry struct {
	hash  [sha256.Size]byte
	ready chan struct{}
	ok    bool
	ref   Ref
}

// New 建立執行器。
func New(cfg Config) *Executor {
	if cfg.Now == nil || cfg.Sessions == nil || cfg.Model == nil || cfg.Orders == nil {
		panic("turn：Config 的 Now、Sessions、Model、Orders 都必填")
	}
	if cfg.BufferCapacity <= 0 {
		cfg.BufferCapacity = DefaultBufferCapacity
	}
	// 緩衝最後一格留給終止事件，所以至少要有一格一般事件加一格終止事件。
	cfg.BufferCapacity = max(cfg.BufferCapacity, minBufferCapacity)
	if cfg.Retention <= 0 {
		cfg.Retention = DefaultRetention
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	base, cancel := context.WithCancel(context.Background())
	return &Executor{cfg: cfg, base: base, cancel: cancel, turns: map[string]*turnState{}, active: map[string]struct{}{}, idem: map[string]*idemEntry{}}
}

// Shutdown 取消所有進行中的輪次（它們以失敗結束）並等它們收尾，或等到 ctx 結束。
func (e *Executor) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	e.cancel()
	done := make(chan struct{})
	go func() { e.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func guardKey(buyerID int64, sessionID string) string {
	return strconv.FormatInt(buyerID, 10) + "/" + sessionID
}

func (e *Executor) sweepLocked(now time.Time) {
	for id, t := range e.turns {
		if end, ok := t.buf.endTime(); ok && now.Sub(end) > e.cfg.Retention {
			delete(e.turns, id)
		}
	}
	for k, ent := range e.idem {
		if ent.ok && e.turns[ent.ref.TurnID] == nil {
			delete(e.idem, k)
		}
	}
}

// register 登記一輪並回它的 ID：隨機 UUID（ADK 的 platform.NewUUID），輪次 ID 同時要買家相符才能訂閱，不是憑證。
func (e *Executor) register(ctx context.Context, buyerID int64) (string, *turnState) {
	t := &turnState{buyerID: buyerID, buf: newBuffer(e.cfg.BufferCapacity)}
	return e.install(ctx, t), t
}

// install 登記一輪並回輪次 ID。
func (e *Executor) install(ctx context.Context, t *turnState) string {
	id := platform.NewUUID(ctx)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sweepLocked(e.cfg.Now())
	e.turns[id] = t
	return id
}

// Submit 受理一則訊息：驗證、解析（或建立）對話、佔用對話，然後在背景跑 agent，立刻回輪次 ID。
//
// 冪等（A9-D17 修訂）：in.RequestID 非空時，鍵是（買家，RequestID）。同一個鍵、同樣的內容再送，原樣回第一次的
// Ref（不開新的一輪，也不看對話是否已有一輪進行中）；同一個鍵、內容不同回 ErrIdempotencyMismatch。紀錄與該輪
// 的保留期一起存活、一起清除（輪次被清掉後鍵就失效）。第一次受理失敗的不留紀錄，重送當作新請求；
// 第一次還在受理中時，重送等它的結果。
func (e *Executor) Submit(ctx context.Context, in SubmitInput) (Ref, error) {
	if _, ok := db.TxFrom(ctx); ok {
		return Ref{}, model.ErrOuterTx
	}
	if in.RequestID == "" {
		return e.submit(ctx, in)
	}
	key := strconv.FormatInt(in.BuyerID, 10) + "/" + in.RequestID
	hash := inputHash(in)
	for {
		e.mu.Lock()
		e.sweepLocked(e.cfg.Now())
		ent, found := e.idem[key]
		if !found {
			ent = &idemEntry{hash: hash, ready: make(chan struct{})}
			e.idem[key] = ent
			e.mu.Unlock()
			return e.submitOwning(ctx, in, key, ent)
		}
		e.mu.Unlock()
		if ent.hash != hash {
			return Ref{}, model.ErrIdempotencyMismatch
		}
		select {
		case <-ent.ready:
		case <-ctx.Done():
			return Ref{}, ctx.Err()
		}
		if ent.ok {
			return ent.ref, nil
		}
	}
}

// submitOwning 受理第一次的請求並結算它的冪等紀錄；panic 也要關閉 ready，等待者才不會卡住。
func (e *Executor) submitOwning(ctx context.Context, in SubmitInput, key string, ent *idemEntry) (ref Ref, err error) {
	succeeded := false
	defer func() {
		e.mu.Lock()
		if succeeded {
			ent.ref, ent.ok = ref, true
		} else {
			delete(e.idem, key)
		}
		close(ent.ready)
		e.mu.Unlock()
	}()
	ref, err = e.submit(ctx, in)
	succeeded = err == nil
	return ref, err
}

// inputHash 是請求內容的雜湊（不含 RequestID）：每個欄位前加長度，避免欄位邊界混淆。
func inputHash(in SubmitInput) [sha256.Size]byte {
	h := sha256.New()
	write := func(s string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		h.Write(n[:])
		h.Write([]byte(s))
	}
	write(strconv.Itoa(int(in.Topic)))
	write(in.SessionID)
	write(in.Message)
	write(in.ImageURL)
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

func (e *Executor) submit(ctx context.Context, in SubmitInput) (Ref, error) {
	if !in.Topic.Valid() {
		return Ref{}, model.ErrInvalidTopic
	}
	loc := uitext.LocaleFrom(ctx)

	if in.Topic == model.TopicReturnPolicy {
		e.mu.Lock()
		closed := e.closed
		e.mu.Unlock()
		if closed {
			return Ref{}, model.ErrShuttingDown
		}
		return e.submitReturnPolicy(ctx, in, loc)
	}

	message := in.Message
	if message == "" {
		return Ref{}, model.ErrMessageRequired
	}
	userID := strconv.FormatInt(in.BuyerID, 10)

	// 順序（A9-D15 第 4 點）：檢查 closed、佔用對話、wg.Add 都在同一把鎖內，session ID 由伺服器先產生，之後才寫 session。
	// 任何一步失敗都釋放佔用並 wg.Done，不留下沒有輪次的 session。
	sessionID := in.SessionID
	newSession := sessionID == ""
	if newSession {
		sessionID = platform.NewUUID(ctx)
	}
	key := guardKey(in.BuyerID, sessionID)
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return Ref{}, model.ErrShuttingDown
	}
	if _, busy := e.active[key]; busy {
		e.mu.Unlock()
		return Ref{}, model.ErrTurnInProgress
	}
	e.active[key] = struct{}{}
	e.wg.Add(1)
	e.mu.Unlock()
	started := false
	defer func() {
		if !started {
			e.release(key)
			e.wg.Done()
		}
	}()

	if newSession {
		// 買家 ID 以字串寫入：數字經 JSONB 往返會被 float64 改寫。
		if _, err := e.cfg.Sessions.Create(ctx, &session.CreateRequest{
			AppName:   buyeragent.AppName,
			UserID:    userID,
			SessionID: sessionID,
			State:     map[string]any{buyeragent.StateKeyBuyer: userID},
		}); err != nil {
			slog.ErrorContext(ctx, "assistant：建立 session 失敗", "error", err)
			return Ref{}, fmt.Errorf("%w: %w", model.ErrSessionCreate, err)
		}
	} else if _, err := e.cfg.Sessions.Get(ctx, &session.GetRequest{AppName: buyeragent.AppName, UserID: userID, SessionID: sessionID}); err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return Ref{}, model.ErrSessionNotFound
		}
		return Ref{}, fmt.Errorf("assistant：讀取 session：%w", err)
	}

	turnID, t := e.register(ctx, in.BuyerID)

	content := genai.NewContentFromText(message, genai.RoleUser)
	if in.ImageURL != "" {
		content.Parts = append(content.Parts, &genai.Part{FileData: &genai.FileData{FileURI: in.ImageURL}})
	}

	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.cfg.Timeout)
	stop := context.AfterFunc(e.base, cancel)
	started = true
	go func() {
		defer e.wg.Done()
		defer cancel()
		defer stop()
		// 先釋放對話、再發出終止事件：client 一收到結束就重送，不會撞到上一輪的佔用。
		terminal := e.execute(runCtx, t, in, userID, sessionID, content, loc)
		e.release(key)
		t.buf.append(terminal, e.cfg.Now())
	}()
	return Ref{TurnID: turnID, SessionID: sessionID}, nil
}

func (e *Executor) release(key string) {
	e.mu.Lock()
	delete(e.active, key)
	e.mu.Unlock()
}

func (e *Executor) submitReturnPolicy(ctx context.Context, in SubmitInput, loc uitext.Locale) (Ref, error) {
	// 事件先寫進還沒登記的緩衝，寫成功才登記：失敗時沒有輪次留在 e.turns（沒有對話佔用，固定文案不建 session）。
	buf := newBuffer(e.cfg.BufferCapacity)
	if !writeReturnPolicy(buf, in.SessionID, loc, e.cfg.Now()) {
		return Ref{}, errors.New("assistant：退貨政策事件寫入緩衝失敗")
	}
	turnID := e.install(ctx, &turnState{buyerID: in.BuyerID, buf: buf})
	return Ref{TurnID: turnID, SessionID: in.SessionID}, nil
}

// writeReturnPolicy 把固定文案與終止事件寫進緩衝；任何一次寫入失敗回 false。照搬：固定文案不建 session，finished 帶的是請求的 session_id（可為空）。
func writeReturnPolicy(buf *buffer, sessionID string, loc uitext.Locale, now time.Time) bool {
	body := text.ReturnPolicyTitle(loc).Text + "\n\n" + text.ReturnPolicyFacts(loc).Text + "\n\n" + text.ReturnPolicyClosing(loc).Text
	return buf.append(model.Event{Kind: model.EventText, Text: body}, now) && buf.append(model.Event{Kind: model.EventFinished, SessionID: sessionID}, now)
}

// execute 跑 agent 並回這一輪的終止事件；panic 也以失敗事件回傳。
//
// finished 的條件（A9-D15 第 4 點）：事件迭代正常結束，而且看過根 agent 的最終回應（IsFinalResponse）。
// 取消、逾時、模型安靜停下、事件數超過上限，只要最終回應沒有出現，一律是 error；迴圈之後不再看 ctx.Err()。
func (e *Executor) execute(ctx context.Context, t *turnState, in SubmitInput, userID, sessionID string, content *genai.Content, loc uitext.Locale) (terminal model.Event) {
	var sent strings.Builder
	fail := func() model.Event {
		if sent.Len() > 0 {
			return model.Event{Kind: model.EventError, Failure: model.FailureInterrupted, Message: text.TurnInterrupted(loc).Text}
		}
		return model.Event{Kind: model.EventError, Failure: model.FailureNoOutput, Message: text.TurnFailed(loc).Text}
	}
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "assistant：輪次 panic", "panic", r, "stack", string(debug.Stack()))
			terminal = fail()
		}
	}()

	// 緩衝寫滿（事件數超過上限）時停掉這一輪：取消 runner，讓它在最終回應之前結束，走 fail()。
	ctx, stopRun := context.WithCancel(ctx)
	defer stopRun()
	var overflow atomic.Bool
	emit := func(ev model.Event) bool {
		if t.buf.append(ev, e.cfg.Now()) {
			return true
		}
		if overflow.CompareAndSwap(false, true) {
			slog.WarnContext(ctx, "assistant：這一輪的事件數超過上限，中止", "limit", e.cfg.BufferCapacity)
		}
		stopRun()
		return false
	}
	deps := buyeragent.Deps{
		Orders:  e.cfg.Orders,
		Model:   e.cfg.Model,
		BuyerID: in.BuyerID,
		Now:     e.cfg.Now,
		Locale:  loc,
		OnTool: func(c buyeragent.ToolCall) {
			if e.cfg.ToolObserver != nil {
				e.cfg.ToolObserver(c)
			}
			emit(model.Event{Kind: model.EventToolResult, Tool: c.Tool, ToolSucceeded: !c.Failed})
		},
	}
	var (
		ag  agent.Agent
		err error
	)
	switch in.Topic {
	case model.TopicOrders:
		ag, err = deps.OrdersSpecialist()
	case model.TopicTracking:
		ag, err = deps.TrackingSpecialist()
	case model.TopicChat:
		ag, err = deps.Coordinator()
	case model.TopicReturnPolicy:
		err = errors.New("退貨政策不經 agent")
	}
	if err != nil {
		slog.ErrorContext(ctx, "assistant：建立 agent 失敗", "error", err)
		return fail()
	}
	r, err := runner.New(runner.Config{AppName: buyeragent.AppName, Agent: ag, SessionService: e.cfg.Sessions})
	if err != nil {
		slog.ErrorContext(ctx, "assistant：建立 runner 失敗", "error", err)
		return fail()
	}

	// ADK 的 SSE 模式先送多個 Partial 的 chunk，最後送一個完整的非 Partial 回應。sent 記這一段已送出的文字：
	// 非 Partial 到達時只補送沒送過的尾巴；tool 回應代表新一段文字開始，重置。
	finalSeen := false
	for ev, err := range r.Run(ctx, userID, sessionID, content, agent.RunConfig{StreamingMode: agent.StreamingModeSSE}) {
		if err != nil {
			slog.WarnContext(ctx, "assistant：agent 執行失敗", "error", err)
			return fail()
		}
		if ev == nil {
			continue
		}
		if ev.Author == ag.Name() && ev.IsFinalResponse() && ev.ErrorCode == "" {
			finalSeen = true
		}
		if ev.Content == nil {
			continue
		}
		hasFuncResp := false
		for _, part := range ev.Content.Parts {
			if part.FunctionResponse != nil {
				hasFuncResp = true
			}
		}
		if hasFuncResp {
			sent.Reset()
		}
		if !ev.Partial && sent.Len() > 0 {
			var full strings.Builder
			for _, part := range ev.Content.Parts {
				if part.Thought {
					continue
				}
				full.WriteString(part.Text)
			}
			if tail, ok := strings.CutPrefix(full.String(), sent.String()); ok && tail != "" {
				emit(model.Event{Kind: model.EventText, Text: tail})
			}
			continue
		}
		for _, part := range ev.Content.Parts {
			if part.Thought {
				continue // thought 不得以一般文字外流
			}
			if part.Text != "" {
				if emit(model.Event{Kind: model.EventText, Text: part.Text}) {
					sent.WriteString(part.Text)
				}
			}
		}
	}
	if overflow.Load() {
		slog.WarnContext(ctx, "assistant：這一輪因事件數超過上限而以失敗結束", "limit", e.cfg.BufferCapacity)
		return fail()
	}
	if !finalSeen {
		slog.WarnContext(ctx, "assistant：事件流結束但沒有根 agent 的最終回應", "ctxErr", ctx.Err())
		return fail()
	}
	return model.Event{Kind: model.EventFinished, SessionID: sessionID}
}

// Watch 訂閱一輪的事件，從 cursor 之後開始（空字串是從頭）。輪次不存在或不屬於 buyerID 回 ErrTurnNotFound，
// cursor 超前於最新事件（或不是伺服器發的值）回 ErrCursorAhead，在回傳迭代器之前判定；緩衝不覆蓋舊事件，所以沒有過期。
//
// 迭代器依序給事件，在終止事件之後結束；ctx 結束時正常結束（呼叫者用自己的期限，例如 100 秒，決定要不要續看）。
func (e *Executor) Watch(ctx context.Context, buyerID int64, turnID, cursor string) (iter.Seq2[model.Event, error], error) {
	e.mu.Lock()
	e.sweepLocked(e.cfg.Now())
	t, ok := e.turns[turnID]
	e.mu.Unlock()
	if !ok || t.buyerID != buyerID {
		return nil, model.ErrTurnNotFound
	}
	pos, err := t.buf.position(cursor)
	if err != nil {
		return nil, err
	}
	return func(yield func(model.Event, error) bool) {
		for {
			evs, wait, finished := t.buf.next(pos)
			for _, ev := range evs {
				if !yield(ev, nil) {
					return
				}
				pos++
			}
			if finished {
				return
			}
			if wait == nil {
				continue
			}
			select {
			case <-wait:
			case <-ctx.Done():
				return
			}
		}
	}, nil
}
