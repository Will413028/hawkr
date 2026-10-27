package usecase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

func TestBuildSurfaceRealizePrompt_IncludesPlanAndFacts(t *testing.T) {
	sc := SurfaceContext{
		Plan: domain.DialoguePlanV1{
			ActCode:           domain.ActAckAndAskOptional,
			AllowedClaimCodes: []string{"REQUEST_FACT_RECORDED"},
			MaxSentences:      2,
			ToneHints:         domain.ToneHints{Verbosity: "CONCISE"},
		},
		Args: domain.SafeRenderArgs{
			SalientFactLabels:  []string{"數量：50"},
			NextQuestionPrompt: "希望什麼時候送達？",
		},
		RecentNarratives: []string{"好的，主要資訊都到齊了。"},
	}
	got := buildSurfaceRealizePrompt(sc)
	for _, want := range []string{"ACK_AND_ASK_OPTIONAL", "REQUEST_FACT_RECORDED", "數量：50", "希望什麼時候送達？", "好的，主要資訊都到齊了。"} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt 缺少 %q：\n%s", want, got)
		}
	}
}

func TestSurfaceRealizeInstruction_ForbidsUnlistedArabicNumerals(t *testing.T) {
	// fact-echo（Task 4）擋的是輸出，這條 prompt 約束擋的是來源——兩者必須並存，
	// 否則模型講「還有 2 項」每次都會被擋成 fallback。
	if !strings.Contains(surfaceRealizeInstruction, "阿拉伯數字") {
		t.Fatal("surfaceRealizeInstruction 必須明文禁止事實清單以外的阿拉伯數字")
	}
}

// TestSurfaceRealizeInstruction_ForbidsCandidateAndSellerCounts 是 hardening
// 回合的直接動機：「除了事實值以外不得出現阿拉伯數字，需要表達數量時用中文
// 數字」這句話本身，同時教會模型一種 fact-echo（阿拉伯數字專用）看不見的
// 講法——「已經幫你找到三個合適的賣家」三層全過，卻同時捏造了 candidate
// count 與 search-completed 副作用，兩者都在設計書 55 §8.1 規則 1 的明文
// 禁止清單裡。這支測試守的是「候選／賣家數量禁令」這一句話本身不會在未來
// 被順手刪掉，不是重新驗證中文數字換寫本身（那件事仍然合法且必要）。
func TestSurfaceRealizeInstruction_ForbidsCandidateAndSellerCounts(t *testing.T) {
	if !strings.Contains(surfaceRealizeInstruction, "候選") || !strings.Contains(surfaceRealizeInstruction, "賣家") {
		t.Fatal("surfaceRealizeInstruction 必須明文禁止提及候選／賣家的數量與狀態")
	}
}

func TestBuildBlocksSchema_HasClosedKindEnum(t *testing.T) {
	schema := buildBlocksSchema()
	blocks, ok := schema.Properties["blocks"]
	if !ok {
		t.Fatal("schema 缺少 blocks 屬性")
	}
	kind, ok := blocks.Items.Properties["kind"]
	if !ok {
		t.Fatal("block 缺少 kind 屬性")
	}
	if len(kind.Enum) != 6 {
		t.Fatalf("kind enum 應該是 doc 57 §3.3 的 6 個值，got %v", kind.Enum)
	}
}

// TestSurfaceRealizer_DeepSeek_SendsReasoningEffortNone 是 whole-branch review
// C1 的守門測試，寫法比照 TestDeepSeekSlotExtractor_Extract_SendsToolCallRequest
// （deepseek_extractor_test.go）：SurfaceRealizer 走的是跟 slot extractor 同一
// 條強制 tool_choice 路徑（applyBlocksToolCallConfig 設
// FunctionCallingConfigModeAny ＝ wire 上的 tool_choice:"required"），因此同一
// 條 DeepSeek 限制對它一字不差地成立，而在補上這支測試之前 SurfaceRealizer 沒有
// 任何傳輸層測試——手刻 client 漏掉 reasoning.effort=none 就是這樣溜過整條分支的。
func TestSurfaceRealizer_DeepSeek_SendsReasoningEffortNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.Equal(t, "/responses", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, llmprovider.DeepSeekModelName, body["model"])

		// tool_choice:"required" 是 applyBlocksToolCallConfig 的 wire 形狀，
		// 也正是 Thinking mode 會拒收的那個東西——兩者要一起釘住，否則單看
		// reasoning 那條斷言看不出它在防什麼。
		require.Equal(t, "required", body["tool_choice"])
		tools, ok := body["tools"].([]any)
		require.True(t, ok, "請求應帶 tools 陣列")
		require.Len(t, tools, 1)

		require.Equal(t, map[string]any{"effort": "none"}, body["reasoning"],
			"少了 reasoning.effort=none，DeepSeek 會因 Thinking mode 拒收強制 tool_choice（HTTP 400）")

		resp := map[string]any{
			"id": "resp_test", "object": "response", "created_at": 0, "status": "completed",
			"model": llmprovider.DeepSeekModelName,
			"output": []map[string]any{{
				"type": "function_call", "id": "fc_test", "call_id": "call_test",
				"name":      realizeBlocksToolName,
				"status":    "completed",
				"arguments": `{"blocks":[{"kind":"UNDERSTOOD","text":"好的，已經幫你記下來了。","claimCodes":["REQUEST_FACT_RECORDED"]}]}`,
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	r := NewSurfaceRealizer(llmprovider.ProcurementForTest(&config.Config{
		ExtractorModel: llmprovider.DeepSeekModelName, DeepSeekAPIKey: "test-key",
	}, srv.URL))

	blocks, err := r.Realize(t.Context(), SurfaceContext{
		Plan: domain.DialoguePlanV1{
			ActCode:           domain.ActAckAndAskOptional,
			AllowedClaimCodes: []string{string(domain.ClaimRequestFactRecorded)},
			MaxSentences:      2,
		},
		Args: domain.SafeRenderArgs{SalientFactLabels: []string{"數量：50"}},
	})
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, domain.BlockUnderstood, blocks[0].Kind)
}

// 未知 provider 不得建出「看起來能用」的 realizer。
//
// ⚠️ 這條斷言的形狀 2026-09-11 改過兩次：原本斷言 NewSurfaceRealizer 回 error，
// 但那一支是 fx.Provide 的直接入口、回 error 就是 fx abort（#3329）；錯誤訊息的
// 形狀現在由 llmprovider 自己的測試守，這裡守的是**消費者這一側的契約**——
// 打錯字時拿到的是 fail-closed 的停用實作，不是一顆會靜默失敗的 realizer。
func TestNewSurfaceRealizer_RejectsUnknownProvider(t *testing.T) {
	r := NewSurfaceRealizer(llmprovider.NewProcurement(&config.Config{ExtractorModel: "nope"}, zap.NewNop()))
	require.IsType(t, disabledSurfaceRealizer{}, r)
}

// SurfaceRealizer 那一側同樣要降級——#3329 修掉另外三個建構子時這支還不存在
// （同日稍早才由 #3347 復原），於是原樣帶回「缺金鑰＝fx abort＝59 支 RPC 全滅」。
// 三個分支各驗一次：共用 llmprovider 的判定是實作細節，契約要各自守。
func TestNewSurfaceRealizer_DegradesFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *config.Config
	}{
		{"預設 gemini 無金鑰", &config.Config{ExtractorModel: "gemini-2.5-flash"}},
		{"明確選 deepseek 無金鑰", &config.Config{ExtractorModel: llmprovider.DeepSeekModelName}},
		{"EXTRACTOR_MODEL 打錯字", &config.Config{ExtractorModel: "deepsekk"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewSurfaceRealizer(llmprovider.NewProcurement(tc.cfg, zap.NewNop()))
			require.IsType(t, disabledSurfaceRealizer{}, r)

			// 陰性斷言：不得靜默回空 blocks。空 blocks 會通過 realizeOrFallback 的
			// 錯誤分支以外的路徑，validator 的 blank_text 才會擋——那是繞了一圈的
			// 偶然保護，不是契約。
			blocks, callErr := r.Realize(t.Context(), SurfaceContext{})
			require.Empty(t, blocks)
			require.Error(t, callErr, "停用實作必須在呼叫時 fail-closed")
			assertProcurementDisabled(t, callErr)
		})
	}
}

func TestModelLLMSurfaceRealizer_RecordsTokenUsage(t *testing.T) {
	collect := observability.InstallManualReader(t)
	fake := &testkit.FakeLLM{Resp: &model.LLMResponse{
		Content: &genai.Content{Parts: []*genai.Part{{
			FunctionCall: &genai.FunctionCall{
				Name: realizeBlocksToolName,
				Args: map[string]any{"blocks": []any{map[string]any{
					"kind": string(domain.BlockNextStep), "text": "接下來請告訴我數量", "claimCodes": []any{"ASK_NEXT"},
				}}},
			},
		}}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 800, CandidatesTokenCount: 40},
	}}
	r := &modelLLMSurfaceRealizer{
		llm:         fake,
		modelName:   "test",
		applyConfig: applyBlocksToolCallConfig,
		parse:       toolCallParseBlocks,
		classifyErr: classifyOpenAICompatExtractErr,
	}

	blocks, err := r.Realize(context.Background(), SurfaceContext{})
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	rm := collect()
	v, ok := observability.CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "realize", "kind": "prompt"})
	require.True(t, ok)
	require.EqualValues(t, 800, v)
	v, ok = observability.CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "realize", "kind": "completion"})
	require.True(t, ok)
	require.EqualValues(t, 40, v)
}
