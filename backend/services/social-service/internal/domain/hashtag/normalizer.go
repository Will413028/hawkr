// Package hashtag implements canonical/display normalization and validation
// rules for hashtag inputs. Mirrors product-service spec 33; see spec 34.
package hashtag

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize applies the canonical/display pipeline to a single raw input:
//  1. trim leading/trailing whitespace (incl. NBSP, full-width space)
//  2. strip leading '#' characters
//  3. Unicode NFC normalize
//  4. canonical: ASCII letters lower-cased (CJK/full-width unchanged)
//     display:   NFC result without lower-casing
//
// Returns ok=false when raw is empty or strips to empty.
func Normalize(raw string) (canonical, display string, ok bool) {
	trimmed := strings.TrimFunc(raw, unicode.IsSpace)
	if trimmed == "" {
		return "", "", false
	}
	stripped := strings.TrimLeft(trimmed, "#")
	if stripped == "" {
		return "", "", false
	}
	nfc := norm.NFC.String(stripped)
	canonical = lowerASCII(nfc)
	return canonical, nfc, true
}

func lowerASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
