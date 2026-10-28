package handler

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	actorpkg "github.com/Will413028/hawkr/ai-service/internal/actor"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func TestDefaultMessages_SellerKeys(t *testing.T) {
	// "orders" 已改為直接查詢，seller 端不再需要此 key；
	// 但 buyer init() 會把 "orders" merge 進來，故此處只驗證賣家 LLM 端點的 keys
	expected := []string{"inventory", "payment"}
	for _, key := range expected {
		if _, ok := DefaultMessages[key]; !ok {
			t.Errorf("DefaultMessages missing seller key %q", key)
		}
	}
}

func TestDefaultMessages_BuyerKeys(t *testing.T) {
	// buyer init() 會把 buyerDefaultMessages 合併到 DefaultMessages
	expected := []string{"orders", "tracking"}
	for _, key := range expected {
		if _, ok := DefaultMessages[key]; !ok {
			t.Errorf("DefaultMessages missing buyer key %q", key)
		}
	}
}

// --- RunAssistantStream 兩種 pre-stream 失敗的 sentinel error 測試 ---
//
// RunAssistantStream 用 ErrRunnerNotReady／ErrSessionCreate 兩個 sentinel error
// 回傳，呼叫端（internal/grpc）用 errors.Is 映回各自的 gRPC status；這裡直接鎖住
// 兩種失敗不會被塌縮成同一個 error。

// failingSessionService 是只會讓 Create 失敗的 session.Service 測試替身；其餘方法
// 在這個測試路徑用不到，回傳 nil 即可。
type failingSessionService struct{}

func (failingSessionService) Create(context.Context, *session.CreateRequest) (*session.CreateResponse, error) {
	return nil, errors.New("boom: db unavailable")
}
func (failingSessionService) Get(context.Context, *session.GetRequest) (*session.GetResponse, error) {
	return nil, nil
}
func (failingSessionService) List(context.Context, *session.ListRequest) (*session.ListResponse, error) {
	return nil, nil
}
func (failingSessionService) Delete(context.Context, *session.DeleteRequest) error { return nil }
func (failingSessionService) AppendEvent(context.Context, session.Session, *session.Event) error {
	return nil
}

func TestRunAssistantStream_NilRunner_ReturnsErrRunnerNotReady(t *testing.T) {
	err := RunAssistantStream(context.Background(), failingSessionService{}, nil, nil,
		actorpkg.NewStoreActor(12345), "", genai.NewContentFromText("hi", genai.RoleUser))

	if !errors.Is(err, ErrRunnerNotReady) {
		t.Fatalf("err = %v, want errors.Is(err, ErrRunnerNotReady)", err)
	}
	if errors.Is(err, ErrSessionCreate) {
		t.Fatalf("err = %v, must NOT also match ErrSessionCreate", err)
	}
}

func TestRunAssistantStream_SessionCreateFailure_ReturnsErrSessionCreate(t *testing.T) {
	// r 非 nil（走過 nil-runner 防護）但 sessionID 為空迫使呼叫 sessionSvc.Create，
	// 該呼叫必失敗。r 本身不會被用到（Create 失敗就直接 return，不會走到 r.Run）。
	r := &runner.Runner{}
	err := RunAssistantStream(context.Background(), failingSessionService{}, nil, r,
		actorpkg.NewStoreActor(12345), "", genai.NewContentFromText("hi", genai.RoleUser))

	if !errors.Is(err, ErrSessionCreate) {
		t.Fatalf("err = %v, want errors.Is(err, ErrSessionCreate)", err)
	}
	if errors.Is(err, ErrRunnerNotReady) {
		t.Fatalf("err = %v, must NOT also match ErrRunnerNotReady", err)
	}
}

// --- Thought-part guard（#2447 BE-01A latent leak 預防）---

// thoughtLeakLLM 分兩個 chunk 回應：先一個一般文字 chunk，再一個 Thought chunk
// ——thought 排在所有文字之後，是 suffix-dedup 分支（Partial=false 的
// HasPrefix/tail 計算）唯一會把思考內容當 tail 外流的排列，讓兩個 Thought guard
// （partial 轉發迴圈與 suffix 全文組裝）都被真正執行到。assistant 目前沒有
// provider 會產 Thought part（唯一的 producer 是 procurement 的 DeepSeek
// thinking，且已停用）；這個 fake 模擬「未來某個 provider 開 thinking」的情況。
type thoughtLeakLLM struct{}

func (thoughtLeakLLM) Name() string { return "thought-leak-fake" }

func (thoughtLeakLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if !yield(&model.LLMResponse{
			Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{
				{Text: "正式回覆"},
			}},
			Partial: true,
		}, nil) {
			return
		}
		if !yield(&model.LLMResponse{
			Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{
				{Text: "思考中：買家想要什麼？", Thought: true},
			}},
			Partial: true,
		}, nil) {
			return
		}
		// 模擬 adk stream aggregator 的收尾 aggregate（Partial=false、含全部
		// runs、thought 排在文字之後）——這是 suffix-dedup 分支唯一的入口。
		yield(&model.LLMResponse{
			Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{
				{Text: "正式回覆"},
				{Text: "思考中：買家想要什麼？", Thought: true},
			}},
			Partial:      false,
			TurnComplete: true,
		}, nil)
	}
}

// captureSink 只記錄 Text 呼叫——本測試只關心文字轉發面。
type captureSink struct {
	texts []string
}

func (c *captureSink) Text(s string)     { c.texts = append(c.texts, s) }
func (c *captureSink) Done(string)       {}
func (c *captureSink) Error(int, string) {}
func (c *captureSink) DraftPatch(string) {}
func (c *captureSink) ToolResult(string) {}

func TestRunAssistantStream_DropsThoughtParts(t *testing.T) {
	ag, err := llmagent.New(llmagent.Config{Name: "test_agent", Model: thoughtLeakLLM{}})
	if err != nil {
		t.Fatalf("llmagent.New 失敗: %v", err)
	}
	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName: "hawkr_ai", Agent: ag,
		SessionService: svc, AutoCreateSession: true,
	})
	if err != nil {
		t.Fatalf("runner.New 失敗: %v", err)
	}

	sink := &captureSink{}
	err = RunAssistantStream(context.Background(), svc, sink, r,
		actorpkg.NewStoreActor(12345), "", genai.NewContentFromText("hi", genai.RoleUser))
	if err != nil {
		t.Fatalf("RunAssistantStream 失敗: %v", err)
	}

	joined := strings.Join(sink.texts, "")
	if strings.Contains(joined, "思考中") {
		t.Fatalf("Thought part 不得以一般文字轉發, got %q", joined)
	}
	// 精確等值：thought 排在文字後面時，suffix 分支若少了 guard，思考內容會以
	// tail 形式黏在正式回覆之後——只驗 Contains(正式回覆) 抓不到那種洩漏。
	if joined != "正式回覆" {
		t.Fatalf("輸出應精確等於正式回覆, got %q", joined)
	}
}

func TestProductAutofill_ValidWeightUnits(t *testing.T) {
	for unit := range ValidWeightUnits {
		if !ValidWeightUnits[unit] {
			t.Errorf("expected unit %q to be valid", unit)
		}
	}
	// 非法值
	invalid := []string{"斤", "盒", "袋", "kg", "catty"}
	for _, u := range invalid {
		if ValidWeightUnits[u] {
			t.Errorf("expected unit %q to be invalid", u)
		}
	}
}

func TestProductAutofill_NoneUnitConverted(t *testing.T) {
	// Gemini schema 不允許空字串 enum，改用 NONE；
	// NONE 不應在 ValidWeightUnits 中（會走轉換分支），最終輸出空字串
	if ValidWeightUnits["NONE"] {
		t.Error("NONE should NOT be in ValidWeightUnits; it must go through the NONE→'' conversion first")
	}
	// 驗證轉換後空字串合法
	if !ValidWeightUnits[""] {
		t.Error("empty string should be valid in ValidWeightUnits after NONE conversion")
	}
}
