package goldenset

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// 多輪情境 corpus。既有的 golden_queries.yaml 是「一句話 → 品項身分」的 IR 評測，
// 這一層驗的是另一件事：**跨輪次的狀態關係**——補充、修正、否定、消歧續答之後，
// 需求語意有沒有維持正確。兩者刻意分開，不合併成一個檔。
//
// ⛔ 這裡的型別不是 production wire contract，不上 API。

const ScenarioSchemaVersion = 1

// 能力分級。corpus 只描述**現在**的能力，不當願望清單：
// 把還不支援的東西寫成 supported，離線測試會綠而買家仍然踩雷。
const (
	CapabilitySupported  = "supported"
	CapabilityPartial    = "partial"
	CapabilityNotModeled = "not_modeled"
)

const (
	ScenarioStatusActive   = "active"
	ScenarioStatusKnownGap = "known_gap"
)

// 一輪可以做的事。op 決定 replay 要走哪一條 production 入口。
const (
	OpTurn   = "turn"   // 買家送出一句話
	OpAnswer = "answer" // 買家回答某個欄位的提問
	OpResume = "resume" // 買家回答消歧提問（續答既有 interrupt）
	OpSearch = "search" // 觸發候選搜尋
)

type ScenarioSet struct {
	SchemaVersion int        `yaml:"schema_version"`
	Scenarios     []Scenario `yaml:"scenarios"`
}

type Scenario struct {
	ID         string         `yaml:"id"`
	Tags       []string       `yaml:"tags"`
	Capability string         `yaml:"capability"`
	Fixture    string         `yaml:"fixture"`
	FrozenAt   string         `yaml:"frozen_at"`
	Status     string         `yaml:"status"`
	TrackedBy  string         `yaml:"tracked_by"`
	Turns      []ScenarioTurn `yaml:"turns"`
}

type ScenarioTurn struct {
	Op      string `yaml:"op"`
	Message string `yaml:"message"`
	SlotKey string `yaml:"slot_key"`
	// FreeText／OptionLabel／UnitOptionID 是買家在提問卡上的作答形狀。
	// ⛔ OptionLabel 必須唯一命中**當輪**的選項，不是固定選第一筆——固定選第一筆
	// 會讓「選項順序改變」這類回歸在 corpus 上完全看不出來。
	FreeText     string `yaml:"free_text"`
	UnitOptionID string `yaml:"unit_option_id"`
	OptionLabel  string `yaml:"option_label"`
	// StubPatch 是**固定的外部抽取回應**，不是期待結果。期望 slots 要由
	// production 的 apply 跑完之後再比，直接把 expect 複製成 observation
	// 等於測試自己跟自己比對。
	StubPatch map[string]any      `yaml:"stub_patch"`
	Expect    ScenarioExpectation `yaml:"expect"`
}

// ScenarioExpectation 的指標欄位是刻意的：YAML 裡「沒寫這個 key」與「明寫成空」
// 是兩件不同的事。前者代表這一輪不對該欄位作斷言，後者代表它必須是空的。
// 用值型別會把兩者壓成同一個零值，於是「本來該是空卻有東西」永遠測不出來。
type ScenarioExpectation struct {
	Slots              map[string]any `yaml:"slots"`
	MissingRequired    *[]string      `yaml:"missing_required"`
	NextAction         string         `yaml:"next_action"`
	CandidateSourceIDs *[]string      `yaml:"candidate_source_ids"`
	NoPublication      bool           `yaml:"no_publication"`
}

// TurnObservation 是 replay 實際觀察到的結果。
type TurnObservation struct {
	Slots              map[string]any
	MissingRequired    []string
	NextAction         string
	CandidateSourceIDs []string
	// PublicationCount 為 nil 代表**沒有取得可驗證的證據**，不等於零。
	// 把「沒觀察到」當成「沒發生」是這類測試最容易說謊的地方。
	PublicationCount *int
	ErrorCode        string
}

type ScenarioFailure struct {
	ScenarioID string `json:"scenarioId"`
	Turn       int    `json:"turn"`
	Field      string `json:"field"`
	Expected   any    `json:"expected"`
	Actual     any    `json:"actual"`
	Category   string `json:"category"`
}

// 失敗分類。與 R10 的 real-model eval 共用同一組值，報告才能對齊。
const (
	CategoryExtraction      = "extraction"
	CategoryIdentityMapping = "identity_mapping"
	CategoryUnitBasis       = "unit_basis"
	CategoryStateTransition = "state_transition"
	CategoryEvaluationError = "evaluation_error"
)

// LoadScenarios 嚴格解析：未知 key、重複 ID、未知 op、known_gap 缺 tracked_by 一律拒絕。
//
// ⛔ 嚴格是刻意的。corpus 的價值在於「加一條新問句主要是改資料」，而那個前提是
// 資料本身不會靜默失效——打錯一個 key 就被忽略的 loader，會讓人以為某條關係有被
// 驗到，實際上那一整段從來沒跑過。
func LoadScenarios(raw []byte) ([]Scenario, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)

	var set ScenarioSet
	if err := dec.Decode(&set); err != nil {
		return nil, fmt.Errorf("parse scenarios: %w", err)
	}
	if set.SchemaVersion != ScenarioSchemaVersion {
		return nil, fmt.Errorf("schema_version = %d，只支援 %d", set.SchemaVersion, ScenarioSchemaVersion)
	}
	if len(set.Scenarios) == 0 {
		return nil, fmt.Errorf("scenarios 不可為空")
	}

	seen := make(map[string]int, len(set.Scenarios))
	for i, s := range set.Scenarios {
		if err := ValidateScenario(s); err != nil {
			return nil, fmt.Errorf("scenario[%d] %q: %w", i, s.ID, err)
		}
		if prev, dup := seen[s.ID]; dup {
			return nil, fmt.Errorf("scenario id %q 重複（scenario[%d] 與 scenario[%d]）", s.ID, prev, i)
		}
		seen[s.ID] = i
	}
	return set.Scenarios, nil
}

func ValidateScenario(spec Scenario) error {
	if strings.TrimSpace(spec.ID) == "" {
		return fmt.Errorf("id 不可為空")
	}
	switch spec.Capability {
	case CapabilitySupported, CapabilityPartial, CapabilityNotModeled:
	default:
		return fmt.Errorf("capability = %q，只允許 supported／partial／not_modeled", spec.Capability)
	}
	switch spec.Status {
	case ScenarioStatusActive:
	case ScenarioStatusKnownGap:
		// ⛔ known_gap 必須指向一個真實的追蹤項。沒有 owner 的 known_gap 會變成
		// 永久豁免：測試不會紅，也沒有人負責讓它有一天變綠。
		if strings.TrimSpace(spec.TrackedBy) == "" {
			return fmt.Errorf("status=known_gap 必須有 tracked_by")
		}
	default:
		return fmt.Errorf("status = %q，只允許 active／known_gap", spec.Status)
	}
	if strings.TrimSpace(spec.Fixture) == "" {
		return fmt.Errorf("fixture 不可為空")
	}
	if strings.TrimSpace(spec.FrozenAt) == "" {
		return fmt.Errorf("frozen_at 不可為空")
	}
	if len(spec.Turns) == 0 {
		return fmt.Errorf("turns 不可為空")
	}
	for i, t := range spec.Turns {
		switch t.Op {
		case OpTurn, OpAnswer, OpResume, OpSearch:
		default:
			return fmt.Errorf("turn[%d] op = %q，只允許 turn／answer／resume／search", i, t.Op)
		}
		if t.Op == OpTurn && strings.TrimSpace(t.Message) == "" {
			return fmt.Errorf("turn[%d] op=turn 必須有 message", i)
		}
		if t.Op == OpAnswer && strings.TrimSpace(t.SlotKey) == "" {
			return fmt.Errorf("turn[%d] op=answer 必須有 slot_key", i)
		}
	}
	return nil
}

// CompareScenario 比對期望與觀察，回傳精準到 case／turn／field 的差異。
//
// ⛔ 只比對明確指定的欄位。未指定 ≠ 期望為空，見 ScenarioExpectation 的說明。
func CompareScenario(spec Scenario, got []TurnObservation) []ScenarioFailure {
	var out []ScenarioFailure
	add := func(turn int, field string, expected, actual any, category string) {
		out = append(out, ScenarioFailure{
			ScenarioID: spec.ID, Turn: turn, Field: field,
			Expected: expected, Actual: actual, Category: category,
		})
	}

	if len(got) != len(spec.Turns) {
		add(len(got), "turns", len(spec.Turns), len(got), CategoryEvaluationError)
		return out
	}

	for i, turn := range spec.Turns {
		obs := got[i]
		if obs.ErrorCode != "" {
			add(i, "error_code", "", obs.ErrorCode, CategoryEvaluationError)
			continue
		}

		for key, want := range turn.Expect.Slots {
			actual, present := obs.Slots[key]
			if !present {
				add(i, "slots."+key, want, nil, slotCategory(key))
				continue
			}
			if !slotValueEqual(want, actual) {
				add(i, "slots."+key, want, actual, slotCategory(key))
			}
		}

		if turn.Expect.MissingRequired != nil {
			want := append([]string(nil), *turn.Expect.MissingRequired...)
			actual := append([]string(nil), obs.MissingRequired...)
			sort.Strings(want)
			sort.Strings(actual)
			if !stringsEqual(want, actual) {
				add(i, "missing_required", want, actual, CategoryStateTransition)
			}
		}

		if turn.Expect.NextAction != "" && turn.Expect.NextAction != obs.NextAction {
			add(i, "next_action", turn.Expect.NextAction, obs.NextAction, CategoryStateTransition)
		}

		if turn.Expect.CandidateSourceIDs != nil {
			// ⛔ 候選集合依 ID 排序後比對：排序是呈現層的事，不是這條關係要守的。
			want := append([]string(nil), *turn.Expect.CandidateSourceIDs...)
			actual := append([]string(nil), obs.CandidateSourceIDs...)
			sort.Strings(want)
			sort.Strings(actual)
			if !stringsEqual(want, actual) {
				add(i, "candidate_source_ids", want, actual, CategoryIdentityMapping)
			}
		}

		if turn.Expect.NoPublication {
			switch {
			case obs.PublicationCount == nil:
				// 沒有觀察到 ≠ 沒有發生。
				add(i, "publication_count", 0, nil, CategoryEvaluationError)
			case *obs.PublicationCount != 0:
				add(i, "publication_count", 0, *obs.PublicationCount, CategoryStateTransition)
			}
		}
	}
	return out
}

// slotCategory 把欄位對應到失敗分類，讓報告能回答「壞的是哪一層」。
func slotCategory(key string) string {
	switch key {
	case "item_resolution", "item_resolution_kind":
		return CategoryIdentityMapping
	case "unit":
		return CategoryUnitBasis
	default:
		return CategoryExtraction
	}
}

// slotValueEqual 先把數值正規化再比。
//
// ⛔ YAML 的整數解成 int、JSON 往返之後是 float64——同一個 50 在兩條路徑上型別
// 不同。不正規化就會把實作細節報成 regression。ID 一律當字串比，不轉數值
// （精度）。
func slotValueEqual(want, actual any) bool {
	if wf, ok := numericValue(want); ok {
		if af, ok := numericValue(actual); ok {
			return wf == af
		}
		return false
	}
	if wm, ok := want.(map[string]any); ok {
		am, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		// 遞迴 subset：只比對有寫出來的鍵。
		for k, v := range wm {
			av, present := am[k]
			if !present || !slotValueEqual(v, av) {
				return false
			}
		}
		return true
	}
	return fmt.Sprintf("%v", want) == fmt.Sprintf("%v", actual)
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
