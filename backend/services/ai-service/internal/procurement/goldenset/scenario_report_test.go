package goldenset

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func reportWith(mode, corpusHash string, results ...ScenarioResult) ScenarioReport {
	return ScenarioReport{
		Metadata: ReportMetadata{SchemaVersion: ReportSchemaVersion, Mode: mode, CorpusHash: corpusHash},
		Results:  results,
		Summary:  Summarize(results),
	}
}

func failure(id, field string, turn int) ScenarioFailure {
	return ScenarioFailure{ScenarioID: id, Field: field, Turn: turn, Category: CategoryStateTransition}
}

// TestSummarize_KnownGapsDoNotInflatePassRate：把缺口標成 known_gap 不得讓通過率
// 變好看。通過率只算 active 的 case。
func TestSummarize_KnownGapsDoNotInflatePassRate(t *testing.T) {
	got := Summarize([]ScenarioResult{
		{ScenarioID: "ok", Status: ScenarioStatusActive},
		{ScenarioID: "bad", Status: ScenarioStatusActive, Failures: []ScenarioFailure{failure("bad", "slots.item", 0)}},
		{ScenarioID: "gap", Status: ScenarioStatusKnownGap, Failures: []ScenarioFailure{failure("gap", "slots.item", 0)}},
	})

	require.Equal(t, 3, got.Total)
	require.Equal(t, 1, got.Passed)
	require.Equal(t, 1, got.Failed)
	require.Equal(t, 1, got.KnownGaps, "known_gap 自成一類，不計入 passed 也不計入 failed")
}

// TestSummarize_ErroredIsNotFailed：沒跑完（transport／逾時）與跑完但不符預期是
// 兩件事，混在一起會讓「工具壞了」看起來像「模型退步了」。
func TestSummarize_ErroredIsNotFailed(t *testing.T) {
	got := Summarize([]ScenarioResult{{ScenarioID: "x", Status: ScenarioStatusActive, Errored: true}})
	require.Equal(t, 1, got.Errored)
	require.Zero(t, got.Failed)
	require.Zero(t, got.Passed)
}

func TestDiffReports_NewFailuresAndFixes(t *testing.T) {
	base := reportWith("offline", "h1",
		ScenarioResult{ScenarioID: "a", Status: ScenarioStatusActive,
			Failures: []ScenarioFailure{failure("a", "slots.item", 0)}},
		ScenarioResult{ScenarioID: "b", Status: ScenarioStatusActive})
	cur := reportWith("offline", "h1",
		ScenarioResult{ScenarioID: "a", Status: ScenarioStatusActive},
		ScenarioResult{ScenarioID: "b", Status: ScenarioStatusActive,
			Failures: []ScenarioFailure{failure("b", "next_action", 1)}})

	got := DiffReports(base, cur)

	require.True(t, got.Comparable)
	require.Equal(t, []string{"b turn 1 next_action"}, got.NewFailures)
	require.Equal(t, []string{"a turn 0 slots.item"}, got.Fixed)
}

// TestDiffReports_RemovedCaseIsNotAFix：把一個一直失敗的 case 從 corpus 拿掉，
// 看起來會像修好了。缺的 case 必須單獨列出來。
func TestDiffReports_RemovedCaseIsNotAFix(t *testing.T) {
	base := reportWith("offline", "h1",
		ScenarioResult{ScenarioID: "gone", Status: ScenarioStatusActive,
			Failures: []ScenarioFailure{failure("gone", "slots.item", 0)}})
	cur := reportWith("offline", "h1")

	got := DiffReports(base, cur)

	require.Equal(t, []string{"gone"}, got.MissingCases)
	require.Equal(t, []string{"gone turn 0 slots.item"}, got.Fixed,
		"欄位層級看起來是修好了，但 MissingCases 同時會說出真相")
}

// TestDiffReports_IncomparableMetadata：corpus 或執行模式不同就不可比，不得
// 硬算出一個差異表讓人誤以為可以對照。
func TestDiffReports_IncomparableMetadata(t *testing.T) {
	require.False(t, DiffReports(reportWith("offline", "h1"), reportWith("offline", "h2")).Comparable)
	require.False(t, DiffReports(reportWith("offline", "h1"), reportWith("live", "h1")).Comparable)
}

func TestDiffReports_GapChangeIsReported(t *testing.T) {
	base := reportWith("offline", "h1", ScenarioResult{ScenarioID: "a", Status: ScenarioStatusKnownGap})
	cur := reportWith("offline", "h1", ScenarioResult{ScenarioID: "a", Status: ScenarioStatusActive})

	require.Equal(t, []string{"a：known_gap → active"}, DiffReports(base, cur).GapChanges)
}

// TestCorpusHash_ChangesWithSpecOnly：corpus 沒變而結果變了才是回歸，所以指紋
// 只吃 spec。
func TestCorpusHash_ChangesWithSpecOnly(t *testing.T) {
	a := []Scenario{{ID: "x", Capability: CapabilitySupported, Status: ScenarioStatusActive,
		FrozenAt: "t", Turns: []ScenarioTurn{{Op: OpTurn, Message: "我要芭樂"}}}}
	same := []Scenario{{ID: "x", Capability: CapabilitySupported, Status: ScenarioStatusActive,
		FrozenAt: "t", Turns: []ScenarioTurn{{Op: OpTurn, Message: "我要芭樂"}}}}
	changed := []Scenario{{ID: "x", Capability: CapabilitySupported, Status: ScenarioStatusActive,
		FrozenAt: "t", Turns: []ScenarioTurn{{Op: OpTurn, Message: "我要香蕉"}}}}

	require.Equal(t, CorpusHash(a), CorpusHash(same))
	require.NotEqual(t, CorpusHash(a), CorpusHash(changed))
}

// TestScenarioReport_IsMachineReadable：報告要能被別的工具讀，欄位名固定。
func TestScenarioReport_IsMachineReadable(t *testing.T) {
	r := reportWith("offline", "h1", ScenarioResult{ScenarioID: "a", Status: ScenarioStatusActive})
	r.Metadata.Model = Unavailable

	raw, err := json.Marshal(r)
	require.NoError(t, err)

	var back map[string]any
	require.NoError(t, json.Unmarshal(raw, &back))
	meta := back["metadata"].(map[string]any)
	require.Equal(t, Unavailable, meta["model"], "取不到就寫 unavailable，不得寫 0 或空字串")
	require.Equal(t, float64(ReportSchemaVersion), meta["schemaVersion"])
}
