package goldenset

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluate_PrecisionAt1AndRecall(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "豬肉", ExpectCategory: "肉類 > 豬肉 > 豬肉"},
			RankedCategories: []string{"肉類 > 豬肉 > 豬肉"}},
		{Query: Query{Text: "白肉雞", ExpectCategory: "肉類 > 雞肉 > 雞肉"},
			RankedCategories: []string{"水果 > 葡萄柚 > 白肉", "肉類 > 雞肉 > 雞肉"}},
		{Query: Query{Text: "糯米", ExpectCategory: "穀物 > 米 > 米"},
			RankedCategories: []string{"蔬菜 > 玉米 > 糯米白"}},
	}
	r := Evaluate(obs, 10)
	require.InDelta(t, 1.0/3.0, r.PrecisionAt1, 0.001)
	require.InDelta(t, 2.0/3.0, r.RecallAtK, 0.001)
	require.InDelta(t, (1.0+0.5+0.0)/3.0, r.MRR, 0.001)
}

func TestEvaluate_SeparatesRankingFromRecallFailures(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "白肉雞", ExpectCategory: "肉類 > 雞肉 > 雞肉"},
			RankedCategories: []string{"水果 > 葡萄柚 > 白肉", "肉類 > 雞肉 > 雞肉"}},
		{Query: Query{Text: "糯米", ExpectCategory: "穀物 > 米 > 米"},
			RankedCategories: []string{"蔬菜 > 玉米 > 糯米白"}},
	}
	r := Evaluate(obs, 10)
	require.Len(t, r.Failures, 2)
	byQuery := map[string]string{}
	for _, f := range r.Failures {
		byQuery[f.Query.Text] = f.Kind
	}
	require.Equal(t, FailureRanking, byQuery["白肉雞"], "正解在候選裡，是排序問題")
	require.Equal(t, FailureRecall, byQuery["糯米"], "正解不在候選裡，是召回/資料問題")
}

func TestEvaluate_RecallRespectsK(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "x", ExpectCategory: "命中"},
			RankedCategories: []string{"a", "b", "c", "命中"}},
	}
	require.InDelta(t, 1.0, Evaluate(obs, 10).RecallAtK, 0.001)
	require.InDelta(t, 0.0, Evaluate(obs, 3).RecallAtK, 0.001, "第 4 名不該算進 Recall@3")
}

func TestEvaluate_NILExpectation(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "鑽石"}, IsNIL: true},
		{Query: Query{Text: "二手汽車"}, RankedCategories: []string{"蔬菜 > 甘藍 > 初秋"}},
	}
	r := Evaluate(obs, 10)
	require.InDelta(t, 0.5, r.PrecisionAt1, 0.001)
	require.Len(t, r.Failures, 1)
	require.Equal(t, "二手汽車", r.Failures[0].Query.Text)
}

func TestEvaluate_KnownGapCountedButNotFailed(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "糯米", ExpectCategory: "穀物 > 米 > 米", Status: StatusKnownGap, TrackedBy: "#1"},
			RankedCategories: []string{"蔬菜 > 玉米 > 糯米白"}},
	}
	r := Evaluate(obs, 10)
	require.Equal(t, 1, r.KnownGap)
	require.Empty(t, r.Failures, "known_gap 不列入 Failures，但仍計入指標與 KnownGap 計數")
	require.InDelta(t, 0.0, r.RecallAtK, 0.001, "指標要誠實反映它還是沒解出來")
}

func TestEvaluate_EmptyInput(t *testing.T) {
	r := Evaluate(nil, 10)
	require.Equal(t, 0, r.Total)
	require.InDelta(t, 0.0, r.PrecisionAt1, 0.001)
}

func TestEvaluate_UnmappableIsItsOwnCategory(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "豬肉", ExpectCategory: "肉類 > 豬肉 > 豬肉"},
			RankedCategories: []string{"肉類 > 豬肉 > 豬肉"}},
		{Query: Query{Text: "高麗菜", ExpectCategory: "蔬菜 > 甘藍 > 初秋"},
			Unmappable: true, ResolvedLabel: "甘藍-初秋"},
	}
	r := Evaluate(obs, 10)

	require.Equal(t, 1, r.Unmappable)
	require.Len(t, r.Failures, 1)
	require.Equal(t, FailureUnmappable, r.Failures[0].Kind)

	require.InDelta(t, 0.5, r.PrecisionAt1, 0.001)

	require.InDelta(t, 1.0, r.PrecisionAt1Upper, 0.001)
}

func TestEvaluate_NoUnmappableMeansBoundsCollapse(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "豬肉", ExpectCategory: "肉類 > 豬肉 > 豬肉"},
			RankedCategories: []string{"肉類 > 豬肉 > 豬肉"}},
	}
	r := Evaluate(obs, 10)
	require.Equal(t, 0, r.Unmappable)
	require.InDelta(t, r.PrecisionAt1, r.PrecisionAt1Upper, 0.001, "沒有 unmappable 時上下界應重合")
}

func TestEvaluate_MatchesByLabelWhenCategoryUnavailable(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "高麗菜", ExpectLabel: "甘藍-初秋"},
			RankedLabels: []string{"甘藍-初秋", "甘藍-改良種"}},
		{Query: Query{Text: "雞蛋", ExpectLabel: "雞蛋產地價"},
			ResolvedLabel: "雞蛋產地價"},
		{Query: Query{Text: "白米", ExpectLabel: "白米零售價（稉種）"},
			ResolvedLabel: "白米零售價（長糯）"},
	}
	r := Evaluate(obs, 10)
	require.Equal(t, 0, r.Unmappable, "有 label 可比就不算 unmappable")
	require.InDelta(t, 2.0/3.0, r.PrecisionAt1, 0.001)
	require.Len(t, r.Failures, 1)
	require.Equal(t, "白米", r.Failures[0].Query.Text)
}

func TestEvaluate_EitherCategoryOrLabelCounts(t *testing.T) {
	obs := []Observation{
		{Query: Query{Text: "豬肉", ExpectCategory: "肉類 > 豬肉", ExpectLabel: "毛豬（規格豬）"},
			RankedCategories: []string{"肉類 > 豬肉"}},
		{Query: Query{Text: "雞肉", ExpectCategory: "肉類 > 雞肉", ExpectLabel: "白肉雞（2.0 公斤以上）產地價"},
			ResolvedLabel: "白肉雞（2.0 公斤以上）產地價"},
	}
	r := Evaluate(obs, 10)
	require.InDelta(t, 1.0, r.PrecisionAt1, 0.001)
	require.Empty(t, r.Failures)
}

func TestRankOf_ExpectedCategoryAcceptsDeeperPath(t *testing.T) {

	obs := Observation{Query: Query{Text: "臺灣豬五花", ExpectCategory: "肉類 > 豬肉"},
		RankedCategories: []string{"肉類 > 豬肉 > 生鮮豬肉"}}
	require.Equal(t, 0, rankOf(obs, 10))

	obs = Observation{Query: Query{Text: "x", ExpectCategory: "肉類 > 豬"},
		RankedCategories: []string{"肉類 > 豬肉 > 豬肉"}}
	require.Equal(t, -1, rankOf(obs, 10))
}
