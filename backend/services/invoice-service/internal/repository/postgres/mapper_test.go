package postgres

import (
	"crypto/rand"
	"encoding/hex"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

func randomSecret(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "AK" + hex.EncodeToString(b)
}

func newCipher(t *testing.T, secret string) *crypto.Cipher {
	t.Helper()
	c, err := crypto.New(secret)
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return c
}


func TestEncryptCredential_EmptyPlaintextIsHardError(t *testing.T) {
	enc := newCipher(t, "unit-test-master-key")

	got, err := encryptCredential(enc, "")
	if err == nil {
		t.Fatalf("encryptCredential(\"\") 必須硬錯；實際回 (%q, nil)", got)
	}
	if got != "" {
		t.Fatalf("硬錯時不得回任何密文，實際 %q", got)
	}

	ring := &KeyRing{Current: enc, CurrentVersion: 1, LookupHMACKey: "lookup"}
	if _, err := ring.deriveCredential("12345678", "   \t\n "); err == nil {
		t.Fatal("deriveCredential 收到全空白 app_key 時必須硬錯")
	}
}


func TestDecryptCredential_WrongKeyIsHardErrorAndLeaksNothing(t *testing.T) {
	plaintext := randomSecret(t)
	right := newCipher(t, "master-key-A")
	wrong := newCipher(t, "master-key-B")

	ciphertext, err := encryptCredential(right, plaintext)
	if err != nil {
		t.Fatalf("encryptCredential: %v", err)
	}

	got, err := decryptCredential(wrong, ciphertext)
	if err == nil {
		t.Fatal("用錯的 key 解密必須硬錯，不得回空字串或垃圾明文")
	}
	if got != "" {
		t.Fatalf("硬錯時不得回任何明文，實際 %q", got)
	}

	msg := err.Error()
	if strings.Contains(msg, plaintext) {
		t.Fatalf("錯誤訊息洩漏了明文 AppKey：%q", msg)
	}
	if strings.Contains(msg, ciphertext) {
		t.Fatalf("錯誤訊息洩漏了密文：%q", msg)
	}
	for i := 0; i+16 <= len(ciphertext); i += 8 {
		if strings.Contains(msg, ciphertext[i:i+16]) {
			t.Fatalf("錯誤訊息含密文片段 %q：%q", ciphertext[i:i+16], msg)
		}
	}
	for i := 0; i+8 <= len(plaintext); i += 4 {
		if strings.Contains(msg, plaintext[i:i+8]) {
			t.Fatalf("錯誤訊息含明文片段 %q：%q", plaintext[i:i+8], msg)
		}
	}
}

// TestRepositoryPackageDoesNotLog 是上一支測試的結構面補充。
//
// 「錯誤訊息不含明文」擋得住錯誤這條路，擋不住「有人在 repository 裡加一行
// logger.Info("appKey", ...)」。本 package 從頭到尾不 log，這條測試把那件事釘住：
// 一旦有人引入 logger，就必須先來改這支測試——而那正是需要有人思考的時刻。
func TestRepositoryPackageDoesNotLog(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	banned := []string{"go.uber.org/zap", "log", "log/slog"}
	parsed := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		parsed++
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, b := range banned {
				if path == b {
					t.Errorf("%s 引入了 %s：AppKey 明文所在的 package 不得有 logger", name, path)
				}
			}
		}
	}
	if parsed < 5 {
		t.Fatalf("只解析到 %d 個檔，測試本身沒有鑑別力", parsed)
	}
}


func TestCredentialFingerprint_BindsTaxIDAndAppKey(t *testing.T) {
	const secret = "lookup-hmac-key"
	appKey := randomSecret(t)
	other := randomSecret(t)

	same := credentialFingerprint(secret, "12345678", appKey)
	if again := credentialFingerprint(secret, "12345678", appKey); same != again {
		t.Fatal("同統編＋同 AppKey 必須得到相同 fingerprint（GCM 密文每次不同，比對只能靠它）")
	}

	if diff := credentialFingerprint(secret, "87654321", appKey); diff == same {
		t.Fatal("不同統編＋同 AppKey 必須得到不同 fingerprint（fingerprint 沒有把統編算進去）")
	}

	if diff := credentialFingerprint(secret, "12345678", other); diff == same {
		t.Fatal("同統編＋不同 AppKey 必須得到不同 fingerprint")
	}

	if diff := credentialFingerprint("another-lookup-key", "12345678", appKey); diff == same {
		t.Fatal("換 lookup key 之後 fingerprint 不該相同")
	}

	if credentialFingerprint(secret, "1234", "5678"+appKey) == credentialFingerprint(secret, "12345678", appKey) {
		t.Fatal("fingerprint 的輸入沒有分隔符，統編與 app_key 的邊界可被構造出碰撞")
	}
}

func TestNormalizeAppKeyAndLast4(t *testing.T) {
	if got := normalizeAppKey("  abcd1234  "); got != "abcd1234" {
		t.Fatalf("normalizeAppKey 只 trim 前後空白，得到 %q", got)
	}
	if got := normalizeAppKey("AbCd"); got != "AbCd" {
		t.Fatalf("normalizeAppKey 不得改大小寫，得到 %q", got)
	}
	if got := credentialLast4("ABCDEFGH1234"); got != "1234" {
		t.Fatalf("credentialLast4 = %q, want 1234", got)
	}
	if got := credentialLast4("ab"); got != "ab" {
		t.Fatalf("短於四碼時原樣回傳，得到 %q", got)
	}
}

func TestDeriveCredential_AllThreeDerivativesComeFromTheSameNormalizedPlaintext(t *testing.T) {
	enc := newCipher(t, "master-key-A")
	ring := &KeyRing{Current: enc, CurrentVersion: 7, LookupHMACKey: "lookup"}
	appKey := randomSecret(t)

	mat, err := ring.deriveCredential("12345678", "  "+appKey+"  ")
	if err != nil {
		t.Fatalf("deriveCredential: %v", err)
	}
	if mat.keyVersion != 7 {
		t.Fatalf("寫入端一律用 current 版本，得到 %d", mat.keyVersion)
	}
	plain, err := decryptCredential(enc, mat.ciphertext)
	if err != nil {
		t.Fatalf("decryptCredential: %v", err)
	}
	if plain != appKey {
		t.Fatalf("密文存的不是正規化後的明文：%q", plain)
	}
	if mat.fingerprint != credentialFingerprint("lookup", "12345678", appKey) {
		t.Fatal("fingerprint 不是由正規化後的明文算出來的")
	}
	if mat.last4 != credentialLast4(appKey) {
		t.Fatal("last4 不是由正規化後的明文取的")
	}
}


func TestKeyRing_CipherForSelectsByVersionAndNeverFallsBack(t *testing.T) {
	cur := newCipher(t, "master-key-new")
	prev := newCipher(t, "master-key-old")
	ring := &KeyRing{Current: cur, Prev: prev, CurrentVersion: 2, PrevVersion: 1, LookupHMACKey: "lookup"}

	oldPlain := randomSecret(t)
	oldCipherText, err := encryptCredential(prev, oldPlain)
	if err != nil {
		t.Fatalf("encrypt with prev: %v", err)
	}

	got, err := ring.openCredential(gen.InvoiceProviderAccount{
		CredentialKeyVersion: 1,
		CredentialCiphertext: oldCipherText,
	})
	if err != nil {
		t.Fatalf("prev-version row 必須解得開：%v", err)
	}
	if got != oldPlain {
		t.Fatal("prev-version row 解出來的明文不對")
	}

	if _, err := ring.openCredential(gen.InvoiceProviderAccount{
		CredentialKeyVersion: 9,
		CredentialCiphertext: oldCipherText,
	}); err == nil {
		t.Fatal("未知的 credential_key_version 必須硬錯")
	}

	retired := &KeyRing{Current: cur, CurrentVersion: 2, PrevVersion: 1, LookupHMACKey: "lookup"}
	if _, err := retired.openCredential(gen.InvoiceProviderAccount{
		CredentialKeyVersion: 1,
		CredentialCiphertext: oldCipherText,
	}); err == nil {
		t.Fatal("指向已移除 key 版本的殘列必須硬錯")
	}
}


func TestProviderAccountProjection_CarriesNoCredentialMaterial(t *testing.T) {
	row := gen.InvoiceProviderAccount{
		ID:                    1,
		CredentialCiphertext:  "SHOULD-NEVER-APPEAR",
		CredentialFingerprint: "FINGERPRINT-SHOULD-NEVER-APPEAR",
		CredentialLast4:       "1234",
		PendingCiphertext:     sp("PENDING-SHOULD-NEVER-APPEAR"),
		PendingFingerprint:    sp("PENDING-FP-SHOULD-NEVER-APPEAR"),
	}
	got := toProviderAccount(row)

	if !got.HasPendingCredential {
		t.Fatal("pending_ciphertext 非空時 HasPendingCredential 必須為 true")
	}

	rt := reflect.TypeOf(got)
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		if strings.Contains(name, "Ciphertext") || strings.Contains(name, "Fingerprint") {
			t.Errorf("domain.ProviderAccount 不得有 %s 欄位（憑證材料不出 repository 層）", name)
		}
	}

	dump := dumpStrings(reflect.ValueOf(got))
	for _, sentinel := range []string{"SHOULD-NEVER-APPEAR", "FINGERPRINT-SHOULD-NEVER-APPEAR", "PENDING-SHOULD-NEVER-APPEAR", "PENDING-FP-SHOULD-NEVER-APPEAR"} {
		if strings.Contains(dump, sentinel) {
			t.Errorf("投影後仍帶出 %q", sentinel)
		}
	}
}

func dumpStrings(v reflect.Value) string {
	var sb strings.Builder
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.String:
			sb.WriteString(v.String())
			sb.WriteByte('\n')
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i))
			}
		}
	}
	walk(v)
	return sb.String()
}


func TestToDocumentLine_UnknownTaxCategoryIsHardError(t *testing.T) {
	for _, tc := range []int16{1, 2, 3} {
		if _, err := toDocumentLine(gen.InvoiceDocumentLine{ID: 1, TaxCategory: tc}); err != nil {
			t.Fatalf("合法課稅別 %d 不該報錯：%v", tc, err)
		}
	}
	for _, tc := range []int16{0, 4, 9, -1} {
		got, err := toDocumentLine(gen.InvoiceDocumentLine{ID: 1, TaxCategory: tc})
		if err == nil {
			t.Fatalf("課稅別 %d 必須硬錯，實際回 %+v", tc, got)
		}
		if got.TaxCategory != 0 {
			t.Fatalf("硬錯時不得回半成品（TaxCategory=%d）", got.TaxCategory)
		}
		if got.TaxCategory == int16(domain.TaxCategoryTaxable) {
			t.Fatal("轉換失敗被 fallback 成應稅：一張免稅發票會被靜默開成應稅")
		}
	}
}

func TestToAdjustmentLine_UnknownTaxCategoryIsHardError(t *testing.T) {
	if _, err := toAdjustmentLine(gen.InvoiceAdjustmentLine{ID: 1, TaxCategory: 3}); err != nil {
		t.Fatalf("免稅（3）不該報錯：%v", err)
	}
	if _, err := toAdjustmentLine(gen.InvoiceAdjustmentLine{ID: 1, TaxCategory: 9}); err == nil {
		t.Fatal("折讓明細的課稅別值域只有 1/2/3，9 必須硬錯")
	}
}

func TestToDocument_HeaderTaxTypeAllowsMixedButNotGarbage(t *testing.T) {
	for _, tt := range []int16{1, 2, 3, 9} {
		if _, err := toDocument(gen.InvoiceDocument{ID: 1, TaxType: tt}); err != nil {
			t.Fatalf("表頭課稅別 %d 應合法（9 ＝ 混合）：%v", tt, err)
		}
	}
	for _, tt := range []int16{0, 4, 10} {
		if _, err := toDocument(gen.InvoiceDocument{ID: 1, TaxType: tt}); err == nil {
			t.Fatalf("表頭課稅別 %d 必須硬錯", tt)
		}
	}
}
