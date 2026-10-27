package handler

import (
	"context"
	"testing"
	"time"

	actorpkg "github.com/Will413028/hawkr/ai-service/internal/actor"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// appendTextEvent 是本檔測試共用的 helper：對 sess 附加一則帶文字內容與明確
// timestamp 的事件。timestamp 直接決定該 session 的 LastUpdateTime（見 adk-go
// session/inmemory.go 的 appendEvent：s.updatedAt = event.Timestamp），讓測試能
// 精準控制「哪個 session 最新」而不必依賴真實時鐘。
func appendTextEvent(t *testing.T, svc session.Service, sess session.Session, role, text string, ts time.Time) {
	t.Helper()
	err := svc.AppendEvent(context.Background(), sess, &session.Event{
		ID:        sess.ID() + "-" + role + "-" + ts.String(),
		Author:    role,
		Timestamp: ts,
		LLMResponse: model.LLMResponse{
			Content: genai.NewContentFromText(text, genai.Role(role)),
		},
	})
	if err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
}

// --- FetchLatestSession（package-level 核心，唯一呼叫端是 internal/grpc 的
// SellerLatestSession／BuyerLatestSession）行為測試 ---

func TestFetchLatestSession_NoSessions_ReturnsEmptyNotNil(t *testing.T) {
	svc := session.InMemoryService()
	actor := actorpkg.NewStoreActor(99999)

	sessionID, messages, err := FetchLatestSession(context.Background(), svc, actor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID != "" {
		t.Errorf("sessionID = %q, want empty", sessionID)
	}
	if messages == nil {
		t.Fatal("messages is nil, want non-nil empty slice (must marshal to [] not null)")
	}
	if len(messages) != 0 {
		t.Errorf("len(messages) = %d, want 0", len(messages))
	}
}

func TestFetchLatestSession_ReturnsSessionWithLatestUpdateTime_NotFirst(t *testing.T) {
	svc := session.InMemoryService()
	actor := actorpkg.NewStoreActor(12345)
	ctx := context.Background()
	base := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)

	createOne := func(text string, ts time.Time) session.Session {
		resp, err := svc.Create(ctx, &session.CreateRequest{
			AppName: "hawkr_ai",
			UserID:  actor.UserID,
			State:   map[string]any{actor.StateKey: actor.StateVal},
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		appendTextEvent(t, svc, resp.Session, genai.RoleModel, text, ts)
		return resp.Session
	}

	// 建立順序刻意與「最新」順序相反：最先建立的 session 反而拿到最新的
	// LastUpdateTime。若實作誤用 listResp.Sessions[0]（只取第一筆）而不是真的
	// 比較全部 session 的 LastUpdateTime，這裡就會抓錯。
	newest := createOne("我是最新的一則", base.Add(2*time.Hour))
	_ = createOne("我是最舊的一則", base)
	_ = createOne("我是中間的一則", base.Add(1*time.Hour))

	sessionID, messages, err := FetchLatestSession(ctx, svc, actor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID != newest.ID() {
		t.Fatalf("sessionID = %q, want %q (the session with the latest LastUpdateTime)", sessionID, newest.ID())
	}
	if len(messages) != 1 || messages[0].Text != "我是最新的一則" {
		t.Fatalf("messages = %+v, want single message with text %q", messages, "我是最新的一則")
	}
	if messages[0].Role != "model" {
		t.Errorf("messages[0].Role = %q, want %q", messages[0].Role, "model")
	}
	wantTimestamp := base.Add(2 * time.Hour).UTC().Format(time.RFC3339)
	if messages[0].Timestamp != wantTimestamp {
		t.Errorf("messages[0].Timestamp = %q, want %q", messages[0].Timestamp, wantTimestamp)
	}
}

// TestFetchLatestSession_DropsThoughtParts：ADK runner 會把完整 model event（含
// Thought parts）持久化進 session store——live stream 的 Thought guard
// （RunAssistantStream）擋得住當下轉發，擋不住歷史回放。這裡釘住 history-replay
// 這一面：session 裡存了 Thought part，latest-session 回放的文字不得包含它
// （#2447 BE-01A latent guard 的鏡像面）。
func TestFetchLatestSession_DropsThoughtParts(t *testing.T) {
	svc := session.InMemoryService()
	actor := actorpkg.NewStoreActor(23456)
	ctx := context.Background()

	resp, err := svc.Create(ctx, &session.CreateRequest{
		AppName: "hawkr_ai",
		UserID:  actor.UserID,
		State:   map[string]any{actor.StateKey: actor.StateVal},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	ts := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	if err := svc.AppendEvent(ctx, resp.Session, &session.Event{
		ID: resp.Session.ID() + "-model-thought", Author: "model", Timestamp: ts,
		LLMResponse: model.LLMResponse{
			Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{
				{Text: "思考中：買家想要什麼？", Thought: true},
				{Text: "正式回覆"},
			}},
		},
	}); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	_, messages, err := FetchLatestSession(ctx, svc, actor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %+v, want exactly 1", messages)
	}
	if messages[0].Text != "正式回覆" {
		t.Fatalf("Text = %q, want 正式回覆（Thought part 不得混入歷史回放）", messages[0].Text)
	}
}

func TestFetchLatestSession_UserRoleMapping(t *testing.T) {
	// event.Content.Role == genai.RoleUser ("user") → messageItem.Role = "user"；
	// 其餘一律映射成 "model"（既有行為，非本次新增邏輯，這裡只是鎖住）。
	svc := session.InMemoryService()
	actor := actorpkg.NewBuyerActor(2685914682)
	ctx := context.Background()
	resp, err := svc.Create(ctx, &session.CreateRequest{
		AppName: "hawkr_ai",
		UserID:  actor.UserID,
		State:   map[string]any{actor.StateKey: actor.StateVal},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	ts := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	appendTextEvent(t, svc, resp.Session, genai.RoleUser, "我要找貨", ts)

	_, messages, err := FetchLatestSession(ctx, svc, actor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Role != "user" {
		t.Fatalf("messages = %+v, want single message with Role=user", messages)
	}
}
