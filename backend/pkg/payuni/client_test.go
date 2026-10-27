package payuni

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	testHashKey = "0123456789abcdef0123456789abcdef" // gitleaks:allow（虛構值 0123…）
	testHashIV  = "0123456789abcdef"
)

// newSearchStatusTestServer spins up a fake PAYUNi search_merchant_status
// endpoint that returns plaintext (a raw PAYUNi-style query string) encrypted
// the same way the real gateway does, so GetSellerStatus exercises its real
// decrypt+parse path end to end.
func newSearchStatusTestServer(t *testing.T, plaintext string) *httptest.Server {
	t.Helper()
	encrypted, err := EncryptString(plaintext, testHashKey, testHashIV)
	if err != nil {
		t.Fatalf("EncryptString fixture setup failed: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/search_merchant_status" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		resp := SearchSellerApiResponse{
			AgentID:     "test-agent",
			MerID:       "HAWK123",
			Version:     "1.0",
			EncryptInfo: encrypted,
			HashInfo:    "irrelevant-for-response",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func newTestClient(server *httptest.Server) *Client {
	return &Client{
		AgentID:     "test-agent",
		HashKey:     testHashKey,
		HashIV:      testHashIV,
		HTTPClient:  server.Client(),
		testBaseURL: server.URL,
	}
}

func TestUpdateMerchantRequest_URLEncoding(t *testing.T) {
	req := &UpdateMerchantRequest{
		MerID:         "HAWK999",
		ContactName:   "王小明",
		ContactMobile: "0912345678",
	}
	encrypted, err := Encrypt(req, "0123456789abcdef0123456789abcdef", "0123456789abcdef")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted == "" {
		t.Fatal("Encrypt returned empty string for a non-empty request")
	}
	decrypted, err := Decrypt(encrypted, "0123456789abcdef0123456789abcdef", "0123456789abcdef")
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !strings.Contains(decrypted, "MerID=HAWK999") || !strings.Contains(decrypted, "ContactMobile=0912345678") {
		t.Fatalf("decrypted plaintext missing expected fields: %s", decrypted)
	}
	// Payment/Fee/Open/Close fields (issue #2806 Phase 2 deny-by-default
	// allowlist) must never be encodable through this type at all — this
	// test compiles only because UpdateMerchantRequest has no such fields,
	// so there is nothing to assert their absence against beyond "the type
	// doesn't have them" (verified by this file compiling).
}

// TestGetSellerStatus_DecodesKnownMerchantMemberChannelCodes covers issue
// #2806 AC "search_merchant_status contract tests 覆蓋 merchant/member/channel
// code" — a normal SUCCESS response with known MerStatus/MemberStatus and all
// four channel statuses populated decodes into every SellerStatusResponse
// field PAYUni's spec (docs #9/138) defines.
func TestGetSellerStatus_DecodesKnownMerchantMemberChannelCodes(t *testing.T) {
	plaintext := (url.Values{
		"Status":       {"SUCCESS"},
		"Message":      {"OK"},
		"MerID":        {"HAWK123"},
		"MerStatus":    {"1"}, // active, per PayuniMerchantVerificationStatus CodeMeta
		"Reason":       {""},
		"MemberStatus": {"1"},
		"CREDIT":       {"1"},
		"ApplePay":     {"1"},
		"GooglePay":    {"0"},
		"ATM":          {"1"},
	}).Encode()

	server := newSearchStatusTestServer(t, plaintext)
	defer server.Close()

	resp, err := newTestClient(server).GetSellerStatus(t.Context(), "HAWK123")
	if err != nil {
		t.Fatalf("GetSellerStatus failed: %v", err)
	}
	if resp.Status != "SUCCESS" || resp.MerID != "HAWK123" {
		t.Fatalf("envelope fields wrong: %+v", resp)
	}
	if resp.MerStatus != "1" || resp.MemberStatus != "1" {
		t.Fatalf("merchant/member status not passed through: %+v", resp)
	}
	if resp.CreditStatus != "1" || resp.ApplePayStatus != "1" || resp.GooglePayStatus != "0" || resp.ATMStatus != "1" {
		t.Fatalf("channel statuses not passed through: %+v", resp)
	}
}

// TestGetSellerStatus_ReasonEmptyForMemberStatusPendingInfo covers the
// documented real scenario (issue #2806 已確認現況：MemberStatus=5 &&
// Reason="") — Reason absent from PAYUni's response must decode as "", not
// error or panic, so the caller can distinguish "no reason given" from
// "provider explained the block".
func TestGetSellerStatus_ReasonEmptyForMemberStatusPendingInfo(t *testing.T) {
	plaintext := (url.Values{
		"Status":       {"SUCCESS"},
		"MerID":        {"HAWK123"},
		"MerStatus":    {"2"},
		"MemberStatus": {"5"},
		// Reason intentionally omitted — PAYUni does not always send it.
	}).Encode()

	server := newSearchStatusTestServer(t, plaintext)
	defer server.Close()

	resp, err := newTestClient(server).GetSellerStatus(t.Context(), "HAWK123")
	if err != nil {
		t.Fatalf("GetSellerStatus failed: %v", err)
	}
	if resp.MemberStatus != "5" {
		t.Fatalf("MemberStatus = %q, want 5", resp.MemberStatus)
	}
	if resp.Reason != "" {
		t.Fatalf("Reason = %q, want empty string not an error/placeholder", resp.Reason)
	}
}

// TestGetSellerStatus_UnknownMerStatusCodePassesThroughWithoutError covers
// the issue's own callout that PAYUni may send MerStatus values outside the
// documented enum (observed: "8"). The client must not validate against a
// known-code allowlist — that decision belongs to the caller/mapper — so an
// unrecognized code must decode successfully and pass through verbatim.
func TestGetSellerStatus_UnknownMerStatusCodePassesThroughWithoutError(t *testing.T) {
	plaintext := (url.Values{
		"Status":       {"SUCCESS"},
		"MerID":        {"HAWK123"},
		"MerStatus":    {"8"}, // undocumented code PAYUni is known to send
		"MemberStatus": {"99"},
	}).Encode()

	server := newSearchStatusTestServer(t, plaintext)
	defer server.Close()

	resp, err := newTestClient(server).GetSellerStatus(t.Context(), "HAWK123")
	if err != nil {
		t.Fatalf("GetSellerStatus must not error on an unrecognized code, got: %v", err)
	}
	if resp.MerStatus != "8" {
		t.Fatalf("MerStatus = %q, want raw passthrough of unrecognized code 8", resp.MerStatus)
	}
	if resp.MemberStatus != "99" {
		t.Fatalf("MemberStatus = %q, want raw passthrough of unrecognized code 99", resp.MemberStatus)
	}
}

// TestGetSellerStatus_RawPreservesFieldsNotDecodedIntoStruct covers Raw's
// documented purpose (client.go SellerStatusResponse doc comment): fields
// PAYUni sends that the struct doesn't decode into named fields (e.g.
// UnionPay, installment plans) must still survive in Raw for jsonb
// persistence, not be silently dropped.
func TestGetSellerStatus_RawPreservesFieldsNotDecodedIntoStruct(t *testing.T) {
	plaintext := (url.Values{
		"Status":    {"SUCCESS"},
		"MerID":     {"HAWK123"},
		"MerStatus": {"1"},
		"UnionPay":  {"1"},
		"Inst3":     {"1"},
	}).Encode()

	server := newSearchStatusTestServer(t, plaintext)
	defer server.Close()

	resp, err := newTestClient(server).GetSellerStatus(t.Context(), "HAWK123")
	if err != nil {
		t.Fatalf("GetSellerStatus failed: %v", err)
	}

	var raw map[string]string
	if err := json.Unmarshal(resp.Raw, &raw); err != nil {
		t.Fatalf("Raw is not valid JSON: %v", err)
	}
	if raw["UnionPay"] != "1" || raw["Inst3"] != "1" {
		t.Fatalf("Raw dropped fields not decoded into the struct: %+v", raw)
	}
	// And still present for struct-decoded fields, since Raw preserves everything.
	if raw["MerStatus"] != "1" {
		t.Fatalf("Raw dropped a struct-decoded field too: %+v", raw)
	}
}
