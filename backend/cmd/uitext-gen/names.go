package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func pascal(path string) string {
	var b strings.Builder
	for _, seg := range strings.FieldsFunc(path, func(r rune) bool { return r == '.' || r == '_' }) {
		b.WriteString(upperFirst(seg))
	}
	return b.String()
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

func lowerCamel(snake string) string {
	return lowerFirst(pascal(snake))
}

func connectCodeName(code string) string {
	var b strings.Builder
	for _, p := range strings.Split(code, "_") {
		b.WriteString(upperFirst(strings.ToLower(p)))
	}
	return b.String()
}

func (m *Model) checkNames() error {
	var errs errList
	seen := map[string]string{}
	for _, f := range m.Funcs {
		n := pascal(f.ID)
		if prev, ok := seen[n]; ok {
			errs.addf("%s 與 %s 產生同一個 API 名稱 %s", f.ID, prev, n)
		}
		seen[n] = f.ID
	}
	return errs.err()
}

func localeConst(loc string) string {
	switch loc {
	case "zh-TW":
		return "ZhTW"
	case "en":
		return "EN"
	}
	return fmt.Sprintf("Locale(%q)", loc)
}
