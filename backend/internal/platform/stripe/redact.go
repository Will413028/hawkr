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
	maxLogBodyBytes = 16 << 10
)

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
	containerKeys = []string{
		"individual",
		"person",
		"billing_details",
		"shipping",
		"attestations",
	}
	identityKeys = []string{
		"dob",
		"id_number",
		"ssn_last_4",
		"tax_id",
		"ip",
		"user_agent",
	}
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
			out[scrub(k)] = redactValue(val)
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
