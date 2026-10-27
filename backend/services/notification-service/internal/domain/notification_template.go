package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type FieldSpec struct {
	Const *string `json:"const,omitempty"`
	Field *string `json:"field,omitempty"`
}

func (s FieldSpec) Resolve(payload map[string]interface{}) (interface{}, bool) {
	if s.Const != nil {
		return *s.Const, true
	}
	if s.Field != nil {
		v, ok := payload[*s.Field]
		return v, ok
	}
	return nil, false
}

// OpNe is the only comparison operator supported by FilterCondition besides
// plain equality. This is intentionally NOT a general query language (issue
// #1467) — just enough to express "payload_filter key must not equal this
// value", which is what excluding a more specific template's trigger value
// from a general-purpose template requires.
const OpNe = "$ne"

// FilterCondition is a single payload_filter match condition for one payload
// key. Two JSON shapes are accepted:
//   - Equality (default): a plain JSON string, e.g. "IN_TRANSIT"
//   - Exclusion: a single-key JSON object {"$ne": "9104"} (value may also be a
//     bare JSON number, e.g. {"$ne": 9104} — tolerated for author convenience)
//
// Any other shape (multi-key object, unsupported operator, array, ...) fails
// to unmarshal — template loading fails loudly rather than silently no-op
// filtering.
type FilterCondition struct {
	Op    string // "" = equality, OpNe = not-equal
	Value string
}

// Eq builds an equality FilterCondition (the default/legacy form).
func Eq(value string) FilterCondition { return FilterCondition{Value: value} }

// Ne builds a $ne (exclusion) FilterCondition.
func Ne(value string) FilterCondition { return FilterCondition{Op: OpNe, Value: value} }

func (c *FilterCondition) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("payload_filter condition: empty value")
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("payload_filter condition: %w", err)
		}
		c.Op = ""
		c.Value = s
		return nil
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("payload_filter condition: %w", err)
		}
		raw, ok := obj[OpNe]
		if !ok || len(obj) != 1 {
			return fmt.Errorf("payload_filter condition: object form must have exactly one key %q, got %s", OpNe, string(data))
		}
		v, err := scalarJSONToString(raw)
		if err != nil {
			return fmt.Errorf("payload_filter condition: %s value: %w", OpNe, err)
		}
		c.Op = OpNe
		c.Value = v
		return nil
	default:
		return fmt.Errorf("payload_filter condition: must be a JSON string or {%q: ...} object, got %s", OpNe, string(data))
	}
}

func (c FilterCondition) MarshalJSON() ([]byte, error) {
	if c.Op == OpNe {
		return json.Marshal(map[string]string{OpNe: c.Value})
	}
	return json.Marshal(c.Value)
}

// scalarJSONToString tolerantly decodes a JSON scalar (string or number) to
// its string form, preserving numeric precision via json.Number so large
// integers don't round-trip through float64.
func scalarJSONToString(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var n json.Number
	if err := dec.Decode(&n); err == nil {
		return n.String(), nil
	}
	return "", fmt.Errorf("must be a string or number, got %s", string(raw))
}

type NotificationTemplate struct {
	DataFields          map[string]FieldSpec
	PayloadFilter       map[string]FilterCondition // e.g. {"status": "IN_TRANSIT"} or {"cancel_reason": {"$ne": "9104"}}; nil = 不過濾
	BodyFallback        *string
	SchemaURL           *string
	ReferenceIDTemplate *string
	ImageKeyTemplate    *string // image objectKey 模板（#{field} 語法）；渲染後由 mediaurl.BuildURL 組顯示 URL（49 §5.3 #10）
	UpdatedBy           *int64
	Exchange            string
	RoutingKey          string
	EventType           string
	TargetUserType      string
	TargetUserField     string
	Category            string
	TitleTemplate       string
	BodyTemplate        string
	DataType            string
	PushMode            string
	ID                  int64
	Enabled             bool
	// IdentityMigrated mirrors notification_templates.identity_migrated. It is a
	// seller-template audit/contract marker retained after the final canonical
	// cutover; it must not be used as an account_id fallback or runtime routing
	// switch. Buyer rows do not use this marker.
	IdentityMigrated bool
}

type NotificationTemplateRepository interface {
	FindAll(ctx context.Context) ([]*NotificationTemplate, error)
	FindEnabled(ctx context.Context) ([]*NotificationTemplate, error)
	FindByID(ctx context.Context, id int64) (*NotificationTemplate, error)
	Insert(ctx context.Context, t *NotificationTemplate) (int64, error)
	Update(ctx context.Context, t *NotificationTemplate) error
	UpdateEnabled(ctx context.Context, id int64, enabled bool) error
}
