package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// pascal 把點與底線分隔的 key 路徑接成 PascalCase：每段只把第一個字母大寫，其餘不動。
// seller.KYCFlowStatus.gate_message → SellerKYCFlowStatusGateMessage。
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

// lowerCamel 把 lower_snake 轉成 lowerCamel：gate_message → gateMessage。
func lowerCamel(snake string) string {
	return lowerFirst(pascal(snake))
}

// connectCodeName 把 Connect code 的 proto 名轉成 PascalCase：INVALID_ARGUMENT → InvalidArgument。
func connectCodeName(code string) string {
	var b strings.Builder
	for _, p := range strings.Split(code, "_") {
		b.WriteString(upperFirst(strings.ToLower(p)))
	}
	return b.String()
}

// checkNames 擋下三端的 API 名稱撞名。
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
