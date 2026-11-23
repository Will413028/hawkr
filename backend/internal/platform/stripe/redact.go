package stripe

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	redacted = "[REDACTED]"
	// maxLogBodyBytes 是記錄裡單一本文（遮罩後）的長度上限，超過截斷。
	maxLogBodyBytes = 16 << 10
)

// 欄位名稱的遮罩表，依類別分組，一個名稱一行（mutation 清單逐行拿掉驗證）。比對的是路徑上任一層的名稱（不分大小寫）：
// 表單的 a[b][c] 與 JSON 的巢狀物件都一樣，命中就整個子樹換成 [REDACTED]。
// 設計是 deny 表加上「個資容器整包遮」：容器底下新增的欄位不需要列名也會被遮。
var (
	credentialKeys = []string{
		"authorization",
		"secret",
		"secret_key",
		"api_key",
		"token",
		"password",
		"url",
	}
	cardKeys = []string{
		"card",
		"number",
		"cvc",
		"cvv",
		"exp_month",
		"exp_year",
		"last4",
		"bank_account",
		"iban",
		"account_number",
		"routing_number",
	}
	contactKeys = []string{
		"email",
		"phone",
	}
	addressKeys = []string{
		"address",
		"line1",
		"line2",
		"postal_code",
		"zip",
		"city",
		"state",
		"town",
	}
	nameKeys = []string{
		"name",
		"surname",
	}
	// 個資容器：底下任何欄位都不記。
	containerKeys = []string{
		"individual",
		"person",
		"billing_details",
		"shipping",
	}
	identityKeys = []string{
		"dob",
		"id_number",
		"ssn_last_4",
		"tax_id",
	}
	// 名稱以這些結尾的欄位一律遮（client_secret、contact_email、receipt_email、first_name、given_name 等），所以這些全名不必再列在上面。
	sensitiveSuffixes = []string{
		"_secret",
		"_email",
		"_phone",
		"_name",
		"_address",
		"_token",
	}
)

var sensitiveKeys = func() map[string]struct{} {
	m := map[string]struct{}{}
	for _, list := range [][]string{credentialKeys, cardKeys, contactKeys, addressKeys, nameKeys, containerKeys, identityKeys} {
		for _, k := range list {
			m[k] = struct{}{}
		}
	}
	return m
}()

func sensitiveKey(k string) bool {
	k = strings.ToLower(k)
	if _, ok := sensitiveKeys[k]; ok {
		return true
	}
	for _, s := range sensitiveSuffixes {
		if strings.HasSuffix(k, s) {
			return true
		}
	}
	return false
}

// 值的遮罩：不論欄位名稱，字串裡長得像密鑰、信箱、卡號、電話的片段都換掉（欄位名稱沒涵蓋到的漏網防線）。
var valuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b[sr]k_[A-Za-z0-9_]+`),
	regexp.MustCompile(`\bwhsec_[A-Za-z0-9]+`),
	regexp.MustCompile(`[A-Za-z0-9_]*_secret_[A-Za-z0-9_]+`),
	regexp.MustCompile(`(?i)bearer\s+\S+`),
	regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+`),
	regexp.MustCompile(`\b\d(?:[ -]?\d){12,18}\b`),
	regexp.MustCompile(`\+\d[\d ()\-]{7,}\d`),
}

func scrub(s string) string {
	for _, re := range valuePatterns {
		s = re.ReplaceAllString(s, redacted)
	}
	return s
}

func truncate(s string) string {
	if len(s) <= maxLogBodyBytes {
		return s
	}
	return strings.ToValidUTF8(s[:maxLogBodyBytes], "") + "…[truncated]"
}

// redactJSON 遞迴遮 JSON 本文；解析不了就整份不記（fail-safe），只留長度。
func redactJSON(body []byte) string {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return omitted(body)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(redactValue(v)); err != nil {
		return omitted(body)
	}
	return truncate(strings.TrimRight(buf.String(), "\n"))
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if sensitiveKey(k) {
				out[k] = redacted
				continue
			}
			out[k] = redactValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redactValue(val)
		}
		return out
	case string:
		return scrub(t)
	default:
		return v
	}
}

// redactForm 遮表單編碼本文（a[b][c]=v）：路徑上任一層命中就遮值；鍵依字母序輸出，記錄穩定可比對。
func redactForm(raw string) string {
	form, err := url.ParseQuery(raw)
	if err != nil {
		return omitted([]byte(raw))
	}
	keys := make([]string, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		sens := formKeySensitive(k)
		for _, v := range form[k] {
			if sens {
				v = redacted
			} else {
				v = scrub(v)
			}
			parts = append(parts, scrub(k)+"="+v)
		}
	}
	return truncate(strings.Join(parts, "&"))
}

func formKeySensitive(k string) bool {
	for _, seg := range strings.FieldsFunc(k, func(r rune) bool { return r == '[' || r == ']' }) {
		if sensitiveKey(seg) {
			return true
		}
	}
	return false
}

func omitted(body []byte) string {
	return "[body omitted, " + strconv.Itoa(utf8.RuneCount(body)) + " chars]"
}

// redactBody 依 Content-Type 選遮罩方式；不認得的類型整份不記。
func redactBody(contentType string, body []byte) string {
	if len(body) == 0 {
		return ""
	}
	switch {
	case strings.HasPrefix(contentType, "application/json"):
		return redactJSON(body)
	case strings.HasPrefix(contentType, "application/x-www-form-urlencoded"):
		return redactForm(string(body))
	default:
		return omitted(body)
	}
}
