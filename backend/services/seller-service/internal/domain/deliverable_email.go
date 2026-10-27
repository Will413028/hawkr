package domain

import (
	"net/mail"
	"strings"
)

func IsDeliverableEmail(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 {
		return false
	}
	domain := s[at+1:]
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1
}
