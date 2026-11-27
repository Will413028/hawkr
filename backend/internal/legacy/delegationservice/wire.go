package delegationservice

import "strings"

func toWire[T ~string](v T) string { return strings.ToLower(string(v)) }

func fromWire[T ~string](s string) T {
	v := T(strings.ToUpper(s))
	if toWire(v) != s {
		return ""
	}
	return v
}
