//go:build integration

package assistant

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"connectrpc.com/connect"
	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/assistant/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func newDB(t *testing.T) *db.DB {
	t.Helper()
	return pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "assistant", Dir: assistant.Migrations()}))
}

type replyLLM struct{ reply string }

func (replyLLM) Name() string { return "reply" }

func (l replyLLM) GenerateContent(context.Context, *adkmodel.LLMRequest, bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		yield(&adkmodel.LLMResponse{Content: genai.NewContentFromText(l.reply, genai.RoleModel), FinishReason: genai.FinishReasonStop}, nil)
	}
}

type noOrders struct{}

func (noOrders) MyOrders(context.Context, int64, string, int, int) ([]assistant.BuyerOrder, error) {
	return nil, nil
}

func (noOrders) OrderDetail(context.Context, int64, string) (assistant.BuyerOrderDetail, error) {
	return assistant.BuyerOrderDetail{}, errors.New("no orders")
}

// 整條路徑：Connect 送出 → 真正的輪次執行器與 Postgres 的對話 → 串流到 finished → 歷史讀得到；
// 同一個 client_request_id 重送回放同一輪、不重複寫對話；別的買家讀不到這一輪也讀不到這段歷史。
func TestHandlerPathSendWatchResendAndLatest(t *testing.T) {
	m := assistant.New(assistant.Deps{DB: newDB(t), LLM: replyLLM{reply: "你好！"}, Orders: noOrders{}, Now: time.Now})
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	c, _ := setup(t, m, 30*time.Second)

	first, err := c.SendAssistantMessage(t.Context(), asAccount(connect.NewRequest(send(pb.AssistantTopic_ASSISTANT_TOPIC_CHAT, "嗨")), accountID))
	if err != nil {
		t.Fatal(err)
	}
	resend, err := c.SendAssistantMessage(t.Context(), asAccount(connect.NewRequest(send(pb.AssistantTopic_ASSISTANT_TOPIC_CHAT, "嗨")), accountID))
	if err != nil || resend.Msg.GetTurnId() != first.Msg.GetTurnId() || resend.Msg.GetSessionId() != first.Msg.GetSessionId() {
		t.Fatalf("重送 = %v, %v；要回放第一次的 %v", resend, err, first)
	}
	if _, err := c.SendAssistantMessage(t.Context(), asAccount(connect.NewRequest(send(pb.AssistantTopic_ASSISTANT_TOPIC_CHAT, "別的內容")), accountID)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("同鍵不同內容 = %v，要 INVALID_ARGUMENT", err)
	}

	events, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: first.Msg.GetTurnId()}, accountID)
	if err != nil || len(events) < 2 {
		t.Fatalf("events = %v, %v", events, err)
	}
	var text string
	for _, ev := range events[:len(events)-1] {
		text += ev.GetText().GetText()
	}
	if last := events[len(events)-1]; text != "你好！" || last.GetFinished().GetSessionId() != first.Msg.GetSessionId() {
		t.Fatalf("text = %q，最後一筆 = %v", text, last)
	}
	rest, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: first.Msg.GetTurnId(), Cursor: events[len(events)-1].GetCursor()}, accountID)
	if err != nil || len(rest) != 0 {
		t.Fatalf("接續 = %v, %v", rest, err)
	}
	if _, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: first.Msg.GetTurnId(), Cursor: "9999"}, accountID); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("cursor 超前 = %v", err)
	}

	hist, err := c.GetLatestAssistantSession(t.Context(), asAccount(connect.NewRequest(&pb.GetLatestAssistantSessionRequest{}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	h := hist.Msg
	if h.GetSessionId() != first.Msg.GetSessionId() || len(h.GetMessages()) != 2 || h.GetMessages()[0].GetText() != "嗨" || h.GetMessages()[1].GetText() != "你好！" {
		t.Fatalf("歷史 = %v（重送不得重複寫對話）", h)
	}

	const otherAccount = int64(8)
	if _, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: first.Msg.GetTurnId()}, otherAccount); err == nil {
		t.Fatal("別的買家訂閱到了這一輪")
	}
}
