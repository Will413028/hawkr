package hashtag

import "unicode"

// MaxLength is the per-hashtag code-point upper bound (inclusive).
const MaxLength = 40

// MaxCount is the per-post hashtag upper bound (inclusive).
const MaxCount = 10

// Normalized is the canonical+display pair returned by RequireSet.
type Normalized struct {
	Canonical string
	Display   string
}

// RequireSet runs normalize+validate over raw inputs and deduplicates by
// canonical name, preserving first-seen display. Returns nil slice + nil
// error for nil / empty input.
func RequireSet(raw []string) ([]Normalized, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]Normalized, 0, len(raw))
	for _, r := range raw {
		canonical, display, ok := Normalize(r)
		if !ok {
			return nil, ErrEmptyAfterNormalize
		}
		if err := validateChars(canonical); err != nil {
			return nil, err
		}
		if err := validateLength(canonical); err != nil {
			return nil, err
		}
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, Normalized{Canonical: canonical, Display: display})
	}
	if len(out) > MaxCount {
		return nil, ErrCountExceeded
	}
	return out, nil
}

func validateChars(canonical string) error {
	for _, r := range canonical {
		if !isAllowed(r) {
			return ErrInvalidChar
		}
	}
	return nil
}

func validateLength(canonical string) error {
	n := 0
	for range canonical {
		n++
		if n > MaxLength {
			return ErrLengthExceeded
		}
	}
	if n < 1 {
		return ErrLengthExceeded
	}
	return nil
}

// isAllowed: ASCII a-z / 0-9 (canonical already lowered) + CJK ideographs.
func isAllowed(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= '0' && r <= '9':
		return true
	}
	return unicode.Is(unicode.Han, r)
}
