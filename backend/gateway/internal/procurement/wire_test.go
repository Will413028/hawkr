package procurement

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

// TestTurnResultFromProto_DropsReasoning：TurnResultDTO 不得含 reasoning key——
// 這個 DTO 同時是 unary /turns、/answers、/rewinds 回應與 SSE
// STATE_SNAPSHOT/RUN_FINISHED 的 payload，一處帶出就是三個面一起漏（#2447 BE-01A）。
//
// proto 的 TurnResult.reasoning 已於 #2447 reserved 化，上游不再可能 populate，
// 主要護欄因此上移到 protoc。這支測試留著守的是 DTO 自己長出這個 key。
func TestTurnResultFromProto_DropsReasoning(t *testing.T) {
	dto := TurnResultFromProto(&pb.TurnResult{Reply: "已記下"})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "reasoning") {
		t.Fatalf("TurnResultDTO 不得含 reasoning, got %s", b)
	}
	if !strings.Contains(string(b), `"reply":"已記下"`) {
		t.Fatalf("reply 應照常帶出, got %s", b)
	}
}

// TestTurnResultFromProto_MapsNextAction 釘住 Task 13：NextAction 從 ai-service
// 的 usecase.TurnResult 一路走 gRPC TurnResult.next_action，經這裡的
// TurnResultFromProto 帶出到 Gateway 的 camelCase nextAction key——SubmitTurn／
// SubmitAnswer／SubmitTurnStream 的終態事件都共用這個函式（見本檔上方
// TurnResultDTO 的 doc comment），一次驗證即涵蓋全部呼叫端。
func TestTurnResultFromProto_MapsNextAction(t *testing.T) {
	dto := TurnResultFromProto(&pb.TurnResult{Reply: "已記下", NextAction: "REVIEW_PRICE_PREVIEW"})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"nextAction":"REVIEW_PRICE_PREVIEW"`) {
		t.Fatalf("nextAction 應該原封不動帶出, got %s", b)
	}
}

// TestTurnResultFromProto_OmitsEmptyNextAction 驗證 nextAction 是 omitempty：
// 空字串（沒有提示的絕大多數情況）不該讓這個 key 出現在回應裡。
func TestTurnResultFromProto_OmitsEmptyNextAction(t *testing.T) {
	dto := TurnResultFromProto(&pb.TurnResult{Reply: "已記下"})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "nextAction") {
		t.Fatalf("nextAction 為空字串時不該出現在回應裡（omitempty）, got %s", b)
	}
}

func TestTurnResultFromProto_MapsBlocksAndDialoguePlanSummary(t *testing.T) {
	dto := TurnResultFromProto(&pb.TurnResult{
		Reply:        "已收到你的訊息。",
		TextFallback: "已收到你的訊息。",
		Blocks:       []*pb.Block{{Kind: "UNDERSTOOD", Text: "已收到你的訊息。"}},
		DialoguePlan: &pb.DialoguePlanSummary{
			ActCode: "ACK_AND_ASK_REQUIRED", PlanHash: "abc123",
			PolicyVersion: "phase1-v1", RenderMode: "TEMPLATE", TemplateVersion: "phase2-v1",
		},
		MessageId: "1", CreatedAt: "2026-08-23T00:00:00Z",
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"blocks":[{"kind":"UNDERSTOOD","text":"已收到你的訊息。"}]`) {
		t.Fatalf("blocks not correctly marshaled, got %s", s)
	}
	if !strings.Contains(s, `"textFallback":"已收到你的訊息。"`) {
		t.Fatalf("textFallback missing, got %s", s)
	}
	if !strings.Contains(s, `"dialogueActCode":"ACK_AND_ASK_REQUIRED"`) {
		t.Fatalf("dialoguePlan.dialogueActCode missing, got %s", s)
	}
	if strings.Contains(s, `"evidenceRefs"`) {
		t.Fatalf("evidenceRefs should be omitted when empty (omitempty), got %s", s)
	}
}

// TestTurnResultFromProto_MapsQuota 釘住 Task 15：買家會員分級的月配額快照從
// ai-service 的 pb.TurnResult.quota 一路映射到 Gateway 的 camelCase quota key。
func TestTurnResultFromProto_MapsQuota(t *testing.T) {
	dto := TurnResultFromProto(&pb.TurnResult{
		Reply: "已記下",
		Quota: &pb.AgentQuotaSnapshot{PlanCode: "premium", Remaining: "42", Limit: "1000", ResetAt: "2026-03-01T00:00:00Z"},
	})
	if dto.Quota.PlanCode != "premium" {
		t.Fatalf("Quota.PlanCode 應為 premium, got %s", dto.Quota.PlanCode)
	}
	if dto.Quota.Remaining != "42" {
		t.Fatalf("Quota.Remaining 應為 42, got %s", dto.Quota.Remaining)
	}
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"quota":{"planCode":"premium","remaining":"42","limit":"1000","resetAt":"2026-03-01T00:00:00Z"}`) {
		t.Fatalf("quota not correctly marshaled, got %s", s)
	}
}

// TestTurnResultFromProto_QuotaNilSafe 釘住 Task 13 的既有行為：idempotent
// replay 時 ai-service 不會 populate TurnResult.Quota（保持 nil），Gateway 端
// 必須拿到零值 AgentQuotaSnapshotDTO 而不是 panic。
func TestTurnResultFromProto_QuotaNilSafe(t *testing.T) {
	dto := TurnResultFromProto(&pb.TurnResult{Reply: "已記下"})
	if dto.Quota != (AgentQuotaSnapshotDTO{}) {
		t.Fatalf("Quota 應為零值 AgentQuotaSnapshotDTO, got %+v", dto.Quota)
	}
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"quota":{"planCode":"","remaining":"","limit":"","resetAt":""}`) {
		t.Fatalf("quota should marshal as zero-value object when nil, got %s", b)
	}
}

// TestStepsFromProto_PropagatesSafeArgs：safeArgs 是前端動態文案模板要用的
// 一小撮 slots 值（見 ai-service domain.Step.SafeArgs），只在少數 code 上非空，
// 其餘 code 必須完全不帶這個 key（omitempty），不能序列化成 "safeArgs":{}。
func TestStepsFromProto_PropagatesSafeArgs(t *testing.T) {
	out := StepsFromProto([]*pb.Step{
		{Code: "EXTRACT_SLOTS", Status: "SUCCEEDED", Label: "抓出品項、數量和交期", SafeArgs: map[string]string{"item": "高麗菜", "city": "台中市"}},
		{Code: "VALIDATE_SLOTS", Status: "SUCCEEDED", Label: "核對條件合不合理"},
	})
	if got := out[0].SafeArgs; got["item"] != "高麗菜" || got["city"] != "台中市" {
		t.Fatalf("safeArgs not carried: %+v", got)
	}
	b, err := json.Marshal(out[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "safeArgs") {
		t.Fatalf("VALIDATE_SLOTS 沒有 safeArgs 時不該序列化出這個 key, got %s", b)
	}
}

// TestSlotsFromProto_ItemResolution_ShapeAndTypes 釘住品項解析結果在對外 JSON 上的
// 形狀。三件事各自都能靜默壞掉，而且壞了以後前端只會看到欄位消失、不會有錯誤：
//
//  1. price_per_kg 是**不帶引號的數字**——SlotsDTO 整組刻意鏡射 domain.Slots 的
//     JSON 形狀（quantity／budget_per_unit 也是這樣從 wire 的字串轉回 int）。
//  2. fallback_group_code **不得**出現：它是 crop 查無行情時退一級重查的內部查詢
//     鍵，買家無從使用，不該進對外契約。
//  3. display_value 一定要在：前端拿 kind＋label 自己拼會跟 recordedThisTurn 的
//     chip 漂成兩套文案，而它們在畫面上相鄰。
func TestSlotsFromProto_ItemResolution_ShapeAndTypes(t *testing.T) {
	price := "95"
	dto := SlotsFromProto(&pb.Slots{
		ItemResolution: &pb.ItemResolution{
			Kind: "SELF_PRICED", Label: "甘藍-初秋",
			Provenance: "BUYER_CHOICE", PricePerKg: &price,
			DisplayValue: "自訂品項，單價 95 元／公斤",
		},
	})

	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, `"price_per_kg":95`) {
		t.Errorf("price_per_kg 必須是不帶引號的數字，got %s", got)
	}
	if strings.Contains(got, "fallback_group_code") {
		t.Errorf("內部查詢鍵不得進對外契約，got %s", got)
	}
	if !strings.Contains(got, `"display_value":"自訂品項，單價 95 元／公斤"`) {
		t.Errorf("display_value 必須由伺服器組好送出，got %s", got)
	}
	if !strings.Contains(got, `"provenance":"BUYER_CHOICE"`) {
		t.Errorf("provenance 是前端分流呈現的依據，got %s", got)
	}
}

// TestSlotsFromProto_NoItemResolution_OmitsTheKey：尚未解析時整個 key 不出現
// （omitempty），前端據此判斷「還沒解析」而不是「解析成一個空物件」。
func TestSlotsFromProto_NoItemResolution_OmitsTheKey(t *testing.T) {
	b, err := json.Marshal(SlotsFromProto(&pb.Slots{}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "item_resolution") {
		t.Errorf("未解析時不得帶這個 key，got %s", b)
	}
}
