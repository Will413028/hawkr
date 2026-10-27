package middleware

import (
	"strings"
	"testing"
)

// helper: redactBody must hide the value but keep the key visible.
func assertRedacted(t *testing.T, body, key string) {
	t.Helper()
	out := redactBody([]byte(body))
	if !strings.Contains(out, `"`+key+`"`) {
		t.Fatalf("key %q should remain in output, got: %s", key, out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("value for %q should be redacted, got: %s", key, out)
	}
}

func assertNotRedacted(t *testing.T, body, key, value string) {
	t.Helper()
	out := redactBody([]byte(body))
	if !strings.Contains(out, value) {
		t.Fatalf("benign value %q for key %q must survive, got: %s", value, key, out)
	}
	if strings.Contains(out, "[REDACTED]") {
		t.Fatalf("benign key %q must not be redacted, got: %s", key, out)
	}
}

func TestRedactBody_Password(t *testing.T) {
	assertRedacted(t, `{"phone":"0912345678","password":"hunter2"}`, "password")
}

func TestRedactBody_AuthTokensSnakeCase(t *testing.T) {
	body := `{"access_token":"eyJabc.def.ghi","refresh_token":"rrr","id_token":"iii"}`
	out := redactBody([]byte(body))
	for _, secret := range []string{"eyJabc.def.ghi", "rrr", "iii"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret %q leaked: %s", secret, out)
		}
	}
}

func TestRedactBody_AuthTokensCamelCase(t *testing.T) {
	body := `{"accessToken":"aaa","refreshToken":"bbb","idToken":"ccc","firebaseIdToken":"ddd"}`
	out := redactBody([]byte(body))
	for _, secret := range []string{"aaa", "bbb", "ccc", "ddd"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret %q leaked: %s", secret, out)
		}
	}
}

func TestRedactBody_CertificationPII(t *testing.T) {
	body := `{"id_card_no":"A123456789","bank_account":"00012345678","tax_id":"12345678"}`
	out := redactBody([]byte(body))
	for _, secret := range []string{"A123456789", "00012345678", "12345678"} {
		if strings.Contains(out, secret) {
			t.Fatalf("PII %q leaked: %s", secret, out)
		}
	}
}

func TestRedactBody_KYCNameAndBirthdate(t *testing.T) {
	// real_name / birth_date 各 case 形態皆須遮蔽（KYC 個資）。
	cases := []struct{ body, secret string }{
		{`{"real_name":"王小明"}`, "王小明"},
		{`{"realName":"王小明"}`, "王小明"},
		{`{"RealName":"王小明"}`, "王小明"},
		{`{"birth_date":"1990-01-01"}`, "1990-01-01"},
		{`{"birthDate":"1990-01-01"}`, "1990-01-01"},
		{`{"BirthDate":"1990-01-01"}`, "1990-01-01"},
	}
	for _, c := range cases {
		out := redactBody([]byte(c.body))
		if strings.Contains(out, c.secret) {
			t.Fatalf("KYC PII %q leaked from %q: %s", c.secret, c.body, out)
		}
		if !strings.Contains(out, "[REDACTED]") {
			t.Fatalf("value should be redacted for %q, got: %s", c.body, out)
		}
	}
}

func TestRedactBody_IDDocumentURLs(t *testing.T) {
	body := `{"id_card_front_url":"https://oss/x.jpg","id_card_back_url":"https://oss/y.jpg","bank_book_url":"https://oss/z.jpg"}`
	out := redactBody([]byte(body))
	for _, secret := range []string{"x.jpg", "y.jpg", "z.jpg"} {
		if strings.Contains(out, secret) {
			t.Fatalf("document URL %q leaked: %s", secret, out)
		}
	}
}

func TestRedactBody_PaymentTokens(t *testing.T) {
	body := `{"applePayToken":"apt","google_pay_token":"gpt","tempBindToken":"tbt"}`
	out := redactBody([]byte(body))
	for _, secret := range []string{"apt", "gpt", "tbt"} {
		if strings.Contains(out, secret) {
			t.Fatalf("payment token %q leaked: %s", secret, out)
		}
	}
}

// Critical: exact normalized-key matching must NOT over-redact look-alike keys.
func TestRedactBody_BenignTokenFamily(t *testing.T) {
	cases := []struct{ body, key, value string }{
		{`{"completion_tokens":1234}`, "completion_tokens", "1234"},
		{`{"prompt_tokens":50}`, "prompt_tokens", "50"},
		{`{"max_tokens":2048}`, "max_tokens", "2048"},
		{`{"photo_tokens":"p1,p2"}`, "photo_tokens", "p1,p2"},
		{`{"evidence_photo_tokens":"ev1"}`, "evidence_photo_tokens", "ev1"},
		{`{"fcm_token":"device-fcm-xyz"}`, "fcm_token", "device-fcm-xyz"},
		{`{"bank_code":"822"}`, "bank_code", "822"},
		{`{"bank_name":"中信"}`, "bank_name", "中信"},
		{`{"account_id":"700123456"}`, "account_id", "700123456"},
		{`{"account_type":2}`, "account_type", "2"},
	}
	for _, c := range cases {
		assertNotRedacted(t, c.body, c.key, c.value)
	}
}

func TestRedactBody_Nested(t *testing.T) {
	body := `{"data":{"user":{"accountId":"700123456","password":"secret"},"tokens":[{"access_token":"zzz"}]}}`
	out := redactBody([]byte(body))
	if strings.Contains(out, "secret") || strings.Contains(out, "zzz") {
		t.Fatalf("nested secret leaked: %s", out)
	}
	if !strings.Contains(out, "700123456") {
		t.Fatalf("benign nested value lost: %s", out)
	}
}

func TestRedactBody_NumericSecret(t *testing.T) {
	// password as a non-string value must still be redacted.
	out := redactBody([]byte(`{"password":1234}`))
	if strings.Contains(out, "1234") {
		t.Fatalf("numeric secret leaked: %s", out)
	}
}

func TestRedactBody_TruncatedJSON(t *testing.T) {
	// Body cut off mid-stream (truncation upstream / oversized) must still redact.
	body := `{"phone":"0912","password":"leakme","extra":"aaaaaaa`
	out := redactBody([]byte(body))
	if strings.Contains(out, "leakme") {
		t.Fatalf("secret leaked from malformed JSON: %s", out)
	}
}

func TestRedactBody_Empty(t *testing.T) {
	if got := redactBody([]byte("")); got != "" {
		t.Fatalf("empty body should stay empty, got %q", got)
	}
}

func TestRedactBody_Binary(t *testing.T) {
	if got := redactBody([]byte{0xff, 0xfe, 0x00}); got != "[binary]" {
		t.Fatalf("binary body should be %q, got %q", "[binary]", got)
	}
}

func TestRedactBody_TruncatesAfterRedaction(t *testing.T) {
	var sb strings.Builder
	// "accountId" (not "message") — the whole-branch review for #2506 found
	// "message" itself leaks free-text AI chat content (ai.go's request
	// body) and removed it from allowedKeys, so it would now collapse to
	// "[REDACTED]" before truncation ever triggers. "accountId" is a
	// business identifier on allowedKeys with no PII shape, so it still
	// survives long enough for truncateBody to have something to truncate.
	sb.WriteString(`{"accountId":"`)
	sb.WriteString(strings.Repeat("x", maxBodyLogSize+500))
	sb.WriteString(`"}`)
	out := redactBody([]byte(sb.String()))
	if !strings.Contains(out, "(truncated)") {
		t.Fatalf("oversized body should be truncated, got len=%d", len(out))
	}
}

// Dash-case keys must redact: the key char class allows '-' and normalizeKey
// strips it, so "id-card-no" collapses onto the same key as "id_card_no".
func TestRedactBody_DashCaseKeys(t *testing.T) {
	body := `{"id-card-no":"A123456789","access-token":"aaa","bank-account":"00012345678"}`
	out := redactBody([]byte(body))
	for _, secret := range []string{"A123456789", "aaa", "00012345678"} {
		if strings.Contains(out, secret) {
			t.Fatalf("dash-case secret %q leaked: %s", secret, out)
		}
	}
}

// In-place value replacement must preserve the original key text and the
// whitespace around the colon (single-pass index replacement, not rebuild).
func TestRedactBody_PreservesFormatting(t *testing.T) {
	// "accountId" (not "phone") — #2506 (Task 2) made phone default-redact
	// (it's general PII), so it can no longer stand in as the "survives"
	// half of this formatting-preservation check; accountId is on
	// allowedKeys and non-PII, same substitution Step 5 used for the
	// integration test.
	out := redactBody([]byte(`{"accountId": "700123456", "password" : "hunter2"}`))
	if !strings.Contains(out, `"accountId": "700123456"`) {
		t.Fatalf("benign field formatting changed: %s", out)
	}
	if !strings.Contains(out, `"password" : "[REDACTED]"`) {
		t.Fatalf("redaction should keep key and spacing intact: %s", out)
	}
	if strings.Contains(out, "hunter2") {
		t.Fatalf("secret leaked: %s", out)
	}
}

// A long token whose value straddles the maxBodyLogSize boundary must still be
// fully redacted: scanLimit's headroom over maxBodyLogSize must exceed any single
// secret length, otherwise the closing quote falls outside the scan window and
// the quoted-string branch fails to match.
func TestRedactBody_SecretStraddlingTruncationBoundary(t *testing.T) {
	secret := strings.Repeat("S", 1500) // JWT-sized, crosses the 2000B boundary
	var sb strings.Builder
	sb.WriteString(`{"pad":"`)
	sb.WriteString(strings.Repeat("p", maxBodyLogSize-40)) // push id_token's value across the boundary
	sb.WriteString(`","id_token":"`)
	sb.WriteString(secret)
	sb.WriteString(`"}`)
	out := redactBody([]byte(sb.String()))
	if strings.Contains(out, secret) {
		t.Fatalf("boundary-straddling secret leaked: out=%q", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected redaction marker, got: %q", out)
	}
}

// Bodies larger than scanLimit must not panic; content past the limit is dropped
// by truncateBody anyway, and any sensitive key within the window is redacted.
func TestRedactBody_BeyondScanLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"password":"leakme","filler":"`)
	sb.WriteString(strings.Repeat("z", scanLimit))
	sb.WriteString(`"}`)
	out := redactBody([]byte(sb.String()))
	if strings.Contains(out, "leakme") {
		t.Fatalf("secret within window leaked on oversized body: %s", out[:80])
	}
}

// staged token 是 media-service 簽出的 JWT，而 admin_delegated_media.go 與
// validateStagedToken 都把「never logged」寫成不變式——在補上遮罩之前那個不變式沒有
// 任何機械保證：access log 同時記 req_body 與 resp_body，掛載端點的請求與 complete
// 端點的回應正好各洩漏一半（56 §6.1.13）。本測試逐一釘住這個家族的每種欄名形態。
func TestRedactBody_StagedTokenFamily(t *testing.T) {
	cases := []struct{ name, body, secret string }{
		{"camelCase 單值（logo / cover 的 request）",
			`{"expectedVersion":"7","stagedToken":"eyJhbGciOi.payload.sig"}`, "eyJhbGciOi.payload.sig"},
		{"snake_case 單值",
			`{"staged_token":"eyJhbGciOi.payload.sig"}`, "eyJhbGciOi.payload.sig"},
		{"complete 端點的回應（resp_body 走同一條路）",
			`{"code":0,"message":"success","data":{"stagedToken":"eyJhbGciOi.payload.sig"}}`, "eyJhbGciOi.payload.sig"},
		{"字串陣列（貼文 / 商品圖一次送多張）",
			`{"stagedTokens":["tok-a","tok-b"]}`, "tok-a"},
		{"KYC 單值鍵",
			`{"id_card_front_staged_token":"kyc-front-jwt"}`, "kyc-front-jwt"},
		{"KYC 陣列鍵",
			`{"document_staged_tokens":["doc-1","doc-2"]}`, "doc-2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := redactBody([]byte(c.body))
			if strings.Contains(out, c.secret) {
				t.Fatalf("staged token leaked: %s", out)
			}
			if !strings.Contains(out, "[REDACTED]") {
				t.Fatalf("expected redaction marker, got: %s", out)
			}
		})
	}
}

// 預簽上傳 URL：授權寫在 URL 裡，單獨持有即可覆寫該暫存物件（§10.3）。
func TestRedactBody_PresignedUploadURL(t *testing.T) {
	body := `{"code":0,"data":{"sessionId":"aacf5737-3501-4d2e-9c1f-0a0","uploadUrl":"https://bucket.r2.cloudflarestorage.com/tmp/20260814/abc.png?X-Amz-Signature=deadbeef","expiresIn":"1800"}}`
	out := redactBody([]byte(body))
	if strings.Contains(out, "X-Amz-Signature") || strings.Contains(out, "cloudflarestorage.com") {
		t.Fatalf("presigned upload URL leaked: %s", out)
	}
	// session id 必須留著：它才是追這次上傳要用的錨。
	if !strings.Contains(out, "aacf5737-3501-4d2e-9c1f-0a0") {
		t.Fatalf("upload session id must survive for debugging: %s", out)
	}
}

// 純量陣列現在會被當成單一 value 比對，所以要同時釘住反向：非敏感的字串陣列必須
// 原樣留在 log 裡（代行工作區的 productIds 這類欄位正是除錯時要看的）。
func TestRedactBody_BenignScalarArraySurvives(t *testing.T) {
	out := redactBody([]byte(`{"productIds":["2685914682","2632049831"],"tags":["水果","梨山"]}`))
	for _, keep := range []string{"2685914682", "2632049831", "水果", "梨山"} {
		if !strings.Contains(out, keep) {
			t.Fatalf("benign array value %q lost: %s", keep, out)
		}
	}
	if strings.Contains(out, "[REDACTED]") {
		t.Fatalf("benign arrays must not be redacted: %s", out)
	}
}
