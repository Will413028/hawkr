package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/goldenset"
)

// TestDevHostAllowed 守住這支工具只能打 dev。它會用合成買家送出真的對話並呼叫
// 真模型——指向 production 等於在正式環境製造資料與費用，而且那些資料混在真買家
// 裡面無法乾淨清掉。
func TestDevHostAllowed(t *testing.T) {
	for _, ok := range []string{
		"http://localhost:8080/api/v1",
		"http://127.0.0.1:8080/api/v1",
		"https://test-api.example.com/api/v1",
	} {
		require.NoError(t, devHostAllowed(ok), ok)
	}
	for _, bad := range []string{
		"https://api.example.com/api/v1",
		"https://example.com/api/v1",
		"https://test-api.evil.example/api/v1",
	} {
		require.Error(t, devHostAllowed(bad), bad)
	}
}

// TestForbiddenPathsAreRefused 是 op allowlist 的第二道防線：就算之後有人加了新
// op 並忘了想清楚，只要它打到會產生對外副作用的路徑就會被擋下來。
func TestForbiddenPathsAreRefused(t *testing.T) {
	for _, bad := range []string{
		"/app/buyers/me/procurement/requests/1/rfq-publications",
		"/app/buyers/me/procurement/requests/1/buyer-quote-acceptances",
		"/app/sellers/me/procurement/seller-responses",
	} {
		require.Error(t, checkPathAllowed(bad), bad)
	}
	require.NoError(t, checkPathAllowed("/app/buyers/me/procurement/turns"))
}

func TestCheckScenarioOps(t *testing.T) {
	ok := goldenset.Scenario{ID: "a", Turns: []goldenset.ScenarioTurn{{Op: goldenset.OpTurn}, {Op: goldenset.OpResume}}}
	require.NoError(t, checkScenarioOps(ok))

	bad := goldenset.Scenario{ID: "a", Turns: []goldenset.ScenarioTurn{{Op: "publish"}}}
	require.Error(t, checkScenarioOps(bad))
}

// TestUnwrapEnvelope_NonZeroCodeIsFailure：業務錯誤不得被當成成功。工具把一次
// 失敗的呼叫算成 PASS，等於把「壞掉」報成「沒問題」。
func TestUnwrapEnvelope_NonZeroCodeIsFailure(t *testing.T) {
	_, err := unwrapEnvelope(map[string]any{"code": float64(40001), "message": "bad"})
	require.Error(t, err)

	got, err := unwrapEnvelope(map[string]any{"code": float64(0), "data": map[string]any{"requestId": "7"}})
	require.NoError(t, err)
	require.Equal(t, "7", got.Data["requestId"])
}

// TestPickOptionID_MustBeUnique：不得固定選第一筆——那樣「選項順序或內容改變」
// 這種回歸在 corpus 上完全看不出來。
func TestPickOptionID_MustBeUnique(t *testing.T) {
	q := map[string]any{"options": []any{
		map[string]any{"optionId": "a", "label": "台灣芭樂"},
		map[string]any{"optionId": "b", "label": "泰國芭樂"},
	}}
	got, err := pickOptionID(q, "泰國芭樂")
	require.NoError(t, err)
	require.Equal(t, "b", got, "必須依 label 命中，不是取第一筆")

	_, err = pickOptionID(q, "不存在的選項")
	require.Error(t, err)

	dup := map[string]any{"options": []any{
		map[string]any{"optionId": "a", "label": "芭樂"},
		map[string]any{"optionId": "b", "label": "芭樂"},
	}}
	_, err = pickOptionID(dup, "芭樂")
	require.Error(t, err, "命中多筆要報錯，讓 corpus 自己改清楚")
}

func TestScenarioLimits(t *testing.T) {
	all := []goldenset.Scenario{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	require.Len(t, scenarioLimits{MaxCases: 2}.selectCases(all), 2)
	require.Equal(t, "b", scenarioLimits{Case: "b"}.selectCases(all)[0].ID)
	require.Empty(t, scenarioLimits{Case: "zzz"}.selectCases(all))

	spec := goldenset.Scenario{Turns: []goldenset.ScenarioTurn{{Op: "turn"}, {Op: "turn"}, {Op: "turn"}}}
	require.Len(t, scenarioLimits{MaxTurns: 2}.turnsOf(spec), 2)
	require.Len(t, scenarioLimits{}.turnsOf(spec), 3)
}

// TestLiveRunner_RoutesFullConversation 用 httptest 驗完整路由與 envelope 解包：
// turn → 消歧提問 → resume（帶當輪選項）→ 下一輪 → search。
//
// ⛔ 這條不呼叫真模型，驗的是**接線**：路徑對不對、resume 有沒有誤送 legacy
// /answers、data.result 有沒有解開、requestId／runId 有沒有在輪次之間帶下去。
func TestLiveRunner_RoutesFullConversation(t *testing.T) {
	var gotPaths []string
	var gotKeys []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(req.Body).Decode(&body)
		if k, _ := body["clientRequestId"].(string); k != "" {
			gotKeys = append(gotKeys, k)
		}
		path := req.URL.Path
		if path != "/app/buyers/me" {
			gotPaths = append(gotPaths, path)
		}

		write := func(data map[string]any) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "ok", "data": data})
		}
		switch {
		case path == "/app/buyers/me":
			write(map[string]any{})
		case path == "/app/buyers/me/procurement/turns" && len(gotPaths) == 1:
			write(map[string]any{
				"requestId": "77", "runId": "901",
				"slots": map[string]any{"item": "芭樂"},
				"question": map[string]any{"slotKey": "crop_disambiguation", "options": []any{
					map[string]any{"optionId": "crop-tw", "label": "台灣芭樂"},
					map[string]any{"optionId": "crop-th", "label": "泰國芭樂"},
				}},
			})
		case path == "/app/buyers/me/procurement/runs/901/resume":
			// resume 的結果包在 data.result 裡。
			write(map[string]any{"result": map[string]any{
				"requestId": "77",
				"slots": map[string]any{"item": "芭樂",
					"item_resolution": map[string]any{"kind": "SELF_PRICED", "label": "臺灣"}},
				"missingRequired": []any{"unit"},
			}})
		case path == "/app/buyers/me/procurement/turns":
			write(map[string]any{"requestId": "77",
				"slots":           map[string]any{"item": "芭樂", "unit": "KG"},
				"missingRequired": []any{}})
		case path == "/app/buyers/me/procurement/requests/77/candidate-searches":
			write(map[string]any{"requestId": "77", "slots": map[string]any{"item": "芭樂", "unit": "KG"}})
		default:
			t.Errorf("非預期的路徑：%s", path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("JWT_SECRET", "test-secret")
	runner, err := newLiveRunner(srv.URL)
	require.NoError(t, err)

	spec := goldenset.Scenario{
		ID: "routing", Capability: goldenset.CapabilitySupported,
		Fixture: "f", FrozenAt: "t", Status: goldenset.ScenarioStatusActive,
		Turns: []goldenset.ScenarioTurn{
			{Op: goldenset.OpTurn, Message: "我要芭樂"},
			{Op: goldenset.OpResume, OptionLabel: "泰國芭樂",
				Expect: goldenset.ScenarioExpectation{
					Slots: map[string]any{"item_resolution_code": "CROP-TW"}}},
			{Op: goldenset.OpTurn, Message: "按公斤"},
			{Op: goldenset.OpSearch},
		},
	}

	res := runner.run(spec, scenarioLimits{})

	require.False(t, res.Errored, "接線測試不得有 transport／業務錯誤：%+v", res.Failures)
	require.Equal(t, 4, res.TurnsRun)
	require.Equal(t, []string{
		"/app/buyers/me/procurement/turns",
		"/app/buyers/me/procurement/runs/901/resume",
		"/app/buyers/me/procurement/turns",
		"/app/buyers/me/procurement/requests/77/candidate-searches",
	}, gotPaths, "消歧答案必須走 resume，不得送 legacy /answers")

	// clientRequestId 每一輪都不同（第一筆是 provision buyer，沒有 key）。
	require.Equal(t, []string{"routing-t0", "routing-t1", "routing-t2", "routing-t3"}, gotKeys)
}

// TestLiveRunner_HTTPFailureIsNotPass：HTTP 或業務層失敗必須記成 errored，
// 不得因為「沒有差異」而看起來像通過。
func TestLiveRunner_HTTPFailureIsNotPass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/app/buyers/me" {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 50000, "message": "boom"})
	}))
	defer srv.Close()

	t.Setenv("JWT_SECRET", "test-secret")
	runner, err := newLiveRunner(srv.URL)
	require.NoError(t, err)

	res := runner.run(goldenset.Scenario{ID: "x", Status: goldenset.ScenarioStatusActive,
		Turns: []goldenset.ScenarioTurn{{Op: goldenset.OpTurn, Message: "m"}}}, scenarioLimits{})

	require.True(t, res.Errored)
	require.False(t, res.Passed(), "失敗的呼叫不得被當成通過")
	require.Equal(t, 1, goldenset.Summarize([]goldenset.ScenarioResult{res}).Errored)
}

func TestReadBaseline_RejectsUnusable(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "nope.json")
	_, err := readBaseline(missing)
	require.Error(t, err)

	notJSON := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(notJSON, []byte("{"), 0o644))
	_, err = readBaseline(notJSON)
	require.Error(t, err)

	wrongVersion := filepath.Join(dir, "v9.json")
	require.NoError(t, os.WriteFile(wrongVersion,
		[]byte(`{"metadata":{"schemaVersion":9,"corpusHash":"h"}}`), 0o644))
	_, err = readBaseline(wrongVersion)
	require.Error(t, err)

	noHash := filepath.Join(dir, "nohash.json")
	require.NoError(t, os.WriteFile(noHash,
		[]byte(`{"metadata":{"schemaVersion":1}}`), 0o644))
	_, err = readBaseline(noHash)
	require.Error(t, err, "缺 corpusHash 就無法判斷可不可比，不得靜默當成可比")

	good := filepath.Join(dir, "ok.json")
	require.NoError(t, os.WriteFile(good,
		[]byte(`{"metadata":{"schemaVersion":1,"corpusHash":"h","mode":"live"}}`), 0o644))
	_, err = readBaseline(good)
	require.NoError(t, err)
}

// TestWriteReport_FailureIsReported：寫不出報告要回錯，不能安靜地跑完卻沒留下
// 任何可比對的東西。
func TestWriteReport_FailureIsReported(t *testing.T) {
	r := goldenset.ScenarioReport{Metadata: goldenset.ReportMetadata{SchemaVersion: 1}}
	require.Error(t, writeReport(filepath.Join(t.TempDir(), "no-such-dir", "r.json"), r))

	path := filepath.Join(t.TempDir(), "r.json")
	require.NoError(t, writeReport(path, r))
	blob, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(blob), `"schemaVersion": 1`)
}

// TestNewScenarioReport_UnknownMetadataIsUnavailable：取不到就寫 unavailable，
// 不寫 0 或空字串。
func TestNewScenarioReport_UnknownMetadataIsUnavailable(t *testing.T) {
	t.Setenv("EVAL_BACKEND_SHA", "")
	t.Setenv("EVAL_WEB_SHA", "")
	got := newScenarioReport(nil, nil, "", "")
	require.Equal(t, goldenset.Unavailable, got.Metadata.Model)
	require.Equal(t, goldenset.Unavailable, got.Metadata.Provider)
	require.Equal(t, goldenset.Unavailable, got.Metadata.BackendSHA)
	require.Equal(t, goldenset.Unavailable, got.Metadata.WebSHA)
	require.Equal(t, "live", got.Metadata.Mode)
}

// TestRunScenarioEval_RefusesNonDevHost 是端到端的守門：連 corpus 都還沒讀就要擋。
func TestRunScenarioEval_RefusesNonDevHost(t *testing.T) {
	code := runScenarioEval(scenarioEvalOptions{
		Base: "https://api.example.com/api/v1", CorpusPath: "does-not-matter.yaml",
	})
	require.Equal(t, 2, code)
}
