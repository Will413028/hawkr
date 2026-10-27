package llmprovider

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/adk/v2/model"
)

// backend#3329：NewProcurement 的三分支與兩支 gauge。
//
// 🔴 為什麼需要 incomplete 這第二支：#3329 把「缺金鑰」從 CrashLoop 改成降級之後，
// **錯字不再自己現形**。全空與全有都是合法狀態，只有「明確選了一個模型卻沒給它的
// provider key」與「模型名不在登記表」是配置錯誤——只有這兩種要有人去修。
// 合成一個布林的話，值班的人分不出「這個環境刻意沒開 AI」與「有人把 EXTRACTOR_MODEL
// 打錯字」，而後者會讓 RFQ 整條功能靜默死掉。
func TestNewProcurement_ThreeBranches(t *testing.T) {
	cases := []struct {
		name                        string
		cfg                         config.Config
		wantOK                      bool
		wantEnabled, wantIncomplete float64
	}{
		{
			// 完全沒設 EXTRACTOR_MODEL ＝ 套預設（gemini），沒給金鑰就是尚未開通
			// ——沒有人「選」過它。
			name:   "未設 EXTRACTOR_MODEL ＋ 無金鑰＝尚未開通",
			cfg:    config.Config{},
			wantOK: false, wantEnabled: 0, wantIncomplete: 0,
		},
		{
			// 明確寫出預設值，語意等同上一格。
			name:   "gemini 無金鑰＝尚未開通",
			cfg:    config.Config{ExtractorModel: "gemini-2.5-flash"},
			wantOK: false, wantEnabled: 0, wantIncomplete: 0,
		},
		{
			name:   "gemini 有金鑰＝已開通",
			cfg:    config.Config{ExtractorModel: "gemini-2.5-flash", GeminiAPIKey: "k"},
			wantOK: true, wantEnabled: 1, wantIncomplete: 0,
		},
		{
			// 明確選了 deepseek 卻沒給 key ＝ 配置錯誤，不是「尚未開通」
			name:   "deepseek 無金鑰＝配置錯誤",
			cfg:    config.Config{ExtractorModel: DeepSeekModelName},
			wantOK: false, wantEnabled: 0, wantIncomplete: 1,
		},
		{
			name:   "deepseek 有金鑰＝已開通",
			cfg:    config.Config{ExtractorModel: DeepSeekModelName, DeepSeekAPIKey: "k"},
			wantOK: true, wantEnabled: 1, wantIncomplete: 0,
		},
		{
			name:   "gemma 無金鑰＝配置錯誤",
			cfg:    config.Config{ExtractorModel: "google/gemma-4-26b-a4b-it:free"},
			wantOK: false, wantEnabled: 0, wantIncomplete: 1,
		},
		{
			// 打錯字。在 #3329 之前這會讓整個 ai-service 起不來。
			name:   "模型名打錯字＝配置錯誤",
			cfg:    config.Config{ExtractorModel: "deepseek-flassh", DeepSeekAPIKey: "k"},
			wantOK: false, wantEnabled: 0, wantIncomplete: 1,
		},
		{
			// 🔴 值域封閉的核心那一格：這是一個**真實存在且金鑰齊全**的 DeepSeek
			// 模型，但沒進過登記表 ⇒ 沒有人驗過它的 quirk。放行的代價是可能靜默
			// 送出被端點拒絕的 request 形狀（2026-09-11 的 C1 形狀）。
			name:   "真實但未登記的模型＝配置錯誤",
			cfg:    config.Config{ExtractorModel: "deepseek-reasoner", DeepSeekAPIKey: "k"},
			wantOK: false, wantEnabled: 0, wantIncomplete: 1,
		},
		{
			// 已驗過 quirk（2026-09-11 對正式端點實打，與 flash 行為相同）
			name:   "deepseek-v4-pro 有金鑰＝已開通",
			cfg:    config.Config{ExtractorModel: "deepseek-v4-pro", DeepSeekAPIKey: "k"},
			wantOK: true, wantEnabled: 1, wantIncomplete: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 先寫進不可能出現的值：沒有這一步，「gauge 根本沒被 Set」與
			// 「被 Set 成 0」完全無法區分，而前者正是要抓的東西。
			procurementLLMEnabled.Set(-1)
			procurementLLMConfigIncomplete.Set(-1)

			got := NewProcurement(&c.cfg, zap.NewNop())
			require.Equal(t, c.wantOK, got.Enabled())
			// 🔴 未開通時 LLM() 仍須非 nil：消費者直接呼叫它的方法，nil 介面
			// 會變成第一個請求上的 nil pointer panic，而不是乾淨的 9703。
			require.NotNil(t, got.LLM())
			if !c.wantOK {
				require.Equal(t, "disabled", got.LLM().Name())
				require.Empty(t, got.Provider(), "未開通時不得回報 provider")
			}
			require.Equal(t, c.wantEnabled, testutil.ToFloat64(procurementLLMEnabled),
				"ai_procurement_llm_enabled")
			require.Equal(t, c.wantIncomplete, testutil.ToFloat64(procurementLLMConfigIncomplete),
				"ai_procurement_llm_config_incomplete")
		})
	}
}

// 停用的 LLM 必須在**生成時** fail-closed。建構成功不等於放行：把停用實作寫成
// 「回空回應」會讓上面那支測試照樣全綠，而那會讓 RFQ 對話靜默地什麼都不回答。
func TestDisabledLLM_FailsClosedOnGenerate(t *testing.T) {
	p := NewProcurement(&config.Config{ExtractorModel: "gemini-2.5-flash"}, zap.NewNop())
	var got error
	for _, err := range p.LLM().GenerateContent(context.Background(), &model.LLMRequest{}, false) {
		got = err
	}
	require.Error(t, got, "停用 model 必須在生成時 fail-closed，不得回空回應")
	var de *pkgerrors.DomainError
	require.True(t, errors.As(got, &de))
	require.Equal(t, 9703, de.DomainCode())
}

// Assistant 側同樣不得回 error——#3329 漏掉的第三條 Gemini 中止路徑
// （internal/agent.NewAgents），2026-09-11 實測坐實：缺 GEMINI_API_KEY 時
// 它回 error ⇒ fx abort ⇒ 整個 ai-service 起不來。
func TestNewAssistant_DegradesFailClosed(t *testing.T) {
	a := NewAssistant(&config.Config{GeminiModel: "gemini-2.5-flash"}, zap.NewNop())
	require.False(t, a.Enabled())
	require.NotNil(t, a.LLM())
	require.Equal(t, "disabled", a.LLM().Name())

	a2 := NewAssistant(&config.Config{GeminiAPIKey: "k", GeminiModel: "gemini-2.5-flash"}, zap.NewNop())
	require.True(t, a2.Enabled())
	require.Equal(t, "gemini-2.5-flash", a2.ModelName())
}

// 登記表的每一筆都必須自洽：provider 合法，且 OpenAI 相容的那些要有 baseURL
// 與 options。少一項的症狀是執行期才炸或靜默送錯形狀，不是編譯錯誤。
func TestKnownExtractorModels_SpecsAreComplete(t *testing.T) {
	require.NotEmpty(t, knownExtractorModels)
	for name, spec := range knownExtractorModels {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, []string{"gemini", "deepseek", "gemma"}, spec.provider)
			if spec.provider == "gemini" {
				return // genai SDK 沒有 baseURL／RequestOption 的概念
			}
			require.NotEmpty(t, spec.baseURL, "OpenAI 相容 provider 必須有端點")
			require.NotNil(t, spec.options, "OpenAI 相容 provider 必須明寫 quirk profile")
			require.NotEmpty(t, spec.options())
		})
	}
}

// DeepSeek 系列的 quirk 必須真的帶 reasoning.effort=none——少了它，任何強制
// tool_choice 的請求在正式端點都是 HTTP 400，而 400 不可重試，症狀只是
// 「模型永遠失敗、一路退回模板」。這一格守的是登記表裡那個 profile 沒被改空。
func TestDeepSeekModels_CarryReasoningEffortNone(t *testing.T) {
	for name, spec := range knownExtractorModels {
		if spec.provider != "deepseek" {
			continue
		}
		require.Len(t, spec.options(), 2, "%s: 期望 MaxRetries(0) ＋ reasoning.effort=none", name)
	}
}

// 🔴 這支守的是「設定有沒有送到 pod」這個問題在外部有答案。
//
// 2026-09-11 把 EXTRACTOR_MODEL 變成設定之後實際撞到：deploy 完成、
// ai_procurement_llm_enabled=1，但**無法從外部確認跑的是哪一顆模型**——
// 設定沒送達時會退回預設的 gemini，而那個環境剛好也有 GEMINI_API_KEY，
// enabled 照樣是 1。兩種情況在當時的訊號上完全一樣。
func TestNewProcurement_ExposesModelInUse(t *testing.T) {
	p := NewProcurement(&config.Config{ExtractorModel: DeepSeekModelName, DeepSeekAPIKey: "k"}, zap.NewNop())
	require.True(t, p.Enabled())
	require.Equal(t, float64(1),
		testutil.ToFloat64(procurementLLMInfo.WithLabelValues(DeepSeekModelName, "deepseek")))

	// 換一顆模型後，舊的那組不得殘留——否則同一個 process 內會同時看到兩顆
	// 「正在使用」的模型，而那比沒有訊號更糟。
	p2 := NewProcurement(&config.Config{ExtractorModel: "gemini-2.5-flash", GeminiAPIKey: "k"}, zap.NewNop())
	require.True(t, p2.Enabled())
	require.Equal(t, 1, testutil.CollectAndCount(procurementLLMInfo),
		"同時只能有一組 label 存在")
	require.Equal(t, float64(1),
		testutil.ToFloat64(procurementLLMInfo.WithLabelValues("gemini-2.5-flash", "gemini")))
}

// 遷移守門：舊名還設著而新名沒設 ⇒ fail-closed，不得靜默退回預設模型。
//
// 🔴 這一格守的是一個**已經發生過**的狀態：2026-09-11 改名當天，本機 .env 仍是
// EXTRACTOR_PROVIDER=deepseek 且沒有 EXTRACTOR_MODEL，於是本機執行靜默換成了
// gemini——而該機器有 GEMINI_API_KEY，所以一切看起來正常。
func TestNewProcurement_StaleExtractorProviderFailsClosed(t *testing.T) {
	// 連金鑰都齊全也不放行：問題不是「跑不起來」，是「跑起來但打錯家」。
	cfg := &config.Config{StaleExtractorProvider: "deepseek", GeminiAPIKey: "k", DeepSeekAPIKey: "k"}
	p := NewProcurement(cfg, zap.NewNop())
	require.False(t, p.Enabled(), "舊名殘留必須 fail-closed，不得退回預設模型")
	require.Equal(t, float64(1), testutil.ToFloat64(procurementLLMConfigIncomplete),
		"這是配置錯誤（有人沒跟上改名），不是「尚未開通」")

	// 兩者都設 ⇒ 已遷移，舊名留著不影響（config.staleExtractorProvider 會回空字串）。
	ok := NewProcurement(&config.Config{ExtractorModel: DeepSeekModelName, DeepSeekAPIKey: "k"}, zap.NewNop())
	require.True(t, ok.Enabled())
}

// #3519 Task 2：ai_procurement_llm_budget_limit 未設定 LLM_TOKEN_BUDGET_PER_MINUTE 時必須
// 完全缺席，不是 export 0。它是比值型告警 AIProcurementLLMTokenBudgetHigh／
// AIProcurementLLMTokenBudgetExceeded（service-alerts.yml）的分母。
//
// ⚠️ 失效方向與直覺相反（2026-09-19 用 promtool 實測，本註解原本寫反）：
// PromQL 的 `正數 / 0` 是 `+Inf`，不是「無結果」——export 0 會讓兩條告警
// **永久紅**，而不是永久靜默。長期紅著的告警等同沒有告警，這是同一個失效類別
// 的另一面。分母缺席時整條 expr 沒有配對的樣本、不產生結果，才是我們要的行為。
// 兩個方向都由 monitoring/prometheus/tests/ai-llm-budget-alerts.test.yml 釘住。
func TestNewProcurement_LLMBudgetGauge_UnsetIsAbsent(t *testing.T) {
	NewProcurement(&config.Config{}, zap.NewNop())
	require.Equal(t, 0, testutil.CollectAndCount(procurementLLMBudgetLimit),
		"LLM_TOKEN_BUDGET_PER_MINUTE 未設時，ai_procurement_llm_budget_limit 必須沒有任何 series")
}

// 有設定時必須真的被 Set 成該值，而不是維持缺席或恰好等於期望值的巧合。
func TestNewProcurement_LLMBudgetGauge_SetIsExported(t *testing.T) {
	NewProcurement(&config.Config{LLMTokenBudgetPerMinute: "20000"}, zap.NewNop())
	require.Equal(t, 1, testutil.CollectAndCount(procurementLLMBudgetLimit))
	require.Equal(t, float64(20000), testutil.ToFloat64(procurementLLMBudgetLimit.WithLabelValues()))
}

// 無效輸入（非數字、零、負數）必須落回缺席，不是 export 0 或負值——分母不可以
// 是一個會讓比例告警恆真或恆假的哨兵值。
func TestNewProcurement_LLMBudgetGauge_InvalidValueIsAbsent(t *testing.T) {
	for _, raw := range []string{"not-a-number", "0", "-5"} {
		NewProcurement(&config.Config{LLMTokenBudgetPerMinute: raw}, zap.NewNop())
		require.Equal(t, 0, testutil.CollectAndCount(procurementLLMBudgetLimit),
			"LLM_TOKEN_BUDGET_PER_MINUTE=%q 應視為無效並維持缺席", raw)
	}
}

// 預算宣告與「LLM 本身有沒有開通」互不影響：即使因為缺金鑰而停用，只要有設定
// 預算，這支 gauge 仍要 export——值班的人在正式開通之前就能先確認預算數字有
// 沒有送達 pod。
func TestNewProcurement_LLMBudgetGauge_ExportedEvenWhenDisabled(t *testing.T) {
	p := NewProcurement(&config.Config{LLMTokenBudgetPerMinute: "5000"}, zap.NewNop())
	require.False(t, p.Enabled(), "本測試要驗證的前提是 LLM 本身尚未開通")
	require.Equal(t, float64(5000), testutil.ToFloat64(procurementLLMBudgetLimit.WithLabelValues()))
}
