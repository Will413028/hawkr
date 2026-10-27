package payuni

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strings"
	"testing"
)

const (
	testKey = "test-only-do-not-use-in-prod!!!!" // 32 bytes
	testIV  = "test-iv-only!!!!"                 // 16 bytes
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plainText := "ContactMobile=0912345678&ContactName=Test&Email=test%40example.com&MerID=HAWK0000000001&Name=Test"

	encrypted, err := EncryptString(plainText, testKey, testIV)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted == "" {
		t.Fatal("Encrypt returned empty string")
	}

	decrypted, err := Decrypt(encrypted, testKey, testIV)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plainText {
		t.Errorf("round-trip mismatch:\n  want: %s\n  got:  %s", plainText, decrypted)
	}
}

func TestEncryptDeterministic(t *testing.T) {
	// AES-GCM with same key/iv/plaintext should produce same ciphertext
	plainText := "MerID=HAWK0000000001&Name=Test"

	enc1, err := EncryptString(plainText, testKey, testIV)
	if err != nil {
		t.Fatalf("Encrypt 1 failed: %v", err)
	}

	enc2, err := EncryptString(plainText, testKey, testIV)
	if err != nil {
		t.Fatalf("Encrypt 2 failed: %v", err)
	}

	if enc1 != enc2 {
		t.Errorf("encryption not deterministic:\n  enc1: %s\n  enc2: %s", enc1[:40], enc2[:40])
	}
}

func TestGenerateHashConsistency(t *testing.T) {
	encryptInfo := "abc123def456"

	got := GenerateHash(encryptInfo, testKey, testIV)

	// Manual calculation
	raw := testKey + encryptInfo + testIV
	hash := sha256.Sum256([]byte(raw))
	want := strings.ToUpper(hex.EncodeToString(hash[:]))

	if got != want {
		t.Errorf("hash mismatch:\n  want: %s\n  got:  %s", want, got)
	}
}

func TestEncryptHashVerification(t *testing.T) {
	// Full flow: encrypt → generate hash → verify hash matches re-calculation
	plainText := "ContactMobile=0912345678&Email=test%40example.com&MerID=HAWK0000000001&Name=Test"

	encryptInfo, err := EncryptString(plainText, testKey, testIV)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	hashInfo := GenerateHash(encryptInfo, testKey, testIV)

	// Re-calculate hash independently
	raw := testKey + encryptInfo + testIV
	hash := sha256.Sum256([]byte(raw))
	expectedHash := strings.ToUpper(hex.EncodeToString(hash[:]))

	if hashInfo != expectedHash {
		t.Errorf("hash verification failed:\n  expected: %s\n  got:      %s", expectedHash, hashInfo)
	}

	// Verify hash is uppercase hex
	if hashInfo != strings.ToUpper(hashInfo) {
		t.Error("hash is not uppercase")
	}

	// Verify hash length (SHA256 = 64 hex chars)
	if len(hashInfo) != 64 {
		t.Errorf("hash length: want 64, got %d", len(hashInfo))
	}
}

func TestEncryptMatchesPHPSDK(t *testing.T) {
	// PHP 官方範例 reference test vector（來自 Java CryptoTest.java）
	phpKey := "12345678901234567890123456789012" // 32 bytes
	phpIV := "1234567890123456"                  // 16 bytes
	phpPlainText := "MerID=AAA&MerTradeNo=BBB"
	phpExpected := "47396636346f66735853533167396942344f587a3775696b34752b596e70452b3a3a3a4373354a5a5143306b7153467531354c6e6f554a69773d3d"

	goEncrypted, err := EncryptString(phpPlainText, phpKey, phpIV)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if goEncrypted != phpExpected {
		t.Errorf("Go output does not match PHP SDK:\n  PHP:  %s\n  Go:   %s", phpExpected, goEncrypted)

		// 額外 debug: decode 兩者比較
		phpDecoded, _ := hex.DecodeString(phpExpected)
		goDecoded, _ := hex.DecodeString(goEncrypted)
		t.Logf("PHP decoded: %s", string(phpDecoded))
		t.Logf("Go  decoded: %s", string(goDecoded))
	}

	// 也驗證 hash 一致
	goHash := GenerateHash(goEncrypted, phpKey, phpIV)
	phpHash := GenerateHash(phpExpected, phpKey, phpIV)
	t.Logf("Go  EncryptInfo hash: %s", goHash)
	t.Logf("PHP EncryptInfo hash: %s", phpHash)
	if goEncrypted == phpExpected && goHash != phpHash {
		t.Error("hash mismatch despite same EncryptInfo (impossible)")
	}
}

func TestEncryptOutputFormat(t *testing.T) {
	plainText := "MerID=HAWK0000000001"

	encrypted, err := EncryptString(plainText, testKey, testIV)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Output should be hex-encoded
	_, err = hex.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}

	// Decoded hex should contain ":::" separator
	decoded, _ := hex.DecodeString(encrypted)
	if !strings.Contains(string(decoded), ":::") {
		t.Error("decoded output missing ':::' separator")
	}
}

func TestEncryptWithStructData(t *testing.T) {
	req := &BindSellerRequest{
		Name:          "測試商家",
		UBN:           "A123456789",
		Email:         "test@example.com",
		MerID:         "HAWK0000000001",
		MerName:       "測試店鋪",
		MerEngName:    "Test Store",
		MerEmail:      "store@example.com",
		MerURL:        "https://example.com/store/HAWK0000000001",
		MerAddr:       "台北市中正區重慶南路一段1號",
		MerAddrPost:   "100",
		MerDesc:       "Hawkr 生鮮食品",
		ContactName:   "測試人員",
		ContactMobile: "0912345678",
		MerType:       2,
		MerMCC:        3,
		GoodsRate:     "100,0,0,0",
		Payment:       "CREDIT",
		DeliverTime:   7,
		EscrowType:    1,
		MerUnitPrice:  1000,
		SellType:      1,
	}

	encrypted, err := Encrypt(req, testKey, testIV)
	if err != nil {
		t.Fatalf("Encrypt struct failed: %v", err)
	}

	// Verify we can decrypt it back
	decrypted, err := Decrypt(encrypted, testKey, testIV)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	// Decrypted should be URL-encoded query string containing our values
	if !strings.Contains(decrypted, "MerID=HAWK0000000001") {
		t.Errorf("decrypted missing MerID, got: %s", decrypted[:100])
	}
	if !strings.Contains(decrypted, "A123456789") {
		t.Errorf("decrypted missing UBN, got: %s", decrypted[:100])
	}
}

func TestEncrypt_DoesNotLogPlaintext(t *testing.T) {
	var buf bytes.Buffer
	origOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(origOutput)

	type req struct {
		ContactMobile string `url:"ContactMobile"`
		MerEmail      string `url:"MerEmail"`
	}
	_, err := Encrypt(req{ContactMobile: "0912345678", MerEmail: "leak-marker@example.com"}, "0123456789abcdef0123456789abcdef", "0123456789abcdef")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	logged := buf.String()
	if strings.Contains(logged, "0912345678") || strings.Contains(logged, "leak-marker@example.com") {
		t.Fatalf("Encrypt logged plaintext PII: %q", logged)
	}
}
