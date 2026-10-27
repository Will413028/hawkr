package goldenset

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// 情境評測報告。離線 replay 與 real-model eval 共用同一個形狀，兩邊的結果才能
// 互相對照，也才能跟上一次的 baseline 對齊比較。
//
// ⛔ 「取不到」與「零」必須分得開。metadata 拿不到時寫 Unavailable，不要寫 0 或
// 空字串——報告一旦把未知說成零，之後任何據此做的判斷都是錯的。
const (
	ReportSchemaVersion = 1
	// Unavailable 是 metadata 取不到時的明確值。
	Unavailable = "unavailable"
)

type ReportMetadata struct {
	SchemaVersion int    `json:"schemaVersion"`
	BackendSHA    string `json:"backendSha"`
	WebSHA        string `json:"webSha"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	PromptVersion string `json:"promptVersion"`
	CatalogVerson string `json:"catalogVersion"`
	CorpusHash    string `json:"corpusHash"`
	StartedAt     string `json:"startedAt"`
	// Mode 是 "offline"（stub 抽取）或 "live"（真模型）。離線綠只證明接線。
	Mode string `json:"mode"`
}

type ScenarioResult struct {
	ScenarioID string            `json:"scenarioId"`
	Capability string            `json:"capability"`
	Status     string            `json:"status"`
	TrackedBy  string            `json:"trackedBy,omitempty"`
	TurnsRun   int               `json:"turnsRun"`
	TurnsTotal int               `json:"turnsTotal"`
	LatencyMS  []int64           `json:"latencyMs,omitempty"`
	Failures   []ScenarioFailure `json:"failures"`
	// Errored 代表這個 case 沒跑完（transport／逾時等），與「跑完但不符預期」不同。
	Errored bool `json:"errored"`
}

func (r ScenarioResult) Passed() bool { return !r.Errored && len(r.Failures) == 0 }

// ScenarioReport 與既有 IR 評測的 Report（metrics.go）是兩份不同的東西：
// 那份量的是單句品項身分的 precision／recall，這份記的是多輪情境的逐輪差異。
type ScenarioReport struct {
	Metadata ReportMetadata   `json:"metadata"`
	Results  []ScenarioResult `json:"results"`
	Summary  ReportSummary    `json:"summary"`
}

type ReportSummary struct {
	Total int `json:"total"`
	// Passed 只算 active 的 case。known_gap 不計入通過率，否則把缺口標成
	// known_gap 就能把分數拉高。
	Passed     int `json:"passed"`
	Failed     int `json:"failed"`
	Errored    int `json:"errored"`
	KnownGaps  int `json:"knownGaps"`
	TurnsRun   int `json:"turnsRun"`
	TurnsTotal int `json:"turnsTotal"`
}

// CorpusHash 是 corpus 內容的指紋，讓兩份報告能判斷彼此可不可比。
// ⛔ 只吃 spec，不吃觀察值：corpus 沒變而結果變了，那才是真的回歸。
func CorpusHash(scenarios []Scenario) string {
	ids := make([]string, 0, len(scenarios))
	for _, s := range scenarios {
		ids = append(ids, s.ID+"|"+s.Capability+"|"+s.Status+"|"+s.FrozenAt)
		for _, t := range s.Turns {
			ids = append(ids, "  "+t.Op+"|"+t.Message+"|"+t.SlotKey+"|"+t.OptionLabel)
		}
	}
	sort.Strings(ids)
	h := sha256.New()
	for _, s := range ids {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func Summarize(results []ScenarioResult) ReportSummary {
	var s ReportSummary
	for _, r := range results {
		s.Total++
		s.TurnsRun += r.TurnsRun
		s.TurnsTotal += r.TurnsTotal
		if r.Status == ScenarioStatusKnownGap {
			s.KnownGaps++
			continue
		}
		switch {
		case r.Errored:
			s.Errored++
		case len(r.Failures) == 0:
			s.Passed++
		default:
			s.Failed++
		}
	}
	return s
}

// ReportDiff 是與 baseline 的差異。使用者只需要看這個。
type ReportDiff struct {
	// Comparable 為 false 時其餘欄位沒有意義——corpus 或執行模式不同就不可比。
	Comparable   bool     `json:"comparable"`
	Incomparable string   `json:"incomparable,omitempty"`
	NewFailures  []string `json:"newFailures"`
	Fixed        []string `json:"fixed"`
	MissingCases []string `json:"missingCases"`
	NewCases     []string `json:"newCases"`
	GapChanges   []string `json:"gapChanges"`
}

// DiffReports 依 scenarioID＋turn＋field 對齊。
//
// ⛔ 缺的 case 要列出來，不能只統計兩邊都有的那些：把一個一直失敗的 case 從
// corpus 拿掉，看起來會像修好了。
func DiffReports(baseline, current ScenarioReport) ReportDiff {
	if baseline.Metadata.CorpusHash != current.Metadata.CorpusHash {
		return ReportDiff{Comparable: false, Incomparable: "corpus 不同（corpusHash 不一致），結果不可比"}
	}
	if baseline.Metadata.Mode != current.Metadata.Mode {
		return ReportDiff{Comparable: false, Incomparable: "執行模式不同（offline／live），結果不可比"}
	}

	type key struct {
		id, field string
		turn      int
	}
	index := func(r ScenarioReport) (map[key]bool, map[string]ScenarioResult) {
		fails := map[key]bool{}
		cases := map[string]ScenarioResult{}
		for _, res := range r.Results {
			cases[res.ScenarioID] = res
			for _, f := range res.Failures {
				fails[key{f.ScenarioID, f.Field, f.Turn}] = true
			}
		}
		return fails, cases
	}
	baseFails, baseCases := index(baseline)
	curFails, curCases := index(current)

	out := ReportDiff{Comparable: true,
		NewFailures: []string{}, Fixed: []string{},
		MissingCases: []string{}, NewCases: []string{}, GapChanges: []string{}}

	describe := func(k key) string {
		return k.id + " turn " + itoa(k.turn) + " " + k.field
	}
	for k := range curFails {
		if !baseFails[k] {
			out.NewFailures = append(out.NewFailures, describe(k))
		}
	}
	for k := range baseFails {
		if !curFails[k] {
			out.Fixed = append(out.Fixed, describe(k))
		}
	}
	for id := range baseCases {
		if _, ok := curCases[id]; !ok {
			out.MissingCases = append(out.MissingCases, id)
		}
	}
	for id, cur := range curCases {
		base, ok := baseCases[id]
		if !ok {
			out.NewCases = append(out.NewCases, id)
			continue
		}
		if base.Status != cur.Status {
			out.GapChanges = append(out.GapChanges, id+"："+base.Status+" → "+cur.Status)
		}
	}
	sort.Strings(out.NewFailures)
	sort.Strings(out.Fixed)
	sort.Strings(out.MissingCases)
	sort.Strings(out.NewCases)
	sort.Strings(out.GapChanges)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
