package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/goldenset"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// 多輪情境的 real-model eval。
//
// 與離線 replay（internal/procurement/usecase/scenario_replay_test.go）跑**同一份
// corpus、同一組期望**，差別只有一個：這裡不餵 stub_patch，讓真模型自己讀買家的話。
// 離線那邊驗接線，這裡驗理解。
//
// ⛔ op allowlist 是安全邊界不是方便設定：這支工具只准打對話類端點。發布、賣家
// 回價、接受報價、付款、寄邀請都會產生對外副作用——商務安全不變式由 hermetic 與
// DB 測試驗，不透過批次 eval 去發通知給真人。

// liveOps 是唯一允許的 op 集合。
var liveOps = map[string]bool{
	goldenset.OpTurn:   true,
	goldenset.OpAnswer: true,
	goldenset.OpResume: true,
	goldenset.OpSearch: true,
}

// forbiddenPathFragments 是第二道防線：就算之後有人加了新 op 並忘了想清楚，
// 只要它打到這些路徑就會被擋下來。第一道（liveOps）擋意圖，這道擋後果。
var forbiddenPathFragments = []string{
	"rfq-publications", "buyer-quote-acceptances", "seller-responses",
	"payments", "invites", "checkout",
}

// devHostAllowed 只允許 dev 與本機。
//
// ⛔ 這支工具會用合成買家送出真的對話並呼叫真模型。指向 production 等於在正式
// 環境製造資料與費用，而且那些資料混在真買家裡面無法乾淨清掉。
func devHostAllowed(base string) error {
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("base URL 不合法：%w", err)
	}
	host := u.Hostname()
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return nil
	}
	// ⛔ 用完整主機名比對，不用前綴。只比前綴的話 test-api.<任何人的網域> 都會
	// 通過——那等於讓別人決定這支工具打去哪裡。
	for _, allowed := range []string{
		"test-api.example.com",
		"dev-api.example.com",
	} {
		if host == allowed {
			return nil
		}
	}
	return fmt.Errorf("base host %q 不在 dev allowlist 內——這支工具只能打 dev", host)
}

func checkScenarioOps(spec goldenset.Scenario) error {
	for i, t := range spec.Turns {
		if !liveOps[t.Op] {
			return fmt.Errorf("%s turn[%d]：op %q 不在 real-model eval 的 allowlist 內", spec.ID, i, t.Op)
		}
	}
	return nil
}

func checkPathAllowed(path string) error {
	for _, bad := range forbiddenPathFragments {
		if strings.Contains(path, bad) {
			return fmt.Errorf("拒絕呼叫 %q：會產生對外副作用，不在 eval 的範圍內", path)
		}
	}
	return nil
}

// scenarioLimits 是「別把 dev 打爆、也別讓一次評測跑到天荒地老」的邊界。
type scenarioLimits struct {
	Case     string
	MaxCases int
	MaxTurns int
}

func (l scenarioLimits) selectCases(all []goldenset.Scenario) []goldenset.Scenario {
	out := make([]goldenset.Scenario, 0, len(all))
	for _, s := range all {
		if l.Case != "" && s.ID != l.Case {
			continue
		}
		out = append(out, s)
		if l.MaxCases > 0 && len(out) >= l.MaxCases {
			break
		}
	}
	return out
}

func (l scenarioLimits) turnsOf(spec goldenset.Scenario) []goldenset.ScenarioTurn {
	if l.MaxTurns > 0 && len(spec.Turns) > l.MaxTurns {
		return spec.Turns[:l.MaxTurns]
	}
	return spec.Turns
}

// envelope 是 Gateway 的統一外層。data 之外的 code／message 決定這次呼叫成不成立。
type envelope struct {
	Code    float64
	Message string
	Data    map[string]any
}

func unwrapEnvelope(resp map[string]any) (envelope, error) {
	e := envelope{}
	if resp == nil {
		return e, fmt.Errorf("空回應")
	}
	e.Code, _ = resp["code"].(float64)
	e.Message, _ = resp["message"].(string)
	e.Data, _ = resp["data"].(map[string]any)
	if e.Code != 0 {
		// ⛔ 非零 code 一律是失敗，不得當成 PASS。
		return e, fmt.Errorf("業務錯誤 code=%v message=%v", resp["code"], e.Message)
	}
	return e, nil
}

// observeLive 把一次對話回應攤成與離線 replay 同形狀的觀察。
//
// ⛔ PublicationCount 維持 nil：這支工具沒有權威來源可以證明「這一輪沒有發布」
// ——它看到的只是自己收到的回應。留 nil 讓 comparator 判成 evaluation_error，
// 比謊報一個 0 誠實。要驗零副作用請用 DB 測試。
func observeLive(data map[string]any) goldenset.TurnObservation {
	obs := goldenset.TurnObservation{Slots: map[string]any{}}
	slots, _ := data["slots"].(map[string]any)
	for k, v := range slots {
		if v == nil {
			continue
		}
		if k == "item_resolution" {
			res, _ := v.(map[string]any)
			if kind, _ := res["kind"].(string); kind != "" {
				obs.Slots["item_resolution_kind"] = kind
			}
			if code, _ := res["code"].(string); code != "" {
				obs.Slots["item_resolution_code"] = code
			}
			continue
		}
		obs.Slots[k] = v
	}
	if missing, ok := data["missingRequired"].([]any); ok {
		for _, m := range missing {
			if s, _ := m.(string); s != "" {
				obs.MissingRequired = append(obs.MissingRequired, s)
			}
		}
	}
	return obs
}

// pickOptionID 依 label 唯一命中當輪的選項。
//
// ⛔ 不得固定選第一筆：那樣「選項順序或內容改變」這種回歸在 corpus 上完全看不
// 出來。命中零筆或多筆都要報錯，讓 corpus 自己去修清楚。
func pickOptionID(question map[string]any, label string) (string, error) {
	opts, _ := question["options"].([]any)
	var hits []string
	for _, raw := range opts {
		opt, _ := raw.(map[string]any)
		text, _ := opt["label"].(string)
		if text == "" {
			text, _ = opt["text"].(string)
		}
		if text == label {
			if id, _ := opt["optionId"].(string); id != "" {
				hits = append(hits, id)
			}
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return "", fmt.Errorf("option_label %q 在這一輪的選項裡找不到", label)
	default:
		return "", fmt.Errorf("option_label %q 命中 %d 個選項，corpus 必須改成唯一", label, len(hits))
	}
}

func writeReport(path string, r goldenset.ScenarioReport) error {
	blob, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化報告失敗：%w", err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		return fmt.Errorf("寫入報告失敗：%w", err)
	}
	return nil
}

func readBaseline(path string) (goldenset.ScenarioReport, error) {
	var r goldenset.ScenarioReport
	blob, err := os.ReadFile(path)
	if err != nil {
		return r, fmt.Errorf("讀不到 baseline：%w", err)
	}
	if err := json.Unmarshal(blob, &r); err != nil {
		return r, fmt.Errorf("baseline 不是合法報告：%w", err)
	}
	if r.Metadata.SchemaVersion != goldenset.ReportSchemaVersion {
		return r, fmt.Errorf("baseline schemaVersion=%d，本工具產的是 %d，不可比",
			r.Metadata.SchemaVersion, goldenset.ReportSchemaVersion)
	}
	if r.Metadata.CorpusHash == "" {
		return r, fmt.Errorf("baseline 缺 corpusHash，無法判斷是否可比")
	}
	return r, nil
}

func newScenarioReport(scenarios []goldenset.Scenario, results []goldenset.ScenarioResult, model, provider string) goldenset.ScenarioReport {
	meta := goldenset.ReportMetadata{
		SchemaVersion: goldenset.ReportSchemaVersion,
		Mode:          "live",
		CorpusHash:    goldenset.CorpusHash(scenarios),
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		// ⛔ 取不到就寫 unavailable，不寫 0 或空字串——報告一旦把未知說成零，
		// 之後據此做的判斷都是錯的。
		BackendSHA: envOr("EVAL_BACKEND_SHA", goldenset.Unavailable),
		WebSHA:     envOr("EVAL_WEB_SHA", goldenset.Unavailable),
		Model:      valueOr(model, goldenset.Unavailable),
		Provider:   valueOr(provider, goldenset.Unavailable),
	}
	return goldenset.ScenarioReport{Metadata: meta, Results: results, Summary: goldenset.Summarize(results)}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func valueOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

type scenarioEvalOptions struct {
	Base         string
	CorpusPath   string
	ReportPath   string
	BaselinePath string
	Limits       scenarioLimits
}

// runScenarioEval 回傳 process exit code。
//
// ⛔ exit code 只表達「工具有沒有跑完」，不表達「模型品質合不合格」。品質差異在
// 報告的 summary 與 diff 裡，工具成功不得被讀成 PASS——否則一次模型退步會因為
// 工具正常結束而被當成綠燈。
func runScenarioEval(opt scenarioEvalOptions) int {
	if err := devHostAllowed(opt.Base); err != nil {
		fmt.Println(err)
		return 2
	}
	raw, err := os.ReadFile(opt.CorpusPath)
	if err != nil {
		fmt.Println("讀不到情境 corpus：", err)
		return 2
	}
	scenarios, err := goldenset.LoadScenarios(raw)
	if err != nil {
		fmt.Println("情境 corpus 不合法：", err)
		return 2
	}
	for _, s := range scenarios {
		if err := checkScenarioOps(s); err != nil {
			fmt.Println(err)
			return 2
		}
	}

	var baseline goldenset.ScenarioReport
	haveBaseline := false
	if opt.BaselinePath != "" {
		baseline, err = readBaseline(opt.BaselinePath)
		if err != nil {
			fmt.Println(err)
			return 2
		}
		haveBaseline = true
	}

	selected := opt.Limits.selectCases(scenarios)
	if len(selected) == 0 {
		fmt.Println("沒有符合條件的 scenario")
		return 2
	}

	runner, err := newLiveRunner(opt.Base)
	if err != nil {
		fmt.Println(err)
		return 2
	}

	results := make([]goldenset.ScenarioResult, 0, len(selected))
	for _, spec := range selected {
		results = append(results, runner.run(spec, opt.Limits))
	}

	report := newScenarioReport(scenarios, results, runner.model, runner.provider)
	if opt.ReportPath != "" {
		if err := writeReport(opt.ReportPath, report); err != nil {
			fmt.Println(err)
			return 2
		}
		fmt.Println("報告已寫入", opt.ReportPath)
	}

	s := report.Summary
	fmt.Printf("\n=== 多輪情境評測（live）===\n")
	fmt.Printf("case %d：通過 %d、未達預期 %d、沒跑完 %d、known_gap %d\n",
		s.Total, s.Passed, s.Failed, s.Errored, s.KnownGaps)
	fmt.Printf("輪數 %d/%d\n", s.TurnsRun, s.TurnsTotal)

	if haveBaseline {
		diff := goldenset.DiffReports(baseline, report)
		if !diff.Comparable {
			fmt.Println("\n⚠️  與 baseline 不可比：", diff.Incomparable)
		} else {
			fmt.Printf("\n新增失敗 %d、修好 %d、缺的 case %d、新 case %d、known_gap 變化 %d\n",
				len(diff.NewFailures), len(diff.Fixed), len(diff.MissingCases),
				len(diff.NewCases), len(diff.GapChanges))
			for _, f := range diff.NewFailures {
				fmt.Println("  ✗ 新增失敗：", f)
			}
			for _, f := range diff.MissingCases {
				fmt.Println("  ？ baseline 有但這次沒跑：", f)
			}
		}
	}

	// 工具跑完就是 0。品質請看上面的 summary 與 diff。
	if s.Errored > 0 {
		return 1
	}
	return 0
}

// liveRunner 用合成買家跑真的對話。
type liveRunner struct {
	client   *evalClient
	model    string
	provider string
}

func newLiveRunner(base string) (*liveRunner, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET 未設定——這支工具要鑄 dev token 才打得了 buyer 端點")
	}
	accountID := identity.AccountID(time.Now().UnixNano() % 900000000)
	tok, err := mintHS256(secret, accountID)
	if err != nil {
		return nil, fmt.Errorf("鑄 token 失敗：%w", err)
	}
	c := &evalClient{base: base, token: tok, http: &http.Client{Timeout: 90 * time.Second}}
	if _, err := c.post("/app/buyers/me", map[string]any{"nickname": "scenario-eval"}); err != nil {
		return nil, fmt.Errorf("provision buyer 失敗：%w", err)
	}
	// ⛔ model／provider 由環境提供；取不到就留空，newScenarioReport 會寫成
	// unavailable。不猜一個名字填進報告——那會讓兩份報告看起來可比而其實不是。
	return &liveRunner{client: c, model: os.Getenv("EVAL_MODEL"), provider: os.Getenv("EVAL_PROVIDER")}, nil
}

// run 跑完一個 scenario。
//
// ⛔ 每個 scenario 開自己的 request（requestId 從空字串起手），彼此不共用狀態。
// clientRequestId 在同一輪固定、下一輪才換——同一個命令重送要能被冪等接住，
// 換一輪就必須是新命令。
func (r *liveRunner) run(spec goldenset.Scenario, limits scenarioLimits) goldenset.ScenarioResult {
	res := goldenset.ScenarioResult{
		ScenarioID: spec.ID, Capability: spec.Capability, Status: spec.Status,
		TrackedBy: spec.TrackedBy, TurnsTotal: len(spec.Turns),
	}
	turns := limits.turnsOf(spec)
	observations := make([]goldenset.TurnObservation, 0, len(turns))

	requestID := ""
	var lastQuestion map[string]any
	var activeRunID string

	for i, turn := range turns {
		started := time.Now()
		obs, next, err := r.step(spec, i, turn, requestID, lastQuestion, activeRunID)
		res.LatencyMS = append(res.LatencyMS, time.Since(started).Milliseconds())
		if err != nil {
			// transport／業務錯誤都算 errored，不是「跑完但不符預期」。
			observations = append(observations, goldenset.TurnObservation{ErrorCode: err.Error()})
			res.Errored = true
			break
		}
		requestID, lastQuestion, activeRunID = next.requestID, next.question, next.runID
		observations = append(observations, obs)
	}

	res.TurnsRun = len(observations)
	// ⛔ 只比實際跑過的輪次。--max-turns 截短時不得把沒跑的輪次算成失敗。
	truncated := spec
	truncated.Turns = turns[:len(observations)]
	res.Failures = goldenset.CompareScenario(truncated, observations)
	return res
}

type stepState struct {
	requestID string
	question  map[string]any
	runID     string
}

func (r *liveRunner) step(
	spec goldenset.Scenario, idx int, turn goldenset.ScenarioTurn,
	requestID string, question map[string]any, runID string,
) (goldenset.TurnObservation, stepState, error) {
	key := fmt.Sprintf("%s-t%d", spec.ID, idx)
	var path string
	var body map[string]any

	switch turn.Op {
	case goldenset.OpTurn:
		path = "/app/buyers/me/procurement/turns"
		body = map[string]any{"requestId": requestID, "message": turn.Message, "clientRequestId": key}
	case goldenset.OpAnswer:
		if requestID == "" {
			return goldenset.TurnObservation{}, stepState{}, fmt.Errorf("op=answer 需要先有 request")
		}
		path = "/app/buyers/me/procurement/requests/" + requestID + "/answers"
		body = map[string]any{"slotKey": turn.SlotKey, "freeText": turn.FreeText, "clientRequestId": key}
		if turn.UnitOptionID != "" {
			body["optionId"] = turn.UnitOptionID
		}
	case goldenset.OpResume:
		// ⛔ 消歧答案走 resume，不送 legacy /answers：後者不會把答案接回掛起的
		// interrupt，會被當成新的一輪重跑。
		if runID == "" {
			return goldenset.TurnObservation{}, stepState{}, fmt.Errorf("op=resume 需要一個掛起中的 run")
		}
		optionID, err := pickOptionID(question, turn.OptionLabel)
		if err != nil {
			return goldenset.TurnObservation{}, stepState{}, err
		}
		path = "/app/buyers/me/procurement/runs/" + runID + "/resume"
		body = map[string]any{"clientRequestId": key, "payload": map[string]any{"optionId": optionID}}
	case goldenset.OpSearch:
		if requestID == "" {
			return goldenset.TurnObservation{}, stepState{}, fmt.Errorf("op=search 需要先有 request")
		}
		path = "/app/buyers/me/procurement/requests/" + requestID + "/candidate-searches"
		body = map[string]any{"clientRequestId": key}
	default:
		return goldenset.TurnObservation{}, stepState{}, fmt.Errorf("op %q 不在 allowlist 內", turn.Op)
	}

	if err := checkPathAllowed(path); err != nil {
		return goldenset.TurnObservation{}, stepState{}, err
	}

	resp, err := r.client.post(path, body)
	if err != nil {
		return goldenset.TurnObservation{}, stepState{}, fmt.Errorf("transport：%w", err)
	}
	env, err := unwrapEnvelope(resp)
	if err != nil {
		return goldenset.TurnObservation{}, stepState{}, err
	}

	data := env.Data
	// resume 的結果包在 data.result 裡。
	if inner, ok := data["result"].(map[string]any); ok && inner != nil {
		data = inner
	}

	next := stepState{requestID: requestID}
	if id, _ := data["requestId"].(string); id != "" {
		next.requestID = id
	}
	next.question, _ = data["question"].(map[string]any)
	if id, _ := data["runId"].(string); id != "" {
		next.runID = id
	} else {
		next.runID = runID
	}
	return observeLive(data), next, nil
}
