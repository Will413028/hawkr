package goldenset

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const StatusKnownGap = "known_gap"

type Query struct {
	Text           string
	ExpectCategory string
	ExpectLabel    string
	ExpectKind     string
	Note           string
	Status         string
	TrackedBy      string
	Message        string
}

func (q Query) ExpectDescription() string {
	if q.ExpectsNIL() {
		return "NIL（逃生艙）"
	}
	parts := make([]string, 0, 2)
	if q.ExpectCategory != "" {
		parts = append(parts, q.ExpectCategory)
	}
	if q.ExpectLabel != "" {
		label := q.ExpectLabel
		if q.ExpectKind != "" {
			label += "（" + q.ExpectKind + "）"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " ／ ")
}

func (q Query) ExpectsNIL() bool {
	return q.ExpectCategory == "" && q.ExpectLabel == "" && q.ExpectKind == ""
}

type rawQuery struct {
	Query   string `yaml:"query"`
	Expect  any    `yaml:"expect"`
	Note    string `yaml:"note"`
	Status  string `yaml:"status"`
	Tracked string `yaml:"tracked_by"`
	Message string `yaml:"message"`
}

func Load(raw []byte) ([]Query, error) {
	var rows []rawQuery
	if err := yaml.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse golden set: %w", err)
	}
	out := make([]Query, 0, len(rows))
	for i, r := range rows {
		if r.Query == "" {
			return nil, fmt.Errorf("row %d: query 不可為空", i)
		}
		q := Query{Text: r.Query, Note: r.Note, Status: r.Status, TrackedBy: r.Tracked, Message: r.Message}
		if m, ok := r.Expect.(map[string]any); ok {
			if v, ok := m["category"].(string); ok {
				q.ExpectCategory = v
			}
			if v, ok := m["label"].(string); ok {
				q.ExpectLabel = v
			}
			if v, ok := m["kind"].(string); ok {
				q.ExpectKind = v
			}
		}
		if q.Status == StatusKnownGap && q.TrackedBy == "" {
			return nil, fmt.Errorf("row %d (%s): status=known_gap 必須有 tracked_by 票號", i, q.Text)
		}
		out = append(out, q)
	}
	return out, nil
}
