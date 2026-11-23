package turn

import (
	"context"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

// 每個有 tool 的 agent 都以 VALIDATED 模式把可呼叫的函式限制在自己宣告的 tool（A9-D15 第 3 點修訂）。
// 預期名單取自同一個請求實際宣告的函式，不另寫一份。
func TestEveryAgentWithToolsRestrictsFunctionCallingToItsOwnToolNames(t *testing.T) {
	cases := []struct {
		topic model.Topic
		agent string
		want  int
	}{
		{model.TopicOrders, "buyer_order_specialist", 2},
		{model.TopicTracking, "buyer_tracking_specialist", 1},
		{model.TopicChat, "buyer_coordinator", 2},
	}
	for _, c := range cases {
		t.Run(c.agent, func(t *testing.T) {
			llm := textLLM("好")
			e := newExecutor(t, llm, &fakeOrders{}, nil)
			ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: c.topic, Message: "查"})
			if err != nil {
				t.Fatal(err)
			}
			collect(t, e, buyer, ref.TurnID, "")
			first := llm.seen()[0]
			if first.Agent != c.agent {
				t.Fatalf("agent = %s，要 %s", first.Agent, c.agent)
			}
			if first.CallingMode != "VALIDATED" {
				t.Fatalf("函式呼叫模式 = %q，要 VALIDATED", first.CallingMode)
			}
			declared := slices.Clone(first.DeclaredNames)
			allowed := slices.Clone(first.AllowedNames)
			slices.Sort(declared)
			slices.Sort(allowed)
			if len(declared) != c.want || !slices.Equal(declared, allowed) {
				t.Fatalf("AllowedFunctionNames = %v，要等於宣告的函式 %v（%d 個）", allowed, declared, c.want)
			}
		})
	}
}
