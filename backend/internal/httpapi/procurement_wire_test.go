package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	procurementv1alpha1 "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
)

const turnTestdata = "../modules/procurement/internal/turn/testdata"

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(turnTestdata, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestProcurementTurnWireMatchesProto(t *testing.T) {
	strict := protojson.UnmarshalOptions{DiscardUnknown: false}
	for _, name := range []string{"turn_result.golden.json", "turn_result_interrupt.golden.json"} {
		var ev procurementv1alpha1.WatchRunResponse
		if err := strict.Unmarshal(readGolden(t, name), &ev); err != nil {
			t.Fatalf("%s 不是合法的 WatchRunResponse：%v", name, err)
		}
		tr := ev.GetTurnResult()
		if tr == nil {
			t.Fatalf("%s：turn_result 沒有解析成 TurnResult", name)
		}
		if tr.GetRequestId() == "" || tr.GetConversationId() == "" || tr.GetRevisionId() == "" || tr.GetMessageId() == "" || tr.GetRunId() == "" ||
			tr.GetReply() == "" || tr.GetTextFallback() == "" || tr.GetCreateTime() == nil || len(tr.GetBlocks()) == 0 {
			t.Errorf("%s：必填內容缺漏：%v", name, tr)
		}
		if len(tr.GetRecordedThisTurn()) == 0 || tr.GetSlots().GetItem() == "" {
			t.Errorf("%s：slots 或 recorded_this_turn 沒有解析出來：%v", name, tr)
		}
		for _, b := range tr.GetBlocks() {
			if b.GetKind() == procurementv1alpha1.BlockKind_BLOCK_KIND_UNSPECIFIED {
				t.Errorf("%s：block kind 未指定", name)
			}
		}
		for _, r := range tr.GetRejected() {
			if r.GetKey() == procurementv1alpha1.SlotKey_SLOT_KEY_UNSPECIFIED || r.GetReason() == procurementv1alpha1.RejectionReason_REJECTION_REASON_UNSPECIFIED {
				t.Errorf("%s：rejected 的列舉值未指定：%v", name, r)
			}
		}
	}

	var ok procurementv1alpha1.WatchRunResponse
	if err := strict.Unmarshal(readGolden(t, "turn_result.golden.json"), &ok); err != nil {
		t.Fatal(err)
	}
	s := ok.GetTurnResult().GetSlots()
	if s.GetItemResolution().GetKind() != procurementv1alpha1.ItemResolutionKind_ITEM_RESOLUTION_KIND_CATEGORY_CONFIRMED ||
		s.GetBudgetPerUnit() != 30 || s.GetQuantity() != 50 || s.GetDeliveryDate().GetDay() != 1 || ok.GetTurnResult().GetInterrupt() != nil {
		t.Errorf("成功輪次的 slots 不對：%v", ok.GetTurnResult())
	}

	var paused procurementv1alpha1.WatchRunResponse
	if err := strict.Unmarshal(readGolden(t, "turn_result_interrupt.golden.json"), &paused); err != nil {
		t.Fatal(err)
	}
	tr := paused.GetTurnResult()
	if tr.GetInterrupt().GetKind() != procurementv1alpha1.InterruptKind_INTERRUPT_KIND_CLARIFICATION || tr.GetInterrupt().GetInterruptId() == "" {
		t.Errorf("中斷輪次要帶 interrupt：%v", tr)
	}
	if q := tr.GetQuestion(); q.GetSlotKey() != procurementv1alpha1.SlotKey_SLOT_KEY_ITEM_DISAMBIGUATION || !q.GetAllowFreeText() || q.GetCanSkip() || len(q.GetOptions()) != 3 {
		t.Errorf("消歧卡不對：%v", q)
	}
}

func TestProcurementInterruptPayloadQuestionMatchesProto(t *testing.T) {
	for _, tc := range []struct {
		file       string
		stage      string
		candidates int
		options    int
		describe   bool
	}{
		{"interrupt_payload.golden.json", "ITEM", 2, 3, false},
		{"interrupt_payload_group.golden.json", "GROUP", 2, 3, false},
		{"interrupt_payload_describe.golden.json", "DESCRIBE", 0, 0, true},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			var payload struct {
				Stage      string            `json:"stage"`
				Query      string            `json:"query"`
				Candidates []json.RawMessage `json:"candidates"`
				Question   json.RawMessage   `json:"question"`
			}
			if err := json.Unmarshal(readGolden(t, tc.file), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Stage != tc.stage || payload.Query == "" || len(payload.Candidates) != tc.candidates {
				t.Errorf("payload = %+v", payload)
			}
			var q procurementv1alpha1.ClarifyQuestion
			if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(payload.Question, &q); err != nil {
				t.Fatalf("payload.question 不是合法的 ClarifyQuestion：%v", err)
			}
			if q.GetSlotKey() != procurementv1alpha1.SlotKey_SLOT_KEY_ITEM_DISAMBIGUATION || len(q.GetOptions()) != tc.options || !q.GetAllowFreeText() || q.GetPrompt() == "" {
				t.Errorf("question = %v", &q)
			}
			if tc.describe && len(q.GetOptions()) != 0 {
				t.Errorf("描述卡沒有選項：%v", &q)
			}
		})
	}
}
