// pkg/urlsafety/validate_test.go
package urlsafety

import "testing"

func TestValidatePublicEvidenceURL_AllowsPlainHTTPS(t *testing.T) {
	if err := ValidatePublicEvidenceURL("https://example.com/product/123"); err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
}

func TestValidatePublicEvidenceURL_RejectsNonHTTPS(t *testing.T) {
	cases := []string{
		"http://example.com/",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"ftp://example.com/",
	}
	for _, raw := range cases {
		if err := ValidatePublicEvidenceURL(raw); err == nil {
			t.Fatalf("expected rejection for %q, got nil error", raw)
		}
	}
}

func TestValidatePublicEvidenceURL_RejectsUserinfo(t *testing.T) {
	if err := ValidatePublicEvidenceURL("https://user:pass@example.com/"); err == nil {
		t.Fatal("expected rejection for URL with userinfo, got nil error")
	}
}

func TestValidatePublicEvidenceURL_RejectsEmptyHost(t *testing.T) {
	if err := ValidatePublicEvidenceURL("https:///no-host"); err == nil {
		t.Fatal("expected rejection for empty host, got nil error")
	}
}

func TestValidatePublicEvidenceURL_RejectsLoopbackAndPrivateLiteralIPs(t *testing.T) {
	cases := []string{
		"https://127.0.0.1/",
		"https://[::1]/",
		"https://10.0.0.5/",
		"https://172.16.0.1/",
		"https://192.168.1.1/",
		"https://169.254.169.254/", // link-local / cloud metadata endpoint
		"https://0.0.0.0/",
	}
	for _, raw := range cases {
		if err := ValidatePublicEvidenceURL(raw); err == nil {
			t.Fatalf("expected rejection for %q, got nil error", raw)
		}
	}
}

func TestValidatePublicEvidenceURL_AllowsPublicLiteralIP(t *testing.T) {
	if err := ValidatePublicEvidenceURL("https://8.8.8.8/"); err != nil {
		t.Fatalf("expected valid public IP literal to pass, got error: %v", err)
	}
}

func TestValidatePublicEvidenceURL_RejectsMalformed(t *testing.T) {
	if err := ValidatePublicEvidenceURL("not a url at all %%%"); err == nil {
		t.Fatal("expected rejection for malformed input, got nil error")
	}
}
