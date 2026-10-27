package middleware

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// withTestTicketSecret 在一個測試內換掉 ticket 的金鑰，結束時還原，不讓金鑰漏到別的測試。
func withTestTicketSecret(t *testing.T, secret string) {
	t.Helper()
	orig := wsTicketSecret
	wsTicketSecret = []byte(secret)
	t.Cleanup(func() { wsTicketSecret = orig })
}

func TestSignVerifyChatWSTicket_RoundTrip(t *testing.T) {
	withTestTicketSecret(t, "round-trip-secret")

	tok, err := SignChatWSTicket(12345, 3, 2, 30*time.Second)
	if err != nil {
		t.Fatalf("SignChatWSTicket: %v", err)
	}
	id, ok := verifyChatWSTicket(tok)
	if !ok {
		t.Fatal("verifyChatWSTicket: ok=false, want true")
	}
	if id.AccountID != 12345 || id.AccountType != 3 || id.LoginRole != 2 {
		t.Fatalf("identity = %+v, want {12345, 3, 2}", id)
	}
}

func TestVerifyChatWSTicket_RejectsExpired(t *testing.T) {
	withTestTicketSecret(t, "expiry-secret")

	tok, err := SignChatWSTicket(1, 1, 1, -time.Second) // already expired
	if err != nil {
		t.Fatalf("SignChatWSTicket: %v", err)
	}
	if _, ok := verifyChatWSTicket(tok); ok {
		t.Fatal("expired ticket verified; want rejected")
	}
}

func TestVerifyChatWSTicket_RejectsTamperedSignature(t *testing.T) {
	withTestTicketSecret(t, "tamper-sig-secret")

	tok, _ := SignChatWSTicket(1, 1, 1, 30*time.Second)
	body, _, _ := strings.Cut(tok, ".")
	forged := body + "." + base64.RawURLEncoding.EncodeToString([]byte("not-the-real-mac"))
	if _, ok := verifyChatWSTicket(forged); ok {
		t.Fatal("tampered signature verified; want rejected")
	}
}

func TestVerifyChatWSTicket_RejectsTamperedPayload(t *testing.T) {
	withTestTicketSecret(t, "tamper-payload-secret")

	tok, _ := SignChatWSTicket(1, 1, 1, 30*time.Second)
	_, sig, _ := strings.Cut(tok, ".")
	// Swap in a payload escalating to a different account, keep the old signature.
	forgedBody := base64.RawURLEncoding.EncodeToString([]byte(`{"a":999,"t":3,"r":2,"e":9999999999}`))
	if _, ok := verifyChatWSTicket(forgedBody + "." + sig); ok {
		t.Fatal("payload tampering did not invalidate the signature; want rejected")
	}
}

func TestVerifyChatWSTicket_RejectsWrongSecret(t *testing.T) {
	withTestTicketSecret(t, "secret-A")
	tok, _ := SignChatWSTicket(1, 1, 1, 30*time.Second)

	wsTicketSecret = []byte("secret-B") // restored by withTestTicketSecret's cleanup
	if _, ok := verifyChatWSTicket(tok); ok {
		t.Fatal("ticket signed under a different key verified; want rejected")
	}
}

func TestVerifyChatWSTicket_RejectsMalformed(t *testing.T) {
	withTestTicketSecret(t, "malformed-secret")
	for _, tok := range []string{"", "noseparator", "a.b.c", "...", "@@@.@@@", "."} {
		if _, ok := verifyChatWSTicket(tok); ok {
			t.Fatalf("malformed token %q verified; want rejected", tok)
		}
	}
}
