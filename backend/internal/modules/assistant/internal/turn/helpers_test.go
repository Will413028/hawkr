package turn

import (
	"context"
	"errors"
	"iter"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

// chunk 是替身 LLM 的一個回應片段（fixture 的 scripted_llm.chunks 同形）。
type chunk struct {
	Partial      bool       `json:"partial"`
	Text         string     `json:"text"`
	Thought      bool       `json:"thought"`
	FunctionCall *chunkCall `json:"function_call"`
	Error        string     `json:"error"`
	// ErrorCode 非空時送出沒有內容的最終回應，形狀同 Gemini 擋下的回應（ErrorCode 與 FinishReason 都帶安全原因）。
	ErrorCode string `json:"error_code"`
}

type chunkCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// scriptedCall 是替身 LLM 的一次呼叫要回的內容。
type scriptedCall struct {
	Agent  string  `json:"agent"`
	Chunks []chunk `json:"chunks"`
	// FinishReason 是非 Partial 回應的結束原因；nil 表示 STOP，指向空字串表示串流被提早關閉。
	FinishReason *genai.FinishReason `json:"finish_reason"`
}

// llmRequest 是替身 LLM 收到的一次請求的摘要。
type llmRequest struct {
	Agent                 string
	LastUserTexts         []string
	FileURIs              []string
	FunctionResponsesBack []map[string]any
	HistoryContents       int
	// CallingMode 與 AllowedNames 是請求的 ToolConfig.FunctionCallingConfig；DeclaredNames 是同一個請求宣告的函式名稱。
	CallingMode   string
	AllowedNames  []string
	DeclaredNames []string
}

// agentOf 由系統提示判斷這次呼叫是哪個 agent（路由是否正確）。
func agentOf(req *adkmodel.LLMRequest) string {
	si := ""
	if req.Config != nil && req.Config.SystemInstruction != nil {
		for _, p := range req.Config.SystemInstruction.Parts {
			si += p.Text
		}
	}
	switch {
	case strings.Contains(si, "Hawkr 購物小幫手"):
		return "buyer_coordinator"
	case strings.Contains(si, "買家訂單查詢助手"):
		return "buyer_order_specialist"
	case strings.Contains(si, "買家包裹追蹤助手"):
		return "buyer_tracking_specialist"
	}
	return "unknown"
}

func summarize(req *adkmodel.LLMRequest) llmRequest {
	s := llmRequest{Agent: agentOf(req), HistoryContents: len(req.Contents)}
	if req.Config != nil {
		if tc := req.Config.ToolConfig; tc != nil && tc.FunctionCallingConfig != nil {
			s.CallingMode = string(tc.FunctionCallingConfig.Mode)
			s.AllowedNames = append([]string(nil), tc.FunctionCallingConfig.AllowedFunctionNames...)
		}
		for _, tl := range req.Config.Tools {
			for _, fd := range tl.FunctionDeclarations {
				s.DeclaredNames = append(s.DeclaredNames, fd.Name)
			}
		}
	}
	if len(req.Contents) == 0 {
		return s
	}
	last := req.Contents[len(req.Contents)-1]
	for _, p := range last.Parts {
		switch {
		case p.FunctionResponse != nil:
			s.FunctionResponsesBack = append(s.FunctionResponsesBack, map[string]any{"name": p.FunctionResponse.Name, "response": p.FunctionResponse.Response})
		case p.FileData != nil:
			s.FileURIs = append(s.FileURIs, p.FileData.FileURI)
		case p.Text != "":
			s.LastUserTexts = append(s.LastUserTexts, p.Text)
		}
	}
	return s
}

// scriptedLLM 依序重播 script，每次 GenerateContent 取下一筆；不依賴 goroutine 的排程順序。
type scriptedLLM struct {
	mu       sync.Mutex
	script   []scriptedCall
	requests []llmRequest
}

func (l *scriptedLLM) Name() string { return "scripted" }

func (l *scriptedLLM) GenerateContent(_ context.Context, req *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	l.mu.Lock()
	idx := len(l.requests)
	l.requests = append(l.requests, summarize(req))
	var call scriptedCall
	if idx < len(l.script) {
		call = l.script[idx]
	} else {
		call = scriptedCall{Chunks: []chunk{{Error: "scripted llm: no more scripted calls"}}}
	}
	l.mu.Unlock()
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		for _, c := range call.Chunks {
			if c.Error != "" {
				yield(nil, errors.New(c.Error))
				return
			}
			if c.ErrorCode != "" {
				yield(&adkmodel.LLMResponse{ErrorCode: c.ErrorCode, FinishReason: genai.FinishReasonSafety}, nil)
				return
			}
			part := &genai.Part{Text: c.Text, Thought: c.Thought}
			if c.FunctionCall != nil {
				part = &genai.Part{FunctionCall: &genai.FunctionCall{Name: c.FunctionCall.Name, Args: c.FunctionCall.Args}}
			}
			resp := &adkmodel.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{part}}, Partial: c.Partial}
			if !c.Partial {
				resp.FinishReason = genai.FinishReasonStop
				if call.FinishReason != nil {
					resp.FinishReason = *call.FinishReason
				}
			}
			if !yield(resp, nil) {
				return
			}
		}
	}
}

func (l *scriptedLLM) seen() []llmRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]llmRequest(nil), l.requests...)
}

// textLLM 每次呼叫回一段固定文字。
func textLLM(text string) *scriptedLLM {
	calls := make([]scriptedCall, 8)
	for i := range calls {
		calls[i] = scriptedCall{Chunks: []chunk{{Text: text}}}
	}
	return &scriptedLLM{script: calls}
}

// fakeOrders 是 buyeragent.Orders 的替身。
type fakeOrders struct {
	orders      []buyeragent.Order
	ordersErr   error
	details     map[string]buyeragent.OrderDetail
	detailErrs  map[string]error
	mu          sync.Mutex
	listCalls   int
	listBuyerID []int64
}

func (f *fakeOrders) MyOrders(_ context.Context, buyerID int64, statusFilter string, _, _ int) ([]buyeragent.Order, error) {
	f.mu.Lock()
	f.listCalls++
	f.listBuyerID = append(f.listBuyerID, buyerID)
	f.mu.Unlock()
	if f.ordersErr != nil {
		return nil, f.ordersErr
	}
	if statusFilter == "" {
		return f.orders, nil
	}
	var out []buyeragent.Order
	for _, o := range f.orders {
		if slices.Contains(strings.Split(statusFilter, ","), o.Status) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeOrders) OrderDetail(_ context.Context, _ int64, orderNo string) (buyeragent.OrderDetail, error) {
	if err := f.detailErrs[orderNo]; err != nil {
		return buyeragent.OrderDetail{}, err
	}
	d, ok := f.details[orderNo]
	if !ok {
		return buyeragent.OrderDetail{}, errors.New("not found")
	}
	return d, nil
}

// failingSessions 讓 Create 失敗。
type failingSessions struct{ session.Service }

func (failingSessions) Create(context.Context, *session.CreateRequest) (*session.CreateResponse, error) {
	return nil, errors.New("session store down")
}

func newExecutor(t *testing.T, llm adkmodel.LLM, orders buyeragent.Orders, mutate func(*Config)) *Executor {
	t.Helper()
	cfg := Config{Sessions: session.InMemoryService(), Model: llm, Orders: orders, Now: time.Now}
	if mutate != nil {
		mutate(&cfg)
	}
	e := New(cfg)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown：%v", err)
		}
	})
	return e
}

// collect 訂閱一輪直到它結束，回全部事件；30 秒只是防止測試卡死的保險，不是被測的行為。
func collect(t *testing.T, e *Executor, buyerID int64, turnID, cursor string) []model.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	seq, err := e.Watch(ctx, buyerID, turnID, cursor)
	if err != nil {
		t.Fatalf("Watch：%v", err)
	}
	var out []model.Event
	for ev, err := range seq {
		if err != nil {
			t.Fatalf("Watch 迭代：%v", err)
		}
		out = append(out, ev)
	}
	if n := len(out); n == 0 || !out[n-1].Kind.Terminal() {
		t.Fatalf("事件沒有以終止事件結束：%+v", out)
	}
	return out
}
