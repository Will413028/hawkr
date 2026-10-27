package grpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
)

func TestToStepsProto_PropagatesSafeArgs(t *testing.T) {
	steps := []domain.Step{
		{Code: domain.StepExtractSlots, Status: "SUCCEEDED", Label: "抓出品項、數量和交期", SafeArgs: map[string]string{"item": "香蕉", "city": "台北市"}},
		{Code: domain.StepValidateSlots, Status: "SUCCEEDED", Label: "核對條件合不合理"},
	}

	out := toStepsProto(steps)

	require.Len(t, out, 2)
	require.Equal(t, map[string]string{"item": "香蕉", "city": "台北市"}, out[0].GetSafeArgs())
	require.Empty(t, out[1].GetSafeArgs())
}

func TestToTurnResultProto_MapsBlocksTextFallbackAndDialoguePlanSummary(t *testing.T) {
	r := usecase.TurnResult{
		RequestID: 1, ConversationID: 1, RevisionID: 1,
		Reply: "已收到你的訊息。 還想請你補充：配送城市",
		Blocks: []domain.Block{
			{Kind: domain.BlockUnderstood, Text: "已收到你的訊息。"},
			{Kind: domain.BlockNextStep, Text: "還想請你補充：配送城市"},
		},
		DialoguePlan: domain.DialoguePlanV1{
			ActCode: domain.ActAckAndAskRequired, PlanHash: "abc123",
			PolicyVersion: "phase1-v1", TemplateVersion: "phase2-v1",
		},
		// #2549 Phase 3: RenderMode 不再是 toTurnResultProto 內部寫死的常量，
		// fixture 要明確設值才能斷言 DialoguePlanSummary 的映射（見
		// TestToTurnResultProto_RenderMode_ReflectsSourceOfBlocks 專門測動態行為）。
		RenderMode: "TEMPLATE",
	}

	got := toTurnResultProto(r)

	require.Len(t, got.GetBlocks(), 2)
	require.Equal(t, "UNDERSTOOD", got.GetBlocks()[0].GetKind())
	require.Equal(t, "已收到你的訊息。", got.GetBlocks()[0].GetText())
	require.Equal(t, "已收到你的訊息。 還想請你補充：配送城市", got.GetTextFallback())
	require.Empty(t, got.GetEvidenceRefs()) // Phase 2 沒有任何 act 產生 evidenceRefs

	plan := got.GetDialoguePlan()
	require.Equal(t, "ACK_AND_ASK_REQUIRED", plan.GetActCode())
	require.Equal(t, "abc123", plan.GetPlanHash())
	require.Equal(t, "phase1-v1", plan.GetPolicyVersion())
	require.Equal(t, "TEMPLATE", plan.GetRenderMode())
	require.Equal(t, "phase2-v1", plan.GetTemplateVersion())

	require.NotEmpty(t, got.GetMessageId())
	require.Equal(t, got.GetRevisionId(), got.GetMessageId(), "Phase 2: messageId == revisionId (one message per revision)")

	_, err := time.Parse(time.RFC3339, got.GetCreatedAt())
	require.NoError(t, err, "createdAt must be RFC3339/ISO8601")
}

// TestToTurnResultProto_RenderMode_ReflectsSourceOfBlocks pins Task 11 (#2549
// Phase 3): DialoguePlanSummary.RenderMode must reflect the actual source of
// this turn's blocks (usecase.TurnResult.RenderMode, set by Tasks 8-10's
// realizeOrFallback), not a hardcoded "TEMPLATE" literal.
func TestToTurnResultProto_RenderMode_ReflectsSourceOfBlocks(t *testing.T) {
	r := usecase.TurnResult{
		DialoguePlan: domain.DialoguePlanV1{ActCode: domain.ActAckAndAskRequired},
		RenderMode:   "MODEL",
	}
	got := toTurnResultProto(r)
	require.Equal(t, "MODEL", got.DialoguePlan.RenderMode)

	r.RenderMode = "TEMPLATE"
	got = toTurnResultProto(r)
	require.Equal(t, "TEMPLATE", got.DialoguePlan.RenderMode)
}

// TestToSlotsProto_ItemResolution_ReachesTheWire 釘住品項解析結果真的上得了 wire。
//
// 這是 ADR 2026-09-04-crop-resolution-into-slots 最後一項 followup 的迴歸哨兵：
// 解析結果決定報價的價格基準（同一大類內品種實測差 2.96 倍），刀 1 讓它出現在
// recordedThisTurn 的 chip，但 slots 面板本身看不到——前端因此無法對「系統猜的」
// 與「你選的」分流呈現。少了這個 mapping，proto 欄位加了也永遠是 nil。
func TestToSlotsProto_ItemResolution_ReachesTheWire(t *testing.T) {
	item := "高麗菜"
	price := 95

	got := toSlotsProto(domain.Slots{
		Item: &item,
		ItemResolution: &domain.ItemResolution{
			Kind: domain.ItemResolvedSelfPriced, Label: "高麗菜", CategoryID: "17",
			PricePerKg: &price, Provenance: domain.ItemResolutionByBuyerChoice,
		},
	})

	require.NotNil(t, got.ItemResolution)
	require.Equal(t, "SELF_PRICED", got.ItemResolution.GetKind())
	require.Equal(t, "高麗菜", got.ItemResolution.GetLabel())
	require.Equal(t, "BUYER_CHOICE", got.ItemResolution.GetProvenance())
	require.Equal(t, "高麗菜，單價 95 元／公斤", got.ItemResolution.GetDisplayValue(),
		"顯示字串由伺服器組——前端拿 kind＋label 自己拼會跟 chip 漂成兩套文案")
	require.Equal(t, "95", got.ItemResolution.GetPricePerKg())
}

// TestToSlotsProto_SelfPriced_PricePerKgIsAString 釘住金額對外一律字串
// （CLAUDE.md §4，避免 JavaScript 精度問題）——這條在 proto 邊界很容易寫成
// int64 而測不出來，因為兩者在 Go 端都能編譯。
func TestToSlotsProto_SelfPriced_PricePerKgIsAString(t *testing.T) {
	item := "阿嬤自種特殊品種高麗菜"

	got := toSlotsProto(domain.Slots{
		Item:           &item,
		ItemResolution: domain.NewSelfPricedItemResolution(95, "", domain.CategoryAbsent("測試")),
	})

	require.NotNil(t, got.ItemResolution)
	require.Equal(t, "SELF_PRICED", got.ItemResolution.GetKind())
	require.Equal(t, "95", got.ItemResolution.GetPricePerKg())
	require.Equal(t, "自訂品項，單價 95 元／公斤", got.ItemResolution.GetDisplayValue())
}

// TestToSlotsProto_NoResolution_OmitsTheField：尚未解析時不帶這個欄位，前端據此
// 判斷「還在解析中」而不是「解析成一個空的東西」。
func TestToSlotsProto_NoResolution_OmitsTheField(t *testing.T) {
	item := "高麗菜"
	got := toSlotsProto(domain.Slots{Item: &item})
	require.Nil(t, got.ItemResolution)
}
