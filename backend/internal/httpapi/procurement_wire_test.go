package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	procurementv1alpha1 "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
)

// procurement 模組不能 import 這裡的生成碼，它的 turn_result 事件與中斷 payload 是手工組的 proto JSON（A9-3b-2c G12）。
// 模組的測試把產生器輸出釘成 golden（testdata），這裡以 protojson（DiscardUnknown=false）把同一批 golden 解析回 proto：
// 欄位名或列舉值名漂移時，一邊的測試會紅。
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
	var payload struct {
		Stage      string            `json:"stage"`
		Query      string            `json:"query"`
		Candidates []json.RawMessage `json:"candidates"`
		Question   json.RawMessage   `json:"question"`
	}
	if err := json.Unmarshal(readGolden(t, "interrupt_payload.golden.json"), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Stage != "ITEM" || payload.Query == "" || len(payload.Candidates) == 0 {
		t.Errorf("payload = %+v", payload)
	}
	var q procurementv1alpha1.ClarifyQuestion
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(payload.Question, &q); err != nil {
		t.Fatalf("payload.question 不是合法的 ClarifyQuestion：%v", err)
	}
	if q.GetSlotKey() != procurementv1alpha1.SlotKey_SLOT_KEY_ITEM_DISAMBIGUATION || len(q.GetOptions()) != len(payload.Candidates)+1 {
		t.Errorf("question = %v", &q)
	}
}
