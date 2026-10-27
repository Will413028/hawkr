package cropidentity

import (
	"context"
	"fmt"
	"iter"
	"testing"
	"time"

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

func TestCropCategoryModel_SchemaAndToolCallOnlyAllowSuppliedIDs(t *testing.T) {
	for _, toolCall := range []bool{false, true} {
		fake := &testkit.FakeLLM{}
		part := &genai.Part{Text: `{"categoryIds":["200","200"]}`}
		if toolCall {
			part = &genai.Part{FunctionCall: &genai.FunctionCall{Name: cropCategoryToolName, Args: map[string]any{"categoryIds": []string{"200", "200"}}}}
		}
		fake.Resp = &model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{part}}}
		adapter := &modelCropCategorySuggester{llm: fake, modelName: "test", toolCall: toolCall}
		ids, err := adapter.Suggest(context.Background(), "臺灣豬五花", domain.CropSelectionGroup, "", []cropCategory{{ID: "200", Label: "豬肉"}})
		require.NoError(t, err)
		require.Equal(t, []string{"200"}, ids)
		var schema *genai.Schema
		if toolCall {
			require.Equal(t, genai.FunctionCallingConfigModeAny, fake.LastReq.Config.ToolConfig.FunctionCallingConfig.Mode)
			schema = fake.LastReq.Config.Tools[0].FunctionDeclarations[0].Parameters
		} else {
			schema = fake.LastReq.Config.ResponseSchema
		}
		require.Equal(t, []string{"200"}, schema.Properties["categoryIds"].Items.Enum)
		require.Contains(t, fake.LastReq.Contents[0].Parts[0].Text, "臺灣豬五花")
	}
}

func TestCropCategoryModel_FabricationMalformedAndEmptyResponses(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		wantErr    bool
	}{
		{name: "invented", text: `{"categoryIds":["999"]}`, wantErr: true},
		{name: "mixed invented", text: `{"categoryIds":["200","999"]}`, wantErr: true},
		{name: "malformed", text: `just choose pork`, wantErr: true},
		{name: "missing response", wantErr: true},
		{name: "no match", text: `{"categoryIds":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &testkit.FakeLLM{Resp: &model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{{Text: tc.text}}}}}
			adapter := &modelCropCategorySuggester{llm: fake}
			ids, err := adapter.Suggest(context.Background(), "原品項", domain.CropSelectionGroup, "", []cropCategory{{ID: "200", Label: "豬肉"}})
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, ids)
		})
	}
}

type categoryDeadlineLLM struct {
	calls    int
	deadline time.Time
}

func (f *categoryDeadlineLLM) Name() string { return "deadline-test" }
func (f *categoryDeadlineLLM) GenerateContent(ctx context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		f.calls++
		f.deadline, _ = ctx.Deadline()
		<-ctx.Done()
		yield(nil, ctx.Err())
	}
}
func TestCropCategoryModel_InheritsEarlierDeadlineAndNeverRetries(t *testing.T) {
	fake := &categoryDeadlineLLM{}
	adapter := &modelCropCategorySuggester{llm: fake}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	deadline, _ := ctx.Deadline()
	_, err := adapter.Suggest(ctx, "品項", domain.CropSelectionGroup, "", []cropCategory{{ID: "200", Label: "豬肉"}})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, fake.calls)
	require.Equal(t, deadline, fake.deadline)
}
func TestCropCategoryModel_DisabledProviderNeedsNoNewClient(t *testing.T) {
	provider := llmprovider.NewProcurement(&config.Config{ExtractorModel: "unregistered-test-model"}, zap.NewNop())
	require.Nil(t, newCropCategorySuggester(provider))
}

func TestCropCategoryModel_RecordsResultDurationAndTokens(t *testing.T) {
	collect := observability.InstallManualReader(t)
	fake := &testkit.FakeLLM{Resp: &model.LLMResponse{
		Content:       &genai.Content{Parts: []*genai.Part{{Text: `{"categoryIds":["200"]}`}}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 300, CandidatesTokenCount: 8},
	}}
	adapter := &modelCropCategorySuggester{llm: fake, modelName: "test"}

	ids, err := adapter.Suggest(context.Background(), "臺灣豬五花", domain.CropSelectionGroup, "", []cropCategory{{ID: "200", Label: "豬肉"}})
	require.NoError(t, err)
	require.Equal(t, []string{"200"}, ids)

	rm := collect()
	v, ok := observability.CounterValue(rm, "procurement_crop_suggest_total", map[string]string{"stage": "GROUP", "result": "success"})
	require.True(t, ok)
	require.EqualValues(t, 1, v)
	c, ok := observability.HistogramCount(rm, "procurement_crop_suggest_duration_seconds", map[string]string{"stage": "GROUP"})
	require.True(t, ok)
	require.EqualValues(t, 1, c)
	v, ok = observability.CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "crop_suggest", "kind": "prompt"})
	require.True(t, ok)
	require.EqualValues(t, 300, v)
}

func TestCropCategoryModel_BoundExceeded_RecordsResultWithoutCall(t *testing.T) {
	collect := observability.InstallManualReader(t)
	fake := &testkit.FakeLLM{}
	adapter := &modelCropCategorySuggester{llm: fake, modelName: "test"}
	categories := make([]cropCategory, cropCategoryModelMaxCandidates+1)
	for i := range categories {
		categories[i] = cropCategory{ID: fmt.Sprint(i), Label: "x"}
	}

	_, err := adapter.Suggest(context.Background(), "x", domain.CropSelectionGroup, "", categories)
	require.ErrorIs(t, err, errCropCategoryBoundExceeded)
	require.Nil(t, fake.LastReq)

	rm := collect()
	v, ok := observability.CounterValue(rm, "procurement_crop_suggest_total", map[string]string{"stage": "GROUP", "result": "bound_exceeded"})
	require.True(t, ok)
	require.EqualValues(t, 1, v)
	_, ok = observability.HistogramCount(rm, "procurement_crop_suggest_duration_seconds", map[string]string{"stage": "GROUP"})
	require.False(t, ok)
}

func TestCropCategoryModel_FabricatedID_RecordsRejected(t *testing.T) {
	collect := observability.InstallManualReader(t)
	fake := &testkit.FakeLLM{Resp: &model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{{Text: `{"categoryIds":["999"]}`}}}}}
	adapter := &modelCropCategorySuggester{llm: fake, modelName: "test"}

	_, err := adapter.Suggest(context.Background(), "x", domain.CropSelectionItem, "豬肉", []cropCategory{{ID: "200", Label: "豬肉"}})
	require.Error(t, err)

	v, ok := observability.CounterValue(collect(), "procurement_crop_suggest_total", map[string]string{"stage": "ITEM", "result": "rejected"})
	require.True(t, ok)
	require.EqualValues(t, 1, v)
}
