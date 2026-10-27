// services/ai-service/internal/procurement/usecase/step_narrator_test.go
package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type fakeStepNarrator struct {
	text string
	err  error
}

func (f *fakeStepNarrator) Narrate(_ context.Context, _ domain.StepCode, _ string) (string, error) {
	return f.text, f.err
}

func TestNarrateStepOutcome_Success_ReturnsNarratorText(t *testing.T) {
	narrator := &fakeStepNarrator{text: "已核對品項與數量，條件皆有效"}
	ctx := newTestAgentContext(t, nil)

	got := narrateStepOutcome(ctx, narrator, domain.StepValidateSlots, "summary", "fallback text")
	require.Equal(t, "已核對品項與數量，條件皆有效", got)
}

func TestNarrateStepOutcome_NarratorError_FallsBackToFixedText(t *testing.T) {
	narrator := &fakeStepNarrator{err: errors.New("provider unavailable")}
	ctx := newTestAgentContext(t, nil)

	got := narrateStepOutcome(ctx, narrator, domain.StepValidateSlots, "summary", "fallback text")
	require.Equal(t, "fallback text", got)
}

func TestNarrateStepOutcome_EmptyOutput_FallsBackToFixedText(t *testing.T) {
	narrator := &fakeStepNarrator{text: "   "}
	ctx := newTestAgentContext(t, nil)

	got := narrateStepOutcome(ctx, narrator, domain.StepValidateSlots, "summary", "fallback text")
	require.Equal(t, "fallback text", got)
}

func TestNarrateStepOutcome_TooLongOutput_FallsBackToFixedText(t *testing.T) {
	narrator := &fakeStepNarrator{text: strings.Repeat("字", shortIntentMaxLen+1)}
	ctx := newTestAgentContext(t, nil)

	got := narrateStepOutcome(ctx, narrator, domain.StepValidateSlots, "summary", "fallback text")
	require.Equal(t, "fallback text", got)
}

func TestNarrateStepOutcome_ExactlyAtLimit_PassesThrough(t *testing.T) {
	sentence := strings.Repeat("字", shortIntentMaxLen)
	narrator := &fakeStepNarrator{text: sentence}
	ctx := newTestAgentContext(t, nil)

	got := narrateStepOutcome(ctx, narrator, domain.StepValidateSlots, "summary", "fallback text")
	require.Equal(t, sentence, got)
}

func TestModelLLMStepNarrator_RecordsDurationAndTokens(t *testing.T) {
	collect := observability.InstallManualReader(t)
	fake := &testkit.FakeLLM{Resp: &model.LLMResponse{
		Content:       &genai.Content{Parts: []*genai.Part{{Text: "已核對品項與數量"}}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 120, CandidatesTokenCount: 15},
	}}
	n := &modelLLMStepNarrator{llm: fake, modelName: "test"}

	got, err := n.Narrate(context.Background(), domain.StepValidateSlots, "summary")
	require.NoError(t, err)
	require.Equal(t, "已核對品項與數量", got)

	rm := collect()
	v, ok := observability.CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "narrate", "kind": "prompt"})
	require.True(t, ok)
	require.EqualValues(t, 120, v)
	v, ok = observability.CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "narrate", "kind": "completion"})
	require.True(t, ok)
	require.EqualValues(t, 15, v)
	_, ok = observability.CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "narrate", "kind": "thoughts"})
	require.False(t, ok)
	c, ok := observability.HistogramCount(rm, "procurement_narration_duration_seconds", map[string]string{"code": string(domain.StepValidateSlots)})
	require.True(t, ok)
	require.EqualValues(t, 1, c)
}

func TestNarrateStepOutcome_RecordsResult(t *testing.T) {
	cases := []struct {
		name     string
		narrator StepNarrator
		want     string
	}{
		{"success", &fakeStepNarrator{text: "核對完成"}, "success"},
		{"error", &fakeStepNarrator{err: errors.New("provider unavailable")}, "error"},
		{"rejected empty", &fakeStepNarrator{text: "  "}, "rejected"},
		{"rejected too long", &fakeStepNarrator{text: strings.Repeat("字", shortIntentMaxLen+1)}, "rejected"},
		{"disabled", disabledStepNarrator{}, "disabled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collect := observability.InstallManualReader(t)
			ctx := newTestAgentContext(t, nil)

			narrateStepOutcome(ctx, tc.narrator, domain.StepCropGate, "summary", "fallback")

			v, ok := observability.CounterValue(collect(), "procurement_narration_total",
				map[string]string{"code": string(domain.StepCropGate), "result": tc.want})
			require.True(t, ok, "series with result=%s missing", tc.want)
			require.EqualValues(t, 1, v)
		})
	}
}
