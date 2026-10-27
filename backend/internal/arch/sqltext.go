package arch

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// StripSQLComments 把 SQL 的 `--` 註解與 `/* */` 註解（可巢狀，Postgres 的寫法）換成一個空白；字串（'…'）、引號識別字（"…"）與
// dollar quote（$tag$…$tag$）裡的不算註解，原樣保留。
func StripSQLComments(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "--"):
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				j = len(s) - i
			}
			b.WriteByte(' ')
			i += j
		case strings.HasPrefix(s[i:], "/*"):
			depth, j := 0, i
			for j < len(s) {
				if strings.HasPrefix(s[j:], "/*") {
					depth++
					j += 2
				} else if strings.HasPrefix(s[j:], "*/") {
					depth--
					j += 2
					if depth == 0 {
						break
					}
				} else {
					j++
				}
			}
			b.WriteByte(' ')
			i = j
		case s[i] == '\'' || s[i] == '"':
			j := skipQuoted(s, i)
			b.WriteString(s[i:j])
			i = j
		case s[i] == '$':
			if m := dollarTag.FindString(s[i:]); m != "" {
				end := strings.Index(s[i+len(m):], m)
				j := len(s)
				if end >= 0 {
					j = i + len(m) + end + len(m)
				}
				b.WriteString(s[i:j])
				i = j
				continue
			}
			b.WriteByte(s[i])
			i++
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

var (
	dollarTag   = regexp.MustCompile(`^\$[A-Za-z_]*\$`)
	quotedIdent = regexp.MustCompile(`"((?:[^"]|"")*)"`)
	dottedSpace = regexp.MustCompile(`\s*\.\s*`)
	trimCall    = regexp.MustCompile(`(?i)\b([blr]?trim)\s*\(`)
	uEscape     = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	trimFrom    = regexp.MustCompile(`(?is)^\s*(?:(?:LEADING|TRAILING|BOTH)\s+)?(.*?)\s*\bFROM\b`)
)

// skipQuoted 回傳從 s[i]（' 或 "）開始的字串或引號識別字結束後的位置；重複的引號是跳脫。
func skipQuoted(s string, i int) int {
	q := s[i]
	j := i + 1
	for j < len(s) {
		if s[j] == q {
			if j+1 < len(s) && s[j+1] == q {
				j += 2
				continue
			}
			return j + 1
		}
		j++
	}
	return len(s)
}

// normalizeSQLNames 去掉註解，把引號識別字換成裡面的名字、`a . b` 換成 `a.b`，讓名稱比對看得到 `"seller"."pub_x"` 與
// `seller /* … */ . pub_x` 這類寫法。
func normalizeSQLNames(s string) string {
	s = StripSQLComments(s)
	s = quotedIdent.ReplaceAllStringFunc(s, func(m string) string { return strings.ReplaceAll(m[1:len(m)-1], `""`, `"`) })
	return dottedSpace.ReplaceAllString(s, ".")
}

// CheckTrimmedText 回報模組的 SQL 檔（schema.sql、objects.sql、pub.sql、queries/*.sql；content 是檔案內容）違反「頭尾空白只有
// platform/trimtext 一個定義」的地方。cmd/hawkr 的 TestTrimmedTextChecksUseTheCanonicalPattern 也拿它檢查 migration 套用後的
// 每個 CHECK 與索引定義（pg_get_constraintdef 與 pg_indexes 的寫法）。
//   - 只去空白的 trim 函式（一參數的 btrim/ltrim/rtrim，與沒有指定字元的 TRIM(...)）：它們只去 U+0020，與寫入端的 trimtext.Trim
//     不同；欄位已由 trimtext.Check 保證頭尾沒有空白，參數由呼叫端以 trimtext.Trim 去掉；
//   - 指定字元的 trim，字元裡有空白（`btrim(x, ' ')`、`TRIM(LEADING E'\t' FROM x)`）或字元不是字面常數（看不出不含空白）：
//     指定字元只能用來去非空白的字元（`btrim(x, '/')`）；
//   - 比對樣式（regexp_* 函式的樣式參數、`~`／`~*`／`!~`／`!~*`、LIKE／ILIKE／SIMILAR TO 的右邊）裡有空白字元：
//     `regexp_replace(x, '^ +| +$', ”)`、`x NOT LIKE ' %'` 這類自己定義的頭尾空白；
//   - 任何字面常數裡的正則空白類別（`\s`、`\S`、`[:space:]`、`[:blank:]`）：Postgres 的類別依 locale，與 trimtext 的集合不同。
//     只有 interiorSpaceFormats 列的格式檢查（整個值不得有空白，不是頭尾空白的定義）例外；
//   - trimtext.Pattern 以外另外列出的空白碼位（\uXXXX 的寫法）：頭尾空白一律寫成 trimtext.Check 的展開。
func CheckTrimmedText(module, file, content string) []string {
	body := StripSQLComments(content)
	calls := blankStrings(body)
	var out []string
	flag := func(format string, a ...any) {
		out = append(out, fmt.Sprintf("%s 的 %s ", module, file)+fmt.Sprintf(format, a...))
	}
	for _, loc := range trimCall.FindAllStringSubmatchIndex(calls, -1) {
		name := strings.ToLower(calls[loc[2]:loc[3]])
		args, ok := parenBody(calls, loc[1]-1)
		if !ok {
			flag("的 %s( 沒有對應的右括號", name)
			continue
		}
		orig := body[loc[1] : loc[1]+len(args)]
		shown := strings.Join(strings.Fields(orig), " ")
		chars, ok := trimChars(name, args, orig)
		if !ok {
			flag("用 %s(%s) 只去 U+0020：欄位用 trimtext.Check 保證頭尾沒有空白，參數由呼叫端以 trimtext.Trim 去掉", name, shown)
			continue
		}
		value, literal := literalValue(chars)
		switch {
		case !literal:
			flag("用 %s(%s) 的指定字元不是字面常數，看不出不含空白：頭尾空白只能用 trimtext", name, shown)
		case strings.ContainsFunc(value, unicode.IsSpace):
			flag("用 %s(%s) 去空白：頭尾空白只能用 trimtext（欄位 trimtext.Check、參數 trimtext.Trim）", name, shown)
		}
	}
	for _, lit := range patternLiterals(body, calls) {
		value, _ := literalValue(lit)
		if value == trimtext.Pattern {
			continue
		}
		if strings.ContainsFunc(value, unicode.IsSpace) {
			flag("的比對樣式 %s 有空白字元：頭尾空白只能用 trimtext.Check", lit)
		}
	}
	for _, lit := range stringLiterals(body) {
		value, _ := literalValue(lit)
		if value == trimtext.Pattern {
			continue
		}
		if c := regexSpaceClass.FindString(value); c != "" {
			if _, ok := interiorSpaceFormats[value]; !ok {
				flag("的 %s 用正則空白類別 %s（依 locale，與 trimtext 的集合不同）：頭尾空白只能用 trimtext.Check", lit, c)
			}
		}
		for _, m := range uEscape.FindAllStringSubmatch(lit, -1) {
			var r rune
			if _, err := fmt.Sscanf(m[1], "%x", &r); err == nil && unicode.IsSpace(r) {
				flag("另外列了空白碼位 %s：頭尾空白一律寫成 trimtext.Check 的展開", m[0])
			}
		}
	}
	return out
}

// interiorSpaceFormats 是允許用正則空白類別的字面常數（值是理由）：要求整個值沒有空白的格式檢查，不是頭尾空白的定義，
// 去掉頭尾空白後再比對的寫入端不受它影響。新增一項要寫明是哪個欄位、為什麼不是頭尾空白。
var interiorSpaceFormats = map[string]string{
	`^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$`: "seller.stores.support_email 的格式：整個 email 沒有空白",
	`^[^@[:space:]]+@[^@[:space:]]+$`:                 "seller.reports.reporter_email 的格式：整個 email 沒有空白",
}

var (
	regexSpaceClass = regexp.MustCompile(`\\[sS]|\[:(?:space|blank):\]`)
	// regexpCall 是樣式在第二個參數的 regexp_* 函式。
	regexpCall = regexp.MustCompile(`(?i)\bregexp_(?:replace|match|matches|like|count|instr|substr|split_to_array|split_to_table)\s*\(`)
	// patternOp 是右邊是比對樣式的運算子：正則（~、~*、!~、!~*）、LIKE 系列（LIKE、ILIKE、~~、~~*、!~~、!~~*）與 SIMILAR TO。
	patternOp  = regexp.MustCompile(`(?i)(?:!?~~?\*?|\b(?:I?LIKE|SIMILAR\s+TO)\b)\s*\(*\s*$`)
	literalAt  = regexp.MustCompile(`(?i)^(?:E|U&)?'`)
	castSuffix = regexp.MustCompile(`(?i)^(.*?)\s*(?:::\s*(?:text|character varying|varchar|bpchar))?$`)
)

// trimChars 回傳 trim 系列呼叫（name，參數 args 是字串已換成空白的版本、orig 是原文）指定的字元；沒有指定字元時 ok 是 false。
// btrim/ltrim/rtrim 的字元是第二個參數；TRIM 是 `[LEADING|TRAILING|BOTH] 字元 FROM x`，或 FROM 之後（或整個參數）的第二個參數。
func trimChars(name, args, orig string) (string, bool) {
	if name == "trim" {
		if m := trimFrom.FindStringSubmatchIndex(args); m != nil {
			if strings.TrimSpace(args[m[2]:m[3]]) != "" {
				return strings.TrimSpace(orig[m[2]:m[3]]), true
			}
			args, orig = args[m[1]:], orig[m[1]:]
		}
	}
	i := topLevelComma(args)
	if i < 0 {
		return "", false
	}
	rest, restOrig := args[i+1:], orig[i+1:]
	if j := topLevelComma(rest); j >= 0 {
		restOrig = restOrig[:j]
	}
	return strings.TrimSpace(restOrig), true
}

// literalValue 回傳 expr（可帶 ::text 這類轉型）是字串字面常數時的值：'…' 的重複引號、E'…' 的反斜線跳脫（\t、\n、\r、\f、\b、
// \xHH、\ooo、\uXXXX、\UXXXXXXXX）都還原。U&'…' 與其他寫法回傳原文與 false。
func literalValue(expr string) (string, bool) {
	expr = castSuffix.FindStringSubmatch(strings.TrimSpace(expr))[1]
	switch {
	case len(expr) >= 2 && expr[0] == '\'' && expr[len(expr)-1] == '\'':
		return strings.ReplaceAll(expr[1:len(expr)-1], "''", "'"), true
	case len(expr) >= 3 && (expr[0] == 'E' || expr[0] == 'e') && expr[1] == '\'' && expr[len(expr)-1] == '\'':
		return unescapeE(expr[2 : len(expr)-1]), true
	}
	return expr, false
}

func unescapeE(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && i+1 < len(s) && s[i+1] == '\'' {
			b.WriteByte('\'')
			i++
			continue
		}
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		hex := func(n int) {
			end := min(i+1+n, len(s))
			j := i + 1
			for j < end && strings.IndexByte("0123456789abcdefABCDEF", s[j]) >= 0 {
				j++
			}
			var r rune
			if _, err := fmt.Sscanf(s[i+1:j], "%x", &r); err == nil {
				b.WriteRune(r)
			}
			i = j - 1
		}
		switch s[i] {
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'x':
			hex(2)
		case 'u':
			hex(4)
		case 'U':
			hex(8)
		case '0', '1', '2', '3', '4', '5', '6', '7':
			j := i
			for j < len(s) && j < i+3 && s[j] >= '0' && s[j] <= '7' {
				j++
			}
			var r rune
			_, _ = fmt.Sscanf(s[i:j], "%o", &r)
			b.WriteRune(r)
			i = j - 1
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// stringLiterals 回傳 body（已去掉註解）裡每個字串字面常數的原文（含引號與 E、U& 前綴）。E'…' 裡的 \' 不結束字串。
func stringLiterals(body string) []string {
	var out []string
	for i := 0; i < len(body); {
		switch body[i] {
		case '"':
			i = skipQuoted(body, i)
		case '\'':
			start := i
			if i > 0 && (body[i-1] == 'E' || body[i-1] == 'e') && (i < 2 || !isIdentByte(body[i-2])) {
				start = i - 1
				i = skipEString(body, i)
			} else {
				if i > 1 && body[i-1] == '&' && (body[i-2] == 'U' || body[i-2] == 'u') {
					start = i - 2
				}
				i = skipQuoted(body, i)
			}
			out = append(out, body[start:i])
		default:
			i++
		}
	}
	return out
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// skipEString 回傳從 s[i]（E'…' 的引號）開始的字串結束後的位置；反斜線跳脫下一個字元，重複的引號也是跳脫。
func skipEString(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++
		case s[j] == '\'' && j+1 < len(s) && s[j+1] == '\'':
			j++
		case s[j] == '\'':
			return j + 1
		}
	}
	return len(s)
}

// patternLiterals 回傳 body 裡當成比對樣式的字面常數原文：regexp_* 函式的第二個參數，與比對運算子（patternOp）右邊緊接的字面常數。
// calls 是 body 的字串換成空白的版本（blankStrings），結構的比對在它上面做，原文以同樣的位置取自 body。
func patternLiterals(body, calls string) []string {
	var out []string
	for _, loc := range regexpCall.FindAllStringIndex(calls, -1) {
		args, ok := parenBody(calls, loc[1]-1)
		if !ok {
			continue
		}
		i := topLevelComma(args)
		if i < 0 {
			continue
		}
		rest := args[i+1:]
		if j := topLevelComma(rest); j >= 0 {
			rest = rest[:j]
		}
		start := loc[1] + i + 1
		if lit := leadingLiteral(body[start : start+len(rest)]); lit != "" {
			out = append(out, lit)
		}
	}
	for i := 0; i < len(calls); i++ {
		if calls[i] != '\'' || i > 0 && calls[i-1] == '\'' {
			continue
		}
		start := i
		if i > 0 && (calls[i-1] == 'E' || calls[i-1] == 'e') {
			start = i - 1
		}
		if patternOp.MatchString(calls[max(0, start-40):start]) {
			if lit := leadingLiteral(body[start:]); lit != "" {
				out = append(out, lit)
			}
		}
		if strings.HasPrefix(body[start:], "E'") || strings.HasPrefix(body[start:], "e'") {
			i = skipEString(body, i) - 1
		} else {
			i = skipQuoted(body, i) - 1
		}
	}
	return out
}

// leadingLiteral 回傳 s 開頭（略過空白）的字串字面常數原文；開頭不是字面常數時回傳空字串。
func leadingLiteral(s string) string {
	t := strings.TrimLeft(s, " \t\n\r")
	if !literalAt.MatchString(t) {
		return ""
	}
	q := strings.IndexByte(t, '\'')
	var end int
	if q == 1 && (t[0] == 'E' || t[0] == 'e') {
		end = skipEString(t, q)
	} else {
		end = skipQuoted(t, q)
	}
	return t[:end]
}

// blankStrings 把字串（'…'）的內容換成同長度的空白（引號保留），呼叫的比對才不會把字串裡的文字當成程式。
func blankStrings(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); {
		if b[i] != '\'' {
			i++
			continue
		}
		j := skipQuoted(s, i)
		for k := i + 1; k < j-1; k++ {
			b[k] = ' '
		}
		i = j
	}
	return string(b)
}

// parenBody 回傳 s[open] 這個左括號到對應右括號之間的內容（略過字串與引號識別字）。
func parenBody(s string, open int) (string, bool) {
	depth := 0
	for i := open; i < len(s); {
		switch s[i] {
		case '\'', '"':
			i = skipQuoted(s, i)
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[open+1 : i], true
			}
		}
		i++
	}
	return "", false
}

// topLevelComma 回傳 args 第一個不在括號、字串或引號識別字裡的逗號的位置，沒有時回傳 -1。
func topLevelComma(args string) int {
	depth := 0
	for i := 0; i < len(args); {
		switch args[i] {
		case '\'', '"':
			i = skipQuoted(args, i)
			continue
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}
