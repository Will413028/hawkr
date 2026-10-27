package seal

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"strings"
	"testing"
)

func key(b byte) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32)) }

func TestSealAndOpenAcrossKeyRotation(t *testing.T) {
	old, err := Parse("k1:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := old.Seal([]byte("rt-1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("rt-1")) {
		t.Fatal("密文裡看得到明文")
	}
	rotated, err := Parse("k2:" + key(2) + ", k1:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := rotated.Open(sealed, nil); err != nil || string(got) != "rt-1" {
		t.Fatalf("換金鑰後解舊的密文 = %q, %v", got, err)
	}
	fresh, _ := rotated.Seal([]byte("rt-2"), nil)
	if _, err := old.Open(fresh, nil); err == nil {
		t.Fatal("新的密文應該用新的金鑰")
	}
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	if _, err := old.Open(tampered, nil); err == nil {
		t.Fatal("被改過的密文應該解不開")
	}
}

// AAD 防止密文在列與欄之間搬動：同一把金鑰加密的密文，換一個脈絡就解不開。
func TestOpenRejectsCiphertextMovedToAnotherContext(t *testing.T) {
	k, err := Parse("k1:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := k.Seal([]byte("A123456789"), []byte("app-1|id_card_no"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := k.Open(sealed, []byte("app-1|id_card_no")); err != nil || string(got) != "A123456789" {
		t.Fatalf("同一個脈絡解密 = %q, %v", got, err)
	}
	for _, aad := range [][]byte{[]byte("app-2|id_card_no"), []byte("app-1|bank_account"), nil} {
		if _, err := k.Open(sealed, aad); err == nil {
			t.Errorf("AAD %q 與加密時不同，應該解不開", aad)
		}
	}
}

func TestParseRejectsBadKeys(t *testing.T) {
	for _, s := range []string{"", "k1", "k1:short", "k1:" + key(1) + ",k1:" + key(2), ":" + key(1)} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) 應該失敗", strings.TrimSpace(s))
		}
	}
}

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
	written := old.Sum([]byte("kyc:id:v1|A123456789"))
	if written.KeyID != "h1" || len(written.Sum) != 32 {
		t.Fatalf("hash = %+v", written)
	}
	rotated, err := ParseMAC("h2:" + key(2) + ",h1:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	if got := rotated.Sum([]byte("kyc:id:v1|A123456789")); got.KeyID != "h2" || bytes.Equal(got.Sum, written.Sum) {
		t.Fatal("輪替後新的 hash 要用第一把金鑰")
	}
	var found bool
	for _, h := range rotated.SumAll([]byte("kyc:id:v1|A123456789")) {
		if h.KeyID == "h1" && bytes.Equal(h.Sum, written.Sum) {
			found = true
		}
	}
	if !found {
		t.Fatal("SumAll 要包含舊金鑰的 hash，輪替前寫入的才查得到")
	}
	if bytes.Equal(old.Sum([]byte("kyc:id:v1|X")).Sum, old.Sum([]byte("kyc:bank:v1|X")).Sum) {
		t.Fatal("不同標籤的輸入不能得到相同的 hash")
	}
}

// 啟動檢查：HMAC 金鑰與加密金鑰相同時失敗（任何一把相同都算）。
func TestDistinctRejectsAMACKeyEqualToAnEncryptionKey(t *testing.T) {
	enc, err := Parse("e2:" + key(2) + ",e1:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	same, err := ParseMAC("h1:" + key(9) + ",h2:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := Distinct(enc, same); err == nil {
		t.Fatal("HMAC 金鑰與加密金鑰相同，應該失敗")
	}
	other, err := ParseMAC("h1:" + key(9))
	if err != nil {
		t.Fatal(err)
	}
	if err := Distinct(enc, other); err != nil {
		t.Fatalf("不同的金鑰不該失敗：%v", err)
	}
}

// identity 原本的 seal 以金鑰 ID 當 AAD、沒有脈絡：那時寫下的密文（格式：ID 長度、ID、nonce、密文）在 aad 為空時
// 必須照樣解得開。
func TestOpenDecryptsCiphertextWrittenWithTheLegacyAAD(t *testing.T) {
	raw := bytes.Repeat([]byte{1}, 32)
	block, err := aes.NewCipher(raw)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := bytes.Repeat([]byte{7}, gcm.NonceSize())
	legacy := append([]byte{2}, "k1"...)
	legacy = append(legacy, nonce...)
	legacy = gcm.Seal(legacy, nonce, []byte("rt-legacy"), []byte("k1"))

	k, err := Parse("k1:" + key(1))
	if err != nil {
		t.Fatal(err)
	}
	for _, aad := range [][]byte{nil, {}} {
		if got, err := k.Open(legacy, aad); err != nil || string(got) != "rt-legacy" {
			t.Fatalf("舊格式的密文 = %q, %v", got, err)
		}
	}
}
