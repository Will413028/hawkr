package seal

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func key(b byte) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32)) }

func TestParseMACRejectsEmptyAndShortKeys(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16))
	for _, s := range []string{"", "h1", "h1:", "h1:" + short, "h1:" + key(1) + ",h1:" + key(2), ":" + key(1), "h1:" + key(1) + ",h2:"} {
		if _, err := ParseMAC(s); err == nil {
			t.Errorf("ParseMAC(%q) 應該失敗", s)
		}
	}
	if _, err := ParseMAC("h1:" + key(1)); err != nil {
		t.Fatal(err)
	}
}

func TestMACKeyRotationAndDomainSeparation(t *testing.T) {
	old, err := ParseMAC("h1:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	written := old.Sum([]byte("kyc:ubn:v1|12345678"))
	if written.KeyID != "h1" || len(written.Sum) != 32 {
		t.Fatalf("hash = %+v", written)
	}
	rotated, err := ParseMAC("h2:" + key(2) + ",h1:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	if got := rotated.Sum([]byte("kyc:ubn:v1|12345678")); got.KeyID != "h2" || bytes.Equal(got.Sum, written.Sum) {
		t.Fatal("輪替後新的 hash 要用第一把金鑰")
	}
	var found bool
	for _, h := range rotated.SumAll([]byte("kyc:ubn:v1|12345678")) {
		if h.KeyID == "h1" && bytes.Equal(h.Sum, written.Sum) {
			found = true
		}
	}
	if !found {
		t.Fatal("SumAll 要包含舊金鑰的 hash，輪替前寫入的才查得到")
	}
	if bytes.Equal(old.Sum([]byte("kyc:ubn:v1|X")).Sum, old.Sum([]byte("provider:bankfp:v1|X")).Sum) {
		t.Fatal("不同標籤的輸入不能得到相同的 hash")
	}
}
