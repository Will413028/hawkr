package history

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

const fixtureDir = "../../../../../docs/plans/2026-11-19-a9-agent/fixtures/assistant/"

type latestFixture struct {
	RPCResult struct {
		LatestSession struct {
			SessionID string `json:"session_id"`
			Messages  []struct {
				Role string `json:"role"`
				Text string `json:"text"`
			} `json:"messages"`
		} `json:"latest_session"`
	} `json:"rpc_result"`
}

func loadFixture(t *testing.T, name string) latestFixture {
	t.Helper()
	b, err := os.ReadFile(fixtureDir + name)
	if err != nil {
		t.Fatal(err)
	}
	var f latestFixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func newSession(t *testing.T, svc session.Service, buyerID int64, id string) session.Session {
	t.Helper()
	resp, err := svc.Create(t.Context(), &session.CreateRequest{AppName: buyeragent.AppName, UserID: strconv.FormatInt(buyerID, 10), SessionID: id})
	if err != nil {
		t.Fatal(err)
	}
	return resp.Session
}

func appendContent(t *testing.T, svc session.Service, s session.Session, author string, c *genai.Content) {
	t.Helper()
	ev := session.NewEvent(t.Context(), "inv")
	ev.Author = author
	ev.Content = c
	if err := svc.AppendEvent(t.Context(), s, ev); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond) // 事件與 session 的時間戳要有先後
}

// latest_session_empty：沒有任何對話時 session_id 空字串、沒有訊息。
func TestLatestWithNoSessionIsEmpty(t *testing.T) {
	want := loadFixture(t, "latest_session_empty.json").RPCResult.LatestSession
	got, err := Latest(t.Context(), session.InMemoryService(), 4242)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != want.SessionID || len(got.Messages) != len(want.Messages) || got.SessionID != "" {
		t.Fatalf("Latest = %+v，fixture = %+v", got, want)
	}
}

// latest_session_after_chat：只列有文字的事件（tool 呼叫與回應略過）、thought 不外流、時間由舊到新，角色 model 對外是助理。
func TestLatestAfterChatMatchesTheRecordedHistory(t *testing.T) {
	want := loadFixture(t, "latest_session_after_chat.json").RPCResult.LatestSession
	svc := session.InMemoryService()
	s := newSession(t, svc, 4242, "s1")
	appendContent(t, svc, s, "user", genai.NewContentFromText(want.Messages[0].Text, genai.RoleUser))
	appendContent(t, svc, s, "buyer_coordinator", &genai.Content{Role: "model", Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "buyer_order_specialist"}}}})
	appendContent(t, svc, s, "buyer_coordinator", &genai.Content{Role: "user", Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{Name: "buyer_order_specialist"}}}})
	appendContent(t, svc, s, "buyer_coordinator", &genai.Content{Role: "model", Parts: []*genai.Part{{Text: "（內部推理）", Thought: true}, {Text: want.Messages[1].Text}}})

	got, err := Latest(t.Context(), svc, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "s1" || len(got.Messages) != len(want.Messages) {
		t.Fatalf("Latest = %+v，fixture = %+v", got, want)
	}
	for i, m := range got.Messages {
		wantRole := model.RoleUser
		if want.Messages[i].Role == "model" {
			wantRole = model.RoleAssistant
		}
		if m.Role != wantRole || m.Text != want.Messages[i].Text || m.Time.IsZero() {
			t.Fatalf("第 %d 則 = %+v，fixture = %+v", i, m, want.Messages[i])
		}
	}
}

// 以最後更新時間挑最新的對話；別的買家的對話一律不會出現。
func TestLatestPicksTheMostRecentlyUpdatedSessionOfThisBuyerOnly(t *testing.T) {
	svc := session.InMemoryService()
	older := newSession(t, svc, 1, "older")
	newer := newSession(t, svc, 1, "newer")
	other := newSession(t, svc, 2, "others")
	appendContent(t, svc, newer, "user", genai.NewContentFromText("新的", genai.RoleUser))
	appendContent(t, svc, older, "user", genai.NewContentFromText("舊的但剛更新", genai.RoleUser))
	appendContent(t, svc, other, "user", genai.NewContentFromText("別人的", genai.RoleUser))

	got, err := Latest(t.Context(), svc, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "older" || len(got.Messages) != 1 || got.Messages[0].Text != "舊的但剛更新" {
		t.Fatalf("Latest = %+v", got)
	}
	none, err := Latest(t.Context(), svc, 3)
	if err != nil || none.SessionID != "" {
		t.Fatalf("沒有對話的買家 = %+v, %v", none, err)
	}
}
