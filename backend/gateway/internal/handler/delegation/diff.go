// Package delegation holds the two things every delegated mutation shares: the
// three-step audit sequence (orchestrator.go) and the projection rules that decide
// what may be written into the permanent audit table (this file).
//
// Both live here rather than in the four admin_delegated_*.go handlers on purpose.
// A handler that assembles its own changed_fields or its own snapshot is a handler
// that can quietly leak a field nobody registered, or skip the confirm step — and
// neither failure produces an error at the time it happens.
package delegation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
)

// Formatter is a closed enumeration. Adding an eighth one requires changing 56 號
// §6.1.12 first — the display rules are a contract with the audit UI, not a local
// convenience.
type Formatter string

const (
	FormatterPlain     Formatter = "plain"
	FormatterQty       Formatter = "qty"
	FormatterMoney     Formatter = "money"
	FormatterBool      Formatter = "bool"
	FormatterObjectKey Formatter = "objectKey"
	FormatterTruncate  Formatter = "truncate"
	FormatterEnum      Formatter = "enum"
)

// FieldRule is the single definition of "how does this resource field appear in the
// audit record, and may it be retained at all".
type FieldRule struct {
	// Label is the Chinese display name. This IS the SSOT: admin repo renders
	// changedFields[].label directly and must not keep a second field→label table,
	// or adding one field would mean editing two repos and the one that gets missed
	// shows an English field name.
	Label string
	// Formatter turns the raw projected value into the display value.
	Formatter Formatter
	// Arg is the unit literal for qty, the character budget for truncate, or the
	// CodeMeta enum name for enum. Always a literal — qty has no "take the product's
	// own unit" mode, because stock_qty is a product-level aggregate while the unit
	// lives on the SKU, and the unit column holds WeightUnit values (3301…) which
	// would print as digits.
	Arg string
	// InSnapshot decides whether the field is retained in before/after snapshot.
	// It is NOT the same question as "may the DTO return it": the DTO answers what
	// may be shown for one request, this answers what may be kept permanently.
	InSnapshot bool
}

// DelegationDiffRules is keyed by resourceType. A field that is not in the table is
// not diffed and not retained — that direction is the fail-safe one. The day a
// seller DTO grows a bankAccount field, it does not flow into a permanently stored,
// admin-readable audit table just because nobody remembered to register it.
var DelegationDiffRules = map[string]map[string]FieldRule{
	"storefront": {
		"name":          {Label: "店家名稱", Formatter: FormatterPlain, InSnapshot: true},
		"description":   {Label: "店家介紹", Formatter: FormatterTruncate, Arg: "60", InSnapshot: true},
		"prize":         {Label: "獲獎紀錄", Formatter: FormatterTruncate, Arg: "60", InSnapshot: true},
		"contactPhone":  {Label: "聯絡電話", Formatter: FormatterPlain, InSnapshot: true},
		"city":          {Label: "縣市", Formatter: FormatterPlain, InSnapshot: true},
		"district":      {Label: "鄉鎮市區", Formatter: FormatterPlain, InSnapshot: true},
		"addressDetail": {Label: "詳細地址", Formatter: FormatterPlain, InSnapshot: true},
		"websiteUrl":    {Label: "官方網站", Formatter: FormatterTruncate, Arg: "60", InSnapshot: true},
		"logoKey":       {Label: "商店 Logo", Formatter: FormatterObjectKey, InSnapshot: true},
		"coverKey":      {Label: "商店封面", Formatter: FormatterObjectKey, InSnapshot: true},
	},
	"product": {
		"name":         {Label: "商品名稱", Formatter: FormatterPlain, InSnapshot: true},
		"categoryName": {Label: "商品分類", Formatter: FormatterPlain, InSnapshot: true},
		"description":  {Label: "商品介紹", Formatter: FormatterTruncate, Arg: "60", InSnapshot: true},
		"origin":       {Label: "產地", Formatter: FormatterPlain, InSnapshot: true},
		// status / priceMin / priceMax / stockQty are read-only observations: they
		// still go into the snapshot, but a PATCH carrying them is ignored. The
		// denormalised writers behind the three aggregates deliberately do not bump
		// version, so expectedVersion is blind to them — making them editable would
		// let a delegated operator read a stale value and write it back with no
		// signal at all.
		"status":   {Label: "發布狀態", Formatter: FormatterEnum, Arg: "ProductStatus", InSnapshot: true},
		"priceMin": {Label: "最低售價", Formatter: FormatterMoney, InSnapshot: true},
		"priceMax": {Label: "最高售價", Formatter: FormatterMoney, InSnapshot: true},
		"stockQty": {Label: "庫存數量", Formatter: FormatterQty, Arg: "件", InSnapshot: true},
		// skus / images project to a COUNT, not to content: the content difference
		// blows past KB per row, and the three-column UI wants "規格從 2 項變 3 項".
		"skus":     {Label: "商品規格", Formatter: FormatterQty, Arg: "項", InSnapshot: true},
		"images":   {Label: "商品圖片", Formatter: FormatterQty, Arg: "張", InSnapshot: true},
		"hashtags": {Label: "主題標籤", Formatter: FormatterTruncate, Arg: "60", InSnapshot: true},
		// `enum` 而不是 `truncate`：投影層 join 的是 CertificationType **碼值**
		// （`3701、3702`），不像 hashtags join 的是本來就可讀的標籤名。詳見 §6.1.12。
		"certifications": {Label: "產品驗證", Formatter: FormatterEnum, Arg: "CertificationType", InSnapshot: true},
	},
	"post": {
		"content":            {Label: "貼文內容", Formatter: FormatterTruncate, Arg: "60", InSnapshot: true},
		"tags":               {Label: "主題標籤", Formatter: FormatterTruncate, Arg: "60", InSnapshot: true},
		"productIds":         {Label: "關聯商品", Formatter: FormatterQty, Arg: "項", InSnapshot: true},
		"imageKeys":          {Label: "貼文圖片", Formatter: FormatterQty, Arg: "張", InSnapshot: true},
		"publishStatus":      {Label: "發布狀態", Formatter: FormatterEnum, Arg: "PostPublishStatus", InSnapshot: true},
		"scheduledPublishAt": {Label: "排程發布時間", Formatter: FormatterPlain, InSnapshot: true},
		"publishedAt":        {Label: "發布時間", Formatter: FormatterPlain, InSnapshot: true},
	},
	// order is read-only: ORDER_PII_REVEAL changes no field, so the rule table is
	// empty and both snapshots carry {"revealedFields":[…]} — never the PII itself.
	"order": {},
}

// ReadOnlyFields lists, per resourceType, the projected fields a mutation request
// must never be able to set. They are silently ignored rather than rejected: a front
// end that echoes a whole DTO back should not fail the entire write.
func ReadOnlyFields(resourceType string) []string {
	switch resourceType {
	case "product":
		return []string{"status", "priceMin", "priceMax", "stockQty"}
	case "post":
		return []string{"publishStatus", "scheduledPublishAt", "publishedAt"}
	default:
		return nil
	}
}

// Project produces the before/after snapshot projection. resource is the domain DTO
// already normalised into scalars (arrays joined or counted, objects reduced to their
// display field); the result contains only InSnapshot=true keys that the resource
// actually carries.
func Project(resourceType string, resource map[string]any) map[string]any {
	rules, ok := DelegationDiffRules[resourceType]
	if !ok || len(resource) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(rules))
	for field, rule := range rules {
		if !rule.InSnapshot {
			continue
		}
		if v, present := resource[field]; present {
			out[field] = v
		}
	}
	return out
}

// ScopeProductStatus is the comparison scope of PRODUCT_PUBLISH / PRODUCT_UNPUBLISH /
// PRODUCT_BATCH_UNPUBLISH. 56 號 §6.1.11 明訂「PRODUCT_PUBLISH 這一列的 changedFields
// 只會有 status」——本函式就是那句話的實作，不是效能優化。
//
// 這四個函式回的是**新的 slice** 而不是套件層變數：套件層的 []string 是可寫的，任何
// 一個呼叫端 append 一次就改掉了其他所有呼叫端的比較範圍，而那個錯誤不會編譯失敗、
// 不會有測試轉紅，只會讓稽核從那天起少記或多記欄位。
func ScopeProductStatus() []string { return []string{"status"} }

// ScopePostPublishState is /posts/{id}/publish 與 /unpublish 的比較範圍。三個欄位而
// 不是一個：一次發布同時寫 publish_status 與 published_at，排程則同時寫
// scheduled_publish_at —— 少列任何一欄，那一欄的真實變更就不會出現在稽核上。
func ScopePostPublishState() []string {
	return []string{"publishStatus", "scheduledPublishAt", "publishedAt"}
}

// Diff produces changed_fields from two projections.
//
// scope restricts the comparison to a subset of the rule table; nil means the whole
// table. It is not an optimisation — it is what keeps the comparison well-defined.
// Diff's one premise is that before and after are two samples of the SAME projection
// over the SAME resource, so a key present on one side and absent on the other can
// only mean "the value changed". A status-flip endpoint breaks that premise: its write
// response is a command acknowledgement, not a resource snapshot, and every field it
// does not carry then reads as cleared.
//
// dev 實測（2026-08-16）：代行換一次店家 logo 產出 10 列 changed_fields，其中 9 列是
// 從未發生的變更（店名、地址、電話…全被記成「被清空」）。收窄比較範圍讓 before 與
// after 依定義取樣同一組欄位，缺鍵歧義從結構上消失——而不是要求十三個呼叫端各自記得
// 回一份完整資源。⚠️ Project() 刻意**不**跟著收窄：before_snapshot 仍存完整白名單投
// 影，「上架當下這件商品長什麼樣」才舉證得出來。
//
// scope 內不在規則表的欄名一律略過（與規則表本身的 fail-safe 同向）。⚠️ 這代表**打錯
// 一個字的 scope 會靜默退化成「什麼都不比較」**，所以每個 Scope* 函式的內容都由
// TestDiffScopes_OnlyReferenceRegisteredFields 逐項對規則表核對。
//
// Comparison uses the RAW values and formatting happens afterwards. The reverse order
// silently drops changes: truncate(60) makes two long texts that differ only at
// character 200 look identical, and comparing the display values would discard that
// edit entirely.
func Diff(resourceType string, before, after map[string]any, scope []string) []*accountpb.ChangedField {
	rules, ok := DelegationDiffRules[resourceType]
	if !ok {
		return nil
	}
	fields := make([]string, 0, len(rules))
	// len(scope)==0 而不是 scope==nil：非 nil 的空 slice 若走收窄分支，for-loop 跑零次
	// ⇒ changed_fields 恆為空，而 AST 閘門只看 key 在不在、看不出這種寫法。退回全表
	// 最壞只是「沒收窄」——那會被逐 action 的範圍測試當場抓到，不會靜默。
	if len(scope) == 0 {
		for f := range rules {
			fields = append(fields, f)
		}
	} else {
		for _, f := range scope {
			if _, registered := rules[f]; registered {
				fields = append(fields, f)
			}
		}
	}
	// Stable output: the audit row is evidence, and evidence that reorders itself
	// between two reads of the same write is harder to trust than it needs to be.
	sort.Strings(fields)

	out := make([]*accountpb.ChangedField, 0, len(fields))
	for _, field := range fields {
		rule := rules[field]
		bv, bok := before[field]
		av, aok := after[field]
		if !bok && !aok {
			continue
		}
		if bok && aok && rawEqual(bv, av) {
			continue
		}
		beforeText, afterText := format(rule, bv, bok), format(rule, av, aok)
		// 單側缺鍵時，缺席那一側渲染成該 formatter 的空形（qty 是「0 張」、plain 與
		// truncate 是「—」）。若在場那一側**也**渲染成同一個字串，這一列描述的是「從沒有
		// 變成沒有」——它不是變更，而它汙染的是不可竄改的稽核列。
		//
		// 建立類沒有 before（orchestrator 對 spec.ReadBefore == nil 給空投影），而
		// POST_CREATE 走全表比較，於是每建一則沒有配圖、沒有關聯商品的貼文都多記兩列
		// imageKeys「0 張 → 0 張」與 productIds「0 項 → 0 項」（dev 3 案獨立重現）。
		// 「從無到有」不受影響：after 有內容時兩側渲染必然不同，那一列照樣輸出。
		//
		// ⚠️ 兩側都在場時**不得**改用顯示值比較：truncate(60) 會讓只差在第 200 個字的
		// 兩段長文渲染成同一字串，那條真實變更會整個消失（見本函式開頭的比較順序說明）。
		if (!bok || !aok) && beforeText == afterText {
			continue
		}
		out = append(out, &accountpb.ChangedField{
			Field:  field,
			Label:  rule.Label,
			Before: beforeText,
			After:  afterText,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// rawEqual compares projected values. Projections are scalars by construction, so
// string comparison of the canonical rendering is exact here and avoids reflect.
func rawEqual(a, b any) bool {
	return rawString(a) == rawString(b)
}

func rawString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprint(t)
	}
}

// format renders one projected value for the three-column UI. present=false means the
// key was absent on that side; the output is the formatter's empty form, never null or
// undefined — ChangedField.before/.after are proto strings and the UI does not handle
// a missing value.
func format(rule FieldRule, v any, present bool) string {
	if !present {
		v = nil
	}
	raw := rawString(v)
	switch rule.Formatter {
	case FormatterQty:
		n := raw
		if n == "" {
			n = "0"
		}
		return n + " " + rule.Arg
	case FormatterMoney:
		n := raw
		if n == "" {
			n = "0"
		}
		return "NT$ " + groupThousands(n)
	case FormatterBool:
		if raw == "true" {
			return "是"
		}
		return "否"
	case FormatterObjectKey:
		if raw == "" {
			return "—"
		}
		// Only the last segment, and never a full URL: the complete key stays in the
		// snapshot, the display value only has to be recognisable.
		if i := strings.LastIndex(raw, "/"); i >= 0 && i+1 < len(raw) {
			return raw[i+1:]
		}
		return raw
	case FormatterTruncate:
		if raw == "" {
			return "—"
		}
		limit, err := strconv.Atoi(rule.Arg)
		if err != nil || limit <= 0 {
			return raw
		}
		runes := []rune(raw)
		if len(runes) <= limit {
			return raw
		}
		return string(runes[:limit]) + "…"
	case FormatterEnum:
		if raw == "" {
			return "—"
		}
		return enumLabelList(rule.Arg, raw)
	default: // FormatterPlain
		if raw == "" {
			return "—"
		}
		return raw
	}
}

// enumLabelList resolves a projected scalar that may hold several CodeMeta values
// joined with "、" (the projection layer's array normalisation, §6.1.12). A single
// value is the degenerate case — no separator means one segment — so this stays ONE
// formatter rather than a list-flavoured eighth one.
//
// Segments that resolve to nothing pass through as-is instead of collapsing the whole
// cell to "—": certifications are regulatory claims, and dropping one because a
// sibling code is unrecognised would erase it from the audit record.
func enumLabelList(enumName, raw string) string {
	parts := strings.Split(raw, "、")
	for i, p := range parts {
		parts[i] = enumLabel(enumName, p)
	}
	return strings.Join(parts, "、")
}

// enumLabel resolves a CodeMeta value to its Chinese label through the generated
// pkg/codemeta table. Writing a second Chinese table by hand fails as DRIFT rather
// than as an error: CodeMeta changes a label, the copy keeps compiling, no test turns
// red, and the audit record silently shows last year's wording.
func enumLabel(enumName, value string) string {
	switch enumName {
	case "ProductStatus":
		return codemeta.ProductStatusLabel(value)
	case "PostPublishStatus":
		return codemeta.PostPublishStatusLabel(value)
	case "OrderStatus":
		return codemeta.OrderStatusLabel(value)
	case "DelegationAuditAction":
		return codemeta.DelegationAuditActionLabel(value)
	case "DelegationReasonCode":
		return codemeta.DelegationReasonCodeLabel(value)
	case "CertificationType":
		return codemeta.CertificationTypeLabel(value)
	default:
		return value
	}
}

func groupThousands(n string) string {
	neg := strings.HasPrefix(n, "-")
	n = strings.TrimPrefix(n, "-")
	intPart, frac := n, ""
	if i := strings.IndexByte(n, '.'); i >= 0 {
		intPart, frac = n[:i], n[i:]
	}
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := b.String() + frac
	if neg {
		return "-" + out
	}
	return out
}

// snapshotMaxBytes is asserted in the application layer, not by the database. The
// column is jsonb and would accept far more; the limit exists so one oversized
// description cannot make an audit row unreadable.
const snapshotMaxBytes = 64 * 1024

// MarshalSnapshot serialises a projection for before_snapshot_json /
// after_snapshot_json, replacing the whole object with a placeholder when it exceeds
// 64 KB.
//
// Two things this must NOT do: fail (a rejected intent is fail-closed and would block
// a legitimate write — trading business availability for audit convenience), and
// truncate the JSON string (the tail would not be valid JSON and jsonb rejects it
// with 22P02). oversize reports whether the placeholder was used so the caller can
// emit the warn log.
func MarshalSnapshot(projection map[string]any) (payload string, oversize bool) {
	if len(projection) == 0 {
		return "", false
	}
	b, err := json.Marshal(projection)
	if err != nil {
		return placeholderSnapshot(projection, 0), true
	}
	if len(b) > snapshotMaxBytes {
		return placeholderSnapshot(projection, len(b)), true
	}
	return string(b), false
}

func placeholderSnapshot(projection map[string]any, originalBytes int) string {
	fields := make([]string, 0, len(projection))
	for f := range projection {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	b, err := json.Marshal(map[string]any{
		"_truncated":     true,
		"_reason":        "snapshot_too_large",
		"_originalBytes": originalBytes,
		"_fields":        fields,
	})
	if err != nil {
		return `{"_truncated":true,"_reason":"snapshot_too_large"}`
	}
	return string(b)
}
