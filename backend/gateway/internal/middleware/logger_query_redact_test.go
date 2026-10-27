package middleware

import "testing"

// The guest cart identifier is a bearer credential with no signature and no
// ownership check behind it (#2295 ③), and it travels by query string on the
// GET and DELETE cart routes. Before this, a value posted in a JSON body was
// redacted while the same value in ?guest_id= went into the access log verbatim.
func TestRedactQuery(t *testing.T) {
	const R = `"[REDACTED]"`
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"guest_id snake", "guest_id=abc-123", "guest_id=" + R},
		{"guestId camel", "guestId=abc-123", "guestId=" + R},
		{"guest-id dash", "guest-id=abc-123", "guest-id=" + R},
		{"GUEST_ID upper", "GUEST_ID=abc-123", "GUEST_ID=" + R},
		{"existing sensitive key also covered", "token=eyJhbG", "token=" + R},
		// 後綴規則（staged token 家族）也必須兩條路都套到，否則今天沒有 query 路徑的
		// 端點，明天多一支就靜默漏遮。
		{"stagedToken camel", "stagedToken=eyJhbG", "stagedToken=" + R},
		{"staged_token snake", "staged_token=eyJhbG", "staged_token=" + R},
		{"KYC 前綴變體", "document_staged_tokens=d1", "document_staged_tokens=" + R},

		// Must not over-redact: an access log with every value masked is an
		// access log nobody can debug with.
		{"ordinary params untouched", "page=1&size=20", "page=1&size=20"},
		{"only the sensitive one is masked", "page=1&guest_id=x&size=20", "page=1&guest_id=" + R + "&size=20"},
		// #2506 (Task 2): redactQuery now calls shouldRedact, the same
		// default-deny decision point as redactBody. "guest_identity" is
		// redacted here because it's unclassified and not on allowedKeys —
		// NOT because isSensitiveKey substring/prefix-matched it against
		// "guestid". This case alone can no longer tell "correct
		// default-deny" apart from "isSensitiveKey regressed to substring
		// matching", since both produce the same [REDACTED] output now; see
		// TestIsSensitiveKey_LookalikeNotSubstringMatched below for the
		// direct exact-match proof this case used to provide on its own,
		// and TestRedactBody_UnknownKeyDefaultsRedacted for the body-side
		// equivalent of the default-deny behavior itself.
		{"substring is not a match", "guest_identity=keep", "guest_identity=" + R},

		// Shape preservation: order, separators and empty values survive so the
		// log line still reads as the request that was actually made.
		// Uses "page" (an allowlisted key) rather than an arbitrary "a" so
		// this stays a genuine allowlisted-survives / sensitive-masked pair
		// post-#2506 instead of both sides defaulting to redacted.
		{"semicolon separator", "page=1;guest_id=x", "page=1;guest_id=" + R},
		{"bare flag has no value to hide", "debug&guest_id=x", "debug&guest_id=" + R},
		{"empty value", "guest_id=", "guest_id=" + R},
		{"trailing separator", "guest_id=x&", "guest_id=" + R + "&"},
		{"percent-encoded key still matched", "guest%5Fid=x", "guest%5Fid=" + R},
		{"empty query", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := redactQuery(c.in); got != c.want {
				t.Errorf("redactQuery(%q)\n got: %s\nwant: %s", c.in, got, c.want)
			}
		})
	}
}

// isSensitiveKey must do exact-match lookup (plus the fixed
// sensitiveKeySuffixes list), never substring/prefix matching against the
// rest of sensitiveKeys. "guest_identity" (normalized "guestidentity")
// begins with "guestid" — the exact key for the guest-cart bearer
// credential (#2295 ③) — but names a completely unrelated field. Before
// #2506 (Task 2) introduced shouldRedact's default-deny, TestRedactQuery's
// "substring is not a match" case proved this on its own (a substring-match
// regression would have redacted it, same as today, but a correct
// implementation left it in plaintext). Under the new allowlist, that case
// redacts "guest_identity" either way — via correct default-deny, or via a
// regressed isSensitiveKey — so the two are no longer distinguishable
// through redactQuery. This test pins the exact-match invariant directly at
// the layer that owns it, independent of allowedKeys/shouldRedact.
func TestIsSensitiveKey_LookalikeNotSubstringMatched(t *testing.T) {
	if isSensitiveKey(normalizeKey("guest_identity")) {
		t.Fatalf(`isSensitiveKey must do exact-match, not substring/prefix match — "guest_identity" is not "guestid"`)
	}
}

// Body-side coverage of the same key, so the two transports cannot drift apart
// again without a test going red.
func TestRedactBodyCoversGuestId(t *testing.T) {
	got := redactBody([]byte(`{"guestId":"abc-123","skuId":"361"}`))
	want := `{"guestId":"[REDACTED]","skuId":"361"}`
	if got != want {
		t.Errorf("redactBody\n got: %s\nwant: %s", got, want)
	}
}
