package middleware

import (
	"strings"
	"testing"
)

// #2506: general PII the old blacklist never covered must now default to
// redacted. These are the exact field names issue #2506's evidence section
// named — auth.go/buyer.go/checkout.go/shipping.go/seller_crm.go/
// aftersales.go/address.go — spanning personal names, contact details,
// free-text notes, and address components.
func TestRedactBody_GeneralPIIDefaultsRedacted(t *testing.T) {
	cases := []struct{ key, value string }{
		{"phone", "0912345678"},
		{"email", "buyer@example.com"},
		{"address", "台北市中正區重慶南路一段1號"},
		{"detailAddress", "測試路100號"},
		{"shippingAddress", "測試路100號"},
		{"storeAddress", "測試路200號"},
		{"receiverName", "王小明"},
		{"receiverPhone", "0912345678"},
		{"recipientName", "王小明"},
		{"recipientPhone", "0912345678"},
		{"senderName", "王小明"},
		{"senderPhone", "0912345678"},
		{"buyerName", "王小明"},
		{"familyName", "王"},
		{"givenName", "小明"},
		{"fullName", "王小明"},
		{"displayName", "小明"},
		{"nickname", "阿明"},
		{"gender", "M"},
		{"job", "工程師"},
		{"memo", "備註內容"},
		{"remark", "備註內容"},
		{"shippingNote", "備註內容"},
		{"description", "自由文字描述"},
		{"reason", "自由文字理由"},
		{"rejectReason", "自由文字理由"},
		{"sellerRejectReason", "自由文字理由"},
		{"negotiationReason", "自由文字理由"},
		{"deductionReason", "自由文字理由"},
		{"caseDetail", "自由文字內容"},
		{"inspectionNote", "自由文字內容"},
		{"userIdentifier", "some-identifier"},
		{"csvContent", "name,phone\n王小明,0912345678"},
		{"csvUrl", "https://example.com/export.csv"},
		{"name", "王小明"},
		{"message", "自由文字訊息內容"},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			body := `{"` + c.key + `":"` + c.value + `"}`
			out := redactBody([]byte(body))
			if strings.Contains(out, c.value) {
				t.Fatalf("PII %q leaked for key %q: %s", c.value, c.key, out)
			}
			if !strings.Contains(out, "[REDACTED]") {
				t.Fatalf("expected redaction marker for key %q, got: %s", c.key, out)
			}
		})
	}
}

// An entirely new/unclassified field name must default to redacted — this
// is the core structural fix: the old blacklist defaulted new fields to
// plaintext, the allowlist defaults them to redacted.
func TestRedactBody_UnknownKeyDefaultsRedacted(t *testing.T) {
	out := redactBody([]byte(`{"someBrandNewFieldNobodyHasClassifiedYet":"secret-shaped-value"}`))
	if strings.Contains(out, "secret-shaped-value") {
		t.Fatalf("unknown key must default to redacted, got: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected redaction marker, got: %s", out)
	}
}

// otpCode/authorizationCode are one-time credentials (Task 2 Step 1), not
// general PII — pin them separately from the PII table above.
func TestRedactBody_OneTimeCredentials(t *testing.T) {
	out := redactBody([]byte(`{"otpCode":"123456","authorizationCode":"c123.abc"}`))
	for _, secret := range []string{"123456", "c123.abc"} {
		if strings.Contains(out, secret) {
			t.Fatalf("one-time credential %q leaked: %s", secret, out)
		}
	}
}

// Everything the pre-#2506 test suite already asserted survives redaction
// must keep surviving — the allowlist is a superset of every key the old
// blacklist implicitly allowed through and that a real test already pinned
// as "benign". This is the regression guard for the flip itself.
func TestRedactBody_AllowlistCoversPreExistingBenignFields(t *testing.T) {
	cases := []struct{ key, value string }{
		{"completion_tokens", "1234"},
		{"prompt_tokens", "50"},
		{"max_tokens", "2048"},
		{"photo_tokens", "p1,p2"},
		{"evidence_photo_tokens", "ev1"},
		{"fcm_token", "device-fcm-xyz"},
		{"bank_code", "822"},
		{"bank_name", "中信"},
		{"account_id", "700123456"},
		{"account_type", "2"},
		{"productIds", "p1"},
		{"tags", "水果"},
		{"sessionId", "aacf5737-3501-4d2e-9c1f-0a0"},
		{"skuId", "361"},
		{"page", "1"},
		{"size", "20"},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			assertNotRedacted(t, `{"`+c.key+`":"`+c.value+`"}`, c.key, c.value)
		})
	}
}

// #2506 whole-branch review Fix C: nothing in the pre-existing suite directly
// proved sensitiveKeys wins over extraAllowed (Task 4's diagnostic escape
// hatch's per-request allowlist addition). shouldRedact's belt-and-suspenders
// security claim rests entirely on isSensitiveKey being checked before
// extraAllowed — this pins that exact ordering.
func TestShouldRedact_SensitiveKeysWinsOverExtraAllowed(t *testing.T) {
	extra := map[string]struct{}{"password": {}}
	if !shouldRedact(normalizeKey("password"), extra) {
		t.Fatal("sensitiveKeys must win even when the key is present in extraAllowed — belt-and-suspenders broken")
	}
}
