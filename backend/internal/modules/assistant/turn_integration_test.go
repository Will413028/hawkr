//go:build integration

package assistant

import (
	"context"
	"errors"
	"iter"
	"strconv"
	"testing"
	"time"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

type replyLLM struct{ reply string }

func (replyLLM) Name() string { return "reply" }

func (l replyLLM) GenerateContent(context.Context, *adkmodel.LLMRequest, bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		yield(&adkmodel.LLMResponse{Content: genai.NewContentFromText(l.reply, genai.RoleModel), FinishReason: genai.FinishReasonStop}, nil)
	}
}

type noOrders struct{}

func (noOrders) MyOrders(context.Context, int64, string, int, int) ([]BuyerOrder, error) {
	return nil, nil
}

func (noOrders) OrderDetail(context.Context, int64, string) (BuyerOrderDetail, error) {
	return BuyerOrderDetail{}, errors.New("no orders")
}

// 輪次用真正的 adksession 跑完：回覆串流到 finished，使用者訊息與助理回覆都寫進 Postgres 的對話。
func TestSubmitTurnPersistsTheConversation(t *testing.T) {
	d := newDB(t)
	m := New(Deps{DB: d, LLM: replyLLM{reply: "你好！"}, Orders: noOrders{}, Now: time.Now})
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })

	const buyerID = int64(4242)
	ref, err := m.SubmitTurn(context.Background(), SubmitTurnInput{BuyerID: buyerID, Topic: TopicChat, Message: "嗨"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	seq, err := m.WatchTurn(ctx, buyerID, ref.TurnID, "")
	if err != nil {
		t.Fatal(err)
	}
	var text string
	var last TurnEvent
	for ev, err := range seq {
		if err != nil {
			t.Fatal(err)
		}
		text += ev.Text
		last = ev
	}
	if text != "你好！" || last.Kind != EventFinished || last.SessionID != ref.SessionID {
		t.Fatalf("事件：text=%q last=%+v", text, last)
	}

	got, err := m.sessions.Get(ctx, &session.GetRequest{AppName: "hawkr_ai", UserID: strconv.FormatInt(buyerID, 10), SessionID: ref.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	if n := got.Session.Events().Len(); n != 2 {
		t.Fatalf("對話裡有 %d 個 event，要 2（使用者、助理）", n)
	}
	if v, err := got.Session.State().Get("buyer_id"); err != nil || v != strconv.FormatInt(buyerID, 10) {
		t.Fatalf("session state buyer_id = %v, %v", v, err)
	}
}

// 輪次在背景寫 session 並呼叫 LLM，不得綁在呼叫者的 transaction 上（規則 5）。
func TestSubmitTurnRefusesAnOuterTransaction(t *testing.T) {
	d := newDB(t)
	m := New(Deps{DB: d, LLM: replyLLM{reply: "x"}, Orders: noOrders{}, Now: time.Now})
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })

	err := d.InTx(context.Background(), func(ctx context.Context) error {
		_, err := m.SubmitTurn(ctx, SubmitTurnInput{BuyerID: 1, Topic: TopicChat, Message: "嗨"})
		return err
	})
	if !errors.Is(err, ErrOuterTx) {
		t.Fatalf("transaction 內送出 = %v，要 ErrOuterTx", err)
	}
}
