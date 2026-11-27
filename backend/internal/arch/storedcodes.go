package arch

import (
	"fmt"
	"regexp"
	"strings"
)


// StoredCodeException 是一個有理由的例外，以（模組、constraint 名）為鍵：同一個 CHECK 的所有值都豁免。
type StoredCodeException struct {
	Module     string
	Constraint string
	Reason     string
}

var (
	checkOpen   = regexp.MustCompile(`(?i)(?:CONSTRAINT\s+("?[A-Za-z0-9_]+"?)\s+)?CHECK\s*\(`)
	codeInList  = regexp.MustCompile(`(?is)\bIN\s*\(([^()]*)\)`)
	codeInArray = regexp.MustCompile(`(?is)\bARRAY\s*\[([^\[\]]*)\]`)
	sqlLiteral  = regexp.MustCompile(`'((?:[^']|'')*)'`)
	hasLower    = regexp.MustCompile(`[a-z]`)
)

// CheckStoredCodeCase 回報 content 裡有小寫值的 CHECK；exceptions 內符合（module、constraint）的略過，並在 used 登記用到哪一筆。
func CheckStoredCodeCase(module, file, content string, exceptions []StoredCodeException, used map[StoredCodeException]bool) []string {
	sql := StripSQLComments(content)
	var out []string
	for _, loc := range checkOpen.FindAllStringSubmatchIndex(sql, -1) {
		start := loc[1]
		depth, end := 1, start
		for end < len(sql) && depth > 0 {
			switch sql[end] {
			case '\'':
				end = skipQuoted(sql, end)
				continue
			case '(':
				depth++
			case ')':
				depth--
			}
			end++
		}
		body := sql[start:end]
		name := "(未命名)"
		if loc[2] >= 0 {
			name = strings.Trim(sql[loc[2]:loc[3]], `"`)
		}
		var bad []string
		for _, re := range []*regexp.Regexp{codeInList, codeInArray} {
			for _, list := range re.FindAllStringSubmatch(body, -1) {
				for _, lit := range sqlLiteral.FindAllStringSubmatch(list[1], -1) {
					if hasLower.MatchString(lit[1]) {
						bad = append(bad, lit[1])
					}
				}
			}
		}
		if len(bad) == 0 {
			continue
		}
		excepted := false
		for _, e := range exceptions {
			if e.Module == module && e.Constraint == name {
				used[e] = true
				excepted = true
			}
		}
		if !excepted {
			out = append(out, fmt.Sprintf("%s 的 %s：CHECK %s 的值 %s 帶小寫；儲存的代碼是 UPPER_SNAKE（ADR 2026-09-25-codemeta-retirement D3），"+
				"要改成大寫並寫 migration，真有理由才加進 storedCodeExceptions", module, file, name, strings.Join(bad, "、")))
		}
	}
	return out
}
