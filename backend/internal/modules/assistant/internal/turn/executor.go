// Package turn 執行買家助理的一輪對話：受理後立刻回輪次 ID，agent 在背景跑，事件放記憶體的環形緩衝（A9-D12）；
// 同一個對話同時只允許一輪進行中（A9-D14）。
//
// 這個套件不開 transaction，也不接受已在 transaction 內的 ctx：agent 執行包含 LLM 呼叫（backend/AGENTS.md 規則 5），
// session 的寫入由 session.Service 自己在最上層開 transaction（adksession 在外層 transaction 內回 ErrInOuterTx）。
package turn

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strconv"
	"strings"
	"sync"
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
	// DefaultBufferCapacity 是每輪保留的事件數；超過時最舊的事件被覆蓋，接續的 cursor 落在被覆蓋的範圍回 ErrCursorExpired。
	DefaultBufferCapacity = 1024
	// DefaultRetention 是一輪結束後，輪次與它的事件還能被訂閱的時間。
	DefaultRetention = 10 * time.Minute
	// DefaultTimeout 是一輪從開始到結束的上限；到了就以失敗結束，釋放對話的佔用。
	DefaultTimeout = 2 * time.Minute
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
}

// SubmitInput 是送出一則訊息。
type SubmitInput struct {
	BuyerID   int64
	Topic     model.Topic
	SessionID string // 空字串表示開新的對話
	Message   string
	ImageURL  string // 空字串表示沒有圖
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

	// events 產生 runner 的事件流；測試用它替換成不經 runner 的來源。
	events func(ctx context.Context, r *runner.Runner, userID, sessionID string, content *genai.Content) iter.Seq2[*session.Event, error]

	base   context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	closed bool
	turns  map[string]*turnState
	active map[string]struct{} // A9-D14：進行中的對話，鍵是 buyerID/sessionID
}

// New 建立執行器。
func New(cfg Config) *Executor {
	if cfg.Now == nil || cfg.Sessions == nil || cfg.Model == nil || cfg.Orders == nil {
		panic("turn：Config 的 Now、Sessions、Model、Orders 都必填")
	}
	if cfg.BufferCapacity <= 0 {
		cfg.BufferCapacity = DefaultBufferCapacity
	}
	if cfg.Retention <= 0 {
		cfg.Retention = DefaultRetention
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	base, cancel := context.WithCancel(context.Background())
	return &Executor{cfg: cfg, events: runnerEvents, base: base, cancel: cancel, turns: map[string]*turnState{}, active: map[string]struct{}{}}
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
}

// register 登記一輪並回它的 ID：隨機 UUID（ADK 的 platform.NewUUID），輪次 ID 同時要買家相符才能訂閱，不是憑證。
func (e *Executor) register(ctx context.Context, buyerID int64) (string, *turnState) {
	id := platform.NewUUID(ctx)
	t := &turnState{buyerID: buyerID, buf: newBuffer(e.cfg.BufferCapacity)}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sweepLocked(e.cfg.Now())
	e.turns[id] = t
	return id, t
}

// Submit 受理一則訊息：驗證、解析（或建立）對話、佔用對話，然後在背景跑 agent，立刻回輪次 ID。
func (e *Executor) Submit(ctx context.Context, in SubmitInput) (Ref, error) {
	if _, ok := db.TxFrom(ctx); ok {
		return Ref{}, model.ErrOuterTx
	}
	if !in.Topic.Valid() {
		return Ref{}, model.ErrInvalidTopic
	}
	e.mu.Lock()
	closed := e.closed
	e.mu.Unlock()
	if closed {
		return Ref{}, model.ErrShuttingDown
	}
	loc := uitext.LocaleFrom(ctx)

	if in.Topic == model.TopicReturnPolicy {
		return e.submitReturnPolicy(ctx, in, loc)
	}

	message := in.Message
	if message == "" {
		// 照搬：按鈕入口沒填訊息時用預設訊息；一般問答沒有預設。疑似已不會發生：proto 把 message 標為必填。
		switch in.Topic {
		case model.TopicOrders:
			message = text.DefaultMessageOrders(loc).Text
		case model.TopicTracking:
			message = text.DefaultMessageTracking(loc).Text
		case model.TopicReturnPolicy, model.TopicChat:
			return Ref{}, model.ErrMessageRequired
		}
	}
	userID := strconv.FormatInt(in.BuyerID, 10)

	sessionID := in.SessionID
	if sessionID == "" {
		// 買家 ID 以字串寫入：數字經 JSONB 往返會被 float64 改寫。
		resp, err := e.cfg.Sessions.Create(ctx, &session.CreateRequest{
			AppName: buyeragent.AppName,
			UserID:  userID,
			State:   map[string]any{buyeragent.StateKeyBuyer: userID},
		})
		if err != nil {
			slog.ErrorContext(ctx, "assistant：建立 session 失敗", "error", err)
			return Ref{}, fmt.Errorf("%w: %w", model.ErrSessionCreate, err)
		}
		sessionID = resp.Session.ID()
	} else if _, err := e.cfg.Sessions.Get(ctx, &session.GetRequest{AppName: buyeragent.AppName, UserID: userID, SessionID: sessionID}); err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return Ref{}, model.ErrSessionNotFound
		}
		return Ref{}, fmt.Errorf("assistant：讀取 session：%w", err)
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

	turnID, t := e.register(ctx, in.BuyerID)

	content := genai.NewContentFromText(message, genai.RoleUser)
	if in.ImageURL != "" {
		content.Parts = append(content.Parts, &genai.Part{FileData: &genai.FileData{FileURI: in.ImageURL}})
	}

	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.cfg.Timeout)
	stop := context.AfterFunc(e.base, cancel)
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
	turnID, t := e.register(ctx, in.BuyerID)
	body := text.ReturnPolicyTitle(loc).Text + "\n\n" + text.ReturnPolicyFacts(loc).Text + "\n\n" + text.ReturnPolicyClosing(loc).Text
	now := e.cfg.Now()
	t.buf.append(model.Event{Kind: model.EventText, Text: body}, now)
	// 照搬：固定文案不建 session，finished 帶的是請求的 session_id（可為空）。
	t.buf.append(model.Event{Kind: model.EventFinished, SessionID: in.SessionID}, now)
	return Ref{TurnID: turnID, SessionID: in.SessionID}, nil
}

// execute 跑 agent 並回這一輪的終止事件；panic 也以失敗事件回傳。
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
			slog.ErrorContext(ctx, "assistant：輪次 panic", "panic", r)
			terminal = fail()
		}
	}()

	emit := func(ev model.Event) { t.buf.append(ev, e.cfg.Now()) }
	deps := buyeragent.Deps{
		Orders:  e.cfg.Orders,
		Model:   e.cfg.Model,
		BuyerID: in.BuyerID,
		Now:     e.cfg.Now,
		OnTool: func(c buyeragent.ToolCall) {
			emit(model.Event{Kind: model.EventToolResult, ToolName: c.Name, ToolFailed: c.Failed, ToolArgs: c.Args})
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
	for ev, err := range e.events(ctx, r, userID, sessionID, content) {
		if err != nil || ctx.Err() != nil {
			slog.WarnContext(ctx, "assistant：agent 執行失敗", "error", errors.Join(err, ctx.Err()))
			return fail()
		}
		if ev == nil || ev.Content == nil {
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
				emit(model.Event{Kind: model.EventText, Text: part.Text})
				sent.WriteString(part.Text)
			}
		}
	}
	if ctx.Err() != nil {
		slog.WarnContext(ctx, "assistant：輪次在 agent 結束前被取消或逾時", "error", ctx.Err())
		return fail()
	}
	return model.Event{Kind: model.EventFinished, SessionID: sessionID}
}

func runnerEvents(ctx context.Context, r *runner.Runner, userID, sessionID string, content *genai.Content) iter.Seq2[*session.Event, error] {
	return r.Run(ctx, userID, sessionID, content, agent.RunConfig{StreamingMode: agent.StreamingModeSSE})
}

// Watch 訂閱一輪的事件，從 cursor 之後開始（空字串是從頭）。輪次不存在或不屬於 buyerID 回 ErrTurnNotFound，
// cursor 早於緩衝回 ErrCursorExpired、超前（或不是伺服器發的值）回 ErrCursorAhead；這些都在回傳迭代器之前判定。
//
// 迭代器依序給事件，在終止事件之後結束；ctx 結束時正常結束（呼叫者用自己的期限，例如 100 秒，決定要不要續看）。
// 消費太慢、事件被覆蓋時，迭代器以 ErrCursorExpired 結束。
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
			evs, wait, finished, err := t.buf.next(pos)
			if err != nil {
				yield(model.Event{}, err)
				return
			}
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
